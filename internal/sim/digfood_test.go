package sim

import "testing"

// The core of the food-control rework: a hungry dwarf with no walk-reachable
// food senses food buried in rock within its radius and tunnels to it. Here a
// shroom sits inside a solid rock block; the dwarf must mine its way across to
// reach and eat it, rather than starve.
func TestHungryDwarfTunnelsToBuriedFood(t *testing.T) {
	w := mineWorld(24, 24) // sunstone lights everything; all-grass by default
	// a solid rock block, columns 8..12, with a shroom walled inside it
	for x := 8; x <= 12; x++ {
		for y := 6; y <= 14; y++ {
			w.Terrain[idx(w, Point{x, y})] = Terrain(3) // rock (10 hp in mineCfg)
		}
	}
	food := w.Spawn("shroom", Point{10, 10}) // buried at the block's center
	_ = food
	d := w.Spawn("dwarf", Point{6, 10}) // hungry, west of the block on open grass
	d.Fullness = 1

	// precondition: the food is genuinely unreachable on foot
	if _, ok := w.nextStepToward(d.Pos, Point{10, 10}); ok {
		t.Fatal("setup: buried food must not be walk-reachable")
	}

	startX := d.Pos.X
	ate := false
	for i := 0; i < 400 && !ate; i++ {
		w.Step()
		ate = d.Fullness > 1.5 // took at least a bite of the buried shroom
	}
	if !ate {
		t.Fatalf("dwarf never reached the buried food: pos %v full %.2f", d.Pos, d.Fullness)
	}
	if d.Pos.X <= startX {
		t.Errorf("dwarf ate without digging inward: start x=%d, ended at %v", startX, d.Pos)
	}
}

// With no food sensed at all, digFoodStep must stay out of the way: the dwarf
// falls through to searching rather than tunnelling at random.
func TestNoSensedFoodNoDigging(t *testing.T) {
	w := mineWorld(24, 24)
	for x := 8; x <= 12; x++ {
		for y := 6; y <= 14; y++ {
			w.Terrain[idx(w, Point{x, y})] = Terrain(3)
		}
	}
	d := w.Spawn("dwarf", Point{6, 10})
	d.Fullness = 1
	for i := 0; i < 20; i++ {
		w.Step()
	}
	// no food anywhere, so it must not have tunnelled into the block
	if d.Pos.X >= 8 {
		t.Errorf("dwarf dug into rock with no food to reach: %v", d.Pos)
	}
	if d.Action == "digging to food" {
		t.Errorf("action = %q with no food sensed", d.Action)
	}
}
