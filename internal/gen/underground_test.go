package gen

import (
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func undergroundCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Gen: data.GenConfig{
			Width: 32, Height: 32,
			ClearingRadius: 4, GoldChance: 0.01,
			Scatter: []data.ScatterRule{{Species: "shroom", Terrain: "dirt", Chance: 0.3}},
		},
		Species: map[string]*data.Species{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.001}}},
		},
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
	if counts[sim.TerrainGold] == 0 {
		t.Error("no gold generated")
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
		if e.Species == "shroom" {
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
