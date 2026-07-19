package sim

import "testing"

// A hungry eater standing on a nearly-empty food source must walk to a full
// one nearby instead of starving in place. Reproduces the live bug: dwarf jack
// sat on a mushroom stub (0.7 left) at full 1.9 with a full mushroom one tile
// away, never eating, because findFood selected any source with >= 0.5 left
// while eatFrom refuses a bite below BiteSize/2 (= 1.0 for the dwarf). The stub
// was pickable but not edible, pinning him to it.
func TestHungryDwarfLeavesDepletedStubForFullFood(t *testing.T) {
	w := mineWorld(20, 20) // sunstone floods light; no darkness flight
	d := w.Spawn("dwarf", Point{5, 5})
	d.Fullness = 2 // below the hunger threshold (4)

	stub := w.Spawn("shroom", Point{5, 5}) // same cell as the dwarf
	stub.Produces[0].Amount = 0.7          // >= old 0.5 floor, < BiteSize/2 (1.0)
	full := w.Spawn("shroom", Point{6, 5}) // one step away, plenty left
	full.Produces[0].Amount = 6

	for i := 0; i < 6; i++ {
		w.Step()
	}
	if d.Fullness <= 2 {
		t.Fatalf("dwarf never ate: fullness %.2f, still stuck on the stub", d.Fullness)
	}
	if d.Fullness < 4 {
		t.Fatalf("fullness %.2f: expected at least one full bite from the fuller shroom", d.Fullness)
	}
}
