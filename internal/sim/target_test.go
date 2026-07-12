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
	// full again: back to idle wandering, the target clears
	r.Fullness = w.Cfg().Types["rabbit"].StomachSize
	w.Step()
	if r.TargetID != 0 {
		t.Fatalf("idle: TargetID = %d, want 0", r.TargetID)
	}
}
