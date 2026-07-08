package sim

import "fmt"

var neighbors = []Point{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func adjacent(a, b Point) bool { return Dist(a, b) <= 1 }

func (w *World) aiStep(e *Entity) []Event {
	s := w.cfg.Species[e.Species]

	// 1. danger (implemented in Task 5)
	if evs, fled := w.fleeStep(e); fled {
		return evs
	}

	// 2. food
	if e.Fullness < s.HungerThreshold {
		food := w.findFood(e)
		if food != nil {
			if adjacent(e.Pos, food.Pos) {
				return w.eatFrom(e, food)
			}
			e.Action = "seeking food"
			w.moveToward(e, food.Pos)
			return nil
		}
		e.Action = "searching"
		w.wander(e)
		return nil
	}

	// 3. shelter (implemented in Task 6)
	if w.shelterStep(e) {
		return nil
	}

	// 4. wander
	e.Action = "idle"
	if w.RandFloat() < 0.15 {
		w.wander(e)
	}
	return nil
}

func (w *World) findFood(e *Entity) *Entity {
	eats := map[string]bool{}
	for _, r := range w.cfg.Species[e.Species].Eats {
		eats[r] = true
	}
	var best *Entity
	bestD := 1 << 30
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID == e.ID || c.Species == e.Species {
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
		if d := Dist(e.Pos, c.Pos); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func (w *World) eatFrom(e *Entity, food *Entity) []Event {
	s := w.cfg.Species[e.Species]
	// Live fauna prey is killed first (Task 5 covers the hunt event path).
	if !food.Dead && w.cfg.Species[food.Species].Kind == "fauna" {
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
			Actor: e.ID, ActorSpecies: e.Species,
			Target: food.ID, TargetSpecies: food.Species,
			Msg: fmt.Sprintf("%s ate %s from %s", s.Name, p.Resource, w.cfg.Species[food.Species].Name),
		}}
	}
	return nil
}

func (w *World) moveToward(e *Entity, target Point) { w.move(e, target, false) }
func (w *World) moveAway(e *Entity, from Point)     { w.move(e, from, true) }

func (w *World) move(e *Entity, ref Point, away bool) {
	e.MoveAcc += w.cfg.Species[e.Species].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		best := e.Pos
		bestD := Dist(e.Pos, ref)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !Passable(w.At(p)) || w.FaunaAt(p) != nil {
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
		e.Pos = best
		w.markDirty(e.ID)
	}
}

func (w *World) wander(e *Entity) {
	e.MoveAcc += w.cfg.Species[e.Species].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		n := neighbors[w.RandN(len(neighbors))]
		p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
		if w.InBounds(p) && Passable(w.At(p)) && w.FaunaAt(p) == nil {
			e.Pos = p
			w.markDirty(e.ID)
		}
	}
}

// Stubs implemented in Tasks 5 and 6.
func (w *World) fleeStep(e *Entity) ([]Event, bool)         { return nil, false }
func (w *World) huntStrike(e *Entity, prey *Entity) []Event { return nil }
func (w *World) shelterStep(e *Entity) bool                 { return false }
