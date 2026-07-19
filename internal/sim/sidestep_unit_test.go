package sim

import "testing"

// The sidestep must never be a retreat: a blocked creature either closes on
// the target or holds position, so a blocker cannot herd it away.
func TestSidestepNeverWalksAwayFromTheTarget(t *testing.T) {
	w := mineWorld(20, 20)
	e := w.Spawn("miner", Point{5, 5})
	target := Point{9, 5}
	before := Dist(e.Pos, target)
	p, ok := w.sidestep(e, target)
	if !ok {
		t.Fatal("open ground: sidestep must find a cell")
	}
	if got := Dist(p, target); got > before {
		t.Fatalf("sidestep to %v moved from distance %d to %d, away from target", p, before, got)
	}
}

// Fully hemmed in, the walk reports no move rather than teleporting or
// picking an occupied cell.
func TestSidestepReportsHemmedIn(t *testing.T) {
	w := mineWorld(20, 20)
	e := w.Spawn("miner", Point{5, 5})
	for _, n := range neighbors {
		w.Terrain[idx(w, Point{5 + n.X, 5 + n.Y})] = TerrainWater
	}
	if _, ok := w.sidestep(e, Point{9, 5}); ok {
		t.Fatal("walled in on every side: sidestep must report false")
	}
}
