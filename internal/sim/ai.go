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
	var best *Entity
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
		if d := Dist(e.Pos, c.Pos); d < bestD {
			best, bestD = c, d
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
	w.moveToward(e, light.Pos)
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
