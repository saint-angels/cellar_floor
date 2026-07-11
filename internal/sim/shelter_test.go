package sim

import "testing"

func TestRabbitRemembersHomeAndReturns(t *testing.T) {
	w := flatWorld(t, 20, 20, 1)
	w.Spawn("bush", Point{3, 3})
	r := w.Spawn("rabbit", Point{4, 3})
	r.Fullness = 10
	w.Step()
	if r.Home == nil {
		t.Fatal("rabbit did not adopt a home")
	}
	r.Pos = Point{18, 18}
	w.Step()
	if r.Action != "going home" {
		t.Errorf("action = %q", r.Action)
	}
	d0 := Dist(r.Pos, *r.Home)
	for i := 0; i < 10; i++ {
		w.Step()
	}
	if Dist(r.Pos, *r.Home) >= d0 {
		t.Error("rabbit not heading home")
	}
}

func TestReproduction(t *testing.T) {
	w := flatWorld(t, 10, 10, 7)
	r := w.Spawn("rabbit", Point{5, 5})
	r.Age = w.Cfg().Types["rabbit"].MatureAge + 1
	r.Fullness = 10
	born := false
	for i := 0; i < 3000 && !born; i++ {
		r.Fullness = 10
		r.Age = w.Cfg().Types["rabbit"].MatureAge + 1
		for _, ev := range w.Step() {
			if ev.Type == "born" {
				born = true
			}
		}
	}
	if !born {
		t.Fatal("no birth in 3000 fertile ticks")
	}
	if w.CountAlive("rabbit") < 2 {
		t.Error("baby not spawned")
	}
}

func TestPopulationFloor(t *testing.T) {
	w := flatWorldFloors(t, 10, 10, 1)
	evs := w.Step() // zero rabbits and wolves: floors kick in
	if w.CountAlive("rabbit") < w.Cfg().Types["rabbit"].PopFloor {
		t.Errorf("rabbits = %d, floor = %d", w.CountAlive("rabbit"), w.Cfg().Types["rabbit"].PopFloor)
	}
	if w.CountAlive("wolf") < w.Cfg().Types["wolf"].PopFloor {
		t.Error("wolf floor not enforced")
	}
	found := false
	for _, e := range evs {
		if e.Type == "spawned" {
			found = true
		}
	}
	if !found {
		t.Error("no spawned events")
	}
}
