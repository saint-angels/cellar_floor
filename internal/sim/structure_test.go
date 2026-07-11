package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// structCfg has a torch that burns out and decays, and an eternal campfire.
func structCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Types: map[string]*data.EntityType{
			"torch": {ID: "torch", Name: "Torch", Kind: "structure", Color: "#ffb347",
				LightRadius: 3, Lifespan: 10, DecayTicks: 5},
			"campfire": {ID: "campfire", Name: "Campfire", Kind: "structure", Color: "#e25822",
				LightRadius: 8, Lifespan: 0},
		},
	}
}

func newStructWorld(t *testing.T) *World {
	t.Helper()
	return NewWorld(20, 20, 1, structCfg())
}

func TestTorchBurnsOutAndDecays(t *testing.T) {
	w := newStructWorld(t)
	torch := w.Spawn("torch", Point{2, 2})
	var burned bool
	for i := 0; i < 11; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "burnout" && ev.Actor == torch.ID {
				burned = true
			}
		}
	}
	if !burned {
		t.Fatal("expected a burnout event within lifespan+1 ticks")
	}
	if !w.Entities[torch.ID].Dead {
		t.Fatal("torch should be dead")
	}
	for i := 0; i < 6; i++ {
		w.Step()
	}
	if _, ok := w.Entities[torch.ID]; ok {
		t.Fatal("torch should be removed after decay_ticks")
	}
}

func TestBurnoutKeepsFaunaOccupancy(t *testing.T) {
	cfg := &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Types: map[string]*data.EntityType{
			"torch": {ID: "torch", Name: "Torch", Kind: "structure", Color: "#ffb347",
				LightRadius: 3, Lifespan: 3, DecayTicks: 5},
			"critter": {ID: "critter", Name: "Critter", Kind: "fauna", Color: "#888888",
				Lifespan: 100000, StomachSize: 10, HungerThreshold: 0, Speed: 0},
		},
	}
	w := NewWorld(20, 20, 1, cfg)
	cell := Point{5, 5}
	fauna := w.Spawn("critter", cell)
	w.Spawn("torch", cell)
	// age the torch past its lifespan so it burns out under the standing fauna
	for i := 0; i < 6; i++ {
		w.Step()
	}
	if w.Entities[fauna.ID].Dead {
		t.Fatal("critter should still be alive")
	}
	if got := w.FaunaAt(cell); got == nil || got.ID != fauna.ID {
		t.Fatalf("fauna occupancy lost after torch burnout: got %v", got)
	}
}

func TestCampfireNeverBurnsOut(t *testing.T) {
	w := newStructWorld(t)
	fire := w.Spawn("campfire", Point{2, 2})
	for i := 0; i < 1000; i++ {
		w.Step()
	}
	if w.Entities[fire.ID].Dead {
		t.Fatal("campfire with lifespan 0 must not die")
	}
}
