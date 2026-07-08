package sim

import (
	"fmt"

	"cellarfloor/internal/data"
)

func speciesEatsProduceOf(eater, victim *data.Species) bool {
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

func (w *World) fleeStep(e *Entity) ([]Event, bool) {
	me := w.cfg.Species[e.Species]
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
		cs := w.cfg.Species[c.Species]
		if !speciesEatsProduceOf(cs, me) {
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
			Actor: e.ID, ActorSpecies: e.Species,
			Target: threat.ID, TargetSpecies: threat.Species,
			Msg: fmt.Sprintf("%s fled from %s", me.Name, w.cfg.Species[threat.Species].Name),
		})
	}
	e.Action = "fleeing"
	w.moveAway(e, threat.Pos)
	return evs, true
}

func (w *World) huntStrike(e *Entity, prey *Entity) []Event {
	s := w.cfg.Species[e.Species]
	ev := w.kill(prey, "killed", fmt.Sprintf("%s was killed by %s", w.cfg.Species[prey.Species].Name, s.Name))
	ev.Target = e.ID
	ev.TargetSpecies = e.Species
	e.Action = "hunting"
	w.markDirty(e.ID)
	hunt := Event{
		Tick: w.Tick, Type: "hunted",
		Actor: e.ID, ActorSpecies: e.Species,
		Target: prey.ID, TargetSpecies: prey.Species,
		Msg: fmt.Sprintf("%s hunted down %s", s.Name, w.cfg.Species[prey.Species].Name),
	}
	return []Event{hunt, ev}
}

func (w *World) shelterStep(e *Entity) bool {
	s := w.cfg.Species[e.Species]
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
