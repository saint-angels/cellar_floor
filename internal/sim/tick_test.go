package sim

import "testing"

func TestRegrow(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	b := w.Spawn("bush", Point{0, 0})
	b.Produces[0].Amount = 0
	w.Step()
	if b.Produces[0].Amount != 0.01 {
		t.Errorf("berries = %v, want 0.01", b.Produces[0].Amount)
	}
	b.Produces[0].Amount = 8
	w.Step()
	if b.Produces[0].Amount > 8 {
		t.Error("regrow exceeded max")
	}
}

func TestStarvation(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	r := w.Spawn("rabbit", Point{0, 0})
	r.Fullness = 0.01
	starve := w.Cfg().Species["rabbit"].StarveTicks
	var events []Event
	for i := 0; i < starve+60; i++ {
		events = append(events, w.Step()...)
		if r.Dead {
			break
		}
	}
	if !r.Dead {
		t.Fatal("rabbit should starve")
	}
	found := false
	for _, e := range events {
		if e.Type == "starved" && e.Actor == r.ID {
			found = true
		}
	}
	if !found {
		t.Error("no starved event")
	}
	if r.DecayLeft != w.Cfg().Species["rabbit"].DecayTicks {
		t.Errorf("decay not set: %d", r.DecayLeft)
	}
}

func TestOldAgeAndDecayRemoval(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	r := w.Spawn("rabbit", Point{0, 0})
	r.Age = w.Cfg().Species["rabbit"].Lifespan
	r.Fullness = 10
	w.Step()
	if !r.Dead {
		t.Fatal("rabbit should die of old age")
	}
	r.DecayLeft = 1
	w.Step()
	if _, ok := w.Entities[r.ID]; ok {
		t.Error("corpse should be removed after decay")
	}
	if len(w.Removed) != 1 || w.Removed[0] != r.ID {
		t.Errorf("Removed = %v", w.Removed)
	}
}
