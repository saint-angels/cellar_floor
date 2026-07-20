package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

// haulCfg extends mineCfg with a bagged "hauler" type, a market structure,
// and a Swift Boots speed upgrade. The mineCfg "dwarf" and "miner" stay
// capacity-0, so the legacy direct-gold path keeps working untouched.
func haulCfg() *data.Config {
	cfg := mineCfg()
	cfg.Types["hauler"] = &data.EntityType{
		ID: "hauler", Name: "Hauler", Kind: "fauna", Color: "#fff",
		BiteSize: 2, StomachSize: 10, HungerThreshold: 0,
		Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
		MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100,
		MineDamage: 1, CarryCapacity: 3,
	}
	cfg.Types["market"] = &data.EntityType{
		ID: "market", Name: "Market", Kind: "structure", Color: "#b8860b",
		Market: true, Lifespan: 0,
	}
	cfg.Upgrades = []data.Upgrade{
		{Name: "Swift Boots", Kind: "speed", Amount: 25, Max: 3},
	}
	return cfg
}

// haulWorld is a light-flooded world whose rock drops exactly amt ore per
// strike at the given chance.
func haulWorld(t *testing.T, width, height int, chance float64, amt int) *World {
	t.Helper()
	cfg := haulCfg()
	cfg.Terrain[3].GoldChance = chance
	cfg.Sim.GoldMin, cfg.Sim.GoldMax = amt, amt
	w := NewWorld(width, height, 1, cfg)
	w.Spawn("sunstone", Point{0, 0}) // flood with light
	return w
}

func TestOreAccruesInsteadOfGold(t *testing.T) {
	w := haulWorld(t, 12, 5, 1.0, 3)
	w.Terrain[idx(w, Point{3, 2})] = TerrainRock
	w.Spawn("market", Point{10, 2})
	e := w.Spawn("hauler", Point{2, 2})
	assignFace(e, 3, 2)
	var ore *Event
	for i := 0; i < 40 && ore == nil; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "ore" && ev.Actor == e.ID {
				v := ev
				ore = &v
			}
			if ev.Type == "gold" {
				t.Fatal("bagged miner emitted a gold event")
			}
		}
	}
	if ore == nil {
		t.Fatal("no ore event fired")
	}
	if ore.Amount != 3 {
		t.Fatalf("ore amount = %d, want 3", ore.Amount)
	}
	if e.Ore == 0 {
		t.Fatal("ore not added to the bag")
	}
	if w.Gold != 0 {
		t.Fatalf("gold = %d, ore must not pay at the rock face", w.Gold)
	}
}

func TestBaglessMinerStillPaysGoldDirectly(t *testing.T) {
	w := haulWorld(t, 5, 5, 1.0, 2)
	w.Terrain[idx(w, Point{3, 2})] = TerrainRock
	e := w.Spawn("miner", Point{2, 2}) // capacity 0 in mineCfg
	assignFace(e, 3, 2)
	var goldEv bool
	for i := 0; i < 30; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "gold" && ev.Actor == e.ID {
				goldEv = true
			}
			if ev.Type == "ore" {
				t.Fatal("a capacity-0 miner must not emit ore")
			}
		}
	}
	if !goldEv {
		t.Fatal("no gold event from the bagless miner")
	}
	if w.Gold != 2 {
		t.Fatalf("gold = %d, want 2", w.Gold)
	}
	if e.Ore != 0 {
		t.Fatalf("ore = %d, want 0 for a bagless miner", e.Ore)
	}
}

func TestFullBagHaulsAndDeposits(t *testing.T) {
	w := haulWorld(t, 12, 5, 1.0, 3)
	w.Terrain[idx(w, Point{3, 2})] = TerrainRock
	w.Spawn("market", Point{10, 2})
	e := w.Spawn("hauler", Point{2, 2})
	assignFace(e, 3, 2)
	var sold *Event
	for i := 0; i < 80 && sold == nil; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "sold" && ev.Actor == e.ID {
				v := ev
				sold = &v
			}
		}
	}
	if sold == nil {
		t.Fatal("full bag never reached the market")
	}
	if sold.Amount != 3 {
		t.Fatalf("sold amount = %d, want 3", sold.Amount)
	}
	if w.Gold != 3 {
		t.Fatalf("gold = %d, want 3 after deposit", w.Gold)
	}
	if w.GoldMined != 3 {
		t.Fatalf("goldMined = %d, want 3 (level bar moves at the market)", w.GoldMined)
	}
	if e.Ore != 0 {
		t.Fatalf("ore = %d, want 0 after selling", e.Ore)
	}
	if e.Action != "selling" {
		t.Fatalf("action = %q, want selling", e.Action)
	}
}

func TestPartialBagSoldWhenNothingToMine(t *testing.T) {
	w := haulWorld(t, 12, 5, 1.0, 3)
	// no rock faces at all: mining always returns false
	w.Spawn("market", Point{10, 2})
	e := w.Spawn("hauler", Point{2, 2})
	e.Ore = 2 // a partial bag, below the capacity of 3
	var sold *Event
	for i := 0; i < 80 && sold == nil; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "sold" && ev.Actor == e.ID {
				v := ev
				sold = &v
			}
		}
	}
	if sold == nil {
		t.Fatal("partial bag never sold when no faces remained")
	}
	if sold.Amount != 2 {
		t.Fatalf("sold amount = %d, want 2", sold.Amount)
	}
	if w.Gold != 2 {
		t.Fatalf("gold = %d, want 2", w.Gold)
	}
}

func TestSpeedFactorSpeedsHauling(t *testing.T) {
	run := func(claim bool) int {
		w := haulWorld(t, 20, 5, 0, 0)
		if claim {
			w.Claims = map[string]int{"Swift Boots": 2}
		}
		w.Spawn("market", Point{18, 2})
		e := w.Spawn("hauler", Point{1, 2})
		e.Ore = 3 // full bag: heads straight for the market
		for i := 1; i < 200; i++ {
			for _, ev := range w.Step() {
				if ev.Type == "sold" && ev.Actor == e.ID {
					return i
				}
			}
		}
		t.Fatal("hauler never reached the market")
		return -1
	}
	base := run(false)
	fast := run(true)
	if fast >= base {
		t.Fatalf("speed claims did not speed the trip: fast %d vs base %d", fast, base)
	}
	// the multiplier itself: 25 * 2 claims / 100 = +0.5
	w := haulWorld(t, 5, 5, 0, 0)
	w.Claims = map[string]int{"Swift Boots": 2}
	if sf := w.SpeedFactor(); sf != 1.5 {
		t.Fatalf("SpeedFactor = %v, want 1.5", sf)
	}
}

func TestMigrationSpawnsOneMarket(t *testing.T) {
	cfg := haulCfg()
	w := NewWorld(11, 11, 1, cfg)
	w.Spawn("campfire", Point{5, 5}) // the clearing center, no market yet
	countMarket := func() int {
		n := 0
		for _, e := range w.Entities {
			if !e.Dead && w.cfg.Types[e.Type].Market {
				n++
			}
		}
		return n
	}
	if countMarket() != 0 {
		t.Fatal("precondition: an old save has no market")
	}
	w.SetConfig(cfg)
	if got := countMarket(); got != 1 {
		t.Fatalf("markets after load = %d, want 1", got)
	}
	w.SetConfig(cfg) // a second load must not duplicate
	if got := countMarket(); got != 1 {
		t.Fatalf("markets after reload = %d, want 1", got)
	}
	for _, e := range w.Entities {
		if !w.cfg.Types[e.Type].Market {
			continue
		}
		if !w.Passable(w.At(e.Pos)) {
			t.Fatalf("market spawned on an impassable tile %v", e.Pos)
		}
		if Dist(e.Pos, Point{5, 5}) > 2 {
			t.Fatalf("market %v not next to the campfire", e.Pos)
		}
	}
}

func TestOreSurvivesSaveLoad(t *testing.T) {
	w := haulWorld(t, 5, 5, 0, 0)
	e := w.Spawn("hauler", Point{2, 2})
	e.Ore = 2
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(haulCfg())
	e2 := w2.Entities[e.ID]
	if e2 == nil {
		t.Fatal("hauler lost across save/load")
	}
	if e2.Ore != 2 {
		t.Fatalf("ore = %d after round-trip, want 2", e2.Ore)
	}
}
