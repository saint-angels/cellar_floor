package sim

import "fmt"

// Removed lists entity IDs deleted during the most recent Step.
// aiStep is implemented in ai.go (Task 4); declare a stub there first.

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
		if w.cfg.Species[e.Species].Kind == "fauna" {
			events = append(events, w.aiStep(e)...)
		}
	}

	// 3. metabolism, starvation, aging
	diedThisTick := map[int]bool{}
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || e.Dead {
			continue
		}
		s := w.cfg.Species[e.Species]
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
			diedThisTick[id] = true
		} else if e.Age > s.Lifespan {
			events = append(events, w.kill(e, "died", fmt.Sprintf("%s died of old age", s.Name)))
			diedThisTick[id] = true
		}
	}

	// 4. reproduction and guardrails (Task 6 fills these in)
	events = append(events, w.reproduceAndGuard()...)

	// 5. corpse decay (entities that died this tick start decaying next tick)
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || !e.Dead || diedThisTick[id] {
			continue
		}
		e.DecayLeft--
		if e.DecayLeft <= 0 {
			delete(w.Entities, id)
			w.Removed = append(w.Removed, id)
		}
	}
	return events
}

func (w *World) kill(e *Entity, evType, msg string) Event {
	s := w.cfg.Species[e.Species]
	e.Dead = true
	e.Action = "dead"
	e.DecayLeft = s.DecayTicks
	w.markDirty(e.ID)
	return Event{Tick: w.Tick, Type: evType, Actor: e.ID, ActorSpecies: e.Species, Msg: msg}
}

// Stubs replaced in Task 4 (ai.go) and Task 6.
func (w *World) aiStep(e *Entity) []Event   { return nil }
func (w *World) reproduceAndGuard() []Event { return nil }
