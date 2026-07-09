package sim_test

import (
	"testing"

	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

// TestFaunaAtOccIndexAgreesWithBruteForce guards the O(1) occupancy index
// (World.occ) against desync with reality. It runs a deterministic world for
// a few thousand ticks and, at several checkpoints, cross-checks FaunaAt
// against a brute-force scan over all entities.
func TestFaunaAtOccIndexAgreesWithBruteForce(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(4242, cfg)

	checkOcc := func(tick int) {
		live := map[sim.Point]*sim.Entity{}
		for _, id := range w.SortedIDs() {
			e := w.Entities[id]
			if e.Dead {
				continue
			}
			s, ok := cfg.Species[e.Species]
			if !ok || s.Kind != "fauna" {
				continue
			}
			if prev, dup := live[e.Pos]; dup {
				t.Fatalf("tick %d: two live fauna on same tile %v: %d and %d", tick, e.Pos, prev.ID, e.ID)
			}
			live[e.Pos] = e
			if got := w.FaunaAt(e.Pos); got != e {
				t.Fatalf("tick %d: FaunaAt(%v) = %v, want entity %d", tick, e.Pos, got, e.ID)
			}
		}
		checked := 0
		for y := 0; y < w.Height && checked < 50; y++ {
			for x := 0; x < w.Width && checked < 50; x++ {
				p := sim.Point{X: x, Y: y}
				if _, occupied := live[p]; occupied {
					continue
				}
				if got := w.FaunaAt(p); got != nil {
					t.Fatalf("tick %d: FaunaAt(%v) = entity %d, want nil (empty tile)", tick, p, got.ID)
				}
				checked++
			}
		}
	}

	checkOcc(0)
	for i := 1; i <= 3000; i++ {
		w.Step()
		if i%500 == 0 {
			checkOcc(i)
		}
	}
}
