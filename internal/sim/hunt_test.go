package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// A hungry dwarf must hunt a rabbit: chase the fleeing prey down, kill it, and
// eat the corpse. This mirrors the production food web (dwarf eats meat, rabbit
// yields meat and flees the dwarf), the behavior the rabbit was added for.
func TestHungryDwarfHuntsAndEatsRabbit(t *testing.T) {
	w := mineWorld(20, 20) // sunstone floods light; open grass field
	cfg := w.Cfg()
	cfg.Types["dwarf"].Eats = append(cfg.Types["dwarf"].Eats, "meat")
	cfg.Types["rabbit"] = &data.EntityType{
		ID: "rabbit", Name: "Rabbit", Kind: "fauna", Color: "#fff",
		Eats:            []string{"shroom"},
		SenseRadius:     6, // prey scent: the dwarf senses it within 6 tiles
		Produces:        []data.Produce{{Resource: "meat", Amount: 4, Max: 4}},
		BiteSize:        1,
		StomachSize:     6,
		HungerThreshold: 3,
		Metabolism:      0.0001,
		StarveTicks:     1 << 30,
		Speed:           0.5, // clearly slower so the catch resolves within the budget
		Lifespan:        1 << 30,
		MatureAge:       1 << 30,
		PopCap:          10,
		DecayTicks:      200,
		FearRadius:      5,
	}
	d := w.Spawn("dwarf", Point{5, 5})
	d.Fullness = 1 // very hungry: the only food around is the rabbit's meat
	rab := w.Spawn("rabbit", Point{8, 5})

	caught := false
	for i := 0; i < 60 && !caught; i++ {
		w.Step()
		caught = rab.Dead
	}
	if !caught {
		t.Fatalf("dwarf never caught the rabbit: dwarf @%v full %.1f, rabbit @%v", d.Pos, d.Fullness, rab.Pos)
	}
	for i := 0; i < 6; i++ {
		w.Step()
	}
	if d.Fullness <= 1 {
		t.Errorf("dwarf killed the rabbit but never ate the corpse: fullness %.2f", d.Fullness)
	}
}
