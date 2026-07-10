# Idle Pivot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the rabbit/wolf ecology with an underground idle world: dwarves mine mostly-rock terrain (~1 real day per cell), sense buried gold, and feed a global gold counter, with per-cell progress bars in the client.

**Architecture:** The sim gains two terrain types (`floor`, `gold`), terrain mutation with dirty tracking, and a mining AI step in a new `internal/sim/mine.go` (BFS pathfinding, face claiming via `Entity.MineTarget`). The generator gains an underground branch (all rock + dirt clearing + gold scatter). Protocol tick/snapshot messages carry terrain diffs, active mining progress, and the gold counter. Existing engine regression tests keep the old rabbit/wolf data as a fixture in `internal/sim/testdata/legacy/` so shipped `data/` can flip to mushroom+dwarf without losing engine coverage.

**Tech Stack:** Go 1.26 stdlib, BurntSushi/toml (existing), TypeScript/Vite client (existing). No new dependencies.

## Global Constraints

- "Most things happen on a scale of hours or real days" at `tick_rate = 2.0` (172,800 ticks per real day); all tuning numbers live in TOML, not code (spec).
- Dwarf numbers (spec): `metabolism ~0.00006`, `speed ~0.004`, `starve_ticks ~350000`, `pop_floor = 3`, `repro_chance = 0`, `mine_ticks = 172800`, `gold_sense = 8`.
- Rock mining yields nothing; only gold increments the global counter (spec).
- Terrain enum keeps existing values; `floor` (4) and `gold` (5) are appended (spec).
- One dwarf per face; progress persists in `world.json` and survives interruptions (spec).
- Simulation must stay deterministic per seed (existing `TestWorldDeterminism`): iterate maps in sorted order anywhere results feed decisions.
- Commit messages: one sentence, under 70 characters, no Claude attribution (user CLAUDE.md).

---

### Task 1: Terrain types floor and gold, terrain mutation with dirty tracking

**Files:**
- Modify: `internal/sim/world.go:9-21` (terrain block), `internal/sim/world.go:43-60` (World struct)
- Modify: `internal/data/data.go:79` (validTerrains)
- Test: `internal/sim/terrain_test.go` (create)

**Interfaces:**
- Produces: `TerrainFloor`, `TerrainGold` constants; `Mineable(t Terrain) bool`; `Passable` now true for `TerrainFloor`; `(w *World) SetTerrain(p Point, t Terrain)`; `(w *World) TerrainDirtyAndReset() []int` (cell indexes `y*Width+x` changed since last call).

- [ ] **Step 1: Write the failing test**

Create `internal/sim/terrain_test.go`:

```go
package sim

import "testing"

func TestTerrainTypesAndMutation(t *testing.T) {
	if !Passable(TerrainFloor) || Passable(TerrainGold) || Passable(TerrainRock) {
		t.Error("passability wrong")
	}
	if !Mineable(TerrainRock) || !Mineable(TerrainGold) || Mineable(TerrainDirt) || Mineable(TerrainFloor) {
		t.Error("mineability wrong")
	}
	if TerrainName(TerrainFloor) != "floor" || TerrainName(TerrainGold) != "gold" {
		t.Error("terrain names wrong")
	}

	w := flatWorld(t, 4, 4, 1)
	w.SetTerrain(Point{2, 1}, TerrainFloor)
	w.SetTerrain(Point{2, 1}, TerrainFloor) // no-op, already floor
	w.SetTerrain(Point{3, 3}, TerrainGold)
	d := w.TerrainDirtyAndReset()
	if len(d) != 2 || d[0] != 1*4+2 || d[1] != 3*4+3 {
		t.Errorf("dirty = %v, want [6 15]", d)
	}
	if w.At(Point{2, 1}) != TerrainFloor {
		t.Error("SetTerrain did not apply")
	}
	if len(w.TerrainDirtyAndReset()) != 0 {
		t.Error("dirty set not reset")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -run TestTerrainTypesAndMutation -v`
Expected: FAIL to compile ("undefined: TerrainFloor")

- [ ] **Step 3: Implement**

In `internal/sim/world.go` replace the terrain block:

```go
const (
	TerrainGrass Terrain = iota
	TerrainDirt
	TerrainWater
	TerrainRock
	TerrainFloor // mined-out stone
	TerrainGold  // gold vein, mineable like rock
)

var terrainNames = [...]string{"grass", "dirt", "water", "rock", "floor", "gold"}

func TerrainName(t Terrain) string { return terrainNames[t] }
func Passable(t Terrain) bool {
	return t == TerrainGrass || t == TerrainDirt || t == TerrainFloor
}
func Mineable(t Terrain) bool { return t == TerrainRock || t == TerrainGold }
```

Add to the World struct (unexported, next to `dirty`):

```go
	terrainDirty []int
```

Add methods (below `At`):

```go
// SetTerrain mutates a cell and records it for the next tick's terrain diff.
func (w *World) SetTerrain(p Point, t Terrain) {
	i := p.Y*w.Width + p.X
	if w.Terrain[i] == t {
		return
	}
	w.Terrain[i] = t
	w.terrainDirty = append(w.terrainDirty, i)
}

// TerrainDirtyAndReset returns cell indexes changed since the last call.
func (w *World) TerrainDirtyAndReset() []int {
	d := w.terrainDirty
	w.terrainDirty = nil
	return d
}
```

In `internal/data/data.go` update:

```go
var validTerrains = map[string]bool{"grass": true, "dirt": true, "water": true, "rock": true, "floor": true, "gold": true}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sim/ -run TestTerrainTypesAndMutation -v && go test ./... -short && go vet ./...`
Expected: PASS, all packages ok

- [ ] **Step 5: Commit**

```bash
git add internal/sim/world.go internal/sim/terrain_test.go internal/data/data.go
git commit -m "Add floor and gold terrain with mutation tracking"
```

---

### Task 2: Species and gen config fields for mining

**Files:**
- Modify: `internal/data/data.go:23-48` (Species), `internal/data/data.go:62-71` (GenConfig), `Validate`
- Test: `internal/data/data_test.go` (append)

**Interfaces:**
- Produces: `Species.MineTicks int` (`mine_ticks`/`mineTicks`), `Species.GoldSense int` (`gold_sense`/`goldSense`), `GenConfig.ClearingRadius int` (`clearing_radius`), `GenConfig.GoldChance float64` (`gold_chance`).

- [ ] **Step 1: Write the failing test**

Append to `internal/data/data_test.go` (it has a helper writing temp TOML dirs; follow its existing pattern — check the file first; if it loads the real `data/` dir instead, write this as a temp-dir test):

```go
func TestMiningFieldsParse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sim.toml", "tick_rate = 2.0\nautosave_minutes = 0\nsave_path = \"w.json\"\n")
	write("gen.toml", "width = 8\nheight = 8\nclearing_radius = 3\ngold_chance = 0.01\nscatter = []\n")
	write("species.toml", `
[species.shroom]
name = "Shroom"
kind = "flora"
color = "#fff"
produces = [{ resource = "shroom", amount = 6, max = 6, regrow = 0.001 }]

[species.dwarf]
name = "Dwarf"
kind = "fauna"
color = "#d9a066"
eats = ["shroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
metabolism = 0.0001
starve_ticks = 1000
speed = 0.5
lifespan = 100000
pop_cap = 10
decay_ticks = 100
mine_ticks = 500
gold_sense = 8
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Species["dwarf"]
	if d.MineTicks != 500 || d.GoldSense != 8 {
		t.Errorf("mining fields: %d %d", d.MineTicks, d.GoldSense)
	}
	if cfg.Gen.ClearingRadius != 3 || cfg.Gen.GoldChance != 0.01 {
		t.Errorf("gen fields: %d %v", cfg.Gen.ClearingRadius, cfg.Gen.GoldChance)
	}
}
```

Add `"os"` and `"path/filepath"` to imports if missing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/data/ -run TestMiningFieldsParse -v`
Expected: FAIL to compile ("d.MineTicks undefined")

- [ ] **Step 3: Implement**

Add to `Species` (after `DecayTicks`):

```go
	MineTicks       int       `toml:"mine_ticks" json:"mineTicks"`
	GoldSense       int       `toml:"gold_sense" json:"goldSense"`
```

Add to `GenConfig` (after `RockAbove`):

```go
	ClearingRadius int     `toml:"clearing_radius"`
	GoldChance     float64 `toml:"gold_chance"`
```

In `Validate`, inside the fauna branch, add:

```go
			if s.MineTicks < 0 || s.GoldSense < 0 {
				return fmt.Errorf("species %s: mine_ticks and gold_sense must be non-negative", id)
			}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/data/ -v && go test ./... -short`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/data/data.go internal/data/data_test.go
git commit -m "Add mining fields to species and gen config"
```

---

### Task 3: Legacy data fixture for engine regression tests

The shipped `data/` flips to mushroom+dwarf in Task 7. Engine tests that exercise eating, fleeing, hunting, reproduction, and stability reference rabbits and wolves, which stay valid engine behaviors. Freeze today's data as a fixture and point those tests at it.

**Files:**
- Create: `internal/sim/testdata/legacy/sim.toml`, `internal/sim/testdata/legacy/gen.toml`, `internal/sim/testdata/legacy/species.toml` (copies of current `data/*.toml`)
- Modify: `internal/sim/world_test.go:12-18` (`testCfg`), `internal/sim/longrun_test.go:13-21` (`loadCfg`)

**Interfaces:**
- Consumes: current `data/*.toml` content.
- Produces: `testCfg(t)` and `loadCfg(t)` load `internal/sim/testdata/legacy` instead of `data/`. All rabbit/wolf-based sim tests (`ai_test`, `predation_test`, `shelter_test`, `tick_test`, `world_test`, `occ_test`, `longrun_test`) keep passing after Task 7.

- [ ] **Step 1: Copy the fixture**

```bash
mkdir -p internal/sim/testdata/legacy
cp data/sim.toml data/gen.toml data/species.toml internal/sim/testdata/legacy/
```

- [ ] **Step 2: Repoint the helpers**

In `internal/sim/world_test.go`, `testCfg`:

```go
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "testdata", "legacy"))
```

In `internal/sim/longrun_test.go`, `loadCfg`:

```go
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "testdata", "legacy"))
```

- [ ] **Step 3: Run the sim tests including the long run**

Run: `go test ./internal/sim/`
Expected: PASS (identical data, just a different path)

- [ ] **Step 4: Commit**

```bash
git add internal/sim/testdata internal/sim/world_test.go internal/sim/longrun_test.go
git commit -m "Freeze rabbit ecology as legacy fixture for engine tests"
```

---

### Task 4: Underground map generator

**Files:**
- Modify: `internal/gen/gen.go:41-62` (terrain section of `Generate`)
- Test: `internal/gen/underground_test.go` (create)

**Interfaces:**
- Consumes: `GenConfig.ClearingRadius`, `GenConfig.GoldChance` (Task 2), `sim.TerrainFloor`/`TerrainGold` (Task 1), `w.RandFloat()`.
- Produces: when `ClearingRadius > 0`, `Generate` emits all-rock terrain with a circular dirt clearing at the map center and gold cells scattered at `GoldChance` per rock cell; the noise path is unchanged when `ClearingRadius == 0`. Scatter rules run identically in both modes.

- [ ] **Step 1: Write the failing test**

Create `internal/gen/underground_test.go`:

```go
package gen

import (
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func undergroundCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Gen: data.GenConfig{
			Width: 32, Height: 32,
			ClearingRadius: 4, GoldChance: 0.01,
			Scatter: []data.ScatterRule{{Species: "shroom", Terrain: "dirt", Chance: 0.3}},
		},
		Species: map[string]*data.Species{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.001}}},
		},
	}
}

func TestUndergroundGeneration(t *testing.T) {
	cfg := undergroundCfg()
	w := Generate(42, cfg)
	counts := map[sim.Terrain]int{}
	for _, tr := range w.Terrain {
		counts[tr]++
	}
	if counts[sim.TerrainRock] < 32*32*6/10 {
		t.Errorf("map not mostly rock: %v", counts)
	}
	if counts[sim.TerrainGold] == 0 {
		t.Error("no gold generated")
	}
	if counts[sim.TerrainFloor] != 0 || counts[sim.TerrainWater] != 0 || counts[sim.TerrainGrass] != 0 {
		t.Errorf("unexpected terrain in underground map: %v", counts)
	}
	center := sim.Point{X: 16, Y: 16}
	if w.At(center) != sim.TerrainDirt {
		t.Error("clearing center is not dirt")
	}
	if w.At(sim.Point{X: 0, Y: 0}) == sim.TerrainDirt {
		t.Error("corner should not be clearing")
	}
	shrooms := 0
	for _, e := range w.Entities {
		if e.Species == "shroom" {
			shrooms++
			if w.At(e.Pos) != sim.TerrainDirt {
				t.Error("shroom outside clearing")
			}
		}
	}
	if shrooms == 0 {
		t.Error("no shrooms scattered in clearing")
	}
	// determinism
	b := Generate(42, cfg)
	for i := range w.Terrain {
		if w.Terrain[i] != b.Terrain[i] {
			t.Fatal("underground gen not deterministic")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gen/ -run TestUndergroundGeneration -v`
Expected: FAIL ("map not mostly rock" — the noise path runs because the underground branch does not exist)

- [ ] **Step 3: Implement**

In `Generate`, wrap the terrain loop in a branch:

```go
	if g.ClearingRadius > 0 {
		cx, cy := g.Width/2, g.Height/2
		r2 := g.ClearingRadius * g.ClearingRadius
		for y := 0; y < g.Height; y++ {
			for x := 0; x < g.Width; x++ {
				dx, dy := x-cx, y-cy
				var t sim.Terrain
				switch {
				case dx*dx+dy*dy <= r2:
					t = sim.TerrainDirt
				case w.RandFloat() < g.GoldChance:
					t = sim.TerrainGold
				default:
					t = sim.TerrainRock
				}
				w.Terrain[y*g.Width+x] = t
			}
		}
	} else {
		for y := 0; y < g.Height; y++ {
			for x := 0; x < g.Width; x++ {
				v := fractal(seed, x, y, g.NoiseScale, g.NoiseOctaves)
				var t sim.Terrain
				switch {
				case v < g.WaterBelow:
					t = sim.TerrainWater
				case v > g.RockAbove:
					t = sim.TerrainRock
				case v > g.DirtAbove:
					t = sim.TerrainDirt
				default:
					t = sim.TerrainGrass
				}
				w.Terrain[y*g.Width+x] = t
			}
		}
	}
```

The scatter loop below stays unchanged.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/gen/ -v && go test ./... -short`
Expected: PASS (old gen tests still use the noise path since `data/gen.toml` has no `clearing_radius` yet)

- [ ] **Step 5: Commit**

```bash
git add internal/gen/gen.go internal/gen/underground_test.go
git commit -m "Add underground map generation with clearing and gold"
```

---

### Task 5: Mining AI with BFS pathfinding, gold sense, and the gold counter

**Files:**
- Create: `internal/sim/mine.go`
- Modify: `internal/sim/world.go` (Entity + World fields, `NewWorld`, `SetConfig`), `internal/sim/ai.go:56-58` (aiStep hook), `internal/sim/tick.go:195-213` (`randomEdgeTile` → any free tile)
- Test: `internal/sim/mine_test.go` (create)

**Interfaces:**
- Consumes: `Mineable`, `SetTerrain` (Task 1), `Species.MineTicks`/`GoldSense` (Task 2), existing `neighbors`, `adjacent`, `Dist`, `markDirty`, `occ`.
- Produces: `Entity.MineTarget *Point` (`json:"mineTarget,omitempty"`); `World.Gold int` (`json:"gold"`); `World.MineProgress map[int]float64` (`json:"mineProgress"`); `(w *World) mineStep(e *Entity) ([]Event, bool)` called from `aiStep`; events with `Type: "gold"` and `Type: "mined"`. Dwarf actions: `"mining"`, `"heading to mine"`.

- [ ] **Step 1: Add the state fields (no test yet, they are inert)**

In `internal/sim/world.go`, add to `Entity` after `MoveAcc`:

```go
	MineTarget  *Point         `json:"mineTarget,omitempty"`
```

Add to `World` after `Rng`:

```go
	Gold         int             `json:"gold"`
	MineProgress map[int]float64 `json:"mineProgress,omitempty"`
```

In `NewWorld`, add `MineProgress: map[int]float64{},` to the literal. In `SetConfig`, add:

```go
	if w.MineProgress == nil {
		w.MineProgress = map[int]float64{}
	}
```

- [ ] **Step 2: Write the failing tests**

Create `internal/sim/mine_test.go`:

```go
package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

// Fast-mining config: speed 1 (a step per tick), 10 ticks per cell.
func mineCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Species: map[string]*data.Species{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.01}}},
			"dwarf": {ID: "dwarf", Name: "Dwarf", Kind: "fauna", Color: "#fff",
				Eats: []string{"shroom"}, BiteSize: 2, StomachSize: 10, HungerThreshold: 4,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100,
				MineTicks: 10, GoldSense: 4},
		},
	}
}

func mineWorld(w, h int) *World {
	return NewWorld(w, h, 1, mineCfg())
}

func idx(w *World, p Point) int { return p.Y*w.Width + p.X }

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

func TestGoldAddsToCounter(t *testing.T) {
	w := mineWorld(5, 5)
	g := Point{3, 2}
	w.Terrain[idx(w, g)] = TerrainGold
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	var events []Event
	for i := 0; i < 15 && w.Gold == 0; i++ {
		events = append(events, w.Step()...)
	}
	if w.Gold != 1 {
		t.Fatalf("gold = %d, want 1", w.Gold)
	}
	struck := false
	for _, ev := range events {
		if ev.Type == "gold" {
			struck = true
		}
	}
	if !struck {
		t.Error("no gold event")
	}
}

func TestGoldSenseBeatsNearerRock(t *testing.T) {
	w := mineWorld(9, 5)
	near := Point{1, 2}  // rock 1 tile from dwarf
	gold := Point{5, 2}  // gold 3 tiles away, within sense 4
	w.Terrain[idx(w, near)] = TerrainRock
	w.Terrain[idx(w, gold)] = TerrainGold
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	w.Step()
	if d.MineTarget == nil || *d.MineTarget != gold {
		t.Fatalf("target = %v, want %v", d.MineTarget, gold)
	}
}

func TestGoldBiasDigsTowardBuriedGold(t *testing.T) {
	w := mineWorld(9, 7)
	for y := 0; y < 7; y++ {
		for x := 5; x < 9; x++ {
			w.Terrain[idx(w, Point{x, y})] = TerrainRock // solid rock mass
		}
	}
	gold := Point{6, 3}
	w.Terrain[idx(w, gold)] = TerrainGold // buried one cell deep
	d := w.Spawn("dwarf", Point{4, 3})
	d.Fullness = 10
	w.Step()
	want := Point{5, 3} // wall face closest to the gold
	if d.MineTarget == nil || *d.MineTarget != want {
		t.Fatalf("target = %v, want %v", d.MineTarget, want)
	}
}

func TestBFSRoutesAroundObstacles(t *testing.T) {
	w := mineWorld(9, 9)
	for x := 0; x < 8; x++ {
		w.Terrain[idx(w, Point{x, 4})] = TerrainWater // wall with gap at x=8
	}
	rock := Point{2, 6} // below the wall; gold sense 4 cannot see it, but it is the only face
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
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/sim/ -run 'TestDwarf|TestGold|TestBFS|TestOneDwarf|TestHungry|TestMineState' -v`
Expected: FAIL (dwarf never mines; actions stay "idle")

- [ ] **Step 4: Implement mine.go**

Create `internal/sim/mine.go`:

```go
package sim

import (
	"fmt"
	"sort"
)

// mineStep runs the mining behavior for species with mine_ticks > 0.
// Returns (events, true) when the entity spent this tick on mining.
func (w *World) mineStep(e *Entity) ([]Event, bool) {
	s := w.cfg.Species[e.Species]
	if s.MineTicks <= 0 {
		return nil, false
	}
	if e.MineTarget != nil && !Mineable(w.At(*e.MineTarget)) {
		e.MineTarget = nil
		w.markDirty(e.ID)
	}
	if e.MineTarget == nil {
		face, ok := w.pickMineTarget(e)
		if !ok {
			return nil, false
		}
		e.MineTarget = &face
		w.markDirty(e.ID)
	}
	target := *e.MineTarget

	if adjacent(e.Pos, target) {
		e.Action = "mining"
		i := target.Y*w.Width + target.X
		w.MineProgress[i] += 1.0 / float64(s.MineTicks)
		w.markDirty(e.ID)
		if w.MineProgress[i] < 1 {
			return nil, true
		}
		wasGold := w.At(target) == TerrainGold
		delete(w.MineProgress, i)
		w.SetTerrain(target, TerrainFloor)
		e.MineTarget = nil
		var evs []Event
		if wasGold {
			w.Gold++
			evs = append(evs, Event{
				Tick: w.Tick, Type: "gold", Actor: e.ID, ActorSpecies: e.Species,
				Msg: fmt.Sprintf("%s struck gold", s.Name),
			})
		} else {
			evs = append(evs, Event{
				Tick: w.Tick, Type: "mined", Actor: e.ID, ActorSpecies: e.Species,
				Msg: fmt.Sprintf("%s mined out a rock", s.Name),
			})
		}
		return evs, true
	}

	// walk toward the face
	next, ok := w.nextStepToward(e.Pos, target)
	if !ok {
		e.MineTarget = nil
		w.markDirty(e.ID)
		return nil, false
	}
	e.Action = "heading to mine"
	e.MoveAcc += s.Speed
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target) {
		e.MoveAcc--
		if w.FaunaAt(next) != nil {
			break // occupied, wait
		}
		delete(w.occ, e.Pos)
		e.Pos = next
		w.occ[e.Pos] = e.ID
		w.markDirty(e.ID)
		next, ok = w.nextStepToward(e.Pos, target)
		if !ok {
			break
		}
	}
	return nil, true
}

// pickMineTarget BFSes the walkable region around e and returns the best
// unclaimed mineable face: near known gold if any is within gold_sense,
// otherwise simply the nearest face.
func (w *World) pickMineTarget(e *Entity) (Point, bool) {
	s := w.cfg.Species[e.Species]

	dist := map[Point]int{e.Pos: 0}
	queue := []Point{e.Pos}
	faceDist := map[Point]int{}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, n := range neighbors {
			q := Point{p.X + n.X, p.Y + n.Y}
			if !w.InBounds(q) {
				continue
			}
			t := w.At(q)
			if Mineable(t) {
				if d, seen := faceDist[q]; !seen || dist[p]+1 < d {
					faceDist[q] = dist[p] + 1
				}
				continue
			}
			if !Passable(t) {
				continue
			}
			if _, seen := dist[q]; seen {
				continue
			}
			dist[q] = dist[p] + 1
			queue = append(queue, q)
		}
	}

	// drop faces claimed by other living miners
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID != e.ID && !c.Dead && c.MineTarget != nil {
			delete(faceDist, *c.MineTarget)
		}
	}
	if len(faceDist) == 0 {
		return Point{}, false
	}

	// nearest gold within sense radius (buried or exposed)
	var gold *Point
	if s.GoldSense > 0 {
		bestD := s.GoldSense + 1
		for y := maxInt(0, e.Pos.Y-s.GoldSense); y <= minInt(w.Height-1, e.Pos.Y+s.GoldSense); y++ {
			for x := maxInt(0, e.Pos.X-s.GoldSense); x <= minInt(w.Width-1, e.Pos.X+s.GoldSense); x++ {
				p := Point{x, y}
				if w.At(p) != TerrainGold {
					continue
				}
				if d := Dist(e.Pos, p); d < bestD {
					bestD = d
					g := p
					gold = &g
				}
			}
		}
	}

	// deterministic choice: sort faces by cell index
	faces := make([]Point, 0, len(faceDist))
	for f := range faceDist {
		faces = append(faces, f)
	}
	sort.Slice(faces, func(i, j int) bool {
		return faces[i].Y*w.Width+faces[i].X < faces[j].Y*w.Width+faces[j].X
	})
	best := faces[0]
	bestScore := 1 << 30
	for _, f := range faces {
		score := faceDist[f]
		if gold != nil {
			score = Dist(f, *gold)*10000 + faceDist[f]
		}
		if score < bestScore {
			best, bestScore = f, score
		}
	}
	return best, true
}

// nextStepToward BFSes over passable terrain and returns the first step of
// the shortest path from start to any cell adjacent to target.
func (w *World) nextStepToward(start, target Point) (Point, bool) {
	prev := map[Point]Point{start: start}
	queue := []Point{start}
	var goal *Point
	for len(queue) > 0 && goal == nil {
		p := queue[0]
		queue = queue[1:]
		for _, n := range neighbors {
			q := Point{p.X + n.X, p.Y + n.Y}
			if !w.InBounds(q) {
				continue
			}
			if _, seen := prev[q]; seen {
				continue
			}
			if !Passable(w.At(q)) {
				continue
			}
			prev[q] = p
			if adjacent(q, target) {
				g := q
				goal = &g
				break
			}
			queue = append(queue, q)
		}
	}
	if goal == nil {
		return Point{}, false
	}
	p := *goal
	for prev[p] != start {
		p = prev[p]
	}
	return p, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 5: Hook into aiStep and fix floor spawning**

In `internal/sim/ai.go`, after the food block (step 2) and before the shelter block, insert:

```go
	// 3. mining
	if evs, mined := w.mineStep(e); mined {
		return evs
	}
```

(Renumber the comments on the shelter and wander blocks to 4 and 5.)

In `internal/sim/tick.go`, replace `randomEdgeTile` — on an underground map every edge tile is rock, so floor spawns must use any free passable tile:

```go
func (w *World) randomFreeTile() (Point, bool) {
	for i := 0; i < 50; i++ {
		p := Point{w.RandN(w.Width), w.RandN(w.Height)}
		if Passable(w.At(p)) && w.FaunaAt(p) == nil {
			return p, true
		}
	}
	return Point{}, false
}
```

Update its call site in `reproduceAndGuard` from `w.randomEdgeTile()` to `w.randomFreeTile()`.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/sim/ -v -short && go test ./... -short && go vet ./...`
Expected: PASS, including all legacy-fixture tests (rabbits ignore mining because their `mine_ticks` is 0)

- [ ] **Step 7: Run the long stability test**

Run: `go test ./internal/sim/ -run TestFiftyThousandTickStability`
Expected: PASS (~40s; the mining hook must not disturb the legacy ecology)

- [ ] **Step 8: Commit**

```bash
git add internal/sim/mine.go internal/sim/mine_test.go internal/sim/world.go internal/sim/ai.go internal/sim/tick.go
git commit -m "Add dwarf mining AI with BFS pathing and gold counter"
```

---

### Task 6: Protocol and debug API carry terrain diffs, mining progress, and gold

**Files:**
- Modify: `internal/server/protocol.go` (SnapshotMsg, TickMsg, BuildSnapshot, BuildTick), `internal/server/api.go:18-41` (stateResp, handleState)
- Test: `internal/server/protocol_test.go` (append), `internal/server/api_test.go` (extend TestAPIState)

**Interfaces:**
- Consumes: `w.Gold`, `w.MineProgress`, `w.TerrainDirtyAndReset()`, `sim.TerrainFloor` (Tasks 1, 5).
- Produces: `TerrainDiff{I int "i"; T uint8 "t"}`; `SnapshotMsg.Gold int "gold"`, `SnapshotMsg.Mining map[int]float64 "mining,omitempty"`; `TickMsg.Gold`, `TickMsg.Mining`, `TickMsg.Terrain []TerrainDiff "terrain,omitempty"`; `/api/state` response gains `"gold"`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/server/protocol_test.go`:

```go
func TestTickCarriesMiningState(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	w.Gold = 3
	w.MineProgress[5] = 0.25
	w.SetTerrain(sim.Point{X: 1, Y: 0}, sim.TerrainFloor)

	snap := BuildSnapshot(w, 1)
	if snap.Gold != 3 || snap.Mining[5] != 0.25 {
		t.Errorf("snapshot missing mining state: gold=%d mining=%v", snap.Gold, snap.Mining)
	}

	tick := BuildTick(w, nil, 1)
	if tick.Gold != 3 || tick.Mining[5] != 0.25 {
		t.Errorf("tick missing mining state: gold=%d mining=%v", tick.Gold, tick.Mining)
	}
	if len(tick.Terrain) != 1 || tick.Terrain[0].I != 1 || tick.Terrain[0].T != uint8(sim.TerrainFloor) {
		t.Errorf("terrain diff = %+v", tick.Terrain)
	}
	tick2 := BuildTick(w, nil, 1)
	if len(tick2.Terrain) != 0 {
		t.Error("terrain dirty set not drained")
	}
}
```

Add `"cellarfloor/internal/sim"` to the file's imports.

In `internal/server/api_test.go`, add to `TestAPIState` after the pops checks (and extend the anonymous struct with `Gold int \`json:"gold"\``):

```go
	w.Gold = 9
	var st2 struct {
		Gold int `json:"gold"`
	}
	if err := json.Unmarshal(apiGet(t, mux, "/api/state").Body.Bytes(), &st2); err != nil {
		t.Fatal(err)
	}
	if st2.Gold != 9 {
		t.Errorf("gold = %d, want 9", st2.Gold)
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestTickCarriesMiningState|TestAPIState' -v`
Expected: FAIL to compile ("snap.Gold undefined")

- [ ] **Step 3: Implement**

In `internal/server/protocol.go` add:

```go
type TerrainDiff struct {
	I int   `json:"i"`
	T uint8 `json:"t"`
}
```

Extend `SnapshotMsg` with:

```go
	Gold      int                      `json:"gold"`
	Mining    map[int]float64          `json:"mining,omitempty"`
```

Extend `TickMsg` with:

```go
	Gold      int             `json:"gold"`
	Mining    map[int]float64 `json:"mining,omitempty"`
	Terrain   []TerrainDiff   `json:"terrain,omitempty"`
```

In `BuildSnapshot`, add to the returned literal: `Gold: w.Gold, Mining: w.MineProgress,`.

In `BuildTick`, before the return:

```go
	var tdiffs []TerrainDiff
	for _, i := range w.TerrainDirtyAndReset() {
		tdiffs = append(tdiffs, TerrainDiff{I: i, T: uint8(w.Terrain[i])})
	}
```

and add to the returned literal: `Gold: w.Gold, Mining: w.MineProgress, Terrain: tdiffs,`.

In `internal/server/api.go`, add `Gold int \`json:"gold"\`` to `stateResp` and `Gold: s.world.Gold,` to the literal in `handleState`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v && go test ./... -short`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/protocol.go internal/server/protocol_test.go internal/server/api.go internal/server/api_test.go
git commit -m "Send terrain diffs, mining progress, and gold over the wire"
```

---

### Task 7: Flip the shipped data to the underground world

**Files:**
- Rewrite: `data/species.toml`, `data/gen.toml`
- Modify: `internal/gen/gen_test.go` (underground assertions + legacy noise test), `internal/server/protocol_test.go:10-41` (rabbit → dwarf), `internal/server/api_test.go` (rabbit → dwarf), `internal/server/persist.go` (prune unknown-species entities on load), `README.md`
- Test: `internal/server/persist_test.go` (append prune test)

**Interfaces:**
- Consumes: everything from Tasks 1-6.
- Produces: shipped world is mushroom+dwarf underground; `LoadWorld` silently drops entities whose species no longer exists (so an old `world.json` degrades instead of panicking).

- [ ] **Step 1: Rewrite the data files**

`data/species.toml`:

```toml
[species.mushroom]
name = "Mushroom"
kind = "flora"
color = "#c4b5d9"
produces = [{ resource = "mushroom", amount = 6, max = 6, regrow = 0.00002 }]

[species.dwarf]
name = "Dwarf"
kind = "fauna"
color = "#d9a066"
eats = ["mushroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
metabolism = 0.00006
starve_ticks = 350000
fear_radius = 0
speed = 0.004
home_range = 0
lifespan = 10000000
mature_age = 1000000
repro_threshold = 9.0
repro_chance = 0.0
repro_cost = 5.0
pop_floor = 3
pop_cap = 10
decay_ticks = 172800
mine_ticks = 172800
gold_sense = 8
```

`data/gen.toml`:

```toml
width = 64
height = 64
clearing_radius = 6
gold_chance = 0.005

scatter = [
  { species = "mushroom", terrain = "dirt", chance = 0.15 },
  { species = "dwarf",    terrain = "dirt", chance = 0.03 },
]
```

`data/sim.toml` is unchanged.

- [ ] **Step 2: Update the gen tests**

In `internal/gen/gen_test.go`, `TestGenerateDeterministic` stays as is (determinism is mode-agnostic). Replace `TestGenerateContents` with:

```go
func TestGenerateContents(t *testing.T) {
	c := cfg(t)
	w := Generate(123, c)
	if w.Width != 64 || w.Height != 64 {
		t.Fatalf("size %dx%d", w.Width, w.Height)
	}
	terrain := map[sim.Terrain]int{}
	for _, tr := range w.Terrain {
		terrain[tr]++
	}
	if terrain[sim.TerrainRock] < 64*64*7/10 {
		t.Errorf("map not mostly rock: %v", terrain)
	}
	if terrain[sim.TerrainGold] == 0 {
		t.Error("no gold veins")
	}
	if w.At(sim.Point{X: 32, Y: 32}) != sim.TerrainDirt {
		t.Error("no clearing at center")
	}
	counts := map[string]int{}
	for _, e := range w.Entities {
		counts[e.Species]++
		if c.Species[e.Species].Kind == "fauna" && !sim.Passable(w.At(e.Pos)) {
			t.Errorf("%s spawned on impassable tile %v", e.Species, e.Pos)
		}
	}
	if counts["mushroom"] == 0 {
		t.Error("no mushrooms generated")
	}
	// dwarves may miss the scatter roll; the pop floor covers them on tick 1
	w.Step()
	if w.CountAlive("dwarf") < c.Species["dwarf"].PopFloor {
		t.Errorf("dwarves = %d, want >= floor %d", w.CountAlive("dwarf"), c.Species["dwarf"].PopFloor)
	}
}

func TestGenerateLegacyNoise(t *testing.T) {
	_, f, _, _ := runtime.Caller(0)
	c, err := data.Load(filepath.Join(filepath.Dir(f), "..", "sim", "testdata", "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	w := Generate(123, c)
	seen := map[sim.Terrain]bool{}
	for _, tr := range w.Terrain {
		seen[tr] = true
	}
	if !seen[sim.TerrainGrass] || !seen[sim.TerrainWater] {
		t.Error("noise path lost grass or water")
	}
}
```

- [ ] **Step 3: Update the server tests**

In `internal/server/protocol_test.go` `TestSnapshotAndTickMessages`: change `snap.Species["rabbit"]` to `snap.Species["dwarf"]`; before `w.Step()` insert `w.Spawn("dwarf", sim.Point{X: 32, Y: 32})` (the clearing center is dirt) so a fauna entity definitely exists; change `tick.Pops["rabbit"]` to `tick.Pops["dwarf"]`.

In `internal/server/api_test.go`: in `newTestAPI`, after `gen.Generate(7, cfg)` add two deterministic dwarves so filters have data:

```go
	w.Spawn("dwarf", sim.Point{X: 30, Y: 32})
	w.Spawn("dwarf", sim.Point{X: 34, Y: 32})
```

(add the `"cellarfloor/internal/sim"` import). Replace every `"rabbit"` with `"dwarf"` in `TestAPIState` and `TestAPIEntities` (species filter URLs become `/api/entities?species=dwarf&...`, the pops check becomes `st.Pops["dwarf"]`, and the non-fauna pops check `st.Pops["grass"]` becomes `st.Pops["mushroom"]`).

- [ ] **Step 4: Make LoadWorld prune unknown species**

Append to `internal/server/persist_test.go`:

```go
func TestLoadPrunesUnknownSpecies(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(5, cfg)
	ghost := w.Spawn("dwarf", sim.Point{X: 32, Y: 32})
	ghost.Species = "rabbit" // simulate a save from before the pivot
	path := filepath.Join(t.TempDir(), "w.json")
	if err := SaveWorld(w, path); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadWorld(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w2.Entities[ghost.ID]; ok {
		t.Error("unknown-species entity survived load")
	}
}
```

(add the `"cellarfloor/internal/sim"` import). In `internal/server/persist.go` `LoadWorld`, before `w.SetConfig(cfg)`:

```go
	for id, e := range w.Entities {
		if _, ok := cfg.Species[e.Species]; !ok {
			delete(w.Entities, id)
		}
	}
```

- [ ] **Step 5: Run everything**

Run: `go test ./... && go vet ./...`
Expected: PASS including the legacy 50k-tick run

- [ ] **Step 6: Update README**

Replace the description paragraph in `README.md`:

```markdown
A little world living on the floor of a dark cellar. Dwarves carve tunnels
through the rock in search of gold, one cell per real day, eating mushrooms
from the clearing where they live. Most things happen on a scale of hours
or days; the world runs at 1x wall-clock on a persistent Go server and is
watched live from the browser.

Spec: docs/superpowers/specs/2026-07-10-idle-pivot-design.md
```

Add after the Run block:

```markdown
The pivot to the underground world made old rabbit-era saves obsolete;
entities of removed species are dropped on load, but for a proper start
run once with -fresh (or delete world.json).
```

- [ ] **Step 7: Commit**

```bash
git add data/species.toml data/gen.toml internal/gen/gen_test.go internal/server/protocol_test.go internal/server/api_test.go internal/server/persist.go internal/server/persist_test.go README.md
git commit -m "Pivot shipped world to dwarves mining for gold"
```

---

### Task 8: Client renders tunnels, progress bars, and the gold counter

**Files:**
- Modify: `client/src/types.ts`, `client/src/world.ts`, `client/src/render.ts`, `client/src/ui.ts`, `client/index.html`

**Interfaces:**
- Consumes: wire fields from Task 6 (`gold`, `mining` keyed by stringified cell index, `terrain` diff array).
- Produces: `world.gold: number`, `world.mining: Record<string, number>`, `world.terrainVersion: number` (bumped whenever terrain content changes); `#gold` element in the side panel.

- [ ] **Step 1: Extend the types**

In `client/src/types.ts` add:

```ts
export interface TerrainDiff {
  i: number;
  t: number;
}
```

Add to `SnapshotMsg`:

```ts
  gold: number;
  mining?: Record<string, number> | null;
```

Add to `TickMsg`:

```ts
  gold: number;
  mining?: Record<string, number> | null;
  terrain?: TerrainDiff[] | null;
```

- [ ] **Step 2: Extend WorldState**

In `client/src/world.ts` add fields after `timeScale`:

```ts
  gold = 0;
  mining: Record<string, number> = {};
  terrainVersion = 0;
```

In `applySnapshot`, after `this.terrain = m.terrain;` add:

```ts
    this.terrainVersion++;
    this.gold = m.gold ?? 0;
    this.mining = m.mining ?? {};
```

In `applyTick`, after the removed loop add:

```ts
    this.gold = m.gold ?? this.gold;
    this.mining = m.mining ?? {};
    const diffs = m.terrain ?? [];
    if (diffs.length) {
      for (const d of diffs) this.terrain[d.i] = d.t;
      this.terrainVersion++;
    }
```

- [ ] **Step 3: Render new terrain, repaint on diffs, draw progress bars**

In `client/src/render.ts`:

```ts
const TERRAIN_COLORS = ["#3d5a36", "#6b5537", "#2b4a63", "#5a5a5a", "#26221e", "#c9a227"]; // grass dirt water rock floor gold
```

Track repaints — replace the `world.onChange` handler in `startRender`:

```ts
  let paintedVersion = -1;
  world.onChange(() => {
    if (!terrainCanvas || terrainCanvas.width !== world.width * TILE || paintedVersion !== world.terrainVersion) {
      renderTerrain();
      paintedVersion = world.terrainVersion;
    }
    canvas.width = world.width * TILE;
    canvas.height = world.height * TILE;
  });
```

In `frame`, after the entity loop and before `positionPopup`, draw the progress bars:

```ts
      for (const [k, p] of Object.entries(world.mining)) {
        const i = Number(k);
        const bx = (i % world.width) * TILE;
        const by = Math.floor(i / world.width) * TILE;
        ctx.fillStyle = "#1a1815";
        ctx.fillRect(bx + 1, by + 2, TILE - 2, 3);
        ctx.fillStyle = "#ffb347";
        ctx.fillRect(bx + 1, by + 2, (TILE - 2) * Math.min(p, 1), 3);
      }
```

- [ ] **Step 4: Gold counter in the side panel**

In `client/index.html`, after the Populations div add:

```html
    <div><h2>Gold</h2><div id="gold">0</div></div>
```

In `client/src/ui.ts` `initUI`, add `world.onChange(renderGold);` and the function:

```ts
function renderGold() {
  const el = document.getElementById("gold")!;
  el.textContent = String(world.gold);
}
```

- [ ] **Step 5: Build**

Run: `cd client && npm run build && cd ..`
Expected: tsc + vite succeed with no errors

- [ ] **Step 6: Live verification (use the project verify skill's recipes)**

```bash
go run ./cmd/cellarfloor -fresh   # background; fresh underground world
curl -s localhost:8080/api/state  # expect pops.dwarf >= 3, gold 0, entities small
curl -s 'localhost:8080/api/entities?species=dwarf'
```

Then with headless Playwright (channel 'chrome') against http://localhost:8080: click 64x, wait ~3-5 minutes, then assert via `/api/entities?species=dwarf` that some dwarf has `action` of `"heading to mine"` or `"mining"`, `/api/state` still ticks, and take a screenshot showing the mostly-rock map, clearing, and (if a dwarf reached a face) the amber progress bar. Click a dwarf and confirm the popup shows `doing: mining` or `heading to mine`. Stop the server (SIGINT so it saves) and confirm `world.json` contains `"gold":` and `"mineProgress"` keys after some progress exists.

- [ ] **Step 7: Commit**

```bash
git add client/src/types.ts client/src/world.ts client/src/render.ts client/src/ui.ts client/index.html
git commit -m "Render tunnels, mining progress bars, and gold counter"
```
