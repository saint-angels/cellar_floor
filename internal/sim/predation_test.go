package sim

import "testing"

func TestWolfHuntsAndEatsRabbit(t *testing.T) {
	w := flatWorld(t, 12, 12, 1)
	r := w.Spawn("rabbit", Point{5, 5})
	r.Fullness = 10 // not hungry, so it only flees
	wolf := w.Spawn("wolf", Point{6, 5})
	wolf.Fullness = 1
	killed := false
	for i := 0; i < 200 && !killed; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "killed" && ev.Actor == r.ID {
				killed = true
			}
		}
	}
	if !killed {
		t.Fatal("wolf never killed rabbit")
	}
	full0 := wolf.Fullness
	for i := 0; i < 20; i++ {
		w.Step()
	}
	if wolf.Fullness <= full0 {
		t.Error("wolf did not eat from corpse")
	}
}

func TestRabbitFleesWolf(t *testing.T) {
	w := flatWorld(t, 20, 20, 1)
	r := w.Spawn("rabbit", Point{10, 10})
	r.Fullness = 10
	wolf := w.Spawn("wolf", Point{12, 10})
	wolf.Fullness = 16 // sated wolf stands around
	d0 := Dist(r.Pos, wolf.Pos)
	w.Step()
	if Dist(r.Pos, wolf.Pos) < d0 {
		t.Error("rabbit moved toward wolf")
	}
	if r.Action != "fleeing" {
		t.Errorf("action = %q, want fleeing", r.Action)
	}
}

func TestNoCannibalism(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	a := w.Spawn("wolf", Point{2, 2})
	a.Fullness = 1
	b := w.Spawn("wolf", Point{3, 2})
	b.Fullness = 16
	w.Step()
	if b.Dead {
		t.Fatal("wolf ate wolf")
	}
	if a.Action == "fleeing" || b.Action == "fleeing" {
		t.Error("wolves fear each other")
	}
}
