package sim

import "testing"

func TestHungryRabbitEatsAdjacentBush(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	b := w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	r.Fullness = 1
	before := b.Produces[0].Amount
	evs := w.Step()
	if b.Produces[0].Amount >= before {
		t.Errorf("bush not eaten: %v", b.Produces[0].Amount)
	}
	if r.Fullness <= 1-w.Cfg().Types["rabbit"].Metabolism {
		t.Errorf("rabbit fullness did not rise: %v", r.Fullness)
	}
	found := false
	for _, e := range evs {
		if e.Type == "ate" && e.Actor == r.ID {
			found = true
		}
	}
	if !found {
		t.Error("no ate event")
	}
}

func TestHungryRabbitWalksTowardFood(t *testing.T) {
	w := flatWorld(t, 16, 16, 1)
	w.Spawn("bush", Point{10, 10})
	r := w.Spawn("rabbit", Point{2, 2})
	r.Fullness = 1
	d0 := Dist(r.Pos, Point{10, 10})
	for i := 0; i < 8; i++ {
		w.Step()
	}
	if Dist(r.Pos, Point{10, 10}) >= d0 {
		t.Errorf("rabbit did not approach food: at %v", r.Pos)
	}
	if r.Action != "seeking food" && r.Action != "eating" {
		t.Errorf("action = %q", r.Action)
	}
}

func TestMovementAvoidsWater(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	for y := 0; y < 8; y++ {
		w.Terrain[y*8+4] = TerrainWater // vertical river at x=4
	}
	w.Spawn("bush", Point{6, 3})
	r := w.Spawn("rabbit", Point{3, 3})
	r.Fullness = 1
	for i := 0; i < 30; i++ {
		w.Step()
		if w.At(r.Pos) == TerrainWater {
			t.Fatal("rabbit walked into water")
		}
	}
}

// Greedy eating: even a completely full eater keeps emptying a beacon in
// range — overeating is wasted, but the source drains, so food is a
// consumable token and never accumulates into a larder.
func TestFullRabbitStillEatsGreedily(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	b := w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	r.Fullness = w.Cfg().Types["rabbit"].StomachSize
	before := b.Produces[0].Amount
	w.Step()
	if b.Produces[0].Amount >= before {
		t.Error("full rabbit must still eat the beacon down")
	}
	if r.Fullness > w.Cfg().Types["rabbit"].StomachSize {
		t.Errorf("fullness %v overflowed the stomach", r.Fullness)
	}
}
