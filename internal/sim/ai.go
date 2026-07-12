package sim

import (
	"fmt"

	"cellarfloor/internal/data"
)

func typeEatsProduceOf(eater, victim *data.EntityType) bool {
	if eater.ID == victim.ID || eater.Kind != "fauna" {
		return false
	}
	prod := map[string]bool{}
	for _, p := range victim.Produces {
		prod[p.Resource] = true
	}
	for _, r := range eater.Eats {
		if prod[r] {
			return true
		}
	}
	return false
}

var neighbors = []Point{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func adjacent(a, b Point) bool { return Dist(a, b) <= 1 }

func (w *World) aiStep(e *Entity) []Event {
	s := w.cfg.Types[e.Type]

	// 0. darkness: a creature caught in the dark panics back toward light
	if w.darkStep(e) {
		return nil
	}

	// 1. danger (implemented in Task 5)
	if evs, fled := w.fleeStep(e); fled {
		w.setTarget(e, 0)
		return evs
	}

	// 2. food; once a meal starts, keep eating until the stomach is full
	// (or nothing edible remains), not just past the hunger threshold
	hungry := e.Fullness < s.HungerThreshold
	topping := (e.Action == "eating" || e.Action == "seeking food") && e.Fullness < s.StomachSize
	if hungry || topping {
		food := w.findFood(e)
		if food != nil {
			w.setTarget(e, food.ID)
			if adjacent(e.Pos, food.Pos) {
				return w.eatFrom(e, food)
			}
			e.Action = "seeking food"
			w.pathToward(e, food.Pos)
			return nil
		}
		if hungry {
			e.Action = "searching"
			w.setTarget(e, 0)
			w.wander(e)
			return nil
		}
		// stomach not full but no food left; fall through to other work
	}

	// 3. company
	if w.socialStep(e) {
		return nil
	}

	// 4. mining
	if evs, mined := w.mineStep(e); mined {
		return evs
	}

	// 5. shelter
	if w.shelterStep(e) {
		return nil
	}

	// 6. wander
	w.setTarget(e, 0)
	e.Action = "idle"
	if w.RandFloat() < s.WanderChance {
		w.wander(e)
	}
	return nil
}

func (w *World) findFood(e *Entity) *Entity {
	eats := map[string]bool{}
	for _, r := range w.cfg.Types[e.Type].Eats {
		eats[r] = true
	}
	var edibles []*Entity
	var nearest *Entity
	bestD := 1 << 30
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID == e.ID || c.Type == e.Type {
			continue
		}
		edible := false
		for _, p := range c.Produces {
			if eats[p.Resource] && p.Amount >= 0.5 {
				edible = true
				break
			}
		}
		if !edible {
			continue
		}
		edibles = append(edibles, c)
		if d := Dist(e.Pos, c.Pos); d < bestD {
			nearest, bestD = c, d
		}
	}
	if nearest == nil {
		return nil
	}
	// fast path: the straight-line nearest is usually reachable; the BFS
	// probe exits early on success, so this stays cheap
	if adjacent(e.Pos, nearest.Pos) {
		return nearest
	}
	if _, ok := w.nextStepToward(e.Pos, nearest.Pos); ok {
		return nearest
	}
	// nearest is walled off (mold pockets): flood once and take the
	// closest actually reachable meal
	dist := w.reachableDist(e.Pos)
	var best *Entity
	bestC := 1 << 30
	for _, c := range edibles {
		if d := w.reachCost(dist, c.Pos); d >= 0 && d < bestC {
			best, bestC = c, d
		}
	}
	return best
}

func (w *World) eatFrom(e *Entity, food *Entity) []Event {
	s := w.cfg.Types[e.Type]
	// Live fauna prey is killed first (Task 5 covers the hunt event path).
	if !food.Dead && w.cfg.Types[food.Type].Kind == "fauna" {
		return w.huntStrike(e, food)
	}
	eats := map[string]bool{}
	for _, r := range s.Eats {
		eats[r] = true
	}
	for i := range food.Produces {
		p := &food.Produces[i]
		if !eats[p.Resource] || p.Amount <= 0 {
			continue
		}
		bite := s.BiteSize
		if p.Amount < bite {
			bite = p.Amount
		}
		if room := s.StomachSize - e.Fullness; room < bite {
			bite = room
		}
		if bite <= 0 {
			return nil
		}
		p.Amount -= bite
		e.Fullness += bite
		e.Action = "eating"
		w.markDirty(e.ID)
		w.markDirty(food.ID)
		return []Event{{
			Tick: w.Tick, Type: "ate",
			Actor: e.ID, ActorType: e.Type,
			Target: food.ID, TargetType: food.Type,
			Msg: fmt.Sprintf("%s ate %s from %s", s.Name, p.Resource, w.cfg.Types[food.Type].Name),
		}}
	}
	return nil
}

func (w *World) moveToward(e *Entity, target Point) { w.move(e, target, false) }
func (w *World) moveAway(e *Entity, from Point)     { w.move(e, from, true) }

// reachableDist floods passable cells from start and returns BFS
// distances per cell index, -1 for unreachable. Array-based for speed:
// hungry fauna flood every tick.
func (w *World) reachableDist(start Point) []int32 {
	dist := make([]int32, w.Width*w.Height)
	for i := range dist {
		dist[i] = -1
	}
	s0 := int32(start.Y*w.Width + start.X)
	dist[s0] = 0
	queue := make([]int32, 0, 256)
	queue = append(queue, s0)
	for qi := 0; qi < len(queue); qi++ {
		p := queue[qi]
		px, py := int(p)%w.Width, int(p)/w.Width
		for _, n := range neighbors {
			x, y := px+n.X, py+n.Y
			if x < 0 || y < 0 || x >= w.Width || y >= w.Height {
				continue
			}
			i := y*w.Width + x
			if dist[i] >= 0 {
				continue
			}
			if !w.Passable(w.Terrain[i]) {
				continue
			}
			dist[i] = dist[p] + 1
			queue = append(queue, int32(i))
		}
	}
	return dist
}

// reachCost is the BFS cost to stand next to p (or on it), or -1 when
// no adjacent cell is reachable. Works even when p itself is impassable,
// like a mushroom molded under.
func (w *World) reachCost(dist []int32, p Point) int {
	best := int32(-1)
	if w.InBounds(p) {
		best = dist[p.Y*w.Width+p.X]
	}
	for _, n := range neighbors {
		q := Point{p.X + n.X, p.Y + n.Y}
		if !w.InBounds(q) {
			continue
		}
		if d := dist[q.Y*w.Width+q.X]; d >= 0 && (best < 0 || d < best) {
			best = d
		}
	}
	return int(best)
}

// pathToward walks the entity along BFS shortest paths, stopping when
// adjacent to the target. Unlike the greedy move it routes around
// obstacles such as mold pockets.
func (w *World) pathToward(e *Entity, target Point) {
	e.MoveAcc += w.cfg.Types[e.Type].Speed
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target) {
		e.MoveAcc--
		next, ok := w.nextStepToward(e.Pos, target)
		if !ok || w.FaunaAt(next) != nil {
			return
		}
		delete(w.occ, e.Pos)
		e.Pos = next
		w.occ[e.Pos] = e.ID
		w.markDirty(e.ID)
	}
}

func (w *World) move(e *Entity, ref Point, away bool) {
	e.MoveAcc += w.cfg.Types[e.Type].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		best := e.Pos
		bestD := Dist(e.Pos, ref)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !w.Passable(w.At(p)) || w.FaunaAt(p) != nil {
				continue
			}
			d := Dist(p, ref)
			if (!away && d < bestD) || (away && d > bestD) {
				best, bestD = p, d
			}
		}
		if best == e.Pos {
			return
		}
		delete(w.occ, e.Pos)
		e.Pos = best
		w.occ[best] = e.ID
		w.markDirty(e.ID)
	}
}

func (w *World) wander(e *Entity) {
	e.MoveAcc += w.cfg.Types[e.Type].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		n := neighbors[w.RandN(len(neighbors))]
		p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
		if w.InBounds(p) && w.Passable(w.At(p)) && w.FaunaAt(p) == nil {
			delete(w.occ, e.Pos)
			e.Pos = p
			w.occ[p] = e.ID
			w.markDirty(e.ID)
		}
	}
}

// darkStep sends a creature standing in an unlit cell toward the nearest
// living light source. In a world with no light at all it does nothing.
func (w *World) darkStep(e *Entity) bool {
	if w.Lit(e.Pos) {
		return false
	}
	var light *Entity
	bestD := 1 << 30
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.Dead {
			continue
		}
		s, ok := w.cfg.Types[c.Type]
		if !ok || s.LightRadius <= 0 {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			light, bestD = c, d
		}
	}
	if light == nil {
		return false
	}
	e.Action = "fleeing the dark"
	w.pathToward(e, light.Pos)
	return true
}

func (w *World) fleeStep(e *Entity) ([]Event, bool) {
	me := w.cfg.Types[e.Type]
	if me.FearRadius <= 0 {
		return nil, false
	}
	var threat *Entity
	bestD := me.FearRadius + 1
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.Dead || c.ID == e.ID {
			continue
		}
		cs := w.cfg.Types[c.Type]
		if !typeEatsProduceOf(cs, me) {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			threat, bestD = c, d
		}
	}
	if threat == nil {
		return nil, false
	}
	var evs []Event
	if e.Action != "fleeing" {
		evs = append(evs, Event{
			Tick: w.Tick, Type: "fled",
			Actor: e.ID, ActorType: e.Type,
			Target: threat.ID, TargetType: threat.Type,
			Msg: fmt.Sprintf("%s fled from %s", me.Name, w.cfg.Types[threat.Type].Name),
		})
	}
	e.Action = "fleeing"
	w.moveAway(e, threat.Pos)
	return evs, true
}

func (w *World) huntStrike(e *Entity, prey *Entity) []Event {
	s := w.cfg.Types[e.Type]
	ev := w.kill(prey, "killed", fmt.Sprintf("%s was killed by %s", w.cfg.Types[prey.Type].Name, s.Name))
	ev.Target = e.ID
	ev.TargetType = e.Type
	e.Action = "hunting"
	w.markDirty(e.ID)
	hunt := Event{
		Tick: w.Tick, Type: "hunted",
		Actor: e.ID, ActorType: e.Type,
		Target: prey.ID, TargetType: prey.Type,
		Msg: fmt.Sprintf("%s hunted down %s", s.Name, w.cfg.Types[prey.Type].Name),
	}
	return []Event{hunt, ev}
}

func (w *World) shelterStep(e *Entity) bool {
	s := w.cfg.Types[e.Type]
	if len(s.Shelters) == 0 {
		return false
	}
	if e.Home == nil {
		want := map[string]bool{}
		for _, r := range s.Shelters {
			want[r] = true
		}
		var best *Entity
		bestD := 1 << 30
		for _, id := range w.SortedIDs() {
			c := w.Entities[id]
			if c.ID == e.ID || c.Dead {
				continue
			}
			ok := false
			for _, p := range c.Produces {
				if want[p.Resource] {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			if d := Dist(e.Pos, c.Pos); d < bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			return false
		}
		h := best.Pos
		e.Home = &h
		w.markDirty(e.ID)
	}
	if Dist(e.Pos, *e.Home) > s.HomeRange {
		e.Action = "going home"
		w.moveToward(e, *e.Home)
		return true
	}
	return false
}
