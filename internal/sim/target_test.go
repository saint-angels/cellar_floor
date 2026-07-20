package sim

import "testing"

func TestInteractionTargetTracksFood(t *testing.T) {
	w := flatWorld(t, 16, 16, 1)
	b := w.Spawn("bush", Point{10, 10})
	r := w.Spawn("rabbit", Point{2, 2})
	r.Fullness = 1
	w.Step()
	if r.TargetID != b.ID {
		t.Fatalf("seeking food: TargetID = %d, want bush %d", r.TargetID, b.ID)
	}
	// greedy: even full, the beacon still pulls — the target holds
	r.Fullness = w.Cfg().Types["rabbit"].StomachSize
	w.Step()
	if r.TargetID != b.ID {
		t.Fatalf("greedy: full rabbit dropped the beacon, TargetID = %d", r.TargetID)
	}
	// the bush runs dry: nothing worth eating, the target finally clears
	b.Produces[0].Amount = 0
	w.Step()
	if r.TargetID != 0 {
		t.Fatalf("dry beacon: TargetID = %d, want 0", r.TargetID)
	}
}

func TestEatsToFullOnceStarted(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	// hungry by half a point; the old behavior stopped one bite past the
	// threshold, eating to full crosses it by several bites
	r.Fullness = w.Cfg().Types["rabbit"].HungerThreshold - 0.5
	for i := 0; i < 12; i++ {
		w.Step()
	}
	if r.Fullness < w.Cfg().Types["rabbit"].StomachSize-0.5 {
		t.Fatalf("fullness = %.2f, want near stomach %.1f", r.Fullness, w.Cfg().Types["rabbit"].StomachSize)
	}
}

func TestMealEndsWhenFull(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	r.Fullness = w.Cfg().Types["rabbit"].HungerThreshold - 0.5
	for i := 0; i < 30; i++ {
		w.Step()
	}
	// filled up long ago; the tick drain must not pin it to the bush
	if r.Action == "eating" || r.Action == "seeking food" {
		t.Fatalf("action = %q, the meal must end once full", r.Action)
	}
}

// A regrowing stub (under half a bite left) is not worth a trek even to a
// hungry eater — without this guard a greedy eater camps the stub and sips
// its per-tick regrowth forever.
func TestNoTrekForARegrowingStub(t *testing.T) {
	w := flatWorld(t, 16, 16, 1)
	far := w.Spawn("bush", Point{14, 14})
	far.Produces[0].Amount = 0.3 // < half a bite, and it regrows
	r := w.Spawn("rabbit", Point{2, 2})
	r.Fullness = 1 // even hungry
	w.Step()
	if r.Action == "seeking food" || r.TargetID == far.ID {
		t.Fatalf("action = %q target %d: a regrowing stub must not start a trek", r.Action, r.TargetID)
	}
}
