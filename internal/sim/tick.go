package sim

import (
	"fmt"
	"sort"
)

// Step advances the simulation by one tick and returns the events it produced.

func (w *World) markDirty(id int) { w.dirty[id] = true }

// DirtyAndReset returns IDs changed during the last Step and clears the set.
func (w *World) DirtyAndReset() []int {
	ids := make([]int, 0, len(w.dirty))
	for id := range w.dirty {
		ids = append(ids, id)
	}
	w.dirty = map[int]bool{}
	sortInts(ids)
	return ids
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func (w *World) Step() []Event {
	w.Tick++
	w.Removed = w.Removed[:0]
	w.diedThisTick = map[int]bool{}
	var events []Event

	ids := w.SortedIDs()

	// 1. flora regrow (and corpse "produces" never regrow since regrow=0)
	for _, id := range ids {
		e := w.Entities[id]
		if e.Dead {
			continue
		}
		for i := range e.Produces {
			p := &e.Produces[i]
			if p.Regrow > 0 && p.Amount < p.Max {
				p.Amount += p.Regrow
				if p.Amount > p.Max {
					p.Amount = p.Max
				}
				w.markDirty(id)
			}
		}
	}

	// 2. fauna AI
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || e.Dead {
			continue
		}
		if w.cfg.Types[e.Type].Kind == "fauna" {
			events = append(events, w.aiStep(e)...)
		}
	}

	// 3. metabolism, starvation, aging
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || e.Dead {
			continue
		}
		s := w.cfg.Types[e.Type]
		if s.Kind == "structure" {
			e.Age++
			if s.Lifespan > 0 && e.Age > s.Lifespan {
				events = append(events, w.kill(e, "burnout", fmt.Sprintf("a %s burned out", s.Name)))
			}
			continue
		}
		if s.Kind != "fauna" {
			continue
		}
		e.Age++
		e.Fullness -= s.Metabolism
		if e.Fullness < 0 {
			e.Fullness = 0
		}
		if e.Fullness == 0 {
			e.StarvingFor++
		} else {
			e.StarvingFor = 0
		}
		w.markDirty(id)
		if e.StarvingFor > s.StarveTicks {
			events = append(events, w.kill(e, "starved", fmt.Sprintf("%s starved", s.Name)))
		} else if e.Age > s.Lifespan {
			events = append(events, w.kill(e, "died", fmt.Sprintf("%s died of old age", s.Name)))
		}
	}

	// 4. reproduction and guardrails (Task 6 fills these in)
	events = append(events, w.reproduceAndGuard()...)

	// 5. corpse decay (entities that died this tick start decaying next tick)
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || !e.Dead || w.diedThisTick[id] {
			continue
		}
		e.DecayLeft--
		if e.DecayLeft <= 0 {
			delete(w.Entities, id)
			w.sortedDirty = true
			w.Removed = append(w.Removed, id)
		}
	}
	return events
}

func (w *World) kill(e *Entity, evType, msg string) Event {
	s := w.cfg.Types[e.Type]
	e.Dead = true
	w.diedThisTick[e.ID] = true
	w.counts[e.Type]--
	if w.occ[e.Pos] == e.ID {
		delete(w.occ, e.Pos)
	}
	e.Action = "dead"
	e.DecayLeft = s.DecayTicks
	w.markDirty(e.ID)
	if s.LightRadius > 0 {
		w.RecomputeLight()
	}
	return Event{Tick: w.Tick, Type: evType, Actor: e.ID, ActorType: e.Type, Msg: msg}
}

func (w *World) reproduceAndGuard() []Event {
	var events []Event

	// births
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		s := w.cfg.Types[e.Type]
		if e.Dead || s.Kind != "fauna" {
			continue
		}
		if e.Age <= s.MatureAge || e.Fullness < s.ReproThreshold {
			continue
		}
		if w.CountAlive(e.Type) >= s.PopCap {
			continue
		}
		if w.RandFloat() >= s.ReproChance {
			continue
		}
		var free *Point
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if w.InBounds(p) && Passable(w.At(p)) && w.FaunaAt(p) == nil {
				free = &p
				break
			}
		}
		if free == nil {
			continue
		}
		baby := w.Spawn(e.Type, *free)
		e.Fullness -= s.ReproCost
		w.markDirty(e.ID)
		events = append(events, Event{
			Tick: w.Tick, Type: "born",
			Actor: baby.ID, ActorType: baby.Type,
			Target: e.ID, TargetType: e.Type,
			Msg: fmt.Sprintf("a %s was born", s.Name),
		})
	}

	// floors
	typeIDs := make([]string, 0, len(w.cfg.Types))
	for id := range w.cfg.Types {
		typeIDs = append(typeIDs, id)
	}
	sort.Strings(typeIDs)
	for _, sid := range typeIDs {
		s := w.cfg.Types[sid]
		if s.Kind != "fauna" || s.PopFloor <= 0 {
			continue
		}
		for w.CountAlive(sid) < s.PopFloor {
			p, ok := w.randomFreeTile()
			if !ok {
				break
			}
			e := w.Spawn(sid, p)
			events = append(events, Event{
				Tick: w.Tick, Type: "spawned",
				Actor: e.ID, ActorType: sid,
				Msg: fmt.Sprintf("a %s wandered in", s.Name),
			})
		}
	}
	return events
}

func (w *World) randomFreeTile() (Point, bool) {
	for i := 0; i < 50; i++ {
		p := Point{w.RandN(w.Width), w.RandN(w.Height)}
		if Passable(w.At(p)) && w.FaunaAt(p) == nil {
			return p, true
		}
	}
	return Point{}, false
}
