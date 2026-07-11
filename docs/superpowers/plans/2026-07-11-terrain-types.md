# Terrain Types Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Terrain moves into `data/terrain.toml`; soft rock generates as connected blob veins and mines in a quarter of the time; the client renders terrain colors from data.

**Architecture:** `data.Config` gains an ordered `Terrain []TerrainType` table (position = save/wire byte, append-only, first five pinned to today's order). `sim.Passable/Mineable/TerrainName` become World methods reading the table; `mineStep` scales a cell's total time by the terrain's `mine_factor`. Gen grows blob veins from seed cells through the world RNG. The snapshot carries the table; the client's hardcoded `TERRAIN_COLORS` dies.

**Tech Stack:** Go stdlib + BurntSushi/toml, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-11-terrain-types-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on Go tasks: failing test first, see it fail, implement, see it pass, commit. `set -o pipefail` when piping `go test`.
- terrain.toml order is the wire/save byte: the first five entries MUST be grass, dirt, water, rock, floor (validated), and evolution is append-only. Old saves must load with no reset.
- Exact soft rock values: id `soft_rock`, color `#575049`, mineable, `mine_factor = 0.25`. Rock keeps color `#3a3a3a` with `mine_factor = 1.0`. Canonical colors: grass `#3d5a36`, dirt `#6b5537`, water `#2b4a63`, floor `#26221e`.
- Vein rule values in data/gen.toml: `veins = [{ terrain = "soft_rock", seeds = 10, size = 14 }]`; veins only replace plain rock; deterministic through the world RNG.
- The engine stays generic: no terrain ids or entity-type names hardcoded in internal/sim or internal/gen beyond the five pinned Go constants.
- Never touch the live server (port 8080) or canonical world.json/players.json; e2e uses :8083 with a scratch data dir owning its save_path.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: Terrain table in the data layer

**Files:**
- Create: `data/terrain.toml`
- Create: `internal/sim/testdata/legacy/terrain.toml`
- Modify: `internal/data/data.go`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: 

```go
type TerrainType struct {
	ID         string  `toml:"id" json:"id"`
	Color      string  `toml:"color" json:"color"`
	Passable   bool    `toml:"passable" json:"passable"`
	Mineable   bool    `toml:"mineable" json:"mineable"`
	MineFactor float64 `toml:"mine_factor" json:"mineFactor"`
}
```

`Config.Terrain []TerrainType`; `(c *Config) TerrainIndex(id string) (int, bool)`; `CanonicalTerrain() []TerrainType` (exported helper returning the pinned five, used by in-code test configs across packages); `GenConfig.Veins []VeinRule` with `type VeinRule struct { Terrain string toml:"terrain"; Seeds int toml:"seeds"; Size int toml:"size" }`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/data/data_test.go`:

```go
func TestTerrainTableParses(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Terrain) != 6 {
		t.Fatalf("terrain types = %d, want 6", len(cfg.Terrain))
	}
	want := []string{"grass", "dirt", "water", "rock", "floor", "soft_rock"}
	for i, id := range want {
		if cfg.Terrain[i].ID != id {
			t.Fatalf("terrain[%d] = %q, want %q", i, cfg.Terrain[i].ID, id)
		}
	}
	soft := cfg.Terrain[5]
	if !soft.Mineable || soft.Passable || soft.MineFactor != 0.25 || soft.Color != "#575049" {
		t.Fatalf("soft_rock wrong: %+v", soft)
	}
	if i, ok := cfg.TerrainIndex("soft_rock"); !ok || i != 5 {
		t.Fatalf("TerrainIndex soft_rock = %d %v", i, ok)
	}
	if len(cfg.Gen.Veins) != 1 || cfg.Gen.Veins[0].Terrain != "soft_rock" ||
		cfg.Gen.Veins[0].Seeds != 10 || cfg.Gen.Veins[0].Size != 14 {
		t.Fatalf("veins wrong: %+v", cfg.Gen.Veins)
	}
}

func TestTerrainTableValidation(t *testing.T) {
	base := func() *Config {
		cfg := minimalConfig()
		cfg.Terrain = CanonicalTerrain()
		return cfg
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("canonical table should validate: %v", err)
	}
	cfg := base()
	cfg.Terrain[0], cfg.Terrain[1] = cfg.Terrain[1], cfg.Terrain[0]
	if err := Validate(cfg); err == nil {
		t.Fatal("reordered canonical terrain must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "rock", Color: "#111"})
	if err := Validate(cfg); err == nil {
		t.Fatal("duplicate id must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Mineable: true, MineFactor: 0.5})
	if err := Validate(cfg); err == nil {
		t.Fatal("missing color must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("mineable without positive mine_factor must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true, MineFactor: 1, Passable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("passable and mineable together must fail")
	}
	cfg = base()
	cfg.Gen.Veins = []VeinRule{{Terrain: "unobtanium", Seeds: 1, Size: 2}}
	if err := Validate(cfg); err == nil {
		t.Fatal("vein referencing unknown terrain must fail")
	}
}
```

Note: `minimalConfig()` exists and returns a Config without a Terrain table today; the new validation requiring the canonical five means every OTHER existing test config (including `minimalConfig` itself and the fixtures loaded by TestMiningFieldsParse and TestUnitFieldsConvertToTicks, which write their own data dirs) must gain the table too. Update `minimalConfig` to set `Terrain: CanonicalTerrain()` and give the temp-dir fixtures a minimal terrain.toml (the canonical five). This ripple is intended and part of this step.

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/data/ -run TestTerrainTable 2>&1 | tail -3`
Expected: build FAIL (TerrainType undefined).

- [ ] **Step 3: Implement**

`internal/data/data.go`:

- Add `TerrainType` and `VeinRule` structs exactly as in Interfaces; `GenConfig` gains `Veins []VeinRule toml:"veins"`.
- `Config` gains `Terrain []TerrainType`.
- Add:

```go
// CanonicalTerrain returns the five pinned base types in wire order.
func CanonicalTerrain() []TerrainType {
	return []TerrainType{
		{ID: "grass", Color: "#3d5a36", Passable: true},
		{ID: "dirt", Color: "#6b5537", Passable: true},
		{ID: "water", Color: "#2b4a63"},
		{ID: "rock", Color: "#3a3a3a", Mineable: true, MineFactor: 1},
		{ID: "floor", Color: "#26221e", Passable: true},
	}
}

func (c *Config) TerrainIndex(id string) (int, bool) {
	for i, t := range c.Terrain {
		if t.ID == id {
			return i, true
		}
	}
	return 0, false
}
```

- In `Load`, decode the new file after gen.toml:

```go
	var tt struct {
		Terrain []TerrainType `toml:"terrain"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "terrain.toml"), &tt); err != nil {
		return nil, fmt.Errorf("terrain.toml: %w", err)
	}
	cfg.Terrain = tt.Terrain
```

- In `Validate`, replace the `validTerrains` map (delete it) with table checks placed before the per-type loop:

```go
	canon := CanonicalTerrain()
	if len(cfg.Terrain) < len(canon) {
		return fmt.Errorf("terrain: table needs at least the %d canonical types", len(canon))
	}
	seen := map[string]bool{}
	for i, tt := range cfg.Terrain {
		if tt.ID == "" || tt.Color == "" {
			return fmt.Errorf("terrain[%d]: id and color are required", i)
		}
		if seen[tt.ID] {
			return fmt.Errorf("terrain: duplicate id %q", tt.ID)
		}
		seen[tt.ID] = true
		if i < len(canon) && tt.ID != canon[i].ID {
			return fmt.Errorf("terrain[%d] must be %q (saves store indices; append only), got %q", i, canon[i].ID, tt.ID)
		}
		if tt.Mineable && tt.MineFactor <= 0 {
			return fmt.Errorf("terrain %s: mineable needs positive mine_factor", tt.ID)
		}
		if tt.Mineable && tt.Passable {
			return fmt.Errorf("terrain %s: cannot be both passable and mineable", tt.ID)
		}
	}
```

- Scatter and vein references validate through the table:

```go
	for _, r := range cfg.Gen.Scatter {
		if _, ok := cfg.Types[r.Type]; !ok {
			return fmt.Errorf("scatter rule references unknown type %q", r.Type)
		}
		if _, ok := cfg.TerrainIndex(r.Terrain); !ok {
			return fmt.Errorf("scatter rule references unknown terrain %q", r.Terrain)
		}
	}
	for _, v := range cfg.Gen.Veins {
		idx, ok := cfg.TerrainIndex(v.Terrain)
		if !ok {
			return fmt.Errorf("vein rule references unknown terrain %q", v.Terrain)
		}
		if !cfg.Terrain[idx].Mineable {
			return fmt.Errorf("vein terrain %q must be mineable", v.Terrain)
		}
		if v.Seeds < 0 || v.Size < 1 {
			return fmt.Errorf("vein %q needs non-negative seeds and size >= 1", v.Terrain)
		}
	}
```

Create `data/terrain.toml` exactly per the spec's Data shape section (six entries, comment about append-only on top). Create `internal/sim/testdata/legacy/terrain.toml` with only the canonical five (same file minus soft_rock). Add the veins line to `data/gen.toml`. Update `minimalConfig` and the two temp-dir test fixtures per the Step 1 note.

- [ ] **Step 4: Run to verify pass**

Run: `set -o pipefail; go test ./internal/data/ 2>&1 | tail -2`
Expected: PASS (other packages will not compile until Task 2 only if something in them referenced validTerrains; they do not, so `go build ./...` should also pass; run it).

- [ ] **Step 5: Commit**

```bash
git add internal/data/ data/terrain.toml data/gen.toml internal/sim/testdata/legacy/terrain.toml
git commit -m "Load an ordered terrain table from terrain.toml"
```

---

### Task 2: World methods for terrain and mine_factor in mining

**Files:**
- Modify: `internal/sim/world.go`, `internal/sim/mine.go`, `internal/sim/ai.go`, `internal/sim/tick.go`
- Modify: `internal/server/torch.go`, `internal/gen/gen.go` (call sites)
- Test: `internal/sim/terrain_test.go`, `internal/sim/mine_test.go`, plus updating every in-code test config

**Interfaces:**
- Consumes: `cfg.Terrain`, `data.CanonicalTerrain()`.
- Produces: `(w *World) Passable(t Terrain) bool`, `(w *World) Mineable(t Terrain) bool`, `(w *World) TerrainName(t Terrain) string` replacing the package functions (which are deleted); out-of-range: impassable, unmineable, "unknown". Mining total ticks = `float64(s.MineTicks) * factor` where factor is the target terrain's MineFactor.

- [ ] **Step 1: Write the failing tests**

Rewrite `TestTerrainTypesAndMutation` in `internal/sim/terrain_test.go` to use methods, and add coverage for an appended type (find the current test and mirror its structure; the flatWorld helper needs its config to gain `Terrain: data.CanonicalTerrain()`):

```go
func TestTerrainTableMethods(t *testing.T) {
	w := flatWorld(t, 4, 4, 1) // its cfg gains CanonicalTerrain + one appended soft type in this step
	if !w.Passable(TerrainFloor) || w.Passable(TerrainWater) || w.Passable(TerrainRock) {
		t.Error("passability wrong")
	}
	if !w.Mineable(TerrainRock) || w.Mineable(TerrainDirt) || w.Mineable(TerrainFloor) {
		t.Error("mineability wrong")
	}
	soft := Terrain(5) // appended in the test cfg
	if !w.Mineable(soft) || w.Passable(soft) {
		t.Error("appended terrain not honored")
	}
	if w.TerrainName(soft) != "softish" || w.TerrainName(TerrainRock) != "rock" {
		t.Error("terrain names wrong")
	}
	if w.Passable(Terrain(99)) || w.Mineable(Terrain(99)) || w.TerrainName(Terrain(99)) != "unknown" {
		t.Error("out of range must be inert")
	}
}
```

Add to `internal/sim/mine_test.go` (mineCfg gains `Terrain: append(data.CanonicalTerrain(), data.TerrainType{ID: "softish", Color: "#575049", Mineable: true, MineFactor: 0.5})`):

```go
func TestSoftRockMinesFaster(t *testing.T) {
	// two identical worlds, one face each; soft face at factor 0.5
	// completes in half the steps of the plain rock face
	steps := func(soft bool) int {
		w := mineWorld(5, 5)
		face := Point{3, 2}
		w.Terrain[idx(w, face)] = TerrainRock
		if soft {
			w.Terrain[idx(w, face)] = Terrain(5)
		}
		d := w.Spawn("dwarf", Point{2, 2})
		d.Fullness = 10
		for i := 0; i < 60; i++ {
			w.Step()
			if w.At(face) == TerrainFloor {
				return i
			}
		}
		t.Fatalf("face never mined (soft=%v)", soft)
		return -1
	}
	hard := steps(false)
	softSteps := steps(true)
	if softSteps >= hard {
		t.Fatalf("soft rock not faster: soft %d vs hard %d", softSteps, hard)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestTerrainTable|TestSoftRock' 2>&1 | tail -3`
Expected: build FAIL (methods undefined / cfg field).

- [ ] **Step 3: Implement**

`internal/sim/world.go`: delete the `terrainNames` var and the package funcs `TerrainName`, `Passable`, `Mineable`; add methods:

```go
func (w *World) terrainAt(t Terrain) *data.TerrainType {
	if int(t) >= len(w.cfg.Terrain) {
		return nil
	}
	return &w.cfg.Terrain[t]
}

func (w *World) Passable(t Terrain) bool {
	tt := w.terrainAt(t)
	return tt != nil && tt.Passable
}

func (w *World) Mineable(t Terrain) bool {
	tt := w.terrainAt(t)
	return tt != nil && tt.Mineable
}

func (w *World) TerrainName(t Terrain) string {
	if tt := w.terrainAt(t); tt != nil {
		return tt.ID
	}
	return "unknown"
}
```

Convert every call site (compiler-guided; the known list):
- sim: tick.go:176 and :225 (`w.Passable(...)`), ai.go:165 and :189, mine.go:15 (`!w.Mineable(...)`), mine.go:101 (`w.Mineable(t)`), mine.go:110 and :166 (`w.Passable`).
- gen.go:87 (`w.TerrainName(w.At(p))`) and :97 (`w.Passable(...)`).
- server torch.go:21 (`s.world.Passable(s.world.At(p))`).

`internal/sim/mine.go`, the dig increment becomes factor-aware:

```go
		total := float64(s.MineTicks)
		if tt := w.terrainAt(w.At(target)); tt != nil && tt.MineFactor > 0 {
			total *= tt.MineFactor
		}
		w.MineProgress[i] += 1.0 / total
```

(replacing `w.MineProgress[i] += 1.0 / float64(s.MineTicks)`).

Every in-code test config in internal/sim (mineCfg, structCfg, darkCfg, socialCfg, flatWorld's cfg, plus internal/gen's undergroundCfg and cfg helpers, and internal/server test helpers that build Configs in code) gains `Terrain: data.CanonicalTerrain()` (mineCfg gains the appended "softish" type per Step 1). Grep for `data.Config{` to find them all; missing ones surface as everything-impassable test failures.

- [ ] **Step 4: Run the full suite**

Run: `set -o pipefail; go test -count=1 ./... 2>&1 | tail -5`
Expected: PASS including the 50k soak (legacy fixture now loads its canonical terrain.toml from Task 1).

- [ ] **Step 5: Commit**

```bash
git add internal/ 
git commit -m "Read terrain properties from the table and scale mining"
```

---

### Task 3: Blob veins in generation

**Files:**
- Modify: `internal/gen/gen.go`
- Test: `internal/gen/underground_test.go`

**Interfaces:**
- Consumes: `cfg.Gen.Veins`, `cfg.TerrainIndex`, `w.Mineable`, world RNG (`w.RandN`).
- Produces: veins carved into the terrain after the base fill, before the campfire spawn and scatter.

- [ ] **Step 1: Write the failing test**

Append to `internal/gen/underground_test.go` (undergroundCfg gains `Terrain: append(data.CanonicalTerrain(), data.TerrainType{ID: "soft_rock", Color: "#575049", Mineable: true, MineFactor: 0.25})`):

```go
func TestVeinsGrowConnectedBlobsInRock(t *testing.T) {
	cfg := undergroundCfg()
	cfg.Gen.Veins = []data.VeinRule{{Terrain: "soft_rock", Seeds: 3, Size: 8}}
	w := Generate(11, cfg)
	soft := sim.Terrain(5)
	cells := map[sim.Point]bool{}
	for y := 0; y < cfg.Gen.Height; y++ {
		for x := 0; x < cfg.Gen.Width; x++ {
			if w.At(sim.Point{X: x, Y: y}) == soft {
				cells[sim.Point{X: x, Y: y}] = true
			}
		}
	}
	if len(cells) != 3*8 {
		t.Fatalf("soft cells = %d, want 24", len(cells))
	}
	// every soft cell touches another soft cell unless it is a size-1 blob
	for p := range cells {
		touching := false
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if cells[sim.Point{X: p.X + dx, Y: p.Y + dy}] {
					touching = true
				}
			}
		}
		if !touching {
			t.Fatalf("isolated soft cell at %v; veins must be connected", p)
		}
	}
	// determinism
	w2 := Generate(11, cfg)
	for i := range w.Terrain {
		if w.Terrain[i] != w2.Terrain[i] {
			t.Fatal("veins not deterministic per seed")
		}
	}
	// clearing untouched
	c := sim.Point{X: cfg.Gen.Width / 2, Y: cfg.Gen.Height / 2}
	if w.At(c) != sim.TerrainDirt {
		t.Fatal("vein replaced the clearing")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/gen/ -run TestVeins 2>&1 | tail -3`
Expected: FAIL (0 soft cells).

- [ ] **Step 3: Implement**

In `internal/gen/gen.go`, inside the `ClearingRadius > 0` branch, after the terrain fill loop and before the campfire spawn:

```go
		for _, vein := range g.Veins {
			idx, ok := cfg.TerrainIndex(vein.Terrain)
			if !ok {
				continue // validated at load; belt and braces
			}
			vt := sim.Terrain(idx)
			for s := 0; s < vein.Seeds; s++ {
				seed, found := randomRockCell(w)
				if !found {
					break
				}
				blob := []sim.Point{seed}
				w.Terrain[seed.Y*g.Width+seed.X] = vt
				for len(blob) < vein.Size {
					p := blob[w.RandN(len(blob))]
					q, ok := randomRockNeighbor(w, p)
					if !ok {
						continue
					}
					w.Terrain[q.Y*g.Width+q.X] = vt
					blob = append(blob, q)
					if len(blob) >= vein.Size {
						break
					}
				}
			}
		}
```

with helpers at file scope (growth must terminate even when a blob is walled in; bound the inner loop):

```go
// randomRockCell picks a uniformly random plain-rock cell, or reports none.
func randomRockCell(w *sim.World) (sim.Point, bool) {
	for i := 0; i < 200; i++ {
		p := sim.Point{X: w.RandN(w.Width), Y: w.RandN(w.Height)}
		if w.At(p) == sim.TerrainRock {
			return p, true
		}
	}
	return sim.Point{}, false
}

// randomRockNeighbor picks a random 8-neighbor of p that is still plain rock.
func randomRockNeighbor(w *sim.World, p sim.Point) (sim.Point, bool) {
	start := w.RandN(8)
	dirs := [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
	for i := 0; i < 8; i++ {
		d := dirs[(start+i)%8]
		q := sim.Point{X: p.X + d[0], Y: p.Y + d[1]}
		if w.InBounds(q) && w.At(q) == sim.TerrainRock {
			return q, true
		}
	}
	return sim.Point{}, false
}
```

IMPORTANT termination detail: the `for len(blob) < vein.Size` loop as sketched can spin forever if every blob cell is walled in (no rock neighbors anywhere). Add an attempt counter (`for tries := 0; len(blob) < vein.Size && tries < vein.Size*20; tries++`) so a boxed-in blob stops early rather than hanging generation. The test's 32x32 world has abundant rock, so the count assertion still holds there.

- [ ] **Step 4: Run to verify pass, full gen package**

Run: `set -o pipefail; go test ./internal/gen/ -count=1 2>&1 | tail -2`
Expected: PASS (TestGenerateDeterministic keeps passing: veins draw from the world RNG, so same seed still gives identical terrain, and its cross-seed entity-scatter assertion is unaffected).

- [ ] **Step 5: Commit**

```bash
git add internal/gen/
git commit -m "Grow soft rock blob veins during generation"
```

---

### Task 4: Terrain table on the wire and in the client

**Files:**
- Modify: `internal/server/protocol.go`
- Modify: `client/src/types.ts`, `client/src/world.ts`, `client/src/render.ts`
- Test: `internal/server/protocol_test.go`

**Interfaces:**
- Consumes: `cfg.Terrain` (already json-tagged).
- Produces: `SnapshotMsg.TerrainTypes []data.TerrainType json:"terrainTypes"` from `w.Cfg().Terrain`; client `world.terrainTypes: TerrainType[]`; render colors via `world.terrainTypes[v]?.color ?? "#000"`.

- [ ] **Step 1: Failing server test**

Append to `internal/server/protocol_test.go`:

```go
func TestSnapshotCarriesTerrainTypes(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	snap := BuildSnapshot(w, 1, nil)
	if len(snap.TerrainTypes) < 5 {
		t.Fatalf("terrainTypes = %d entries, want the full table", len(snap.TerrainTypes))
	}
	if snap.TerrainTypes[3].ID != "rock" || snap.TerrainTypes[3].Color == "" {
		t.Fatalf("terrainTypes[3] = %+v, want rock with a color", snap.TerrainTypes[3])
	}
}
```

Run: `set -o pipefail; go test ./internal/server/ -run TestSnapshotCarriesTerrain 2>&1 | tail -3`
Expected: build FAIL.

- [ ] **Step 2: Implement server side**

`internal/server/protocol.go`: `SnapshotMsg` gains `TerrainTypes []data.TerrainType json:"terrainTypes"`; `BuildSnapshot` sets `TerrainTypes: w.Cfg().Terrain`.

Run the test: PASS. Full package: PASS.

- [ ] **Step 3: Client**

- `client/src/types.ts`:

```ts
export interface TerrainType {
  id: string;
  color: string;
  passable: boolean;
  mineable: boolean;
  mineFactor: number;
}
```

`SnapshotMsg` gains `terrainTypes: TerrainType[];`.
- `client/src/world.ts`: field `terrainTypes: TerrainType[] = [];`, set in `applySnapshot` (`this.terrainTypes = m.terrainTypes ?? [];` before the terrain decode) and bump `terrainVersion` as it already does.
- `client/src/render.ts`: delete the `TERRAIN_COLORS` const; `renderTerrain` uses `world.terrainTypes[world.terrain[y * world.width + x]]?.color ?? "#000"`.

- [ ] **Step 4: Build gate and commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add internal/server/ client/
git commit -m "Send the terrain table to clients and drop hardcoded colors"
```

---

### Task 5: End-to-end verification, docs, push

**Files:** throwaway scripts in the scratchpad; `.claude/skills/verify/SKILL.md`.

- [ ] **Step 1: Isolated server**

Scratch data dir copy with its own save_path; build client; `go run ./cmd/cellarfloor -addr :8083 -data <scratch>/data -static client/dist -fresh` FROM THE REPO ROOT (a prior run failed by launching from client/). Sanity: `curl -s localhost:8083/api/state`.

- [ ] **Step 2: Veins visible and data-colored**

Headless Playwright (chromium `channel: 'chrome'`): spawn a dwarf via the overlay, then pixel-scan the canvas for both `#3a3a3a` (rock) and `#575049` (soft rock) among LIT cells near the campfire ring, or zoom is unnecessary: sample the whole canvas and require both colors present with soft_rock count in a plausible band (>= 20 pixels given 10 veins x 14 cells x 144 px/cell, many veiled; assert > 0 under the veil-free ring or scan the raw expected RGB after multiplying the 0.75 veil: veiled soft rock reads as 0.25 * (0x57,0x50,0x49) = about (22,20,18) which collides with the veil over other terrain, so scan only the lit ring around the campfire OR fast-forward until a soft vein is inside torch light). Simplest deterministic route: read `/api/state` width, then fetch the terrain grid indirectly by screenshotting and checking the lit clearing ring; if no soft vein happens to be lit on this seed, verify vein existence via the sim instead: run a tiny Go check `go run` snippet or hit a debug route? None exists for terrain, so instead regenerate with a few seeds via `-seed` flag until a vein touches the lit ring (seed is a server flag; 2-3 restarts are cheap). Record which seed was used.

- [ ] **Step 3: Soft rock mines faster end-to-end**

Using /api/entities and /api/advance: advance ~45000 ticks (0.26 day). A dwarf that happened to claim a soft face has finished (cell became floor, `mined`/`gold` event fired) while plain-rock faces sit at ~0.26 progress in the `mining` map of the snapshot (fetch via a ws snapshot or check MineProgress through a second advance completing at ~1 day). At minimum assert: some cell became floor before 50000 ticks (impossible for factor-1 rock, which needs 172800), proving the factor path works live. Screenshot the vein being chewed.

- [ ] **Step 4: Docs and gate**

`.claude/skills/verify/SKILL.md`: terrain colors now come from `data/terrain.toml` (update the color list note; add soft rock `#575049`).

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: green. Kill YOUR :8083 server.

- [ ] **Step 5: Commit and push**

```bash
git add -A
git commit -m "Verify terrain types end to end and refresh docs"
git push
```

Note in the report: existing world.json keeps working (soft_rock is appended); new veins only appear in newly generated worlds, so the user sees soft rock after their next reset or -fresh, not before. Relay this.

---

## Self-Review Notes

- Spec coverage: table + pinned order + validation (T1), methods + factor mining (T2), blob veins deterministic/connected/rock-only (T3), wire + client colors (T4), e2e + docs (T5). Out of scope respected.
- The passable+mineable exclusivity check protects `pickMineTarget`, whose BFS treats mineable and passable as disjoint branches.
- In-code config ripple (every test cfg gains CanonicalTerrain) is called out in both T1 and T2; missing ones fail loudly as everything-impassable.
- Old saves: indices pinned, soft_rock appended; no reset needed, but veins only exist in newly generated worlds (relayed to user in T5).
