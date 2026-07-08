# Cellar Floor v1 (Terrarium) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A persistent Go-hosted ecology (grass, bushes, trees, rabbits, wolves) on a procgen tile map, watched live from the browser by any number of spectators.

**Architecture:** Pure deterministic `sim` package driven entirely by TOML data files loaded by `data`; `gen` produces worlds from a seed; `server` owns the tick loop and broadcasts JSON diffs over WebSocket; a TypeScript canvas client renders, inspects, and graphs. Spec: `docs/superpowers/specs/2026-07-08-cellar-floor-design.md`.

**Tech Stack:** Go 1.22+, github.com/BurntSushi/toml, github.com/gorilla/websocket, TypeScript + Vite (vanilla, no UI framework), HTML canvas.

## Global Constraints

- Everything behavioral/balance lives in `data/*.toml`, never in Go code. The engine must not mention "rabbit" or "wolf" outside data files and tests.
- The sim is deterministic: same seed + same tick count = identical world. All randomness via the world's own serializable RNG. Entity iteration always in sorted ID order (Go map order is random).
- Predator/prey/fear are emergent: prey = different-species fauna whose Produces intersects my Eats; predator = different-species fauna whose Eats intersects my Produces. Never a hardcoded species pair. Same species excluded (no cannibalism).
- The tick loop must never die: recover from panics per tick, log, continue.
- Commit messages: one sentence, under 70 characters, no long dashes, no Claude attribution.
- No long dashes in any text or docs.
- Population guardrail starting values: rabbit floor 4 cap 60, wolf floor 2 cap 15.
- Base tick rate 2/s; time scales 0, 1, 8, 64. Autosave every 5 minutes to a single JSON file.

## File Structure

```
cellar-floor/
├── go.mod                      module cellarfloor
├── data/
│   ├── sim.toml                tick rate, autosave
│   ├── gen.toml                map size, noise, scatter
│   └── species.toml            all species definitions
├── internal/data/data.go       TOML loading + validation
├── internal/data/data_test.go
├── internal/sim/world.go       World, Entity, Terrain, RNG, Spawn
├── internal/sim/tick.go        Step(): regrow, AI, deaths, births, guardrails
├── internal/sim/ai.go          Maslow decision loop + movement
├── internal/sim/events.go      Event type
├── internal/sim/*_test.go
├── internal/gen/gen.go         noise terrain + scatter
├── internal/gen/gen_test.go
├── internal/sim/longrun_test.go  50k-tick stability test (loads real data/)
├── internal/server/persist.go  save/load world JSON
├── internal/server/protocol.go message structs, views
├── internal/server/hub.go      WebSocket clients + broadcast
├── internal/server/server.go   HTTP + WS + tick loop + timescale
├── internal/server/*_test.go
├── cmd/cellarfloor/main.go
└── client/
    ├── package.json, tsconfig.json, vite.config.ts, index.html
    └── src/types.ts, net.ts, world.ts, render.ts, ui.ts, main.ts
```

---

### Task 1: Scaffolding and the data package

**Files:**
- Create: `go.mod`, `.gitignore`, `data/sim.toml`, `data/gen.toml`, `data/species.toml`
- Create: `internal/data/data.go`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: `data.Load(dir string) (*data.Config, error)`; types `Config{Sim SimConfig; Gen GenConfig; Species map[string]*Species}`, `Species`, `Produce{Resource string; Amount, Max, Regrow float64}`, `Desire`, `SimConfig{TickRate float64; AutosaveMinutes int; SavePath string}`, `GenConfig{Width, Height int; NoiseScale float64; NoiseOctaves int; WaterBelow, DirtAbove, RockAbove float64; Scatter []ScatterRule}`, `ScatterRule{Species, Terrain string; Chance float64}`. All fields exactly as in Step 3 below.

- [ ] **Step 1: Scaffold module**

```bash
cd ~/cellar-floor
go mod init cellarfloor
go get github.com/BurntSushi/toml@latest
printf 'world.json\nclient/node_modules/\nclient/dist/\n' > .gitignore
```

- [ ] **Step 2: Write the data files**

`data/sim.toml`:
```toml
tick_rate = 2.0
autosave_minutes = 5
save_path = "world.json"
```

`data/gen.toml`:
```toml
width = 64
height = 64
noise_scale = 12.0
noise_octaves = 3
water_below = 0.30
dirt_above = 0.75
rock_above = 0.85

scatter = [
  { species = "grass",  terrain = "grass", chance = 0.25 },
  { species = "bush",   terrain = "grass", chance = 0.05 },
  { species = "tree",   terrain = "grass", chance = 0.04 },
  { species = "tree",   terrain = "dirt",  chance = 0.02 },
  { species = "rabbit", terrain = "grass", chance = 0.008 },
  { species = "wolf",   terrain = "grass", chance = 0.002 },
]
```

`data/species.toml`:
```toml
[species.grass]
name = "Grass"
kind = "flora"
color = "#69a85c"
produces = [{ resource = "grass", amount = 5, max = 5, regrow = 0.02 }]

[species.bush]
name = "Berry Bush"
kind = "flora"
color = "#2d6a4f"
produces = [{ resource = "berries", amount = 8, max = 8, regrow = 0.01 }]

[species.tree]
name = "Tree"
kind = "flora"
color = "#40531b"
produces = [{ resource = "wood", amount = 20, max = 20, regrow = 0 }]

[species.rabbit]
name = "Rabbit"
kind = "fauna"
color = "#e8e4dc"
produces = [
  { resource = "meat", amount = 2, max = 2, regrow = 0 },
  { resource = "fur",  amount = 1, max = 1, regrow = 0 },
]
eats = ["grass", "berries"]
shelters = ["berries"]
bite_size = 1.0
stomach_size = 10.0
hunger_threshold = 6.0
metabolism = 0.02
starve_ticks = 600
fear_radius = 5
speed = 0.5
home_range = 8
lifespan = 8000
mature_age = 800
repro_threshold = 8.0
repro_chance = 0.004
repro_cost = 4.0
pop_floor = 4
pop_cap = 60
decay_ticks = 400

[species.wolf]
name = "Wolf"
kind = "fauna"
color = "#8a8d91"
produces = [
  { resource = "meat", amount = 4, max = 4, regrow = 0 },
  { resource = "fur",  amount = 2, max = 2, regrow = 0 },
]
eats = ["meat"]
shelters = ["wood"]
bite_size = 2.0
stomach_size = 16.0
hunger_threshold = 8.0
metabolism = 0.02
starve_ticks = 900
fear_radius = 0
speed = 0.6
home_range = 12
lifespan = 10000
mature_age = 1200
repro_threshold = 13.0
repro_chance = 0.002
repro_cost = 6.0
pop_floor = 2
pop_cap = 15
decay_ticks = 400
```

- [ ] **Step 3: Write the failing test**

`internal/data/data_test.go`:
```go
package data

import (
	"path/filepath"
	"runtime"
	"testing"
)

func dataDir(t *testing.T) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "data")
}

func TestLoadRealData(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sim.TickRate != 2.0 {
		t.Errorf("tick_rate = %v, want 2.0", cfg.Sim.TickRate)
	}
	r, ok := cfg.Species["rabbit"]
	if !ok {
		t.Fatal("no rabbit species")
	}
	if r.Kind != "fauna" || r.ID != "rabbit" || len(r.Eats) != 2 {
		t.Errorf("rabbit mis-parsed: %+v", r)
	}
	if cfg.Gen.Width != 64 || len(cfg.Gen.Scatter) == 0 {
		t.Errorf("gen mis-parsed: %+v", cfg.Gen)
	}
}

func TestValidationRejectsUnknownResource(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Species["rabbit"].Eats = append(cfg.Species["rabbit"].Eats, "plutonium")
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unknown eaten resource")
	}
}

func TestValidationRejectsBadFauna(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Species["wolf"].StomachSize = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for zero stomach_size")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/data/ -v`
Expected: FAIL (package does not compile, `Load` undefined).

- [ ] **Step 5: Write the implementation**

`internal/data/data.go`:
```go
package data

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Produce struct {
	Resource string  `toml:"resource" json:"resource"`
	Amount   float64 `toml:"amount" json:"amount"`
	Max      float64 `toml:"max" json:"max"`
	Regrow   float64 `toml:"regrow" json:"regrow"`
}

type Desire struct {
	Resource string  `toml:"resource" json:"resource"`
	Amount   float64 `toml:"amount" json:"amount"`
	Aversion bool    `toml:"aversion" json:"aversion"`
}

type Species struct {
	ID              string    `toml:"-" json:"id"`
	Name            string    `toml:"name" json:"name"`
	Kind            string    `toml:"kind" json:"kind"`
	Color           string    `toml:"color" json:"color"`
	Produces        []Produce `toml:"produces" json:"produces"`
	Eats            []string  `toml:"eats" json:"eats"`
	Shelters        []string  `toml:"shelters" json:"shelters"`
	Desires         []Desire  `toml:"desires" json:"desires"`
	BiteSize        float64   `toml:"bite_size" json:"biteSize"`
	StomachSize     float64   `toml:"stomach_size" json:"stomachSize"`
	HungerThreshold float64   `toml:"hunger_threshold" json:"hungerThreshold"`
	Metabolism      float64   `toml:"metabolism" json:"metabolism"`
	StarveTicks     int       `toml:"starve_ticks" json:"starveTicks"`
	FearRadius      int       `toml:"fear_radius" json:"fearRadius"`
	Speed           float64   `toml:"speed" json:"speed"`
	HomeRange       int       `toml:"home_range" json:"homeRange"`
	Lifespan        int       `toml:"lifespan" json:"lifespan"`
	MatureAge       int       `toml:"mature_age" json:"matureAge"`
	ReproThreshold  float64   `toml:"repro_threshold" json:"reproThreshold"`
	ReproChance     float64   `toml:"repro_chance" json:"reproChance"`
	ReproCost       float64   `toml:"repro_cost" json:"reproCost"`
	PopFloor        int       `toml:"pop_floor" json:"popFloor"`
	PopCap          int       `toml:"pop_cap" json:"popCap"`
	DecayTicks      int       `toml:"decay_ticks" json:"decayTicks"`
}

type SimConfig struct {
	TickRate        float64 `toml:"tick_rate"`
	AutosaveMinutes int     `toml:"autosave_minutes"`
	SavePath        string  `toml:"save_path"`
}

type ScatterRule struct {
	Species string  `toml:"species"`
	Terrain string  `toml:"terrain"`
	Chance  float64 `toml:"chance"`
}

type GenConfig struct {
	Width        int           `toml:"width"`
	Height       int           `toml:"height"`
	NoiseScale   float64       `toml:"noise_scale"`
	NoiseOctaves int           `toml:"noise_octaves"`
	WaterBelow   float64       `toml:"water_below"`
	DirtAbove    float64       `toml:"dirt_above"`
	RockAbove    float64       `toml:"rock_above"`
	Scatter      []ScatterRule `toml:"scatter"`
}

type Config struct {
	Sim     SimConfig
	Gen     GenConfig
	Species map[string]*Species
}

var validTerrains = map[string]bool{"grass": true, "dirt": true, "water": true, "rock": true}

func Load(dir string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(filepath.Join(dir, "sim.toml"), &cfg.Sim); err != nil {
		return nil, fmt.Errorf("sim.toml: %w", err)
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "gen.toml"), &cfg.Gen); err != nil {
		return nil, fmt.Errorf("gen.toml: %w", err)
	}
	var sp struct {
		Species map[string]*Species `toml:"species"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "species.toml"), &sp); err != nil {
		return nil, fmt.Errorf("species.toml: %w", err)
	}
	cfg.Species = sp.Species
	for id, s := range cfg.Species {
		s.ID = id
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Validate(cfg *Config) error {
	produced := map[string]bool{}
	for _, s := range cfg.Species {
		for _, p := range s.Produces {
			produced[p.Resource] = true
		}
	}
	for id, s := range cfg.Species {
		if s.Kind != "flora" && s.Kind != "fauna" {
			return fmt.Errorf("species %s: kind must be flora or fauna, got %q", id, s.Kind)
		}
		if s.Name == "" || s.Color == "" {
			return fmt.Errorf("species %s: name and color are required", id)
		}
		for _, r := range s.Eats {
			if !produced[r] {
				return fmt.Errorf("species %s eats %q which nothing produces", id, r)
			}
		}
		for _, r := range s.Shelters {
			if !produced[r] {
				return fmt.Errorf("species %s shelters in %q which nothing produces", id, r)
			}
		}
		if s.Kind == "fauna" {
			if s.StomachSize <= 0 || s.BiteSize <= 0 || s.Speed <= 0 ||
				s.Metabolism <= 0 || s.StarveTicks <= 0 || s.DecayTicks <= 0 ||
				s.Lifespan <= 0 || s.PopCap <= 0 {
				return fmt.Errorf("species %s: fauna requires positive stomach_size, bite_size, speed, metabolism, starve_ticks, decay_ticks, lifespan, pop_cap", id)
			}
			if len(s.Eats) == 0 {
				return fmt.Errorf("species %s: fauna must eat something", id)
			}
		}
	}
	if cfg.Sim.TickRate <= 0 {
		return fmt.Errorf("sim: tick_rate must be positive")
	}
	if cfg.Gen.Width <= 0 || cfg.Gen.Height <= 0 {
		return fmt.Errorf("gen: width and height must be positive")
	}
	for _, r := range cfg.Gen.Scatter {
		if _, ok := cfg.Species[r.Species]; !ok {
			return fmt.Errorf("scatter rule references unknown species %q", r.Species)
		}
		if !validTerrains[r.Terrain] {
			return fmt.Errorf("scatter rule references unknown terrain %q", r.Terrain)
		}
	}
	return nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/data/ -v`
Expected: PASS (3 tests).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "Add TOML data files and data loader with validation"
```

---

### Task 2: Sim core types, RNG, world, spawn, determinism

**Files:**
- Create: `internal/sim/world.go`, `internal/sim/events.go`
- Test: `internal/sim/world_test.go`

**Interfaces:**
- Consumes: `data.Config`, `data.Species`, `data.Produce`.
- Produces:
  - `sim.Terrain` (uint8): `TerrainGrass=0, TerrainDirt=1, TerrainWater=2, TerrainRock=3`; `TerrainName(t Terrain) string`; `Passable(t Terrain) bool` (grass, dirt).
  - `sim.Point{X, Y int}`.
  - `sim.Entity{ID int; Species string; Pos Point; Produces []data.Produce; Fullness float64; StarvingFor int; Age int; Home *Point; Dead bool; DecayLeft int; Action string; MoveAcc float64}`.
  - `sim.World{Width, Height int; Terrain []Terrain; Entities map[int]*Entity; NextID int; Tick int64; Rng uint64}` plus unexported `cfg *data.Config`.
  - `sim.NewWorld(w, h int, seed uint64, cfg *data.Config) *World`, `(*World) SetConfig(cfg *data.Config)`, `(*World) Cfg() *data.Config`.
  - `(*World) Spawn(speciesID string, p Point) *Entity` (copies species Produces, Fullness = StomachSize/2 for fauna).
  - RNG: `(*World) RandFloat() float64`, `(*World) RandN(n int) int` (xorshift64star on `World.Rng`).
  - Helpers: `(*World) At(p Point) Terrain`, `(*World) InBounds(p Point) bool`, `(*World) FaunaAt(p Point) *Entity`, `(*World) SortedIDs() []int`, `(*World) CountAlive(speciesID string) int`, `Dist(a, b Point) int` (Chebyshev).
  - `sim.Event{Tick int64; Type string; Actor int; ActorSpecies string; Target int; TargetSpecies string; Msg string}` with Type one of: `ate, hunted, killed, starved, died, born, spawned, fled`.

- [ ] **Step 1: Write the failing test**

`internal/sim/world_test.go`:
```go
package sim

import (
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
)

func testCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatalf("load data: %v", err)
	}
	return cfg
}

func flatWorld(t *testing.T, w, h int, seed uint64) *World {
	world := NewWorld(w, h, seed, testCfg(t))
	return world // all grass terrain by default
}

func TestSpawnCopiesSpeciesData(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	r := w.Spawn("rabbit", Point{2, 3})
	if r == nil || w.Entities[r.ID] != r {
		t.Fatal("spawn failed")
	}
	if len(r.Produces) != 2 || r.Produces[0].Resource != "meat" {
		t.Errorf("produces not copied: %+v", r.Produces)
	}
	r.Produces[0].Amount = 0
	if w.Cfg().Species["rabbit"].Produces[0].Amount == 0 {
		t.Error("spawn shares Produces slice with species template")
	}
	if r.Fullness != w.Cfg().Species["rabbit"].StomachSize/2 {
		t.Errorf("fullness = %v", r.Fullness)
	}
}

func TestRngDeterministic(t *testing.T) {
	a, b := flatWorld(t, 4, 4, 42), flatWorld(t, 4, 4, 42)
	for i := 0; i < 100; i++ {
		if a.RandFloat() != b.RandFloat() {
			t.Fatal("same seed diverged")
		}
	}
	if a.RandN(10) < 0 || a.RandN(10) > 9 {
		t.Error("RandN out of range")
	}
}

func TestHelpers(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	if !w.InBounds(Point{7, 7}) || w.InBounds(Point{8, 0}) {
		t.Error("InBounds wrong")
	}
	w.Spawn("rabbit", Point{1, 1})
	if w.FaunaAt(Point{1, 1}) == nil || w.FaunaAt(Point{0, 0}) != nil {
		t.Error("FaunaAt wrong")
	}
	if w.CountAlive("rabbit") != 1 {
		t.Error("CountAlive wrong")
	}
	if Dist(Point{0, 0}, Point{3, 2}) != 3 {
		t.Error("Dist should be Chebyshev")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -v`
Expected: FAIL, package does not compile.

- [ ] **Step 3: Write the implementation**

`internal/sim/events.go`:
```go
package sim

type Event struct {
	Tick          int64  `json:"tick"`
	Type          string `json:"type"`
	Actor         int    `json:"actor"`
	ActorSpecies  string `json:"actorSpecies"`
	Target        int    `json:"target,omitempty"`
	TargetSpecies string `json:"targetSpecies,omitempty"`
	Msg           string `json:"msg"`
}
```

`internal/sim/world.go`:
```go
package sim

import (
	"sort"

	"cellarfloor/internal/data"
)

type Terrain uint8

const (
	TerrainGrass Terrain = iota
	TerrainDirt
	TerrainWater
	TerrainRock
)

var terrainNames = [...]string{"grass", "dirt", "water", "rock"}

func TerrainName(t Terrain) string { return terrainNames[t] }
func Passable(t Terrain) bool      { return t == TerrainGrass || t == TerrainDirt }

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Entity struct {
	ID          int            `json:"id"`
	Species     string         `json:"species"`
	Pos         Point          `json:"pos"`
	Produces    []data.Produce `json:"produces"`
	Fullness    float64        `json:"fullness"`
	StarvingFor int            `json:"starvingFor"`
	Age         int            `json:"age"`
	Home        *Point         `json:"home,omitempty"`
	Dead        bool           `json:"dead"`
	DecayLeft   int            `json:"decayLeft"`
	Action      string         `json:"action"`
	MoveAcc     float64        `json:"moveAcc"`
}

type World struct {
	Width    int             `json:"width"`
	Height   int             `json:"height"`
	Terrain  []Terrain       `json:"terrain"`
	Entities map[int]*Entity `json:"entities"`
	NextID   int             `json:"nextId"`
	Tick     int64           `json:"tick"`
	Rng      uint64          `json:"rng"`

	cfg   *data.Config
	dirty map[int]bool
}

func NewWorld(w, h int, seed uint64, cfg *data.Config) *World {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &World{
		Width: w, Height: h,
		Terrain:  make([]Terrain, w*h),
		Entities: map[int]*Entity{},
		NextID:   1,
		Rng:      seed,
		cfg:      cfg,
		dirty:    map[int]bool{},
	}
}

func (w *World) SetConfig(cfg *data.Config) {
	w.cfg = cfg
	if w.dirty == nil {
		w.dirty = map[int]bool{}
	}
}
func (w *World) Cfg() *data.Config { return w.cfg }

func (w *World) rand() uint64 {
	x := w.Rng
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	w.Rng = x
	return x * 0x2545F4914F6CDD1D
}

func (w *World) RandFloat() float64 { return float64(w.rand()>>11) / (1 << 53) }
func (w *World) RandN(n int) int    { return int(w.rand() % uint64(n)) }

func (w *World) InBounds(p Point) bool {
	return p.X >= 0 && p.X < w.Width && p.Y >= 0 && p.Y < w.Height
}
func (w *World) At(p Point) Terrain { return w.Terrain[p.Y*w.Width+p.X] }

func (w *World) FaunaAt(p Point) *Entity {
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if e.Pos == p && w.cfg.Species[e.Species].Kind == "fauna" && !e.Dead {
			return e
		}
	}
	return nil
}

func (w *World) SortedIDs() []int {
	ids := make([]int, 0, len(w.Entities))
	for id := range w.Entities {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (w *World) CountAlive(speciesID string) int {
	n := 0
	for _, e := range w.Entities {
		if e.Species == speciesID && !e.Dead {
			n++
		}
	}
	return n
}

func (w *World) Spawn(speciesID string, p Point) *Entity {
	s, ok := w.cfg.Species[speciesID]
	if !ok {
		return nil
	}
	e := &Entity{
		ID:       w.NextID,
		Species:  speciesID,
		Pos:      p,
		Produces: append([]data.Produce(nil), s.Produces...),
	}
	if s.Kind == "fauna" {
		e.Fullness = s.StomachSize / 2
	}
	w.NextID++
	w.Entities[e.ID] = e
	w.dirty[e.ID] = true
	return e
}

func Dist(a, b Point) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
```

Note: `FaunaAt` is O(n) per call; fine at v1 scale (64x64, ~100 entities). Do not optimize yet.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sim/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add sim core types, world, spawn and deterministic rng"
```

---

### Task 3: Tick basics: regrow, metabolism, starvation, aging, decay

**Files:**
- Create: `internal/sim/tick.go`
- Test: `internal/sim/tick_test.go`

**Interfaces:**
- Consumes: everything from Task 2.
- Produces: `(*World) Step() []Event` performing, in order: (1) flora regrow, (2) fauna AI (stub `aiStep` for now), (3) metabolism/starvation/aging deaths, (4) reproduction+guardrails (stub for now), (5) corpse decay removal. Also `(*World) DirtyAndReset() []int` returning sorted IDs marked changed this tick (spawned, moved, mutated, died) and clearing the set; removal of decayed entities reported via return value `(*World) removedThisTick` pattern: `Step` fills exported `w.Removed []int` (reset each Step).
- Produces internal helper: `(w *World) markDirty(id int)`.

- [ ] **Step 1: Write the failing test**

`internal/sim/tick_test.go`:
```go
package sim

import "testing"

func TestRegrow(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	b := w.Spawn("bush", Point{0, 0})
	b.Produces[0].Amount = 0
	w.Step()
	if b.Produces[0].Amount != 0.01 {
		t.Errorf("berries = %v, want 0.01", b.Produces[0].Amount)
	}
	b.Produces[0].Amount = 8
	w.Step()
	if b.Produces[0].Amount > 8 {
		t.Error("regrow exceeded max")
	}
}

func TestStarvation(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	r := w.Spawn("rabbit", Point{0, 0})
	r.Fullness = 0.01
	starve := w.Cfg().Species["rabbit"].StarveTicks
	var events []Event
	for i := 0; i < starve+60; i++ {
		events = append(events, w.Step()...)
		if r.Dead {
			break
		}
	}
	if !r.Dead {
		t.Fatal("rabbit should starve")
	}
	found := false
	for _, e := range events {
		if e.Type == "starved" && e.Actor == r.ID {
			found = true
		}
	}
	if !found {
		t.Error("no starved event")
	}
	if r.DecayLeft != w.Cfg().Species["rabbit"].DecayTicks {
		t.Errorf("decay not set: %d", r.DecayLeft)
	}
}

func TestOldAgeAndDecayRemoval(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	r := w.Spawn("rabbit", Point{0, 0})
	r.Age = w.Cfg().Species["rabbit"].Lifespan
	r.Fullness = 10
	w.Step()
	if !r.Dead {
		t.Fatal("rabbit should die of old age")
	}
	r.DecayLeft = 1
	w.Step()
	if _, ok := w.Entities[r.ID]; ok {
		t.Error("corpse should be removed after decay")
	}
	if len(w.Removed) != 1 || w.Removed[0] != r.ID {
		t.Errorf("Removed = %v", w.Removed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -run 'TestRegrow|TestStarvation|TestOldAge' -v`
Expected: FAIL, `Step` undefined.

- [ ] **Step 3: Write the implementation**

`internal/sim/tick.go`:
```go
package sim

import "fmt"

// Removed lists entity IDs deleted during the most recent Step.
// aiStep is implemented in ai.go (Task 4); declare a stub there first.

func (w *World) markDirty(id int) { w.dirty[id] = true }

// DirtyAndReset returns IDs changed during the last Step and clears the set.
func (w *World) DirtyAndReset() []int {
	ids := make([]int, 0, len(w.dirty))
	for id := range w.dirty {
		ids = append(ids, id)
	}
	w.dirty = map[int]bool{}
	sortInts(ids)
	return ids
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func (w *World) Step() []Event {
	w.Tick++
	w.Removed = w.Removed[:0]
	var events []Event

	ids := w.SortedIDs()

	// 1. flora regrow (and corpse "produces" never regrow since regrow=0)
	for _, id := range ids {
		e := w.Entities[id]
		if e.Dead {
			continue
		}
		for i := range e.Produces {
			p := &e.Produces[i]
			if p.Regrow > 0 && p.Amount < p.Max {
				p.Amount += p.Regrow
				if p.Amount > p.Max {
					p.Amount = p.Max
				}
				w.markDirty(id)
			}
		}
	}

	// 2. fauna AI
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || e.Dead {
			continue
		}
		if w.cfg.Species[e.Species].Kind == "fauna" {
			events = append(events, w.aiStep(e)...)
		}
	}

	// 3. metabolism, starvation, aging
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || e.Dead {
			continue
		}
		s := w.cfg.Species[e.Species]
		if s.Kind != "fauna" {
			continue
		}
		e.Age++
		e.Fullness -= s.Metabolism
		if e.Fullness < 0 {
			e.Fullness = 0
		}
		if e.Fullness == 0 {
			e.StarvingFor++
		} else {
			e.StarvingFor = 0
		}
		w.markDirty(id)
		if e.StarvingFor > s.StarveTicks {
			events = append(events, w.kill(e, "starved", fmt.Sprintf("%s starved", s.Name)))
		} else if e.Age > s.Lifespan {
			events = append(events, w.kill(e, "died", fmt.Sprintf("%s died of old age", s.Name)))
		}
	}

	// 4. reproduction and guardrails (Task 6 fills these in)
	events = append(events, w.reproduceAndGuard()...)

	// 5. corpse decay
	for _, id := range ids {
		e, ok := w.Entities[id]
		if !ok || !e.Dead {
			continue
		}
		e.DecayLeft--
		if e.DecayLeft <= 0 {
			delete(w.Entities, id)
			w.Removed = append(w.Removed, id)
		}
	}
	return events
}

func (w *World) kill(e *Entity, evType, msg string) Event {
	s := w.cfg.Species[e.Species]
	e.Dead = true
	e.Action = "dead"
	e.DecayLeft = s.DecayTicks
	w.markDirty(e.ID)
	return Event{Tick: w.Tick, Type: evType, Actor: e.ID, ActorSpecies: e.Species, Msg: msg}
}
```

Add field `Removed []int `json:"-"`` to `World` in `world.go`. Add temporary stubs at the bottom of `tick.go` so the package compiles (they move to their own files in Tasks 4 and 6):

```go
// Stubs replaced in Task 4 (ai.go) and Task 6.
func (w *World) aiStep(e *Entity) []Event      { return nil }
func (w *World) reproduceAndGuard() []Event    { return nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sim/ -v`
Expected: PASS (all tests including Task 2's).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add tick loop with regrow, starvation, aging and decay"
```

---

### Task 4: Foraging: food search, movement, eating flora

**Files:**
- Create: `internal/sim/ai.go` (move `aiStep` stub here and implement; delete stub from `tick.go`)
- Test: `internal/sim/ai_test.go`

**Interfaces:**
- Consumes: Tasks 2 and 3.
- Produces:
  - `(*World) aiStep(e *Entity) []Event`: Maslow order: flee (Task 5), eat, shelter (Task 6), wander.
  - `(*World) findFood(e *Entity) *Entity`: nearest entity (by `Dist`, tie-break lower ID) with any `Produces` amount >= 0.5 whose resource is in `e`'s species `Eats`; excludes self and same species; includes dead fauna (corpses); includes live fauna (prey) only if different species.
  - `(*World) moveToward(e *Entity, target Point)` and `(*World) moveAway(e *Entity, from Point)`: consume `e.MoveAcc += Speed`; per whole step pick the 8-neighbor minimizing (maximizing) `Dist` to target that is in bounds, `Passable`, and has no live fauna; stay put if none.
  - `(*World) eatFrom(e *Entity, food *Entity) []Event`: transfer `min(BiteSize, available, StomachSize-Fullness)` from the first matching produce; emits `ate` event.
  - `adjacent(a, b Point) bool` = `Dist(a,b) <= 1`.

- [ ] **Step 1: Write the failing test**

`internal/sim/ai_test.go`:
```go
package sim

import "testing"

func TestHungryRabbitEatsAdjacentBush(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	b := w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	r.Fullness = 1
	before := b.Produces[0].Amount
	evs := w.Step()
	if b.Produces[0].Amount >= before {
		t.Errorf("bush not eaten: %v", b.Produces[0].Amount)
	}
	if r.Fullness <= 1-w.Cfg().Species["rabbit"].Metabolism {
		t.Errorf("rabbit fullness did not rise: %v", r.Fullness)
	}
	found := false
	for _, e := range evs {
		if e.Type == "ate" && e.Actor == r.ID {
			found = true
		}
	}
	if !found {
		t.Error("no ate event")
	}
}

func TestHungryRabbitWalksTowardFood(t *testing.T) {
	w := flatWorld(t, 16, 16, 1)
	w.Spawn("bush", Point{10, 10})
	r := w.Spawn("rabbit", Point{2, 2})
	r.Fullness = 1
	d0 := Dist(r.Pos, Point{10, 10})
	for i := 0; i < 8; i++ {
		w.Step()
	}
	if Dist(r.Pos, Point{10, 10}) >= d0 {
		t.Errorf("rabbit did not approach food: at %v", r.Pos)
	}
	if r.Action != "seeking food" && r.Action != "eating" {
		t.Errorf("action = %q", r.Action)
	}
}

func TestMovementAvoidsWater(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	for y := 0; y < 8; y++ {
		w.Terrain[y*8+4] = TerrainWater // vertical river at x=4
	}
	w.Spawn("bush", Point{6, 3})
	r := w.Spawn("rabbit", Point{3, 3})
	r.Fullness = 1
	for i := 0; i < 30; i++ {
		w.Step()
		if w.At(r.Pos) == TerrainWater {
			t.Fatal("rabbit walked into water")
		}
	}
}

func TestFullRabbitDoesNotEat(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	b := w.Spawn("bush", Point{2, 2})
	r := w.Spawn("rabbit", Point{2, 3})
	r.Fullness = w.Cfg().Species["rabbit"].StomachSize
	before := b.Produces[0].Amount
	w.Step()
	if b.Produces[0].Amount < before {
		t.Error("sated rabbit ate anyway")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -run 'Rabbit|Movement' -v`
Expected: FAIL (rabbit does nothing; stub AI).

- [ ] **Step 3: Write the implementation**

Delete the `aiStep` stub from `tick.go`. Create `internal/sim/ai.go`:
```go
package sim

import "fmt"

var neighbors = []Point{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func adjacent(a, b Point) bool { return Dist(a, b) <= 1 }

func (w *World) aiStep(e *Entity) []Event {
	s := w.cfg.Species[e.Species]

	// 1. danger (implemented in Task 5)
	if evs, fled := w.fleeStep(e); fled {
		return evs
	}

	// 2. food
	if e.Fullness < s.HungerThreshold {
		food := w.findFood(e)
		if food != nil {
			if adjacent(e.Pos, food.Pos) {
				return w.eatFrom(e, food)
			}
			e.Action = "seeking food"
			w.moveToward(e, food.Pos)
			return nil
		}
		e.Action = "searching"
		w.wander(e)
		return nil
	}

	// 3. shelter (implemented in Task 6)
	if w.shelterStep(e) {
		return nil
	}

	// 4. wander
	e.Action = "idle"
	if w.RandFloat() < 0.15 {
		w.wander(e)
	}
	return nil
}

func (w *World) findFood(e *Entity) *Entity {
	eats := map[string]bool{}
	for _, r := range w.cfg.Species[e.Species].Eats {
		eats[r] = true
	}
	var best *Entity
	bestD := 1 << 30
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID == e.ID || c.Species == e.Species {
			continue
		}
		edible := false
		for _, p := range c.Produces {
			if eats[p.Resource] && p.Amount >= 0.5 {
				edible = true
				break
			}
		}
		if !edible {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func (w *World) eatFrom(e *Entity, food *Entity) []Event {
	s := w.cfg.Species[e.Species]
	// Live fauna prey is killed first (Task 5 covers the hunt event path).
	if !food.Dead && w.cfg.Species[food.Species].Kind == "fauna" {
		return w.huntStrike(e, food)
	}
	eats := map[string]bool{}
	for _, r := range s.Eats {
		eats[r] = true
	}
	for i := range food.Produces {
		p := &food.Produces[i]
		if !eats[p.Resource] || p.Amount <= 0 {
			continue
		}
		bite := s.BiteSize
		if p.Amount < bite {
			bite = p.Amount
		}
		if room := s.StomachSize - e.Fullness; room < bite {
			bite = room
		}
		if bite <= 0 {
			return nil
		}
		p.Amount -= bite
		e.Fullness += bite
		e.Action = "eating"
		w.markDirty(e.ID)
		w.markDirty(food.ID)
		return []Event{{
			Tick: w.Tick, Type: "ate",
			Actor: e.ID, ActorSpecies: e.Species,
			Target: food.ID, TargetSpecies: food.Species,
			Msg: fmt.Sprintf("%s ate %s from %s", s.Name, p.Resource, w.cfg.Species[food.Species].Name),
		}}
	}
	return nil
}

func (w *World) moveToward(e *Entity, target Point) { w.move(e, target, false) }
func (w *World) moveAway(e *Entity, from Point)     { w.move(e, from, true) }

func (w *World) move(e *Entity, ref Point, away bool) {
	e.MoveAcc += w.cfg.Species[e.Species].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		best := e.Pos
		bestD := Dist(e.Pos, ref)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !Passable(w.At(p)) || w.FaunaAt(p) != nil {
				continue
			}
			d := Dist(p, ref)
			if (!away && d < bestD) || (away && d > bestD) {
				best, bestD = p, d
			}
		}
		if best == e.Pos {
			return
		}
		e.Pos = best
		w.markDirty(e.ID)
	}
}

func (w *World) wander(e *Entity) {
	e.MoveAcc += w.cfg.Species[e.Species].Speed
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		n := neighbors[w.RandN(len(neighbors))]
		p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
		if w.InBounds(p) && Passable(w.At(p)) && w.FaunaAt(p) == nil {
			e.Pos = p
			w.markDirty(e.ID)
		}
	}
}
```

Add temporary stubs at the bottom of `ai.go` (implemented in Tasks 5 and 6):
```go
func (w *World) fleeStep(e *Entity) ([]Event, bool) { return nil, false }
func (w *World) huntStrike(e *Entity, prey *Entity) []Event { return nil }
func (w *World) shelterStep(e *Entity) bool { return false }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sim/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add foraging AI with movement and flora eating"
```

---

### Task 5: Predation and fear: hunting, kills, corpses, fleeing

**Files:**
- Modify: `internal/sim/ai.go` (replace `fleeStep` and `huntStrike` stubs)
- Test: `internal/sim/predation_test.go`

**Interfaces:**
- Consumes: Tasks 2 to 4.
- Produces:
  - `(*World) isPredatorOf(hunter, prey *data.Species) bool`: different species, hunter fauna, hunter.Eats intersects prey.Produces resources.
  - `(*World) fleeStep(e *Entity) ([]Event, bool)`: nearest live predator within `FearRadius`; if found, `moveAway`, Action "fleeing", emit `fled` event only on the first tick of a flee (when Action was not already "fleeing").
  - `(*World) huntStrike(e *Entity, prey *Entity) []Event`: adjacency assumed by caller; kills prey via `w.kill(prey, "killed", ...)`, emits `hunted` event for hunter; hunter eats from the corpse on subsequent ticks via normal `eatFrom` path.

- [ ] **Step 1: Write the failing test**

`internal/sim/predation_test.go`:
```go
package sim

import "testing"

func TestWolfHuntsAndEatsRabbit(t *testing.T) {
	w := flatWorld(t, 12, 12, 1)
	r := w.Spawn("rabbit", Point{5, 5})
	r.Fullness = 10 // not hungry, so it only flees
	wolf := w.Spawn("wolf", Point{6, 5})
	wolf.Fullness = 1
	killed := false
	for i := 0; i < 200 && !killed; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "killed" && ev.Actor == r.ID {
				killed = true
			}
		}
	}
	if !killed {
		t.Fatal("wolf never killed rabbit")
	}
	full0 := wolf.Fullness
	for i := 0; i < 20; i++ {
		w.Step()
	}
	if wolf.Fullness <= full0 {
		t.Error("wolf did not eat from corpse")
	}
}

func TestRabbitFleesWolf(t *testing.T) {
	w := flatWorld(t, 20, 20, 1)
	r := w.Spawn("rabbit", Point{10, 10})
	r.Fullness = 10
	wolf := w.Spawn("wolf", Point{12, 10})
	wolf.Fullness = 16 // sated wolf stands around
	d0 := Dist(r.Pos, wolf.Pos)
	w.Step()
	if Dist(r.Pos, wolf.Pos) < d0 {
		t.Error("rabbit moved toward wolf")
	}
	if r.Action != "fleeing" {
		t.Errorf("action = %q, want fleeing", r.Action)
	}
}

func TestNoCannibalism(t *testing.T) {
	w := flatWorld(t, 8, 8, 1)
	a := w.Spawn("wolf", Point{2, 2})
	a.Fullness = 1
	b := w.Spawn("wolf", Point{3, 2})
	b.Fullness = 16
	w.Step()
	if b.Dead {
		t.Fatal("wolf ate wolf")
	}
	if a.Action == "fleeing" || b.Action == "fleeing" {
		t.Error("wolves fear each other")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -run 'Wolf|Flees|Cannibal' -v`
Expected: FAIL (stubs return nothing, wolf never kills; rabbit never flees).

- [ ] **Step 3: Write the implementation**

Delete the `fleeStep` and `huntStrike` stubs from `ai.go` and add (near the top of `ai.go`, plus `"cellarfloor/internal/data"` to its imports):
```go
func speciesEatsProduceOf(eater, victim *data.Species) bool {
	if eater.ID == victim.ID || eater.Kind != "fauna" {
		return false
	}
	prod := map[string]bool{}
	for _, p := range victim.Produces {
		prod[p.Resource] = true
	}
	for _, r := range eater.Eats {
		if prod[r] {
			return true
		}
	}
	return false
}

func (w *World) fleeStep(e *Entity) ([]Event, bool) {
	me := w.cfg.Species[e.Species]
	if me.FearRadius <= 0 {
		return nil, false
	}
	var threat *Entity
	bestD := me.FearRadius + 1
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.Dead || c.ID == e.ID {
			continue
		}
		cs := w.cfg.Species[c.Species]
		if !speciesEatsProduceOf(cs, me) {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			threat, bestD = c, d
		}
	}
	if threat == nil {
		return nil, false
	}
	var evs []Event
	if e.Action != "fleeing" {
		evs = append(evs, Event{
			Tick: w.Tick, Type: "fled",
			Actor: e.ID, ActorSpecies: e.Species,
			Target: threat.ID, TargetSpecies: threat.Species,
			Msg: fmt.Sprintf("%s fled from %s", me.Name, w.cfg.Species[threat.Species].Name),
		})
	}
	e.Action = "fleeing"
	w.moveAway(e, threat.Pos)
	return evs, true
}

func (w *World) huntStrike(e *Entity, prey *Entity) []Event {
	s := w.cfg.Species[e.Species]
	ev := w.kill(prey, "killed", fmt.Sprintf("%s was killed by %s", w.cfg.Species[prey.Species].Name, s.Name))
	ev.Target = e.ID
	ev.TargetSpecies = e.Species
	e.Action = "hunting"
	w.markDirty(e.ID)
	hunt := Event{
		Tick: w.Tick, Type: "hunted",
		Actor: e.ID, ActorSpecies: e.Species,
		Target: prey.ID, TargetSpecies: prey.Species,
		Msg: fmt.Sprintf("%s hunted down %s", s.Name, w.cfg.Species[prey.Species].Name),
	}
	return []Event{hunt, ev}
}
```

`speciesEatsProduceOf` is the single predator/prey primitive; use it everywhere, never a species-name comparison.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sim/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add emergent predation and fleeing behavior"
```

---

### Task 6: Shelter, reproduction, population guardrails

**Files:**
- Modify: `internal/sim/ai.go` (replace `shelterStep` stub), `internal/sim/tick.go` (replace `reproduceAndGuard` stub)
- Test: `internal/sim/shelter_test.go`

**Interfaces:**
- Consumes: Tasks 2 to 5.
- Produces:
  - `(*World) shelterStep(e *Entity) bool`: if `Home == nil`, find nearest entity producing any of `Shelters` resources and remember its position as `Home` (dirty). If `Dist(Pos, Home) > HomeRange`, move toward home, Action "going home", return true. Else return false.
  - `(*World) reproduceAndGuard() []Event`: pass 1 per fauna entity (sorted IDs): if alive, `Age > MatureAge`, `Fullness >= ReproThreshold`, `CountAlive(species) < PopCap`, and `RandFloat() < ReproChance`: spawn same species at a free passable adjacent tile (skip if none), parent `Fullness -= ReproCost`, emit `born`. Pass 2 per fauna species (sorted species IDs): while `CountAlive < PopFloor`, spawn at a random passable edge tile (up to 50 attempts), emit `spawned`.

- [ ] **Step 1: Write the failing test**

`internal/sim/shelter_test.go`:
```go
package sim

import "testing"

func TestRabbitRemembersHomeAndReturns(t *testing.T) {
	w := flatWorld(t, 20, 20, 1)
	w.Spawn("bush", Point{3, 3})
	r := w.Spawn("rabbit", Point{4, 3})
	r.Fullness = 10
	w.Step()
	if r.Home == nil {
		t.Fatal("rabbit did not adopt a home")
	}
	r.Pos = Point{18, 18}
	w.Step()
	if r.Action != "going home" {
		t.Errorf("action = %q", r.Action)
	}
	d0 := Dist(r.Pos, *r.Home)
	for i := 0; i < 10; i++ {
		w.Step()
	}
	if Dist(r.Pos, *r.Home) >= d0 {
		t.Error("rabbit not heading home")
	}
}

func TestReproduction(t *testing.T) {
	w := flatWorld(t, 10, 10, 7)
	r := w.Spawn("rabbit", Point{5, 5})
	r.Age = w.Cfg().Species["rabbit"].MatureAge + 1
	r.Fullness = 10
	born := false
	for i := 0; i < 3000 && !born; i++ {
		r.Fullness = 10
		r.Age = w.Cfg().Species["rabbit"].MatureAge + 1
		for _, ev := range w.Step() {
			if ev.Type == "born" {
				born = true
			}
		}
	}
	if !born {
		t.Fatal("no birth in 3000 fertile ticks")
	}
	if w.CountAlive("rabbit") < 2 {
		t.Error("baby not spawned")
	}
}

func TestPopulationFloor(t *testing.T) {
	w := flatWorld(t, 10, 10, 1)
	evs := w.Step() // zero rabbits and wolves: floors kick in
	if w.CountAlive("rabbit") < w.Cfg().Species["rabbit"].PopFloor {
		t.Errorf("rabbits = %d, floor = %d", w.CountAlive("rabbit"), w.Cfg().Species["rabbit"].PopFloor)
	}
	if w.CountAlive("wolf") < w.Cfg().Species["wolf"].PopFloor {
		t.Error("wolf floor not enforced")
	}
	found := false
	for _, e := range evs {
		if e.Type == "spawned" {
			found = true
		}
	}
	if !found {
		t.Error("no spawned events")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sim/ -run 'Home|Reproduction|Floor' -v`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

Replace `shelterStep` stub in `ai.go`:
```go
func (w *World) shelterStep(e *Entity) bool {
	s := w.cfg.Species[e.Species]
	if len(s.Shelters) == 0 {
		return false
	}
	if e.Home == nil {
		want := map[string]bool{}
		for _, r := range s.Shelters {
			want[r] = true
		}
		var best *Entity
		bestD := 1 << 30
		for _, id := range w.SortedIDs() {
			c := w.Entities[id]
			if c.ID == e.ID || c.Dead {
				continue
			}
			ok := false
			for _, p := range c.Produces {
				if want[p.Resource] {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			if d := Dist(e.Pos, c.Pos); d < bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			return false
		}
		h := best.Pos
		e.Home = &h
		w.markDirty(e.ID)
	}
	if Dist(e.Pos, *e.Home) > s.HomeRange {
		e.Action = "going home"
		w.moveToward(e, *e.Home)
		return true
	}
	return false
}
```

Replace `reproduceAndGuard` stub in `tick.go`:
```go
func (w *World) reproduceAndGuard() []Event {
	var events []Event

	// births
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		s := w.cfg.Species[e.Species]
		if e.Dead || s.Kind != "fauna" {
			continue
		}
		if e.Age <= s.MatureAge || e.Fullness < s.ReproThreshold {
			continue
		}
		if w.CountAlive(e.Species) >= s.PopCap {
			continue
		}
		if w.RandFloat() >= s.ReproChance {
			continue
		}
		var free *Point
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if w.InBounds(p) && Passable(w.At(p)) && w.FaunaAt(p) == nil {
				free = &p
				break
			}
		}
		if free == nil {
			continue
		}
		baby := w.Spawn(e.Species, *free)
		e.Fullness -= s.ReproCost
		w.markDirty(e.ID)
		events = append(events, Event{
			Tick: w.Tick, Type: "born",
			Actor: baby.ID, ActorSpecies: baby.Species,
			Target: e.ID, TargetSpecies: e.Species,
			Msg: fmt.Sprintf("a %s was born", s.Name),
		})
	}

	// floors
	speciesIDs := make([]string, 0, len(w.cfg.Species))
	for id := range w.cfg.Species {
		speciesIDs = append(speciesIDs, id)
	}
	sort.Strings(speciesIDs)
	for _, sid := range speciesIDs {
		s := w.cfg.Species[sid]
		if s.Kind != "fauna" || s.PopFloor <= 0 {
			continue
		}
		for w.CountAlive(sid) < s.PopFloor {
			p, ok := w.randomEdgeTile()
			if !ok {
				break
			}
			e := w.Spawn(sid, p)
			events = append(events, Event{
				Tick: w.Tick, Type: "spawned",
				Actor: e.ID, ActorSpecies: sid,
				Msg: fmt.Sprintf("a %s wandered in", s.Name),
			})
		}
	}
	return events
}

func (w *World) randomEdgeTile() (Point, bool) {
	for i := 0; i < 50; i++ {
		var p Point
		switch w.RandN(4) {
		case 0:
			p = Point{w.RandN(w.Width), 0}
		case 1:
			p = Point{w.RandN(w.Width), w.Height - 1}
		case 2:
			p = Point{0, w.RandN(w.Height)}
		default:
			p = Point{w.Width - 1, w.RandN(w.Height)}
		}
		if Passable(w.At(p)) && w.FaunaAt(p) == nil {
			return p, true
		}
	}
	return Point{}, false
}
```

Add `"sort"` to `tick.go` imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sim/ -v`
Expected: PASS. Note `TestPopulationFloor` may make earlier tests see floor-spawned animals; earlier tests in this plan were written with specific worlds and remain valid because floor spawns happen at map edges and earlier assertions do not count populations, except `TestOldAgeAndDecayRemoval` which checks `w.Removed`; floor spawn does not remove entities, so it still passes. If any earlier test breaks due to floor spawns interfering (e.g. a spawned wolf eats the test rabbit), pin that test by setting `PopFloor = 0` on the loaded config copy at the top of the test:

```go
cfg := testCfg(t)
for _, s := range cfg.Species { s.PopFloor = 0 }
w := NewWorld(8, 8, 1, cfg)
```

Apply that pattern to any older test that starts failing, and switch `flatWorld` helpers to accept a config. Simplest: change `flatWorld` to zero all floors by default and add `flatWorldFloors` used only by `TestPopulationFloor`:

```go
func flatWorld(t *testing.T, w, h int, seed uint64) *World {
	cfg := testCfg(t)
	for _, s := range cfg.Species {
		s.PopFloor = 0
	}
	return NewWorld(w, h, seed, cfg)
}

func flatWorldFloors(t *testing.T, w, h int, seed uint64) *World {
	return NewWorld(w, h, seed, testCfg(t))
}
```

Update `TestPopulationFloor` to use `flatWorldFloors`. Do this refactor now, not lazily.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add shelter homing, reproduction and population floors"
```

---

### Task 7: Procedural generation

**Files:**
- Create: `internal/gen/gen.go`
- Test: `internal/gen/gen_test.go`

**Interfaces:**
- Consumes: `data.Config`, `data.GenConfig`, `sim.NewWorld`, `sim.World.Spawn`, terrain constants.
- Produces: `gen.Generate(seed int64, cfg *data.Config) *sim.World`. Value-noise terrain (octaves, bilinear interpolation), thresholds from `GenConfig` (water below, rock above, dirt above, else grass), then scatter rules applied per tile in row-major order using the world's own RNG (world seeded with `uint64(seed)`), fauna only on free passable tiles.

- [ ] **Step 1: Write the failing test**

`internal/gen/gen_test.go`:
```go
package gen

import (
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func cfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	c, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGenerateDeterministic(t *testing.T) {
	c := cfg(t)
	a, b := Generate(123, c), Generate(123, c)
	for i := range a.Terrain {
		if a.Terrain[i] != b.Terrain[i] {
			t.Fatal("terrain differs for same seed")
		}
	}
	if len(a.Entities) != len(b.Entities) {
		t.Fatalf("entity counts differ: %d vs %d", len(a.Entities), len(b.Entities))
	}
	c2 := Generate(124, c)
	same := true
	for i := range a.Terrain {
		if a.Terrain[i] != c2.Terrain[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical terrain")
	}
}

func TestGenerateContents(t *testing.T) {
	w := Generate(123, cfg(t))
	if w.Width != 64 || w.Height != 64 {
		t.Fatalf("size %dx%d", w.Width, w.Height)
	}
	counts := map[string]int{}
	for _, e := range w.Entities {
		counts[e.Species]++
		if cfg(t).Species[e.Species].Kind == "fauna" && !sim.Passable(w.At(e.Pos)) {
			t.Errorf("%s spawned on impassable tile %v", e.Species, e.Pos)
		}
	}
	for _, s := range []string{"grass", "bush", "rabbit", "wolf"} {
		if counts[s] == 0 {
			t.Errorf("no %s generated", s)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gen/ -v`
Expected: FAIL, `Generate` undefined.

- [ ] **Step 3: Write the implementation**

`internal/gen/gen.go`:
```go
package gen

import (
	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

// valueNoise returns a deterministic pseudo-random float in [0,1) for a lattice point.
func lattice(seed int64, x, y int) float64 {
	h := uint64(seed)*0x9E3779B97F4A7C15 + uint64(x)*0xBF58476D1CE4E5B9 + uint64(y)*0x94D049BB133111EB
	h ^= h >> 30
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 27
	return float64(h>>11) / (1 << 53)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func noiseAt(seed int64, fx, fy float64) float64 {
	x0, y0 := int(fx), int(fy)
	tx, ty := smooth(fx-float64(x0)), smooth(fy-float64(y0))
	top := lerp(lattice(seed, x0, y0), lattice(seed, x0+1, y0), tx)
	bot := lerp(lattice(seed, x0, y0+1), lattice(seed, x0+1, y0+1), tx)
	return lerp(top, bot, ty)
}

func fractal(seed int64, x, y int, scale float64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	freq := 1.0 / scale
	for o := 0; o < octaves; o++ {
		sum += amp * noiseAt(seed+int64(o)*7919, float64(x)*freq, float64(y)*freq)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

func Generate(seed int64, cfg *data.Config) *sim.World {
	g := cfg.Gen
	w := sim.NewWorld(g.Width, g.Height, uint64(seed), cfg)

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

	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := sim.Point{X: x, Y: y}
			tname := sim.TerrainName(w.At(p))
			for _, rule := range g.Scatter {
				if rule.Terrain != tname {
					continue
				}
				if w.RandFloat() >= rule.Chance {
					continue
				}
				s := cfg.Species[rule.Species]
				if s.Kind == "fauna" {
					if !sim.Passable(w.At(p)) || w.FaunaAt(p) != nil {
						continue
					}
				}
				w.Spawn(rule.Species, p)
			}
		}
	}
	w.DirtyAndReset()
	return w
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gen/ -v`
Expected: PASS. If `TestGenerateContents` fails on a missing species, the noise thresholds and the 64x64 map should statistically guarantee all four; if wolf count is 0 for seed 123, bump `wolf` scatter chance in `data/gen.toml` to 0.003 and rerun.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add value noise terrain generation and species scatter"
```

---

### Task 8: 50k-tick stability test and tuning

**Files:**
- Create: `internal/sim/longrun_test.go`

**Interfaces:**
- Consumes: `gen.Generate`, real `data/` files, full sim.
- Produces: the permanent ecology regression check plus a whole-world determinism check.

- [ ] **Step 1: Write the test**

`internal/sim/longrun_test.go`:
```go
package sim_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
)

func loadCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestFiftyThousandTickStability(t *testing.T) {
	if testing.Short() {
		t.Skip("long run")
	}
	cfg := loadCfg(t)
	w := gen.Generate(2026, cfg)
	guardrailSpawns := 0
	popSum := map[string]int{}
	samples := 0
	for i := 0; i < 50000; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "spawned" {
				guardrailSpawns++
			}
		}
		if i%100 == 0 {
			for sid, s := range cfg.Species {
				if s.Kind == "fauna" {
					popSum[sid] += w.CountAlive(sid)
				}
			}
			samples++
		}
	}
	if guardrailSpawns > 200 {
		t.Errorf("ecology leans on guardrails: %d floor spawns in 50k ticks", guardrailSpawns)
	}
	for sid, s := range cfg.Species {
		if s.Kind != "fauna" {
			continue
		}
		avg := float64(popSum[sid]) / float64(samples)
		t.Logf("%s avg population %.1f (floor %d cap %d)", sid, avg, s.PopFloor, s.PopCap)
		if avg < float64(s.PopFloor) {
			t.Errorf("%s average %.1f below floor %d", sid, avg, s.PopFloor)
		}
		if n := w.CountAlive(sid); n > s.PopCap {
			t.Errorf("%s final population %d exceeds cap %d", sid, n, s.PopCap)
		}
	}
}

func TestWorldDeterminism(t *testing.T) {
	cfg := loadCfg(t)
	a, b := gen.Generate(99, cfg), gen.Generate(99, cfg)
	for i := 0; i < 2000; i++ {
		a.Step()
		b.Step()
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("same seed diverged after 2000 ticks")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/sim/ -run 'FiftyThousand|WorldDeterminism' -v -timeout 300s`
Expected: it runs to completion. It may FAIL on the stability assertions on the first attempt; that is the point of the test.

- [ ] **Step 3: Tune until green**

Tuning is data-only: edit `data/species.toml` and rerun. Knobs, in order of leverage:
1. Rabbit starving fast: raise grass `regrow` (0.02 to 0.04) or grass scatter chance.
2. Rabbits exploding to cap: lower `repro_chance` or raise `repro_cost`.
3. Wolves starving (most likely failure): raise rabbit `repro_chance` slightly, raise wolf `starve_ticks`, or lower wolf `metabolism`.
4. Wolves wiping rabbits out: lower wolf scatter chance, raise rabbit `fear_radius` to 6 or `speed` to 0.55.

Record each attempt's `t.Logf` averages in the commit message body if useful. Loop until the test passes twice in a row.

- [ ] **Step 4: Run the full suite**

Run: `go test ./... -timeout 300s`
Expected: PASS everywhere.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add 50k tick stability test and tune ecology balance"
```

---

### Task 9: Persistence

**Files:**
- Create: `internal/server/persist.go`
- Test: `internal/server/persist_test.go`

**Interfaces:**
- Consumes: `sim.World` (all exported fields JSON-tagged already), `data.Config`.
- Produces: `server.SaveWorld(w *sim.World, path string) error` (atomic: write `path+".tmp"`, rename); `server.LoadWorld(path string, cfg *data.Config) (*sim.World, error)` (unmarshals, calls `SetConfig`, returns `os.ErrNotExist` wrapped if missing).

- [ ] **Step 1: Write the failing test**

`internal/server/persist_test.go`:
```go
package server

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
)

func loadCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(5, cfg)
	for i := 0; i < 500; i++ {
		w.Step()
	}
	path := filepath.Join(t.TempDir(), "world.json")
	if err := SaveWorld(w, path); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadWorld(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w2.Tick != w.Tick || len(w2.Entities) != len(w.Entities) {
		t.Fatalf("round trip mismatch: tick %d vs %d, %d vs %d entities",
			w2.Tick, w.Tick, len(w2.Entities), len(w.Entities))
	}
	// loaded world must keep stepping identically to the original
	for i := 0; i < 200; i++ {
		w.Step()
		w2.Step()
	}
	if w2.Rng != w.Rng || len(w2.Entities) != len(w.Entities) {
		t.Error("loaded world diverged from original")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := LoadWorld(filepath.Join(t.TempDir(), "nope.json"), loadCfg(t))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -v`
Expected: FAIL, package missing.

- [ ] **Step 3: Write the implementation**

`internal/server/persist.go`:
```go
package server

import (
	"encoding/json"
	"os"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func SaveWorld(w *sim.World, path string) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadWorld(path string, cfg *data.Config) (*sim.World, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var w sim.World
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	w.SetConfig(cfg)
	return &w, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add atomic world save and load"
```

---

### Task 10: Server: protocol, hub, tick loop, timescale

**Files:**
- Create: `internal/server/protocol.go`, `internal/server/hub.go`, `internal/server/server.go`
- Test: `internal/server/protocol_test.go`

**Interfaces:**
- Consumes: sim, data, persist.
- Produces:
  - `protocol.go`: `EntityView{ID int; S string; X, Y int; Dead bool; Full float64; Action string; Home *sim.Point; Res map[string]float64}`; `ViewOf(e *sim.Entity) EntityView`; `SnapshotMsg{Type "snapshot"; Tick int64; Width, Height int; Terrain []uint8; Species map[string]*data.Species; Entities []EntityView; TimeScale int}`; `TickMsg{Type "tick"; Tick int64; TimeScale int; Changed []EntityView; Removed []int; Events []sim.Event; Pops map[string]int}`; `ClientMsg{Type string; Scale int}`; `BuildSnapshot(w *sim.World, scale int) SnapshotMsg`; `BuildTick(w *sim.World, events []sim.Event, scale int) TickMsg` (uses `w.DirtyAndReset()` and `w.Removed`).
  - `hub.go`: `Hub` with `Register(c *Client)`, `Unregister(c *Client)`, `Broadcast(b []byte)`; `Client{conn *websocket.Conn, send chan []byte}`; slow clients dropped (send channel full = unregister).
  - `server.go`: `Run(cfg *data.Config, w *sim.World, addr, staticDir string) error`: serves `staticDir` at `/`, WebSocket at `/ws`; owns tick loop goroutine: `time.Ticker` at `1/TickRate` seconds; each real tick runs `scale` sim steps (scale in {0,1,8,64}, atomic int, set by `ClientMsg{Type:"timescale"}`), collects all events and one merged `TickMsg` per real tick; per-tick `defer recover()` so the loop never dies; autosave every `AutosaveMinutes`; save on SIGINT then exit.

- [ ] **Step 1: Add dependency**

```bash
go get github.com/gorilla/websocket@latest
```

- [ ] **Step 2: Write the failing test**

`internal/server/protocol_test.go`:
```go
package server

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/gen"
)

func TestSnapshotAndTickMessages(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	snap := BuildSnapshot(w, 1)
	if snap.Type != "snapshot" || snap.Width != 64 || len(snap.Entities) == 0 {
		t.Fatalf("bad snapshot: %+v", snap.Type)
	}
	if _, ok := snap.Species["rabbit"]; !ok {
		t.Error("snapshot missing species table")
	}
	b, err := json.Marshal(snap)
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}

	evs := w.Step()
	tick := BuildTick(w, evs, 1)
	if tick.Type != "tick" || tick.Tick != w.Tick {
		t.Fatalf("bad tick msg")
	}
	if len(tick.Changed) == 0 {
		t.Error("expected changed entities on first tick")
	}
	if tick.Pops["rabbit"] == 0 {
		t.Error("pops missing")
	}
	// diff semantics: after building, dirty set is drained
	tick2 := BuildTick(w, nil, 1)
	if len(tick2.Changed) != 0 {
		t.Error("dirty set not drained by BuildTick")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/server/ -run Snapshot -v`
Expected: FAIL, `BuildSnapshot` undefined.

- [ ] **Step 4: Write protocol.go**

```go
package server

import (
	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

type EntityView struct {
	ID     int                `json:"id"`
	S      string             `json:"s"`
	X      int                `json:"x"`
	Y      int                `json:"y"`
	Dead   bool               `json:"dead,omitempty"`
	Full   float64            `json:"full"`
	Action string             `json:"action,omitempty"`
	Home   *sim.Point         `json:"home,omitempty"`
	Res    map[string]float64 `json:"res,omitempty"`
}

func ViewOf(e *sim.Entity) EntityView {
	res := map[string]float64{}
	for _, p := range e.Produces {
		res[p.Resource] = p.Amount
	}
	return EntityView{
		ID: e.ID, S: e.Species, X: e.Pos.X, Y: e.Pos.Y,
		Dead: e.Dead, Full: e.Fullness, Action: e.Action, Home: e.Home, Res: res,
	}
}

type SnapshotMsg struct {
	Type      string                   `json:"type"`
	Tick      int64                    `json:"tick"`
	Width     int                      `json:"width"`
	Height    int                      `json:"height"`
	Terrain   []uint8                  `json:"terrain"`
	Species   map[string]*data.Species `json:"species"`
	Entities  []EntityView             `json:"entities"`
	TimeScale int                      `json:"timeScale"`
}

type TickMsg struct {
	Type      string         `json:"type"`
	Tick      int64          `json:"tick"`
	TimeScale int            `json:"timeScale"`
	Changed   []EntityView   `json:"changed"`
	Removed   []int          `json:"removed"`
	Events    []sim.Event    `json:"events"`
	Pops      map[string]int `json:"pops"`
}

type ClientMsg struct {
	Type  string `json:"type"`
	Scale int    `json:"scale"`
}

func BuildSnapshot(w *sim.World, scale int) SnapshotMsg {
	terrain := make([]uint8, len(w.Terrain))
	for i, t := range w.Terrain {
		terrain[i] = uint8(t)
	}
	ents := make([]EntityView, 0, len(w.Entities))
	for _, id := range w.SortedIDs() {
		ents = append(ents, ViewOf(w.Entities[id]))
	}
	return SnapshotMsg{
		Type: "snapshot", Tick: w.Tick,
		Width: w.Width, Height: w.Height,
		Terrain: terrain, Species: w.Cfg().Species,
		Entities: ents, TimeScale: scale,
	}
}

func BuildTick(w *sim.World, events []sim.Event, scale int) TickMsg {
	changed := []EntityView{}
	for _, id := range w.DirtyAndReset() {
		if e, ok := w.Entities[id]; ok {
			changed = append(changed, ViewOf(e))
		}
	}
	pops := map[string]int{}
	for sid, s := range w.Cfg().Species {
		if s.Kind == "fauna" {
			pops[sid] = w.CountAlive(sid)
		}
	}
	if events == nil {
		events = []sim.Event{}
	}
	removed := append([]int{}, w.Removed...)
	return TickMsg{
		Type: "tick", Tick: w.Tick, TimeScale: scale,
		Changed: changed, Removed: removed, Events: events, Pops: pops,
	}
}
```

Caveat: `BuildTick` reads `w.Removed` from the last `Step` only; when the loop runs multiple sim steps per real tick (8x, 64x), the server must accumulate `Removed` and events across steps. `Run` handles that below by collecting after each `Step`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -v`
Expected: PASS.

- [ ] **Step 6: Write hub.go and server.go**

`internal/server/hub.go`:
```go
package server

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[*Client]bool
}

func NewHub() *Hub { return &Hub{clients: map[*Client]bool{}} }

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if h.clients[c] {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- b:
		default: // slow client: drop it
			delete(h.clients, c)
			close(c.send)
			log.Println("dropped slow client")
		}
	}
}
```

`internal/server/server.go`:
```go
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var validScales = map[int]bool{0: true, 1: true, 8: true, 64: true}

type Server struct {
	cfg   *data.Config
	world *sim.World
	hub   *Hub
	scale atomic.Int64
	mu    sync.Mutex // guards world during snapshot vs tick
}

func Run(cfg *data.Config, w *sim.World, addr, staticDir string) error {
	s := &Server{cfg: cfg, world: w, hub: NewHub()}
	s.scale.Store(1)

	go s.tickLoop()
	go s.autosaveLoop()
	go s.saveOnInterrupt()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	mux.HandleFunc("/ws", s.handleWS)
	log.Printf("cellar floor listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) tickLoop() {
	interval := time.Duration(float64(time.Second) / s.cfg.Sim.TickRate)
	t := time.NewTicker(interval)
	for range t.C {
		s.safeTick()
	}
}

func (s *Server) safeTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tick panic recovered: %v", r)
		}
	}()
	scale := int(s.scale.Load())
	if scale == 0 {
		return
	}
	s.mu.Lock()
	var events []sim.Event
	var removed []int
	for i := 0; i < scale; i++ {
		events = append(events, s.world.Step()...)
		removed = append(removed, s.world.Removed...)
	}
	msg := BuildTick(s.world, events, scale)
	msg.Removed = removed
	s.mu.Unlock()
	if len(msg.Events) > 200 {
		msg.Events = msg.Events[len(msg.Events)-200:]
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal tick: %v", err)
		return
	}
	s.hub.Broadcast(b)
}

func (s *Server) autosaveLoop() {
	if s.cfg.Sim.AutosaveMinutes <= 0 {
		return
	}
	t := time.NewTicker(time.Duration(s.cfg.Sim.AutosaveMinutes) * time.Minute)
	for range t.C {
		s.save()
	}
}

func (s *Server) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := SaveWorld(s.world, s.cfg.Sim.SavePath); err != nil {
		log.Printf("save: %v", err)
	} else {
		log.Printf("world saved at tick %d", s.world.Tick)
	}
}

func (s *Server) saveOnInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	s.save()
	os.Exit(0)
}

func (s *Server) handleWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}
	c := &Client{conn: conn, send: make(chan []byte, 64)}
	s.hub.Register(c)

	s.mu.Lock()
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load())))
	s.mu.Unlock()
	if err == nil {
		c.send <- snap
	}

	go func() { // writer
		for b := range c.send {
			if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				break
			}
		}
		c.conn.Close()
	}()

	go func() { // reader
		defer s.hub.Unregister(c)
		for {
			_, b, err := c.conn.ReadMessage()
			if err != nil {
				return
			}
			var m ClientMsg
			if json.Unmarshal(b, &m) != nil {
				log.Printf("bad client message ignored: %s", b)
				continue
			}
			if m.Type == "timescale" && validScales[m.Scale] {
				s.scale.Store(int64(m.Scale))
			}
		}
	}()
}
```

Known subtlety: `BuildSnapshot` sends entities from the live dirty world, and a `DirtyAndReset` from a concurrent `BuildTick` is serialized by `s.mu`, so a new client may receive one tick msg containing entities it already has in the snapshot; the client treats `changed` as upsert, so this is harmless.

- [ ] **Step 7: Build everything**

Run: `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "Add websocket server with tick loop, timescale and autosave"
```

---

### Task 11: Client core: scaffold, networking, canvas rendering

**Files:**
- Create: `client/package.json`, `client/tsconfig.json`, `client/vite.config.ts`, `client/index.html`, `client/src/types.ts`, `client/src/world.ts`, `client/src/net.ts`, `client/src/render.ts`, `client/src/main.ts`

**Interfaces:**
- Consumes: WebSocket protocol from Task 10 (snapshot and tick JSON shapes, field names exactly as the Go json tags: `id, s, x, y, dead, full, action, home, res`; species JSON uses camelCase tags from Task 1).
- Produces: a Vite app that builds to `client/dist`; `world.ts` exports `WorldState` singleton with `applySnapshot(msg)`, `applyTick(msg)`; `render.ts` exports `startRender(canvas, world)` drawing at 60fps with per-entity position interpolation between server ticks; `main.ts` wires everything. `ui.ts` (Task 12) hooks: `world.onEvents(cb)`, `world.onPops(cb)`, `world.onChange(cb)`.

- [ ] **Step 1: Scaffold**

`client/package.json`:
```json
{
  "name": "cellar-floor-client",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "typescript": "^5.5.0",
    "vite": "^5.4.0"
  }
}
```

`client/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

`client/vite.config.ts`:
```ts
import { defineConfig } from "vite";

export default defineConfig({
  server: {
    proxy: { "/ws": { target: "ws://localhost:8080", ws: true } },
  },
});
```

`client/index.html`:
```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Cellar Floor</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; background: #14110f; color: #cfc9bf; font: 13px/1.4 ui-monospace, monospace; display: flex; height: 100vh; }
    #map { flex: 1; display: flex; align-items: center; justify-content: center; min-width: 0; }
    canvas { image-rendering: pixelated; max-width: 100%; max-height: 100%; }
    #side { width: 320px; border-left: 1px solid #333; padding: 10px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
    h2 { font-size: 12px; text-transform: uppercase; letter-spacing: 1px; color: #8d857a; margin: 0 0 4px; }
    #timescale button { background: #262220; color: #cfc9bf; border: 1px solid #444; padding: 4px 10px; cursor: pointer; }
    #timescale button.active { background: #4a7c3f; color: #fff; }
    #inspector { min-height: 120px; white-space: pre-wrap; }
    #events { flex: 1; overflow-y: auto; min-height: 100px; }
    #events div { padding: 1px 0; border-bottom: 1px dotted #2a2a2a; }
    .pop-row { display: flex; align-items: center; gap: 8px; }
    .pop-row canvas { width: 120px; height: 24px; }
  </style>
</head>
<body>
  <div id="map"><canvas id="canvas"></canvas></div>
  <div id="side">
    <div id="timescale"><h2>Speed</h2></div>
    <div><h2>Populations</h2><div id="pops"></div></div>
    <div><h2>Inspector</h2><div id="inspector">click a creature</div></div>
    <div style="flex:1;display:flex;flex-direction:column;min-height:0">
      <h2>Events</h2><div id="events"></div>
    </div>
  </div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
```

Run: `cd client && npm install`

- [ ] **Step 2: Write types.ts and world.ts**

`client/src/types.ts`:
```ts
export interface Species {
  id: string;
  name: string;
  kind: "flora" | "fauna";
  color: string;
  stomachSize: number;
  fearRadius: number;
  popFloor: number;
  popCap: number;
  eats: string[] | null;
  shelters: string[] | null;
}

export interface EntityView {
  id: number;
  s: string;
  x: number;
  y: number;
  dead?: boolean;
  full: number;
  action?: string;
  home?: { x: number; y: number };
  res?: Record<string, number>;
}

export interface SimEvent {
  tick: number;
  type: string;
  actor: number;
  actorSpecies: string;
  target?: number;
  targetSpecies?: string;
  msg: string;
}

export interface SnapshotMsg {
  type: "snapshot";
  tick: number;
  width: number;
  height: number;
  terrain: number[];
  species: Record<string, Species>;
  entities: EntityView[];
  timeScale: number;
}

export interface TickMsg {
  type: "tick";
  tick: number;
  timeScale: number;
  changed: EntityView[];
  removed: number[];
  events: SimEvent[];
  pops: Record<string, number>;
}

export interface RenderEntity extends EntityView {
  px: number; // previous x/y for interpolation
  py: number;
  movedAt: number; // performance.now() of last move
}
```

`client/src/world.ts`:
```ts
import type { EntityView, RenderEntity, SimEvent, SnapshotMsg, TickMsg, Species } from "./types";

type Cb = () => void;

export class WorldState {
  width = 0;
  height = 0;
  terrain: number[] = [];
  species: Record<string, Species> = {};
  entities = new Map<number, RenderEntity>();
  tick = 0;
  timeScale = 1;
  tickIntervalMs = 500;
  popHistory: Record<string, number[]> = {};
  selectedId: number | null = null;

  private eventCbs: ((evs: SimEvent[]) => void)[] = [];
  private changeCbs: Cb[] = [];

  onEvents(cb: (evs: SimEvent[]) => void) { this.eventCbs.push(cb); }
  onChange(cb: Cb) { this.changeCbs.push(cb); }
  private fireChange() { for (const cb of this.changeCbs) cb(); }

  applySnapshot(m: SnapshotMsg) {
    this.width = m.width;
    this.height = m.height;
    this.terrain = m.terrain;
    this.species = m.species;
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    this.entities.clear();
    for (const e of m.entities) this.upsert(e);
    this.fireChange();
  }

  applyTick(m: TickMsg) {
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    for (const e of m.changed) this.upsert(e);
    for (const id of m.removed) this.entities.delete(id);
    for (const [sid, n] of Object.entries(m.pops)) {
      (this.popHistory[sid] ??= []).push(n);
      if (this.popHistory[sid].length > 120) this.popHistory[sid].shift();
    }
    if (m.events.length) for (const cb of this.eventCbs) cb(m.events);
    this.fireChange();
  }

  private upsert(e: EntityView) {
    const prev = this.entities.get(e.id);
    const re = e as RenderEntity;
    if (prev && (prev.x !== e.x || prev.y !== e.y)) {
      re.px = prev.x; re.py = prev.y; re.movedAt = performance.now();
    } else {
      re.px = prev?.px ?? e.x; re.py = prev?.py ?? e.y; re.movedAt = prev?.movedAt ?? 0;
    }
    this.entities.set(e.id, re);
  }
}

export const world = new WorldState();
```

- [ ] **Step 3: Write net.ts and render.ts**

`client/src/net.ts`:
```ts
import { world } from "./world";
import type { SnapshotMsg, TickMsg } from "./types";

let ws: WebSocket | null = null;

export function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data) as SnapshotMsg | TickMsg;
    if (msg.type === "snapshot") world.applySnapshot(msg);
    else if (msg.type === "tick") world.applyTick(msg);
  };
  ws.onclose = () => setTimeout(connect, 1000);
}

export function sendTimescale(scale: number) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "timescale", scale }));
}
```

`client/src/render.ts`:
```ts
import { world } from "./world";

const TILE = 12;
const TERRAIN_COLORS = ["#3d5a36", "#6b5537", "#2b4a63", "#5a5a5a"]; // grass dirt water rock

let terrainCanvas: HTMLCanvasElement | null = null;

function renderTerrain() {
  terrainCanvas = document.createElement("canvas");
  terrainCanvas.width = world.width * TILE;
  terrainCanvas.height = world.height * TILE;
  const g = terrainCanvas.getContext("2d")!;
  for (let y = 0; y < world.height; y++) {
    for (let x = 0; x < world.width; x++) {
      g.fillStyle = TERRAIN_COLORS[world.terrain[y * world.width + x]] ?? "#000";
      g.fillRect(x * TILE, y * TILE, TILE, TILE);
    }
  }
}

export function startRender(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext("2d")!;
  world.onChange(() => {
    if (!terrainCanvas || terrainCanvas.width !== world.width * TILE) renderTerrain();
    canvas.width = world.width * TILE;
    canvas.height = world.height * TILE;
  });

  function frame(now: number) {
    if (terrainCanvas) {
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(terrainCanvas, 0, 0);
      const lerpMs = world.tickIntervalMs / Math.max(world.timeScale, 1);
      for (const e of world.entities.values()) {
        const sp = world.species[e.s];
        if (!sp) continue;
        const t = Math.min(1, (now - e.movedAt) / lerpMs);
        const x = (e.px + (e.x - e.px) * t) * TILE;
        const y = (e.py + (e.y - e.py) * t) * TILE;
        ctx.fillStyle = e.dead ? "#443c38" : sp.color;
        if (sp.kind === "flora") {
          ctx.fillRect(x + 2, y + 2, TILE - 4, TILE - 4);
        } else {
          ctx.beginPath();
          ctx.arc(x + TILE / 2, y + TILE / 2, TILE / 2 - 1, 0, Math.PI * 2);
          ctx.fill();
        }
        if (e.id === world.selectedId) {
          ctx.strokeStyle = "#ffd75e";
          ctx.lineWidth = 2;
          ctx.strokeRect(x - 1, y - 1, TILE + 2, TILE + 2);
        }
      }
    }
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

export function tileFromPixel(canvas: HTMLCanvasElement, cx: number, cy: number) {
  const r = canvas.getBoundingClientRect();
  const sx = canvas.width / r.width, sy = canvas.height / r.height;
  return {
    x: Math.floor(((cx - r.left) * sx) / TILE),
    y: Math.floor(((cy - r.top) * sy) / TILE),
  };
}
```

- [ ] **Step 4: Write main.ts (minimal, UI lands in Task 12)**

`client/src/main.ts`:
```ts
import { connect } from "./net";
import { startRender, tileFromPixel } from "./render";
import { world } from "./world";
import { initUI } from "./ui";

const canvas = document.getElementById("canvas") as HTMLCanvasElement;
startRender(canvas);
connect();
initUI(canvas, tileFromPixel);
```

Create a placeholder `client/src/ui.ts` so it compiles (Task 12 replaces it):
```ts
export function initUI(
  _canvas: HTMLCanvasElement,
  _tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {}
```

- [ ] **Step 5: Verify it builds**

Run: `cd client && npm run build`
Expected: `tsc` clean, `vite build` outputs `client/dist/`.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "Add TypeScript canvas client with networking and interpolation"
```

---

### Task 12: Client UI: inspector, event feed, sparklines, timescale

**Files:**
- Modify: `client/src/ui.ts` (replace placeholder entirely)

**Interfaces:**
- Consumes: `world` singleton, `sendTimescale`, `tileFromPixel`, DOM ids from index.html: `timescale`, `pops`, `inspector`, `events`.
- Produces: full spectator UI.

- [ ] **Step 1: Write ui.ts**

```ts
import { world } from "./world";
import { sendTimescale } from "./net";
import type { SimEvent } from "./types";

export function initUI(
  canvas: HTMLCanvasElement,
  tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {
  initTimescale();
  initEvents();
  initInspector(canvas, tileFromPixel);
  world.onChange(renderPops);
  world.onChange(renderInspector);
}

function initTimescale() {
  const box = document.getElementById("timescale")!;
  for (const s of [0, 1, 8, 64]) {
    const b = document.createElement("button");
    b.textContent = s === 0 ? "pause" : `${s}x`;
    b.dataset.scale = String(s);
    b.onclick = () => sendTimescale(s);
    box.appendChild(b);
  }
  world.onChange(() => {
    for (const b of box.querySelectorAll("button")) {
      b.classList.toggle("active", Number(b.dataset.scale) === world.timeScale);
    }
  });
}

function initEvents() {
  const box = document.getElementById("events")!;
  world.onEvents((evs: SimEvent[]) => {
    for (const ev of evs) {
      const d = document.createElement("div");
      d.textContent = `[${ev.tick}] ${ev.msg}`;
      box.prepend(d);
    }
    while (box.children.length > 100) box.lastChild!.remove();
  });
}

const sparks: Record<string, HTMLCanvasElement> = {};

function renderPops() {
  const box = document.getElementById("pops")!;
  for (const [sid, hist] of Object.entries(world.popHistory)) {
    let c = sparks[sid];
    if (!c) {
      const row = document.createElement("div");
      row.className = "pop-row";
      const label = document.createElement("span");
      label.style.color = world.species[sid]?.color ?? "#fff";
      label.dataset.sid = sid;
      c = document.createElement("canvas");
      c.width = 120;
      c.height = 24;
      sparks[sid] = c;
      row.append(label, c);
      box.appendChild(row);
    }
    const label = c.parentElement!.querySelector("span")!;
    label.textContent = `${world.species[sid]?.name ?? sid}: ${hist[hist.length - 1] ?? 0}`;
    const g = c.getContext("2d")!;
    g.clearRect(0, 0, c.width, c.height);
    const max = Math.max(...hist, world.species[sid]?.popCap ?? 1);
    g.strokeStyle = world.species[sid]?.color ?? "#fff";
    g.beginPath();
    hist.forEach((v, i) => {
      const x = (i / 119) * c.width;
      const y = c.height - (v / max) * (c.height - 2) - 1;
      i === 0 ? g.moveTo(x, y) : g.lineTo(x, y);
    });
    g.stroke();
  }
}

function initInspector(
  canvas: HTMLCanvasElement,
  tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {
  canvas.addEventListener("click", (ev) => {
    const t = tileFromPixel(canvas, ev.clientX, ev.clientY);
    let picked: number | null = null;
    let bestD = 3;
    for (const e of world.entities.values()) {
      const sp = world.species[e.s];
      const d = Math.max(Math.abs(e.x - t.x), Math.abs(e.y - t.y));
      // prefer fauna over flora on the same tile
      const score = d + (sp?.kind === "flora" ? 0.5 : 0);
      if (score < bestD) {
        bestD = score;
        picked = e.id;
      }
    }
    world.selectedId = picked;
    renderInspector();
  });
}

function renderInspector() {
  const box = document.getElementById("inspector")!;
  const e = world.selectedId ? world.entities.get(world.selectedId) : null;
  if (!e) {
    box.textContent = "click a creature";
    return;
  }
  const sp = world.species[e.s];
  const lines = [
    `${sp?.name ?? e.s} #${e.id}${e.dead ? " (dead)" : ""}`,
    `at ${e.x},${e.y}`,
  ];
  if (sp?.kind === "fauna" && !e.dead) {
    lines.push(`fullness ${e.full.toFixed(1)} / ${sp.stomachSize}`);
    lines.push(`doing: ${e.action || "idle"}`);
    if (e.home) lines.push(`home: ${e.home.x},${e.home.y}`);
  }
  if (e.res) {
    for (const [r, v] of Object.entries(e.res)) lines.push(`${r}: ${v.toFixed(1)}`);
  }
  box.textContent = lines.join("\n");
}
```

- [ ] **Step 2: Verify it builds**

Run: `cd client && npm run build`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "Add inspector, event feed, sparklines and timescale UI"
```

---

### Task 13: Wiring: main.go, README, end-to-end verification

**Files:**
- Create: `cmd/cellarfloor/main.go`, `README.md`

**Interfaces:**
- Consumes: everything.
- Produces: `go run ./cmd/cellarfloor` serves the whole app on :8080.

- [ ] **Step 1: Write main.go**

```go
package main

import (
	"errors"
	"flag"
	"io/fs"
	"log"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
	"cellarfloor/internal/server"
	"cellarfloor/internal/sim"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "data", "data directory")
	staticDir := flag.String("static", "client/dist", "client build directory")
	seed := flag.Int64("seed", 2026, "world seed when no save exists")
	fresh := flag.Bool("fresh", false, "ignore existing save and regenerate")
	flag.Parse()

	cfg, err := data.Load(*dataDir)
	if err != nil {
		log.Fatalf("data: %v", err)
	}

	var w *sim.World
	if !*fresh {
		w, err = server.LoadWorld(cfg.Sim.SavePath, cfg)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("save file unreadable (%v), regenerating", err)
		}
	}
	if w == nil {
		w = gen.Generate(*seed, cfg)
		log.Printf("generated world from seed %d: %d entities", *seed, len(w.Entities))
	} else {
		log.Printf("loaded world at tick %d: %d entities", w.Tick, len(w.Entities))
	}

	log.Fatal(server.Run(cfg, w, *addr, *staticDir))
}
```

- [ ] **Step 2: Write README.md**

```markdown
# Cellar Floor

A little world living on the floor of a dark cellar. A persistent ecology
of grass, bushes, trees, rabbits and wolves, simulated on a Go server and
watched live from the browser.

Spec: docs/superpowers/specs/2026-07-08-cellar-floor-design.md

## Run

    cd client && npm install && npm run build && cd ..
    go run ./cmd/cellarfloor

Open http://localhost:8080. Flags: -addr, -seed, -fresh, -data, -static.

## Develop

    go test ./...                  # includes the 50k-tick stability test
    go test ./... -short           # skip the long run
    cd client && npm run dev       # Vite dev server proxying /ws to :8080

All species and balance live in data/*.toml. The engine knows nothing
about rabbits.
```

- [ ] **Step 3: Full test suite and build**

Run: `go test ./... -timeout 300s && go build ./... && cd client && npm run build`
Expected: all PASS, clean builds.

- [ ] **Step 4: Manual end-to-end verification**

1. `go run ./cmd/cellarfloor -fresh`
2. Open http://localhost:8080 in a browser: terrain visible, creatures moving smoothly, populations and sparklines updating, events scrolling.
3. Click a rabbit: inspector shows fullness, action, home.
4. Click 64x: populations visibly wave; click pause: everything freezes.
5. Open a second browser window: same world, same time scale.
6. Ctrl+C the server (it saves), restart without -fresh: world resumes at the saved tick.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Wire main entrypoint and add README"
```

---

## Self-Review Notes

- Spec coverage: data-driven principle (Task 1), world model + procgen (Tasks 2, 7), resource schema (Tasks 1, 2), emergent AI (Tasks 4, 5, 6), tick loop + determinism + guardrails + events (Tasks 3, 6, 8), spectator client with inspector, feed, sparklines, timescale (Tasks 11, 12), protocol snapshot/diff including species table (Task 10), persistence (Task 9), error handling in tick loop and WS (Task 10), 50k stability test (Task 8). Desires load and validate (Task 1) but no v1 species uses them, matching the spec.
- Type consistency: `speciesEatsProduceOf` is the single predator/prey primitive; `EntityView` JSON field names match `types.ts`; `data.Species` json tags are camelCase and `types.ts` mirrors the subset it reads.
- Known simplifications accepted for v1: O(n) entity scans, insertion-sort for dirty IDs, whole-entity views in diffs, global mutex around world.
