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
	c := cfg(t)
	w := Generate(123, c)
	if w.Width != 64 || w.Height != 64 {
		t.Fatalf("size %dx%d", w.Width, w.Height)
	}
	terrain := map[sim.Terrain]int{}
	for _, tr := range w.Terrain {
		terrain[tr]++
	}
	if terrain[sim.TerrainRock] < 64*64*7/10 {
		t.Errorf("map not mostly rock: %v", terrain)
	}
	if terrain[sim.TerrainGold] == 0 {
		t.Error("no gold veins")
	}
	if w.At(sim.Point{X: 32, Y: 32}) != sim.TerrainDirt {
		t.Error("no clearing at center")
	}
	counts := map[string]int{}
	for _, e := range w.Entities {
		counts[e.Species]++
		if c.Species[e.Species].Kind == "fauna" && !sim.Passable(w.At(e.Pos)) {
			t.Errorf("%s spawned on impassable tile %v", e.Species, e.Pos)
		}
	}
	if counts["mushroom"] == 0 {
		t.Error("no mushrooms generated")
	}
	if counts["dwarf"] != 0 {
		t.Error("dwarves must not generate; players spawn them")
	}
}

func TestGenerateLegacyNoise(t *testing.T) {
	_, f, _, _ := runtime.Caller(0)
	c, err := data.Load(filepath.Join(filepath.Dir(f), "..", "sim", "testdata", "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	w := Generate(123, c)
	seen := map[sim.Terrain]bool{}
	for _, tr := range w.Terrain {
		seen[tr] = true
	}
	if !seen[sim.TerrainGrass] || !seen[sim.TerrainWater] {
		t.Error("noise path lost grass or water")
	}
}
