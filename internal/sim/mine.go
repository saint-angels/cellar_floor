package sim

import (
	"fmt"

	"cellarfloor/internal/data"
)

// mineStep runs the mining behavior for entity types with mine_damage > 0.
// Returns (events, true) when the entity spent this tick on mining.
func (w *World) mineStep(e *Entity) ([]Event, bool) {
	s := w.spec(e)
	if s.MineDamage <= 0 {
		return nil, false
	}
	// hardcore mining: a dwarf never picks a face on its own. A MineTarget is
	// assigned only by food-digging (digFoodStep), the sole driver of mining;
	// with no assignment there is nothing to mine.
	if e.MineTarget == nil {
		return nil, false
	}
	if !w.Mineable(w.At(*e.MineTarget)) || !w.Lit(*e.MineTarget) {
		e.MineTarget = nil
		w.markDirty(e.ID)
		return nil, false
	}
	target := *e.MineTarget

	if adjacent(e.Pos, target) {
		w.setTarget(e, 0) // the mined face rides MineTarget instead
		e.Action = "mining"
		w.markDirty(e.ID)
		cells := make([]int, 0, 8)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !w.Mineable(w.At(p)) || !w.Lit(p) {
				continue
			}
			cells = append(cells, p.Y*w.Width+p.X)
		}
		sortInts(cells)
		base := s.MineDamage + w.MineBonus()
		beam := w.BeamBonus()
		ti := target.Y*w.Width + target.X
		var evs []Event
		for _, i := range cells {
			dmg := base
			if i == ti {
				dmg += beam // beam weapons concentrate on the chosen face
			}
			w.MineDamage[i] += dmg
			var tt *data.TerrainType
			if t := w.terrainAt(w.Terrain[i]); t != nil {
				tt = t
			}
			hp := 0
			if tt != nil {
				hp = tt.HitPoints
			}
			if w.MineDamage[i] < hp {
				continue
			}
			p := Point{X: i % w.Width, Y: i / w.Width}
			delete(w.MineDamage, i)
			w.SetTerrain(p, TerrainFloor)
			w.BlocksMined++
			if p == target {
				e.MineTarget = nil
			}
			sc := w.cfg.Sim
			if tt != nil && tt.GoldChance > 0 && w.RandFloat() < tt.GoldChance {
				lo := sc.GoldMin + w.LuckBonus()
				hi := sc.GoldMax + w.LuckBonus()
				amt := lo
				if hi > lo {
					amt += w.RandN(hi - lo + 1)
				}
				if s.CarryCapacity > 0 {
					// bagged miners carry ore home; the gold is paid at the
					// market on deposit, not here at the rock face
					e.Ore += amt
					w.markDirty(e.ID)
					evs = append(evs, Event{
						Tick: w.Tick, Type: "ore", Actor: e.ID, ActorType: e.Type,
						Amount: amt,
						Msg:    fmt.Sprintf("%s struck ore", s.Name),
					})
				} else {
					w.Gold += amt
					w.GoldMined += amt
					e.GoldStrikes = append(e.GoldStrikes, GoldStrike{Tick: w.Tick, Amount: amt})
					w.GoldLast24h(e)
					evs = append(evs, Event{
						Tick: w.Tick, Type: "gold", Actor: e.ID, ActorType: e.Type,
						Msg: fmt.Sprintf("%s struck gold", s.Name),
					})
				}
			} else {
				evs = append(evs, Event{
					Tick: w.Tick, Type: "mined", Actor: e.ID, ActorType: e.Type,
					Msg: fmt.Sprintf("%s mined out a rock", s.Name),
				})
			}
		}
		return evs, true
	}

	// walk toward the face
	next, ok := w.nextStepToward(e.Pos, target)
	if !ok {
		e.MineTarget = nil
		w.markDirty(e.ID)
		return nil, false
	}
	e.Action = "heading to mine"
	e.MoveAcc += w.moveSpeed(e)
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target) {
		e.MoveAcc--
		if !w.walkStep(e, next, target) {
			break // hemmed in, wait
		}
		next, ok = w.nextStepToward(e.Pos, target)
		if !ok {
			break
		}
	}
	return nil, true
}

// nextStepToward BFSes over passable terrain and returns the first step
// of the shortest path from start to any cell adjacent to target. Array
// based: every walking miner and seeker calls it each step.
func (w *World) nextStepToward(start, target Point) (Point, bool) {
	prev := make([]int32, w.Width*w.Height)
	for i := range prev {
		prev[i] = -1
	}
	s0 := int32(start.Y*w.Width + start.X)
	prev[s0] = s0
	queue := make([]int32, 0, 256)
	queue = append(queue, s0)
	goal := int32(-1)
	for qi := 0; qi < len(queue) && goal < 0; qi++ {
		p := queue[qi]
		px, py := int(p)%w.Width, int(p)/w.Width
		for _, nb := range neighbors {
			x, y := px+nb.X, py+nb.Y
			if x < 0 || y < 0 || x >= w.Width || y >= w.Height {
				continue
			}
			i := int32(y*w.Width + x)
			if prev[i] >= 0 {
				continue
			}
			if !w.Passable(w.Terrain[i]) {
				continue
			}
			prev[i] = p
			if adjacent(Point{X: x, Y: y}, target) {
				goal = i
				break
			}
			queue = append(queue, i)
		}
	}
	if goal < 0 {
		return Point{}, false
	}
	p := goal
	for prev[p] != s0 {
		p = prev[p]
	}
	return Point{X: int(p) % w.Width, Y: int(p) / w.Width}, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
