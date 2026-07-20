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

// While tunnelling toward buried food the dwarf must stay committed to that
// food entity (TargetID), even on the ticks it spends breaking rock — the
// client draws a persistent line from the digger to its committed food, so a
// flicker to 0 mid-dig would break it.
func TestBuriedDigKeepsFoodTarget(t *testing.T) {
	w := mineWorld(24, 24)
	for x := 8; x <= 12; x++ {
		for y := 6; y <= 14; y++ {
			w.Terrain[idx(w, Point{x, y})] = Terrain(3) // rock
		}
	}
	food := w.Spawn("shroom", Point{10, 10})
	d := w.Spawn("dwarf", Point{6, 10})
	d.Fullness = 1

	// step until the dwarf is actively mining a rock face toward the food
	mined := false
	for i := 0; i < 400 && !mined; i++ {
		w.Step()
		if d.Action == "mining" {
			mined = true
		}
	}
	if !mined {
		t.Fatalf("dwarf never started mining toward the buried food: %v", d.Action)
	}
	if d.TargetID != food.ID {
		t.Fatalf("TargetID = %d while mining toward food, want the food id %d", d.TargetID, food.ID)
	}
}

// Greedy eating: a completely FULL dwarf still pursues and eats a beacon in
// range, and a non-regrowing food eaten clean dies and is removed — food is a
// consumable command token, never a stockpile.
func TestFullDwarfGreedilyConsumesFood(t *testing.T) {
	w := mineWorld(12, 8)
	f := w.Spawn("shroom", Point{6, 4})
	f.Produces[0].Regrow = 0 // a planted token, not a regrowing patch
	d := w.Spawn("dwarf", Point{4, 4})
	d.Fullness = 10 // completely full: the old rules would ignore the food
	consumed := false
	for i := 0; i < 20 && !consumed; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "consumed" && ev.Actor == f.ID {
				consumed = true
			}
		}
	}
	if !consumed {
		t.Fatalf("full dwarf never ate the token clean: %.2f left, dwarf %q at %v",
			f.Produces[0].Amount, d.Action, d.Pos)
	}
	w.Step() // decay 0: the husk is swept the next tick
	if _, ok := w.Entities[f.ID]; ok {
		t.Error("consumed food never removed from the world")
	}
}

// Two equidistant beacons must not tug an eater back and forth one step each
// way: once committed, a dwarf finishes its chosen mushroom before the other
// one may pull it (sticky food commitment).
func TestCommittedFoodIsFinishedFirst(t *testing.T) {
	w := mineWorld(12, 8)
	a := w.Spawn("shroom", Point{2, 4})
	b := w.Spawn("shroom", Point{6, 4})
	a.Produces[0].Regrow = 0
	b.Produces[0].Regrow = 0
	d := w.Spawn("dwarf", Point{4, 4}) // exactly between the two
	d.Fullness = 10

	first := 0
	for i := 0; i < 60; i++ {
		w.Step()
		if first == 0 {
			first = d.TargetID
			continue
		}
		fe := w.Entities[first]
		if fe == nil || fe.Dead {
			break // the committed one is finished
		}
		if d.TargetID != first {
			t.Fatalf("step %d: dwarf switched from %d to %d before finishing its meal", i, first, d.TargetID)
		}
	}
	if first == 0 {
		t.Fatal("dwarf never committed to a mushroom")
	}
	// and the other one is eaten next, not abandoned
	other := a
	if first == a.ID {
		other = b
	}
	for i := 0; i < 80 && !other.Dead; i++ {
		w.Step()
	}
	if !other.Dead {
		t.Fatalf("second mushroom never finished: %.2f left", other.Produces[0].Amount)
	}
}

// The beacon model: sensing range is a property of the FOOD, not the eater. A
// hungry dwarf beyond a food's sense_radius never pursues it — even across
// open, walkable ground — while one inside the radius goes and eats.
func TestFoodBeaconRadiusGatesPursuit(t *testing.T) {
	w := mineWorld(30, 8) // all-grass, sunstone-lit: pure open ground
	w.Cfg().Types["shroom"].SenseRadius = 3
	w.Spawn("shroom", Point{25, 4})

	far := w.Spawn("dwarf", Point{4, 4}) // Dist 21: far outside the beacon
	far.Fullness = 1
	for i := 0; i < 8; i++ {
		w.Step()
		if far.Action == "seeking food" || far.Action == "eating" {
			t.Fatalf("dwarf pursued food from outside its beacon (step %d)", i)
		}
	}
	if far.Fullness > 1 {
		t.Fatalf("dwarf ate food it could not sense: fullness %.2f", far.Fullness)
	}

	w2 := mineWorld(30, 8)
	w2.Cfg().Types["shroom"].SenseRadius = 3
	w2.Spawn("shroom", Point{25, 4})
	near := w2.Spawn("dwarf", Point{23, 4}) // Dist 2: inside the beacon
	near.Fullness = 1
	for i := 0; i < 10 && near.Fullness <= 1; i++ {
		w2.Step()
	}
	if near.Fullness <= 1 {
		t.Fatalf("dwarf inside the beacon never ate: action %q", near.Action)
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
