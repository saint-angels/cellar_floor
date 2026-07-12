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
	// The clearing geometry is fixed, but veins draw from the world RNG, so a
	// different seed shifts the soft-rock blobs and must also shuffle scatter.
	c2 := Generate(124, c)
	terrainSame := true
	for i := range a.Terrain {
		if a.Terrain[i] != c2.Terrain[i] {
			terrainSame = false
			break
		}
	}
	if terrainSame {
		t.Fatal("different seeds produced identical terrain; veins must vary")
	}
	positions := func(w *sim.World) map[sim.Point]bool {
		m := map[sim.Point]bool{}
		for _, e := range w.Entities {
			m[e.Pos] = true
		}
		return m
	}
	pa, pc := positions(a), positions(c2)
	same := len(pa) == len(pc)
	for p := range pa {
		if !pc[p] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical entity scatter")
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
	if w.At(sim.Point{X: 32, Y: 32}) != sim.TerrainDirt {
		t.Error("no clearing at center")
	}
	counts := map[string]int{}
	for _, e := range w.Entities {
		counts[e.Type]++
		if c.Types[e.Type].Kind == "fauna" && !w.Passable(w.At(e.Pos)) {
			t.Errorf("%s spawned on impassable tile %v", e.Type, e.Pos)
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

// The intended opening: the clearing is bigger than the campfire's light,
// so every mineable face starts dark and dwarves cannot mine until a
// player places a torch to direct them.
func TestFreshWorldStartsWithAllFacesDark(t *testing.T) {
	c := cfg(t)
	w := Generate(123, c)
	for y := 0; y < c.Gen.Height; y++ {
		for x := 0; x < c.Gen.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if w.Mineable(w.At(p)) && w.Lit(p) {
				t.Fatalf("mineable cell %v starts lit; the opening should be fully dark", p)
			}
		}
	}
}
