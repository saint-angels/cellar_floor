package sim

import (
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
)

func testCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatalf("load data: %v", err)
	}
	return cfg
}

func flatWorld(t *testing.T, w, h int, seed uint64) *World {
	cfg := testCfg(t)
	for _, s := range cfg.Species {
		s.PopFloor = 0
	}
	return NewWorld(w, h, seed, cfg) // all grass terrain by default
}

func flatWorldFloors(t *testing.T, w, h int, seed uint64) *World {
	return NewWorld(w, h, seed, testCfg(t))
}

func TestSpawnCopiesSpeciesData(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	r := w.Spawn("rabbit", Point{2, 3})
	if r == nil || w.Entities[r.ID] != r {
		t.Fatal("spawn failed")
	}
	if len(r.Produces) != 2 || r.Produces[0].Resource != "meat" {
		t.Errorf("produces not copied: %+v", r.Produces)
	}
	r.Produces[0].Amount = 0
	if w.Cfg().Species["rabbit"].Produces[0].Amount == 0 {
		t.Error("spawn shares Produces slice with species template")
	}
	if r.Fullness != w.Cfg().Species["rabbit"].StomachSize/2 {
		t.Errorf("fullness = %v", r.Fullness)
	}
}

func TestRngDeterministic(t *testing.T) {
	a, b := flatWorld(t, 4, 4, 42), flatWorld(t, 4, 4, 42)
	for i := 0; i < 100; i++ {
		if a.RandFloat() != b.RandFloat() {
			t.Fatal("same seed diverged")
		}
	}
	if a.RandN(10) < 0 || a.RandN(10) > 9 {
		t.Error("RandN out of range")
	}
}

func TestHelpers(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	if !w.InBounds(Point{7, 7}) || w.InBounds(Point{8, 0}) {
		t.Error("InBounds wrong")
	}
	w.Spawn("rabbit", Point{1, 1})
	if w.FaunaAt(Point{1, 1}) == nil || w.FaunaAt(Point{0, 0}) != nil {
		t.Error("FaunaAt wrong")
	}
	if w.CountAlive("rabbit") != 1 {
		t.Error("CountAlive wrong")
	}
	if Dist(Point{0, 0}, Point{3, 2}) != 3 {
		t.Error("Dist should be Chebyshev")
	}
}
