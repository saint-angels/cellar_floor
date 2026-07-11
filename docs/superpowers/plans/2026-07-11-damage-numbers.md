# Damage Numbers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integer hit-point mining (terrain `hit_points`, dwarf `mine_damage`) with Vampire Survivors floating damage numbers on every tool strike.

**Architecture:** Data swaps `mine_factor`/`mine_hours` for `hit_points`/`mine_damage` ints. The sim counts damage per cell (`MineDamage map[int]int`, complete at hp). The wire's `mining` map becomes ints; `hitPoints` rides the terrainTypes table. The client derives bars as damage/hp and pops eased floating numbers from the existing orbit-strike detection in fx.ts.

**Tech Stack:** Go stdlib + BurntSushi/toml, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-11-damage-numbers-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on Go tasks; `set -o pipefail` when piping `go test`.
- Exact live values: rock `hit_points = 172800`, soft_rock `hit_points = 43200` (comments: 24h / 6h at 1 damage per tick, 2 ticks per second); dwarf `mine_damage = 1`. Mining time must be unchanged for live data.
- Validation: mineable terrain requires positive `hit_points`; `mine_damage` non-negative; mining capability gates on `MineDamage > 0`.
- Animation: rise ~8 px over 400 ms with ease-in quad on position AND alpha (0 to 1), then stationary alpha fade 1 to 0 over 600 ms; 9 px monospace #e8e2d8; pool cap 40.
- Never touch the live server (port 8080) or canonical world.json/players.json; e2e uses :8083 with a scratch data dir owning its save_path, launched FROM THE REPO ROOT.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: Hit points and mine damage in the data layer

**Files:**
- Modify: `internal/data/data.go`, `data/terrain.toml`, `data/entities.toml`, `internal/sim/testdata/legacy/terrain.toml`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: `TerrainType.HitPoints int` (toml `hit_points`, json `hitPoints`) REPLACING `MineFactor` (field deleted); `EntityType.MineDamage int` (toml `mine_damage`, json `mineDamage`) REPLACING `MineTicks` and `MineHours` (both deleted); `CanonicalTerrain()` rock entry gains `HitPoints: 172800`.

- [ ] **Step 1: Write the failing tests**

In `internal/data/data_test.go`, REPLACE the terrain assertions that reference MineFactor and add damage coverage:

```go
func TestHitPointsAndDamageParse(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if hp := cfg.Terrain[3].HitPoints; hp != 172800 {
		t.Fatalf("rock hp = %d, want 172800", hp)
	}
	if hp := cfg.Terrain[5].HitPoints; hp != 43200 {
		t.Fatalf("soft rock hp = %d, want 43200", hp)
	}
	if d := cfg.Types["dwarf"].MineDamage; d != 1 {
		t.Fatalf("dwarf mine_damage = %d, want 1", d)
	}
}

func TestMineableNeedsHitPoints(t *testing.T) {
	cfg := minimalConfig()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("mineable without positive hit_points must fail")
	}
	cfg.Terrain[len(cfg.Terrain)-1].HitPoints = 100
	if err := Validate(cfg); err != nil {
		t.Fatalf("mineable with hp should validate: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/data/ -run 'TestHitPoints|TestMineableNeeds' 2>&1 | tail -3`
Expected: build FAIL (`HitPoints` undefined).

- [ ] **Step 3: Implement the data layer**

`internal/data/data.go`:
- `TerrainType`: replace `MineFactor float64 toml:"mine_factor" json:"mineFactor"` with `HitPoints int toml:"hit_points" json:"hitPoints"`.
- `EntityType`: delete `MineTicks` and `MineHours`; add `MineDamage int toml:"mine_damage" json:"mineDamage"` in their place.
- `CanonicalTerrain()`: rock entry becomes `{ID: "rock", Color: "#3a3a3a", Mineable: true, HitPoints: 172800}`.
- `resolveTimes`: delete the `t.MineTicks = hours(t.MineHours)` line.
- `Validate`:
  - terrain loop: replace the mine_factor check with `if tt.Mineable && tt.HitPoints <= 0 { return fmt.Errorf("terrain %s: mineable needs positive hit_points", tt.ID) }` (keep the passable+mineable exclusion).
  - unit-fields block: drop `s.MineHours < 0` from the time-fields check.
  - fauna block: replace the MineTicks message with `if s.MineDamage < 0 { return fmt.Errorf("type %s: mine_damage must be non-negative", id) }`.

Data files:
- `data/terrain.toml`: rock block loses `mine_factor = 1.0`, gains `hit_points = 172800  # 24h at 1 damage per tick, 2 ticks per second`; soft_rock loses `mine_factor = 0.25`, gains `hit_points = 43200   # 6h`.
- `internal/sim/testdata/legacy/terrain.toml`: rock gains `hit_points = 172800`, loses `mine_factor`.
- `data/entities.toml`: dwarf swaps `mine_hours = 24` for `mine_damage = 1`.

Existing data tests that reference the removed fields (the terrain-parse test asserting MineFactor 0.25, TestMiningFieldsParse's `mine_hours` fixture line and MineTicks assertion, TestUnitFieldsConvertToTicks's `mine_hours = 24` line and MineTicks assertion) update in this step: fixtures write `mine_damage = 1` and assert `MineDamage == 1`; the terrain-parse test asserts `HitPoints` instead of `MineFactor`. Do not weaken unrelated assertions.

- [ ] **Step 4: Run data tests to verify pass**

Run: `set -o pipefail; go test ./internal/data/ 2>&1 | tail -2`
Expected: PASS. `go build ./...` will FAIL at internal/sim (mine.go references MineTicks/MineFactor); that is expected and Task 2 fixes it, so do NOT run the full build gate here.

- [ ] **Step 5: Commit**

```bash
git add internal/data/ data/terrain.toml data/entities.toml internal/sim/testdata/legacy/terrain.toml
git commit -m "Define rock health in hit points and dwarf mine damage"
```

NOTE: this commit intentionally leaves internal/sim uncompilable; Task 2 lands within the same push cycle. Do not push in this task.

---

### Task 2: Integer damage mining in the sim

**Files:**
- Modify: `internal/sim/world.go`, `internal/sim/mine.go`
- Modify: every in-code sim test config that used MineTicks (mine_test.go, dark_test.go? no: only where mining types exist, social_test.go's dwarfish has MineTicks 100)
- Test: `internal/sim/mine_test.go`

**Interfaces:**
- Consumes: `TerrainType.HitPoints`, `EntityType.MineDamage`.
- Produces: `World.MineDamage map[int]int` (json `mineDamage,omitempty`) replacing `MineProgress`; `mineStep` gates on `s.MineDamage > 0`, accrues `MineDamage[i] += s.MineDamage`, completes when `>= hp` from `w.terrainAt(w.At(target)).HitPoints`.

- [ ] **Step 1: Adjust the test configs and write the failing test**

`internal/sim/mine_test.go` config surgery (this re-expresses the old per-miner speeds as terrain hp):
- mineCfg terrain: `Terrain: canonicalWithHP()` where a small file-local helper builds the table for tests:

```go
// canonicalWithHP is CanonicalTerrain with a fast test rock (10 hp) plus
// a soft test type (5 hp) and a hard one (10000 hp) for gate tests.
func canonicalWithHP() []data.TerrainType {
	tt := data.CanonicalTerrain()
	tt[3].HitPoints = 10
	tt = append(tt,
		data.TerrainType{ID: "softish", Color: "#575049", Mineable: true, HitPoints: 5},
		data.TerrainType{ID: "hardish", Color: "#222", Mineable: true, HitPoints: 10000},
	)
	return tt
}
```

- mineCfg types: dwarf swaps `MineTicks: 10` for `MineDamage: 1`; miner swaps `MineTicks: 100` for `MineDamage: 1`; goldDropWorld drops its `cfg.Types["miner"].MineTicks = 10` line and instead the world uses the 10 hp rock (already the table default here).
- `newMineWorldDark` sets its face to `Terrain(6)` (hardish) instead of TerrainRock so the light-gate tests never complete a face mid-test; the water walls and campfire stay. IMPORTANT: `randomRock*` does not exist here; just assign `w.Terrain[idx(w, Point{4, 2})] = Terrain(6)`.
- `TestSoftRockMinesFaster` switches its soft face to `Terrain(5)` (softish, 5 hp) versus plain rock (10 hp); assertions unchanged.
- social_test.go dwarfish: `MineTicks: 100` becomes `MineDamage: 1` (socialCfg has no rock cells, so speed is irrelevant; the field only enables the mining branch).
- world_test.go flatWorld appended type: `MineFactor: 0.5` becomes `HitPoints: 5`.
- terrain_test.go: no MineFactor references (methods only), leave as is.

New failing test appended to mine_test.go:

```go
func TestDamageAccrualAndCompletion(t *testing.T) {
	w := mineWorld(5, 5)
	face := Point{3, 2}
	w.Terrain[idx(w, face)] = TerrainRock // 10 hp in this cfg
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	w.Step() // adjacent: first tick of damage
	if got := w.MineDamage[idx(w, face)]; got != 1 {
		t.Fatalf("damage after one tick = %d, want 1", got)
	}
	for i := 0; i < 9; i++ {
		w.Step()
	}
	if w.At(face) != TerrainFloor {
		t.Fatal("10 hp face should be floor after 10 damage")
	}
	if _, ok := w.MineDamage[idx(w, face)]; ok {
		t.Fatal("completed cell must leave the damage map")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run TestDamageAccrual 2>&1 | tail -3`
Expected: build FAIL (MineDamage undefined on World; config fields).

- [ ] **Step 3: Implement**

`internal/sim/world.go`:
- World field: `MineDamage map[int]int json:"mineDamage,omitempty"` replaces `MineProgress map[int]float64` (delete the old field; old saves' float map is intentionally dropped).
- `NewWorld` and `SetConfig` init `MineDamage` the way they did MineProgress.

`internal/sim/mine.go`:
- Gate: `if s.MineDamage <= 0 { return nil, false }` replaces the MineTicks gate.
- Dig block replaces the fraction math:

```go
		e.Action = "mining"
		i := target.Y*w.Width + target.X
		w.MineDamage[i] += s.MineDamage
		w.markDirty(e.ID)
		hp := 0
		if tt := w.terrainAt(w.At(target)); tt != nil {
			hp = tt.HitPoints
		}
		if w.MineDamage[i] < hp {
			return nil, true
		}
		delete(w.MineDamage, i)
```

(the SetTerrain/gold-roll/eventing below is untouched.)

Fix remaining compile references: mine_test.go's save/load test asserts `MineDamage` instead of `MineProgress` (`w2.MineDamage[idx(w, rock)] != w.MineDamage[idx(w, rock)]`); players_test/api tests in internal/server touch MineProgress (TestResetWorld sets `s.world.MineProgress[42] = 0.5`; change to `s.world.MineDamage[42] = 5`); server compiles against sim, so these edits happen NOW even though the server protocol swap is Task 3; use `git grep -n MineProgress` to find every reference and convert it (protocol.go's `Mining: w.MineProgress` becomes `Mining: w.MineDamage`, which forces the protocol type change early: change `Mining map[int]float64` to `Mining map[int]int` in both messages in this task and note it; Task 3 then only covers the client and tests of the wire).

- [ ] **Step 4: Full Go suite**

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... 2>&1 | tail -5`
Expected: PASS including the soak (legacy fauna have MineDamage 0, so mining never triggers for them, same as before).

- [ ] **Step 5: Commit**

```bash
git add internal/
git commit -m "Count mining as integer damage against terrain hit points"
```

---

### Task 3: Int mining on the wire and damage-aware client bars

**Files:**
- Modify: `internal/server/protocol_test.go` (mining int assertions)
- Modify: `client/src/types.ts`, `client/src/render.ts`
- Test: `internal/server/protocol_test.go`

**Interfaces:**
- Consumes: `Mining map[int]int` (landed in Task 2), `TerrainType.hitPoints` in terrainTypes json.
- Produces: client types `mining: Record<string, number>` (unchanged shape, now ints), `TerrainType.hitPoints: number`; progress bar fraction computed as damage/hitPoints.

- [ ] **Step 1: Server test for int mining**

Update `TestTickCarriesMiningState` in `internal/server/protocol_test.go`: `w.MineProgress[5] = 0.25` becomes `w.MineDamage[5] = 25`, assertions compare `snap.Mining[5] != 25` (and the tick equivalent). Run the server package; expected PASS (behavior landed in Task 2; this locks it).

- [ ] **Step 2: Client**

- `client/src/types.ts`: `TerrainType` gains `hitPoints: number;` (mineFactor field name never existed client-side; nothing to delete unless it does: check and remove `mineFactor` from the interface added by the terrain feature).
- `client/src/render.ts` bar loop: fraction becomes

```ts
      for (const [k, dmg] of Object.entries(world.mining)) {
        const i = Number(k);
        const hp = world.terrainTypes[world.terrain[i]]?.hitPoints ?? 0;
        const p = hp > 0 ? dmg / hp : 0;
        ...same two fillRects with Math.min(p, 1)...
      }
```

- [ ] **Step 3: Gates and commit**

Run: `set -o pipefail; go test ./internal/server/ && (cd client && npx tsc --noEmit && npm run build)`
Expected: green.

```bash
git add internal/server/ client/
git commit -m "Send mining damage as integers and derive bars from hp"
```

---

### Task 4: Floating damage numbers in fx.ts

**Files:**
- Modify: `client/src/fx.ts`

**Interfaces:**
- Consumes: `world.mining` (int damage per cell), `world.terrain`, `world.terrainTypes[].hitPoints`, the existing orbit-strike edge detection (`inside && !wasInside.get(e.id) && running`).
- Produces: floating number pool drawn inside `drawEffects`.

- [ ] **Step 1: Implement the float pool**

Add to `client/src/fx.ts`:

```ts
const FLOAT_RISE_MS = 400;
const FLOAT_FADE_MS = 600;
const FLOAT_RISE_PX = 8;
const MAX_FLOATS = 40;
const FLOAT_COLOR = "#e8e2d8";

interface FloatText {
  x: number; y: number;
  text: string;
  age: number;
}

let floats: FloatText[] = [];
// damage already shown per cell index; baseline set silently on first sight
const shownDamage = new Map<number, number>();

const easeInQuad = (t: number) => t * t;

function spawnFloat(cellX: number, cellY: number, text: string) {
  if (floats.length >= MAX_FLOATS) floats.shift();
  floats.push({ x: cellX * TILE + TILE / 2, y: cellY * TILE - 2, text, age: 0 });
}
```

In `drawEffects`, inside the orbit loop where the strike fires (the existing `if (inside && !wasInside.get(e.id) && running)` block), after `spawnDebris(...)`:

```ts
      const cell = e.mt.y * world.width + e.mt.x;
      const dealt = world.mining[cell] ?? 0;
      const prev = shownDamage.get(cell);
      if (prev == null) {
        shownDamage.set(cell, dealt); // baseline silently (fresh page load)
      } else if (dealt > prev) {
        spawnFloat(e.mt.x, e.mt.y, String(dealt - prev));
        shownDamage.set(cell, dealt);
      }
```

After the orbit loop (once per frame, not per entity), sweep completed cells:

```ts
  for (const [cell, prev] of shownDamage) {
    if (world.mining[cell] != null) continue;
    const hp = world.terrainTypes[world.terrain[cell]]?.hitPoints;
    if (hp != null && world.terrain[cell] !== undefined && hp > prev && world.terrainTypes[world.terrain[cell]]?.mineable === false) {
      // cell became floor: pop the remainder using the PRE-mine hp is not
      // recoverable from floor; remainder pop uses tracked prev only when
      // the last known mineable hp is stored alongside; see note below
    }
    shownDamage.delete(cell);
  }
```

NOTE, resolve it this way: the terrain index flips to floor on completion, so the pre-mine hp must be captured when the baseline/updates happen. Store `{shown: number, hp: number}` in `shownDamage` instead of a bare number (hp from `world.terrainTypes[world.terrain[cell]]?.hitPoints ?? 0` at set time). The sweep then pops `hp - shown` when it is positive and the cell has left the mining map:

```ts
  for (const [cell, rec] of shownDamage) {
    if (world.mining[cell] != null) continue;
    if (rec.hp > rec.shown) {
      spawnFloat(cell % world.width, Math.floor(cell / world.width), String(rec.hp - rec.shown));
    }
    shownDamage.delete(cell);
  }
```

(Use the record form from the start; the first snippet's `prev` becomes `rec.shown`.)

Draw and age the pool at the end of `drawEffects`, near the particle draw (floats age only while `running`, matching the particle pause behavior):

```ts
  if (running) for (const f of floats) f.age += dt;
  floats = floats.filter((f) => f.age < FLOAT_RISE_MS + FLOAT_FADE_MS);
  ctx.font = "9px ui-monospace, monospace";
  ctx.textAlign = "center";
  for (const f of floats) {
    let alpha: number;
    let y = f.y;
    if (f.age < FLOAT_RISE_MS) {
      const t = easeInQuad(f.age / FLOAT_RISE_MS);
      alpha = t;
      y = f.y - FLOAT_RISE_PX * t;
    } else {
      alpha = 1 - (f.age - FLOAT_RISE_MS) / FLOAT_FADE_MS;
      y = f.y - FLOAT_RISE_PX;
    }
    ctx.globalAlpha = Math.max(0, Math.min(1, alpha));
    ctx.fillStyle = FLOAT_COLOR;
    ctx.fillText(f.text, f.x, y);
  }
  ctx.globalAlpha = 1;
  ctx.textAlign = "start";
```

- [ ] **Step 2: Build gate and commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add client/src/fx.ts
git commit -m "Pop floating damage numbers on tool strikes"
```

---

### Task 5: End-to-end verification, docs, push

**Files:** throwaway scripts in the scratchpad; `.claude/skills/verify/SKILL.md`.

- [ ] **Step 1: Isolated server and a mining dwarf**

Scratch data copy with own save_path, client built, server on :8083 from the repo root with `-fresh`. Spawn a dwarf via the overlay UI. Advance a few thousand ticks so it reaches a face and accumulates damage (`/api/advance?ticks=4000`, then confirm the ws snapshot mining map holds int damage via `/api/entities` action "mining" plus a page screenshot of the progress bar).

- [ ] **Step 2: Numbers appear**

At 1x, watch the canvas region above the miner's target cell across ~4 s (two strikes): capture frames and assert light `#e8e2d8`-ish pixels appear above the face and then disappear (pixel scan of the band, similar to the thought-bubble cadence test). Then send `{type:"timescale", scale:64}` (or click 64x) and assert a popped number region reappears; a screenshot at 64x should catch a visibly multi-digit number (the delta per strike at 64x is ~60). Screenshots for the record.

- [ ] **Step 3: Bars and completion**

Assert the mining bar renders (existing colors #1a1815/#ffb347) with the int wire. Optionally advance past completion on a soft cell if the seed offers one lit; not mandatory (sim tests cover completion).

- [ ] **Step 4: Docs, gate, push**

`.claude/skills/verify/SKILL.md`: update the mining note: mining map now carries int damage (bar fraction = damage / terrain hitPoints from data/terrain.toml: rock 172800, soft 43200); dwarf mine_damage in entities.toml; floating damage numbers pop above struck faces (#e8e2d8, ~1 s lifetime).

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: green. Kill your :8083 server. Commit docs, push everything (Tasks 1-4 commits are local; this push publishes the whole feature).

```bash
git add -A
git commit -m "Verify damage numbers end to end and refresh docs"
git push
```

Relay: restarting the live server is required (wire shape changed: mining map is ints, hitPoints in terrainTypes); partial mining progress from the old float save is dropped once.

---

## Self-Review Notes

- Spec coverage: data swap (T1), sim damage counter + completion + gold untouched (T2), int wire + hp bars (T3), floating numbers with exact animation spec + baseline + remainder pop (T4), e2e + docs + push (T5).
- Task 1 deliberately leaves the repo uncompilable at internal/sim until Task 2; both land before any push (T5 pushes). This is called out in both tasks.
- The shownDamage record form ({shown, hp}) is specified precisely because terrain flips to floor at completion and the pre-mine hp would otherwise be unrecoverable; the plan text contains one superseded sketch followed by the authoritative record-form snippet, and Task 4 says to use the record form from the start.
- Old-save impact (float mineProgress dropped) is stated in T2 and relayed to the user in T5.
