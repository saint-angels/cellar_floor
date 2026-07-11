package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// darkCfg has a fauna miner that moves a step per tick and a small campfire
// light source used to carve a lit island out of an otherwise dark world.
func darkCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
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
