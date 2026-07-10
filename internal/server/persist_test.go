package server

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

func loadCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(5, cfg)
	for i := 0; i < 500; i++ {
		w.Step()
	}
	path := filepath.Join(t.TempDir(), "world.json")
	if err := SaveWorld(w, path); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadWorld(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w2.Tick != w.Tick || len(w2.Entities) != len(w.Entities) {
		t.Fatalf("round trip mismatch: tick %d vs %d, %d vs %d entities",
			w2.Tick, w.Tick, len(w2.Entities), len(w.Entities))
	}
	// loaded world must keep stepping identically to the original
	for i := 0; i < 200; i++ {
		w.Step()
		w2.Step()
	}
	if w2.Rng != w.Rng || len(w2.Entities) != len(w.Entities) {
		t.Error("loaded world diverged from original")
	}
}

func TestLoadPrunesUnknownSpecies(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(5, cfg)
	ghost := w.Spawn("dwarf", sim.Point{X: 32, Y: 32})
	ghost.Species = "rabbit" // simulate a save from before the pivot
	path := filepath.Join(t.TempDir(), "w.json")
	if err := SaveWorld(w, path); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadWorld(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w2.Entities[ghost.ID]; ok {
		t.Error("unknown-species entity survived load")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := LoadWorld(filepath.Join(t.TempDir(), "nope.json"), loadCfg(t))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}
