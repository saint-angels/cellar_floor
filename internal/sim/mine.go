package sim

import (
	"fmt"
	"sort"
)

// mineStep runs the mining behavior for entity types with mine_ticks > 0.
// Returns (events, true) when the entity spent this tick on mining.
func (w *World) mineStep(e *Entity) ([]Event, bool) {
	s := w.cfg.Types[e.Type]
	if s.MineTicks <= 0 {
		return nil, false
	}
	if e.MineTarget != nil && (!Mineable(w.At(*e.MineTarget)) || !w.Lit(*e.MineTarget)) {
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
		i := target.Y*w.Width + target.X
		w.MineProgress[i] += 1.0 / float64(s.MineTicks)
		w.markDirty(e.ID)
		if w.MineProgress[i] < 1 {
			return nil, true
		}
		wasGold := w.At(target) == TerrainGold
		delete(w.MineProgress, i)
		w.SetTerrain(target, TerrainFloor)
		e.MineTarget = nil
		var evs []Event
		if wasGold {
			w.Gold++
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

// pickMineTarget BFSes the walkable region around e and returns the best
// unclaimed mineable face: near known gold if any is within gold_sense,
// otherwise simply the nearest face.
func (w *World) pickMineTarget(e *Entity) (Point, bool) {
	s := w.cfg.Types[e.Type]

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
			if Mineable(t) {
				if !w.Lit(q) {
					continue
				}
				if d, seen := faceDist[q]; !seen || dist[p]+1 < d {
					faceDist[q] = dist[p] + 1
				}
				continue
			}
			if !Passable(t) {
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

	// nearest gold within sense radius (buried or exposed)
	var gold *Point
	if s.GoldSense > 0 {
		bestD := s.GoldSense + 1
		for y := maxInt(0, e.Pos.Y-s.GoldSense); y <= minInt(w.Height-1, e.Pos.Y+s.GoldSense); y++ {
			for x := maxInt(0, e.Pos.X-s.GoldSense); x <= minInt(w.Width-1, e.Pos.X+s.GoldSense); x++ {
				p := Point{x, y}
				if w.At(p) != TerrainGold {
					continue
				}
				if d := Dist(e.Pos, p); d < bestD {
					bestD = d
					g := p
					gold = &g
				}
			}
		}
	}

	// deterministic choice: sort faces by cell index
	faces := make([]Point, 0, len(faceDist))
	for f := range faceDist {
		faces = append(faces, f)
	}
	sort.Slice(faces, func(i, j int) bool {
		return faces[i].Y*w.Width+faces[i].X < faces[j].Y*w.Width+faces[j].X
	})
	best := faces[0]
	bestScore := 1 << 30
	for _, f := range faces {
		score := faceDist[f]
		if gold != nil {
			score = Dist(f, *gold)*10000 + faceDist[f]
		}
		if score < bestScore {
			best, bestScore = f, score
		}
	}
	return best, true
}

// nextStepToward BFSes over passable terrain and returns the first step of
// the shortest path from start to any cell adjacent to target.
func (w *World) nextStepToward(start, target Point) (Point, bool) {
	prev := map[Point]Point{start: start}
	queue := []Point{start}
	var goal *Point
	for len(queue) > 0 && goal == nil {
		p := queue[0]
		queue = queue[1:]
		for _, n := range neighbors {
			q := Point{p.X + n.X, p.Y + n.Y}
			if !w.InBounds(q) {
				continue
			}
			if _, seen := prev[q]; seen {
				continue
			}
			if !Passable(w.At(q)) {
				continue
			}
			prev[q] = p
			if adjacent(q, target) {
				g := q
				goal = &g
				break
			}
			queue = append(queue, q)
		}
	}
	if goal == nil {
		return Point{}, false
	}
	p := *goal
	for prev[p] != start {
		p = prev[p]
	}
	return p, true
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
