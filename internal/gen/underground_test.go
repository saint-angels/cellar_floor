package gen

import (
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func undergroundCfg() *data.Config {
	return &data.Config{
		Sim:     data.SimConfig{TickRate: 2},
		Terrain: append(data.CanonicalTerrain(), data.TerrainType{ID: "soft_rock", Color: "#575049", Mineable: true, HitPoints: 43200}),
		Gen: data.GenConfig{
			Width: 32, Height: 32,
			ClearingRadius: 4,
			Scatter:        []data.ScatterRule{{Type: "shroom", Terrain: "dirt", Chance: 0.3}},
		},
		Types: map[string]*data.EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.001}}},
			"campfire": {ID: "campfire", Name: "Campfire", Kind: "structure", Color: "#e25822",
				LightRadius: 8, Lifespan: 0},
		},
	}
}

func TestCampfireAtClearingCenter(t *testing.T) {
	cfg := undergroundCfg()
	cfg.Gen.Center = "campfire"
	w := Generate(7, cfg)
	cx, cy := cfg.Gen.Width/2, cfg.Gen.Height/2
	found := false
	for _, e := range w.Entities {
		if e.Type == "campfire" && e.Pos == (sim.Point{X: cx, Y: cy}) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected one campfire at the clearing center")
	}
}

func TestUndergroundGeneration(t *testing.T) {
	cfg := undergroundCfg()
	w := Generate(42, cfg)
	counts := map[sim.Terrain]int{}
	for _, tr := range w.Terrain {
		counts[tr]++
	}
	if counts[sim.TerrainRock] < 32*32*6/10 {
		t.Errorf("map not mostly rock: %v", counts)
	}
	for tr := range counts {
		if tr > sim.TerrainFloor {
			t.Errorf("terrain value %d exceeds floor: %v", tr, counts)
		}
	}
	if counts[sim.TerrainFloor] != 0 || counts[sim.TerrainWater] != 0 || counts[sim.TerrainGrass] != 0 {
		t.Errorf("unexpected terrain in underground map: %v", counts)
	}
	center := sim.Point{X: 16, Y: 16}
	if w.At(center) != sim.TerrainDirt {
		t.Error("clearing center is not dirt")
	}
	if w.At(sim.Point{X: 0, Y: 0}) == sim.TerrainDirt {
		t.Error("corner should not be clearing")
	}
	shrooms := 0
	for _, e := range w.Entities {
		if e.Type == "shroom" {
			shrooms++
			if w.At(e.Pos) != sim.TerrainDirt {
				t.Error("shroom outside clearing")
			}
		}
	}
	if shrooms == 0 {
		t.Error("no shrooms scattered in clearing")
	}
	// determinism
	b := Generate(42, cfg)
	for i := range w.Terrain {
		if w.Terrain[i] != b.Terrain[i] {
			t.Fatal("underground gen not deterministic")
		}
	}
}

func TestVeinsGrowConnectedBlobsInRock(t *testing.T) {
	cfg := undergroundCfg()
	cfg.Gen.Veins = []data.VeinRule{{Terrain: "soft_rock", Seeds: 3, Size: 8}}
	w := Generate(11, cfg)
	soft := sim.Terrain(5)
	cells := map[sim.Point]bool{}
	for y := 0; y < cfg.Gen.Height; y++ {
		for x := 0; x < cfg.Gen.Width; x++ {
			if w.At(sim.Point{X: x, Y: y}) == soft {
				cells[sim.Point{X: x, Y: y}] = true
			}
		}
	}
	if len(cells) != 3*8 {
		t.Fatalf("soft cells = %d, want 24", len(cells))
	}
	// every soft cell touches another soft cell unless it is a size-1 blob
	for p := range cells {
		touching := false
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if cells[sim.Point{X: p.X + dx, Y: p.Y + dy}] {
					touching = true
				}
			}
		}
		if !touching {
			t.Fatalf("isolated soft cell at %v; veins must be connected", p)
		}
	}
	// determinism
	w2 := Generate(11, cfg)
	for i := range w.Terrain {
		if w.Terrain[i] != w2.Terrain[i] {
			t.Fatal("veins not deterministic per seed")
		}
	}
	// clearing untouched
	c := sim.Point{X: cfg.Gen.Width / 2, Y: cfg.Gen.Height / 2}
	if w.At(c) != sim.TerrainDirt {
		t.Fatal("vein replaced the clearing")
	}
}
