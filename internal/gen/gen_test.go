package gen

import (
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func cfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	c, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGenerateDeterministic(t *testing.T) {
	c := cfg(t)
	a, b := Generate(123, c), Generate(123, c)
	for i := range a.Terrain {
		if a.Terrain[i] != b.Terrain[i] {
			t.Fatal("terrain differs for same seed")
		}
	}
	if len(a.Entities) != len(b.Entities) {
		t.Fatalf("entity counts differ: %d vs %d", len(a.Entities), len(b.Entities))
	}
	c2 := Generate(124, c)
	same := true
	for i := range a.Terrain {
		if a.Terrain[i] != c2.Terrain[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical terrain")
	}
}

func TestGenerateContents(t *testing.T) {
	w := Generate(123, cfg(t))
	if w.Width != 64 || w.Height != 64 {
		t.Fatalf("size %dx%d", w.Width, w.Height)
	}
	counts := map[string]int{}
	for _, e := range w.Entities {
		counts[e.Species]++
		if cfg(t).Species[e.Species].Kind == "fauna" && !sim.Passable(w.At(e.Pos)) {
			t.Errorf("%s spawned on impassable tile %v", e.Species, e.Pos)
		}
	}
	for _, s := range []string{"grass", "bush", "rabbit", "wolf"} {
		if counts[s] == 0 {
			t.Errorf("no %s generated", s)
		}
	}
}
