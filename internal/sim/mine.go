package sim

import (
	"fmt"
	"sort"

	"cellarfloor/internal/data"
)

// mineStep runs the mining behavior for entity types with mine_damage > 0.
// Returns (events, true) when the entity spent this tick on mining.
func (w *World) mineStep(e *Entity) ([]Event, bool) {
	s := w.cfg.Types[e.Type]
	if s.MineDamage <= 0 {
		return nil, false
	}
	if e.MineTarget != nil && (!w.Mineable(w.At(*e.MineTarget)) || !w.Lit(*e.MineTarget)) {
		e.MineTarget = nil
		w.markDirty(e.ID)
	}
	if e.MineTarget == nil {
		face, ok := w.pickMineTarget(e)
		if !ok {
			return nil, false
		}
		e.MineTarget = &face
		w.markDirty(e.ID)
	}
	target := *e.MineTarget

	if adjacent(e.Pos, target) {
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
		dmg := s.MineDamage + w.MineBonus()
		var evs []Event
		for _, i := range cells {
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
				w.Gold += amt
				w.GoldMined += amt
				e.GoldStrikes = append(e.GoldStrikes, GoldStrike{Tick: w.Tick, Amount: amt})
				w.GoldLast24h(e)
				evs = append(evs, Event{
					Tick: w.Tick, Type: "gold", Actor: e.ID, ActorType: e.Type,
					Msg: fmt.Sprintf("%s struck gold", s.Name),
				})
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
	e.MoveAcc += s.Speed
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target) {
		e.MoveAcc--
		if w.FaunaAt(next) != nil {
			break // occupied, wait
		}
		delete(w.occ, e.Pos)
		e.Pos = next
		w.occ[e.Pos] = e.ID
		w.markDirty(e.ID)
		next, ok = w.nextStepToward(e.Pos, target)
		if !ok {
			break
		}
	}
	return nil, true
}

// pickMineTarget BFSes the walkable region around e and returns the nearest
// unclaimed, lit mineable face.
func (w *World) pickMineTarget(e *Entity) (Point, bool) {
	dist := map[Point]int{e.Pos: 0}
	queue := []Point{e.Pos}
	faceDist := map[Point]int{}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, n := range neighbors {
			q := Point{p.X + n.X, p.Y + n.Y}
			if !w.InBounds(q) {
				continue
			}
			t := w.At(q)
			if w.Mineable(t) {
				if !w.Lit(q) {
					continue
				}
				if d, seen := faceDist[q]; !seen || dist[p]+1 < d {
					faceDist[q] = dist[p] + 1
				}
				continue
			}
			if !w.Passable(t) {
				continue
			}
			if _, seen := dist[q]; seen {
				continue
			}
			dist[q] = dist[p] + 1
			queue = append(queue, q)
		}
	}

	// drop faces claimed by other living miners
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID != e.ID && !c.Dead && c.MineTarget != nil {
			delete(faceDist, *c.MineTarget)
		}
	}
	if len(faceDist) == 0 {
		return Point{}, false
	}

	// deterministic choice: sort faces by cell index, take the nearest
	faces := make([]Point, 0, len(faceDist))
	for f := range faceDist {
		faces = append(faces, f)
	}
	sort.Slice(faces, func(i, j int) bool {
		return faces[i].Y*w.Width+faces[i].X < faces[j].Y*w.Width+faces[j].X
	})
	best := faces[0]
	for _, f := range faces {
		if faceDist[f] < faceDist[best] {
			best = f
		}
	}
	return best, true
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
