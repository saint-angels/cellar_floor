package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

// torchLifespan is short so light-death tests run quickly.
const torchLifespan = 10

// Fast-mining config: speed 1 (a step per tick), 10 ticks per cell.
func mineCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Types: map[string]*data.EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.01}}},
			"dwarf": {ID: "dwarf", Name: "Dwarf", Kind: "fauna", Color: "#fff",
				Eats: []string{"shroom"}, BiteSize: 2, StomachSize: 10, HungerThreshold: 4,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100,
				MineTicks: 10},
			// Slow-mining miner used by the lit-face gate tests: MineTicks is
			// high so a face never mines out before a torch burns dark.
			"miner": {ID: "miner", Name: "Miner", Kind: "fauna", Color: "#fff",
				BiteSize: 2, StomachSize: 10, HungerThreshold: 0,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100, MineTicks: 100},
			"torch": {ID: "torch", Name: "Torch", Kind: "structure", Color: "#ffb347",
				LightRadius: 3, Lifespan: torchLifespan, DecayTicks: 5},
			// A pinpoint campfire lights only the miner's own cell, keeping it
			// from fleeing the dark while leaving rock faces unlit.
			"campfire": {ID: "campfire", Name: "Campfire", Kind: "structure", Color: "#e25822",
				LightRadius: 1, Lifespan: 0},
			// A wide light that floods the whole small test world so plain
			// mining tests find their faces lit.
			"sunstone": {ID: "sunstone", Name: "Sunstone", Kind: "structure", Color: "#fff8dc",
				LightRadius: 30, Lifespan: 0},
		},
	}
}

func mineWorld(w, h int) *World {
	world := NewWorld(w, h, 1, mineCfg())
	world.Spawn("sunstone", Point{0, 0}) // flood the play area with light
	return world
}

// newMineWorldDark builds a world whose only rock face is unlit. Water walls
// box the face in so the sole approach cell is {3,2}, which the pinpoint
// campfire lights; that keeps the miner lit (never fleeing) while the face
// itself stays dark unless a torch is added.
func newMineWorldDark(t *testing.T) *World {
	t.Helper()
	w := NewWorld(20, 20, 1, mineCfg())
	w.Terrain[idx(w, Point{4, 2})] = TerrainRock // the sole face, dark by default
	// wall every neighbor of the face except the lit approach cell {3,2}
	for _, n := range neighbors {
		p := Point{4 + n.X, 2 + n.Y}
		if p == (Point{3, 2}) {
			continue
		}
		w.Terrain[idx(w, p)] = TerrainWater
	}
	w.Spawn("campfire", Point{2, 2}) // lights the approach cell {3,2}, not the face
	return w
}

func idx(w *World, p Point) int { return p.Y*w.Width + p.X }

// goldDropWorld builds a lit 5x5 world with one rock face beside the spawn
// point {2,2} and a miner that mines a face out within the test's step budget.
func goldDropWorld(t *testing.T, chance float64, lo, hi int) *World {
	t.Helper()
	cfg := mineCfg()
	cfg.Sim.GoldChance = chance
	cfg.Sim.GoldMin = lo
	cfg.Sim.GoldMax = hi
	cfg.Types["miner"].MineTicks = 10
	w := NewWorld(5, 5, 1, cfg)
	w.Spawn("sunstone", Point{0, 0})             // flood the world with light
	w.Terrain[idx(w, Point{3, 2})] = TerrainRock // the sole face, beside {2,2}
	return w
}

// newMineWorld guarantees a drop of exactly 2 gold per mined rock.
func newMineWorld(t *testing.T) *World { return goldDropWorld(t, 1.0, 2, 2) }

// newMineWorldNoGold is newMineWorld with the drop chance set to zero.
func newMineWorldNoGold(t *testing.T) *World { return goldDropWorld(t, 0, 1, 3) }

func TestMinedRockRollsGoldDrop(t *testing.T) {
	w := newMineWorld(t) // gold_chance 1.0, gold_min 2, gold_max 2 in this test's cfg
	e := w.Spawn("miner", Point{2, 2})
	var goldEv bool
	for i := 0; i < 30; i++ { // mine_ticks 10 in test cfg plus walking
		for _, ev := range w.Step() {
			if ev.Type == "gold" && ev.Actor == e.ID {
				goldEv = true
			}
		}
	}
	if w.Gold != 2 {
		t.Fatalf("gold = %d, want 2", w.Gold)
	}
	if !goldEv {
		t.Fatal("expected a struck gold event")
	}
}

func TestNoDropFiresMinedEvent(t *testing.T) {
	w := newMineWorldNoGold(t) // same but gold_chance 0
	e := w.Spawn("miner", Point{2, 2})
	var mined bool
	for i := 0; i < 30; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "mined" && ev.Actor == e.ID {
				mined = true
			}
		}
	}
	if w.Gold != 0 {
		t.Fatalf("gold = %d, want 0", w.Gold)
	}
	if !mined {
		t.Fatal("expected a mined out a rock event")
	}
}

func TestDwarfMinesAdjacentRock(t *testing.T) {
	w := mineWorld(5, 5)
	rock := Point{3, 2}
	w.Terrain[idx(w, rock)] = TerrainRock
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10

	w.Step()
	if d.Action != "mining" {
		t.Fatalf("action = %q, want mining", d.Action)
	}
	if p := w.MineProgress[idx(w, rock)]; p < 0.09 || p > 0.11 {
		t.Fatalf("progress = %v, want ~0.1", p)
	}
	var events []Event
	for i := 0; i < 12 && w.At(rock) != TerrainFloor; i++ {
		events = append(events, w.Step()...)
	}
	if w.At(rock) != TerrainFloor {
		t.Fatal("rock never became floor")
	}
	if _, ok := w.MineProgress[idx(w, rock)]; ok {
		t.Error("progress not cleared on completion")
	}
	if w.Gold != 0 {
		t.Error("plain rock must not add gold")
	}
	mined := false
	for _, ev := range events {
		if ev.Type == "mined" {
			mined = true
		}
	}
	if !mined {
		t.Error("no mined event")
	}
	if len(w.TerrainDirtyAndReset()) == 0 {
		t.Error("terrain change not in dirty set")
	}
}

func TestBFSRoutesAroundObstacles(t *testing.T) {
	w := mineWorld(9, 9)
	for x := 0; x < 8; x++ {
		w.Terrain[idx(w, Point{x, 4})] = TerrainWater // wall with gap at x=8
	}
	rock := Point{2, 6} // below the wall; the only face
	w.Terrain[idx(w, rock)] = TerrainRock
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	for i := 0; i < 40 && w.At(rock) != TerrainFloor; i++ {
		w.Step()
	}
	if w.At(rock) != TerrainFloor {
		t.Fatalf("dwarf never routed around the wall; at %v action %q", d.Pos, d.Action)
	}
}

func TestOneDwarfPerFace(t *testing.T) {
	w := mineWorld(5, 5)
	rock := Point{2, 1}
	w.Terrain[idx(w, rock)] = TerrainRock
	a := w.Spawn("dwarf", Point{1, 1})
	a.Fullness = 10
	b := w.Spawn("dwarf", Point{3, 1})
	b.Fullness = 10
	w.Step()
	if a.MineTarget == nil {
		t.Fatal("first dwarf has no target")
	}
	if b.MineTarget != nil && *b.MineTarget == *a.MineTarget {
		t.Error("both dwarves claimed the same face")
	}
}

func TestHungryDwarfEatsThenResumesMining(t *testing.T) {
	w := mineWorld(6, 5)
	rock := Point{4, 2}
	w.Terrain[idx(w, rock)] = TerrainRock
	w.Spawn("shroom", Point{1, 2})
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 1 // below hunger threshold 4
	w.Step()
	if d.Action == "mining" || d.Action == "heading to mine" {
		t.Fatalf("hungry dwarf mined instead of eating: %q", d.Action)
	}
	for i := 0; i < 30 && d.Action != "mining"; i++ {
		w.Step()
	}
	if d.Action != "mining" {
		t.Fatalf("dwarf never resumed mining, action %q fullness %v", d.Action, d.Fullness)
	}
}

func TestMineStateSurvivesSaveLoad(t *testing.T) {
	w := mineWorld(5, 5)
	rock := Point{3, 2}
	w.Terrain[idx(w, rock)] = TerrainRock
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	for i := 0; i < 3; i++ {
		w.Step()
	}
	w.Gold = 7
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(mineCfg())
	if w2.Gold != 7 {
		t.Errorf("gold lost: %d", w2.Gold)
	}
	if w2.MineProgress[idx(w, rock)] != w.MineProgress[idx(w, rock)] {
		t.Errorf("progress lost: %v vs %v", w2.MineProgress, w.MineProgress)
	}
	e2 := w2.Entities[d.ID]
	if e2.MineTarget == nil || *e2.MineTarget != rock {
		t.Errorf("mine target lost: %v", e2.MineTarget)
	}
}

func TestGoldWindowTracksLast24h(t *testing.T) {
	w := newMineWorld(t) // chance 1.0, drop exactly 2
	e := w.Spawn("miner", Point{2, 2})
	for i := 0; i < 30; i++ {
		w.Step()
	}
	if got := w.GoldLast24h(e); got != 2 {
		t.Fatalf("gold last 24h = %d, want 2", got)
	}
	if len(e.GoldStrikes) != 1 {
		t.Fatalf("strikes = %d, want 1", len(e.GoldStrikes))
	}
	// push the strike out of the window: 24h at tick_rate 2 is 172800 ticks
	w.Tick += 172801
	if got := w.GoldLast24h(e); got != 0 {
		t.Fatalf("stale gold still counted: %d", got)
	}
	if len(e.GoldStrikes) != 0 {
		t.Fatal("stale strikes must be pruned")
	}
}

func TestOnlyLitFacesPicked(t *testing.T) {
	// world where the only reachable rock face is dark: no target
	w := newMineWorldDark(t) // rock faces exist, zero light sources
	e := w.Spawn("miner", Point{2, 2})
	w.Step()
	if e.MineTarget != nil {
		t.Fatal("no face is lit, no target may be picked")
	}
}

func TestTargetDroppedWhenLightDies(t *testing.T) {
	w := newMineWorldDark(t)
	w.Spawn("torch", Point{3, 2}) // lights the face at {4,2}
	e := w.Spawn("miner", Point{2, 2})
	w.Step()
	if e.MineTarget == nil {
		t.Fatal("lit face should be picked")
	}
	// kill the torch (age it out), face goes dark
	for i := 0; i < torchLifespan+1; i++ {
		w.Step()
	}
	if e.MineTarget != nil {
		t.Fatal("target must be dropped when its face goes dark")
	}
}
