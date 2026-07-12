package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

func TestBeamHitsOnlyTheTarget(t *testing.T) {
	w := mineWorld(20, 20)
	w.Cfg().Upgrades = []data.Upgrade{
		{Name: "Lance", Kind: "beam", Amount: 3, Max: 1, Color: "#ffffff", Radius: 24, PeriodMs: 1200},
	}
	w.Claims = map[string]int{"Lance": 1}

	if got := w.MineBonus(); got != 0 {
		t.Fatalf("MineBonus = %d, beams must not join the AOE sweep", got)
	}
	if got := w.BeamBonus(); got != 3 {
		t.Fatalf("BeamBonus = %d, want 3", got)
	}

	// two lit faces adjacent to the miner; the beam lands on the chosen one
	w.Terrain[idx(w, Point{4, 2})] = Terrain(3)
	w.Terrain[idx(w, Point{4, 3})] = Terrain(3)
	e := w.Spawn("miner", Point{3, 2})
	w.Step()

	if e.MineTarget == nil {
		t.Fatal("miner must pick a face")
	}
	ti := e.MineTarget.Y*w.Width + e.MineTarget.X
	oi := idx(w, Point{4, 2})
	if ti == oi {
		oi = idx(w, Point{4, 3})
	}
	if w.MineDamage[oi] != 1 {
		t.Fatalf("swept face damage = %d, want base 1", w.MineDamage[oi])
	}
	if w.MineDamage[ti] != 4 {
		t.Fatalf("target face damage = %d, want base 1 + beam 3", w.MineDamage[ti])
	}
}
