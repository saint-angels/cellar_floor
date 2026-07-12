package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// darkCfg has a fauna miner that moves a step per tick and a small campfire
// light source used to carve a lit island out of an otherwise dark world.
func darkCfg() *data.Config {
	return &data.Config{
		Sim:     data.SimConfig{TickRate: 2},
		Terrain: data.CanonicalTerrain(),
		Types: map[string]*data.EntityType{
			"miner": {ID: "miner", Name: "Miner", Kind: "fauna", Color: "#fff",
				BiteSize: 2, StomachSize: 10, HungerThreshold: 0,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1.0, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100},
			"campfire": {ID: "campfire", Name: "Campfire", Kind: "structure", Color: "#e25822",
				LightRadius: 3, Lifespan: 0},
		},
	}
}

func newDarkWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld(20, 20, 1, darkCfg())
	w.Spawn("campfire", Point{2, 2})
	return w
}

func TestDwarfInDarkFleesTowardLight(t *testing.T) {
	w := newDarkWorld(t) // 20x20 floor, campfire (radius 3) at {2,2}
	e := w.Spawn("miner", Point{15, 15})
	before := Dist(e.Pos, Point{2, 2})
	w.Step()
	if e.Action != "fleeing the dark" {
		t.Fatalf("action = %q, want fleeing the dark", e.Action)
	}
	if Dist(e.Pos, Point{2, 2}) >= before {
		t.Fatal("dwarf must move toward the light")
	}
}

func TestDwarfInLightDoesNotPanic(t *testing.T) {
	w := newDarkWorld(t)
	e := w.Spawn("miner", Point{3, 3}) // inside campfire radius
	w.Step()
	if e.Action == "fleeing the dark" {
		t.Fatal("a lit dwarf must not flee")
	}
}

func TestWanderChanceControlsIdleMovement(t *testing.T) {
	cfg := darkCfg()
	cfg.Types["miner"].WanderChance = 1
	w := NewWorld(20, 20, 1, cfg)
	w.Spawn("campfire", Point{10, 10}) // keep the wanderer lit and fearless
	e := w.Spawn("miner", Point{10, 10})
	start := e.Pos
	moved := false
	for i := 0; i < 10 && !moved; i++ {
		w.Step()
		moved = e.Pos != start
	}
	if !moved {
		t.Fatal("wander_chance 1 idle fauna must move within a few ticks")
	}

	cfg2 := darkCfg() // WanderChance zero value
	w2 := NewWorld(20, 20, 1, cfg2)
	w2.Spawn("campfire", Point{10, 10})
	e2 := w2.Spawn("miner", Point{10, 10})
	start2 := e2.Pos
	for i := 0; i < 30; i++ {
		w2.Step()
	}
	if e2.Pos != start2 {
		t.Fatal("wander_chance 0 idle fauna must stay put")
	}
}

// A mold-style maze: the nearest mushroom by straight-line distance is
// fully walled off, a farther one is reachable around a wall. Greedy
// movement starves here; BFS seeking must route and eat.
func TestHungrySeekerRoutesAroundWalls(t *testing.T) {
	cfg := darkCfg()
	cfg.Types["miner"].HungerThreshold = 9
	cfg.Types["shroomy"] = &data.EntityType{ID: "shroomy", Name: "Shroomy", Kind: "flora", Color: "#fff",
		Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6}}}
	cfg.Types["miner"].Eats = []string{"shroom"}
	w := NewWorld(11, 11, 1, cfg)
	w.Spawn("campfire", Point{5, 5}) // small light; irrelevant, keep dwarf calm
	cfg.Types["campfire"].LightRadius = 20
	w.RecomputeLight()

	// walled pocket around the near mushroom at {7,5}: dwarf at {5,5}
	near := w.Spawn("shroomy", Point{7, 5})
	for _, p := range []Point{{6, 4}, {7, 4}, {8, 4}, {6, 5}, {8, 5}, {6, 6}, {7, 6}, {8, 6}} {
		w.Terrain[idx(w, p)] = TerrainWater
	}
	_ = near
	far := w.Spawn("shroomy", Point{5, 9}) // open path straight south
	_ = far
	d := w.Spawn("miner", Point{5, 5})
	d.Fullness = 1

	for i := 0; i < 40 && d.Fullness < 3; i++ {
		w.Step()
	}
	if d.Fullness < 3 {
		t.Fatalf("dwarf never ate: fullness %v at %v; seeker must pick reachable food and route to it", d.Fullness, d.Pos)
	}
}
