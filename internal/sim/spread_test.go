package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// spreadCfg has a fast-spreading goo terrain at index 5 and a campfire.
func spreadCfg() *data.Config {
	terrain := data.CanonicalTerrain()
	terrain = append(terrain, data.TerrainType{
		ID: "goo", Color: "#7a8a4d", Mineable: true, HitPoints: 6, SpreadChance: 1,
	})
	return &data.Config{
		Sim:     data.SimConfig{TickRate: 2},
		Terrain: terrain,
		Types: map[string]*data.EntityType{
			"campfire": {ID: "campfire", Name: "Campfire", Kind: "structure", Color: "#e25822",
				LightRadius: 2, Lifespan: 0},
			"critter": {ID: "critter", Name: "Critter", Kind: "fauna", Color: "#888",
				Lifespan: 1 << 30, StomachSize: 10, HungerThreshold: 0, Speed: 0,
				Metabolism: 0.0001, StarveTicks: 1 << 30, DecayTicks: 10, PopCap: 5,
				Eats: []string{"x"}, BiteSize: 1},
		},
	}
}

func TestSpreadClaimsDarkPassableNeighbors(t *testing.T) {
	w := NewWorld(9, 9, 1, spreadCfg())
	// all grass (passable) and fully dark; one goo cell in the middle
	w.Terrain[idx(w, Point{4, 4})] = Terrain(5)
	w.Step()
	count := 0
	for _, tr := range w.Terrain {
		if tr == Terrain(5) {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("goo cells = %d after one step at chance 1, want growth", count)
	}
	// determinism
	w2 := NewWorld(9, 9, 1, spreadCfg())
	w2.Terrain[idx(w2, Point{4, 4})] = Terrain(5)
	w2.Step()
	for i := range w.Terrain {
		if w.Terrain[i] != w2.Terrain[i] {
			t.Fatal("spread not deterministic")
		}
	}
}

func TestSpreadRespectsLightAndFauna(t *testing.T) {
	w := NewWorld(9, 9, 1, spreadCfg())
	w.Terrain[idx(w, Point{4, 4})] = Terrain(5)
	w.Spawn("campfire", Point{2, 4}) // radius 2 lights cells within dist 2, incl {4,4}
	crit := w.Spawn("critter", Point{5, 4})
	for i := 0; i < 30; i++ {
		w.Step()
	}
	// every lit cell must stay grass; the critter's cell must stay grass.
	// The seed {4,4} is itself a lit goo cell by construction (a lit source
	// still spreads to its dark neighbors), so it is exempt from the check.
	seed := Point{4, 4}
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			p := Point{x, y}
			if p == seed {
				continue
			}
			if w.Lit(p) && w.At(p) == Terrain(5) {
				t.Fatalf("lit cell %v was molded", p)
			}
		}
	}
	if w.At(crit.Pos) == Terrain(5) {
		t.Fatal("occupied cell was molded")
	}
	_ = crit
}
