package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// marketCfg extends mineCfg with a market structure and a Swift Boots speed
// upgrade. The market survives as a structure (worldgen and save migration
// keep placing it) even though ore hauling is gone: all mined gold pays at
// the rock face now, instantly.
func marketCfg() *data.Config {
	cfg := mineCfg()
	cfg.Types["market"] = &data.EntityType{
		ID: "market", Name: "Market", Kind: "structure", Color: "#b8860b",
		Market: true, Lifespan: 0,
	}
	cfg.Upgrades = []data.Upgrade{
		{Name: "Swift Boots", Kind: "speed", Amount: 25, Max: 3},
	}
	return cfg
}

// Claimed speed upgrades multiply every walk. A miner crossing the map to an
// assigned face arrives measurably sooner with Swift Boots claimed.
func TestSpeedFactorSpeedsWalking(t *testing.T) {
	run := func(claim bool) int {
		w := NewWorld(20, 5, 1, marketCfg())
		w.Spawn("sunstone", Point{0, 0})
		w.Terrain[idx(w, Point{18, 2})] = Terrain(6) // hardish: never mines out
		e := w.Spawn("miner", Point{1, 2})
		assignFace(e, 18, 2)
		if claim {
			w.Claims = map[string]int{"Swift Boots": 2}
		}
		for i := 1; i < 200; i++ {
			w.Step()
			if e.Action == "mining" {
				return i
			}
		}
		t.Fatal("miner never reached the face")
		return -1
	}
	base := run(false)
	fast := run(true)
	if fast >= base {
		t.Fatalf("speed claims did not speed the walk: fast %d vs base %d", fast, base)
	}
	// the multiplier itself: 25 * 2 claims / 100 = +0.5
	w := NewWorld(5, 5, 1, marketCfg())
	w.Claims = map[string]int{"Swift Boots": 2}
	if sf := w.SpeedFactor(); sf != 1.5 {
		t.Fatalf("SpeedFactor = %v, want 1.5", sf)
	}
}

func TestMigrationSpawnsOneMarket(t *testing.T) {
	cfg := marketCfg()
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
