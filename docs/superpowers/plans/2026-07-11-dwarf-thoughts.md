# Dwarf Thoughts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every dwarf shows a dominant-thought bubble (hunger, loneliness, gold today, seen-a-friend) backed by a real social need that pulls lonely dwarves together.

**Architecture:** The sim gains a social meter (drain alone, mutual refill in company, seen-tracking) processed in the metabolism pass, a seek-company AI step between food and mining, and a pruned per-entity gold-strike window. The server streams four small fields on EntityView (`soc`, `g24`, `seenId`, `seenTick`). The client composes all thought text and draws the bubble; no copy lives server-side.

**Tech Stack:** Go stdlib sim/server, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-11-dwarf-thoughts-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on Go tasks: failing test first, see it fail, implement, see it pass, commit. When piping `go test`, use `set -o pipefail`.
- The engine stays generic: no entity-type name literals in internal/sim; all social behavior gates on `SocialSize > 0` from data.
- Unit-named data fields per the durations convention: `social_drain_days`, `social_refill_hours`; rates derived in `resolveTimes`, never stored in TOML.
- Exact rate formulas: `SocialDrain = SocialSize / (social_drain_days * 86400 * tick_rate)`, `SocialRefill = SocialSize / (social_refill_hours * 3600 * tick_rate)`.
- Thought dominance order (client): starving > hungry > lonely > struck gold today > seen recently > content.
- Never touch the live server (port 8080) or canonical world.json/players.json; e2e uses :8083 with a scratch data dir owning its save_path.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: Social data fields and derived rates

**Files:**
- Modify: `internal/data/data.go`
- Modify: `data/entities.toml`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: `EntityType` fields `SocialSize float64` (toml `social_size`, json `socialSize`), `SocialThreshold float64` (toml `social_threshold`, json `socialThreshold`), `SocialRadius int` (toml `social_radius`, json `-`), `SocialDrainDays float64` (toml `social_drain_days`, json `-`), `SocialRefillHours float64` (toml `social_refill_hours`, json `-`), and derived `SocialDrain`, `SocialRefill float64` (toml `-`, json `-`). Dwarf data: size 10, threshold 4, radius 3, drain 2 days, refill 1 hour.

- [ ] **Step 1: Write the failing test**

Append to `internal/data/data_test.go`:

```go
func TestSocialFieldsConvert(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if d.SocialSize != 10 || d.SocialThreshold != 4 || d.SocialRadius != 3 {
		t.Fatalf("social basics: %v %v %d", d.SocialSize, d.SocialThreshold, d.SocialRadius)
	}
	if want := 10.0 / (2 * 86400 * 2); d.SocialDrain != want {
		t.Errorf("drain = %v, want %v", d.SocialDrain, want)
	}
	if want := 10.0 / (1 * 3600 * 2); d.SocialRefill != want {
		t.Errorf("refill = %v, want %v", d.SocialRefill, want)
	}
	if m := cfg.Types["mushroom"]; m.SocialSize != 0 || m.SocialDrain != 0 {
		t.Errorf("mushroom must have no social: %v %v", m.SocialSize, m.SocialDrain)
	}
}

func TestSocialValidation(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["shroom"].SocialSize = 5
	if err := Validate(cfg); err == nil {
		t.Fatal("social_size without drain and refill times must fail")
	}
	cfg.Types["shroom"].SocialDrainDays = 2
	cfg.Types["shroom"].SocialRefillHours = 1
	cfg.Types["shroom"].SocialRadius = 3
	if err := Validate(cfg); err != nil {
		t.Fatalf("complete social block should validate: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `set -o pipefail; go test ./internal/data/ -run TestSocial -v`
Expected: build FAIL (`SocialSize` undefined).

- [ ] **Step 3: Implement**

In `internal/data/data.go`, add to `EntityType` after the `MineHours` line:

```go
	SocialSize        float64 `toml:"social_size" json:"socialSize"`
	SocialThreshold   float64 `toml:"social_threshold" json:"socialThreshold"`
	SocialRadius      int     `toml:"social_radius" json:"-"`
	SocialDrainDays   float64 `toml:"social_drain_days" json:"-"`
	SocialRefillHours float64 `toml:"social_refill_hours" json:"-"`
	SocialDrain       float64 `toml:"-" json:"-"`
	SocialRefill      float64 `toml:"-" json:"-"`
```

In `resolveTimes`, inside the type loop:

```go
		if t.SocialSize > 0 {
			t.SocialDrain = t.SocialSize / (t.SocialDrainDays * 86400 * tr)
			t.SocialRefill = t.SocialSize / (t.SocialRefillHours * 3600 * tr)
		}
```

In `Validate`, next to the time-fields block:

```go
		if s.SocialSize < 0 || s.SocialThreshold < 0 || s.SocialRadius < 0 ||
			s.SocialDrainDays < 0 || s.SocialRefillHours < 0 {
			return fmt.Errorf("type %s: social fields must be non-negative", id)
		}
		if s.SocialSize > 0 && (s.SocialDrainDays <= 0 || s.SocialRefillHours <= 0 || s.SocialRadius <= 0) {
			return fmt.Errorf("type %s: social_size needs positive social_drain_days, social_refill_hours, social_radius", id)
		}
```

In `data/entities.toml`, dwarf block after `mine_hours = 24`:

```toml
social_size = 10
social_threshold = 4
social_radius = 3
social_drain_days = 2
social_refill_hours = 1
```

- [ ] **Step 4: Run to verify pass**

Run: `set -o pipefail; go test ./internal/data/ -run TestSocial -v`
Expected: PASS. Note `resolveTimes` guards on `SocialSize > 0` before dividing, so types without the block keep zero rates and the guard in TestSocialValidation's first half comes from Validate, not a division panic.

- [ ] **Step 5: Full package and commit**

Run: `set -o pipefail; go test ./internal/data/`
Expected: PASS.

```bash
git add internal/data/ data/entities.toml
git commit -m "Add social need data fields with derived rates"
```

---

### Task 2: Social meter, seen-tracking, and seek-company AI in the sim

**Files:**
- Create: `internal/sim/social.go`
- Modify: `internal/sim/world.go` (Entity fields, Spawn seed, SetConfig migration)
- Modify: `internal/sim/tick.go` (drain/refill in metabolism pass)
- Modify: `internal/sim/ai.go` (priority slot)
- Test: `internal/sim/social_test.go` (new)

**Interfaces:**
- Consumes: Task 1's `SocialSize`, `SocialThreshold`, `SocialRadius`, `SocialDrain`, `SocialRefill`.
- Produces: `Entity.Social float64` (json `social`), `Entity.SeenID int` (json `seenId,omitempty`), `Entity.SeenTick int64` (json `seenTick,omitempty`); `(w *World) companionInRadius(e *Entity, r int) *Entity`; `(w *World) socialStep(e *Entity) bool`; actions `"seeking company"` and `"socializing"`.

- [ ] **Step 1: Write the failing tests**

Create `internal/sim/social_test.go`:

```go
package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

// socialCfg: fast social dynamics so tests run in a handful of ticks.
// Drain 1/tick from size 10, refill 2/tick, radius 2, threshold 4.
func socialCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Types: map[string]*data.EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6}}},
			"dwarfish": {ID: "dwarfish", Name: "Dwarfish", Kind: "fauna", Color: "#fff",
				Eats: []string{"shroom"}, BiteSize: 2, StomachSize: 10, HungerThreshold: 0,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100, MineTicks: 100,
				SocialSize: 10, SocialThreshold: 4, SocialRadius: 2,
				SocialDrain: 1, SocialRefill: 2},
			"sunstone": {ID: "sunstone", Name: "Sunstone", Kind: "structure", Color: "#fff",
				LightRadius: 40, Lifespan: 0},
		},
	}
}

func newSocialWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld(20, 20, 1, socialCfg())
	w.Spawn("sunstone", Point{0, 0}) // fully lit: no fear of the dark in these tests
	return w
}

func TestSocialSeededAndDrains(t *testing.T) {
	w := newSocialWorld(t)
	e := w.Spawn("dwarfish", Point{5, 5})
	if e.Social != 5 {
		t.Fatalf("spawn seeds social = %v, want half of 10", e.Social)
	}
	w.Step()
	if e.Social >= 5 {
		t.Fatalf("social should drain alone: %v", e.Social)
	}
}

func TestCompanyRefillsBothAndRecordsSeen(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{6, 5})
	a.Social, b.Social = 3, 3
	w.Step()
	if a.Social <= 3 || b.Social <= 3 {
		t.Fatalf("both should refill in company: %v %v", a.Social, b.Social)
	}
	if a.SeenID != b.ID || b.SeenID != a.ID || a.SeenTick != w.Tick {
		t.Fatalf("seen not recorded: a saw %d@%d, b saw %d", a.SeenID, a.SeenTick, b.SeenID)
	}
}

func TestLonelySeeksNearestCompanion(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{2, 2})
	b := w.Spawn("dwarfish", Point{15, 15})
	a.Social, b.Social = 2, 10
	before := Dist(a.Pos, b.Pos)
	w.Step()
	if a.Action != "seeking company" {
		t.Fatalf("action = %q, want seeking company", a.Action)
	}
	if Dist(a.Pos, b.Pos) >= before {
		t.Fatal("lonely dwarf must move toward its companion")
	}
}

func TestSocializesUntilFullThenReturnsToWork(t *testing.T) {
	w := newSocialWorld(t)
	w.Terrain[3+7*20] = TerrainRock // a face at {3,7} so mining is available
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{6, 5})
	a.Social, b.Social = 2, 10
	w.Step()
	if a.Action != "socializing" {
		t.Fatalf("action = %q, want socializing (companion already in radius)", a.Action)
	}
	for i := 0; i < 10 && a.Social < 10; i++ {
		w.Step()
	}
	if a.Social < 10 {
		t.Fatalf("social never filled: %v", a.Social)
	}
	w.Step()
	if a.Action == "socializing" || a.Action == "seeking company" {
		t.Fatalf("full dwarf should return to work, action = %q", a.Action)
	}
}

func TestHungryLonelyEatsFirst(t *testing.T) {
	w := newSocialWorld(t)
	cfg := w.Cfg()
	cfg.Types["dwarfish"].HungerThreshold = 4
	shroom := w.Spawn("shroom", Point{6, 5})
	_ = shroom
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{15, 15})
	_ = b
	a.Social, a.Fullness = 2, 1
	w.Step()
	if a.Action == "seeking company" || a.Action == "socializing" {
		t.Fatalf("hunger must outrank loneliness, action = %q", a.Action)
	}
}

func TestLoneSurvivorSkipsSeeking(t *testing.T) {
	w := newSocialWorld(t)
	w.Terrain[3+7*20] = TerrainRock
	a := w.Spawn("dwarfish", Point{5, 5})
	a.Social = 1
	w.Step()
	if a.Action == "seeking company" {
		t.Fatal("a lone survivor has nobody to seek and should work instead")
	}
}

func TestSocialSurvivesSaveLoadWithMigration(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{5, 5})
	a.Social = 7.5
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(socialCfg())
	if w2.Entities[a.ID].Social != 7.5 {
		t.Fatalf("social lost in round trip: %v", w2.Entities[a.ID].Social)
	}
	// migration: an old save has Social 0 on a living social fauna
	w2.Entities[a.ID].Social = 0
	w2.SetConfig(socialCfg())
	if w2.Entities[a.ID].Social != 5 {
		t.Fatalf("migration should seed half-full: %v", w2.Entities[a.ID].Social)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestSocial|TestCompany|TestLonely|TestSocializes|TestHungryLonely|TestLoneSurvivor' 2>&1 | tail -5`
Expected: build FAIL (`Social` undefined on Entity).

- [ ] **Step 3: Implement**

`internal/sim/world.go`, Entity gains (after `MineTarget`):

```go
	Social   float64 `json:"social,omitempty"`
	SeenID   int     `json:"seenId,omitempty"`
	SeenTick int64   `json:"seenTick,omitempty"`
```

In `Spawn`, next to the Fullness seed:

```go
	if s.Kind == "fauna" {
		e.Fullness = s.StomachSize / 2
		if s.SocialSize > 0 {
			e.Social = s.SocialSize / 2
		}
	}
```

In `SetConfig`, before `w.rebuildOcc()`:

```go
	// older saves predate the social meter; wake up half-full, like Spawn
	for _, e := range w.Entities {
		if e.Dead {
			continue
		}
		if s, ok := cfg.Types[e.Type]; ok && s.SocialSize > 0 && e.Social == 0 {
			e.Social = s.SocialSize / 2
		}
	}
```

Create `internal/sim/social.go`:

```go
package sim

// companionInRadius returns the nearest living same-type entity within r,
// ties broken by lowest id via the sorted scan order.
func (w *World) companionInRadius(e *Entity, r int) *Entity {
	var best *Entity
	bestD := r + 1
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID == e.ID || c.Dead || c.Type != e.Type {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// nearestCompanion returns the nearest living same-type entity anywhere.
func (w *World) nearestCompanion(e *Entity) *Entity {
	return w.companionInRadius(e, w.Width+w.Height)
}

// socialStep handles loneliness: seek the nearest companion when the
// meter is below threshold, then stay socializing until full. Returns
// true when the entity spent this tick on company.
func (w *World) socialStep(e *Entity) bool {
	s := w.cfg.Types[e.Type]
	if s.SocialSize <= 0 {
		return false
	}
	if c := w.companionInRadius(e, s.SocialRadius); c != nil {
		wasSocial := e.Action == "socializing" || e.Action == "seeking company"
		if e.Social < s.SocialSize && (wasSocial || e.Social < s.SocialThreshold) {
			e.Action = "socializing"
			w.markDirty(e.ID)
			return true
		}
		return false
	}
	if e.Social >= s.SocialThreshold {
		return false
	}
	target := w.nearestCompanion(e)
	if target == nil {
		return false
	}
	e.Action = "seeking company"
	w.moveToward(e, target.Pos)
	return true
}
```

`internal/sim/tick.go`, in the section 3 fauna branch after the Fullness/StarvingFor block (before the starve/lifespan kills is fine; keep it adjacent to the metabolism lines):

```go
		if s.SocialSize > 0 {
			if c := w.companionInRadius(e, s.SocialRadius); c != nil {
				e.Social += s.SocialRefill
				if e.Social > s.SocialSize {
					e.Social = s.SocialSize
				}
				e.SeenID = c.ID
				e.SeenTick = w.Tick
			} else {
				e.Social -= s.SocialDrain
				if e.Social < 0 {
					e.Social = 0
				}
			}
		}
```

`internal/sim/ai.go`, in `aiStep` between the food block and mining:

```go
	// 3. company
	if w.socialStep(e) {
		return nil
	}

	// 4. mining
```

(renumber the later comments: mining 4, shelter 5, wander 6.)

- [ ] **Step 4: Run to verify pass**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestSocial|TestCompany|TestLonely|TestSocializes|TestHungryLonely|TestLoneSurvivor' -v 2>&1 | tail -12`
Expected: all PASS.

- [ ] **Step 5: Full sim package, gofmt, commit**

Run: `set -o pipefail; gofmt -l internal/sim/ | (! grep .) && go test -count=1 ./internal/sim/ 2>&1 | tail -2`
Expected: PASS (the 50k soak runs; legacy types have SocialSize 0 so nothing changes for them).

```bash
git add internal/sim/
git commit -m "Add social need with mutual refill and company seeking"
```

---

### Task 3: Per-dwarf gold window in the sim

**Files:**
- Modify: `internal/sim/world.go` (GoldStrike type, Entity field, method)
- Modify: `internal/sim/mine.go` (append on strike)
- Test: `internal/sim/mine_test.go`

**Interfaces:**
- Consumes: existing gold drop in `mineStep`.
- Produces: `type GoldStrike struct { Tick int64; Amount int }` (json `tick`, `amount`); `Entity.GoldStrikes []GoldStrike` (json `goldStrikes,omitempty`); `(w *World) GoldLast24h(e *Entity) int` which prunes in place and sums.

- [ ] **Step 1: Write the failing test**

Append to `internal/sim/mine_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run TestGoldWindow 2>&1 | tail -3`
Expected: build FAIL (`GoldLast24h` undefined).

- [ ] **Step 3: Implement**

`internal/sim/world.go`:

```go
// GoldStrike records one gold drop for the rolling last-24h count.
type GoldStrike struct {
	Tick   int64 `json:"tick"`
	Amount int   `json:"amount"`
}
```

Entity gains (next to the social fields): `GoldStrikes []GoldStrike json:"goldStrikes,omitempty"` (backticks as usual).

```go
// GoldLast24h prunes strikes older than 24 hours and sums the rest.
func (w *World) GoldLast24h(e *Entity) int {
	window := int64(86400 * w.cfg.Sim.TickRate)
	keep := e.GoldStrikes[:0]
	sum := 0
	for _, g := range e.GoldStrikes {
		if w.Tick-g.Tick <= window {
			keep = append(keep, g)
			sum += g.Amount
		}
	}
	e.GoldStrikes = keep
	return sum
}
```

`internal/sim/mine.go`, in the gold-drop branch after `w.Gold += amt`:

```go
			e.GoldStrikes = append(e.GoldStrikes, GoldStrike{Tick: w.Tick, Amount: amt})
			w.GoldLast24h(e)
```

- [ ] **Step 4: Run to verify pass, commit**

Run: `set -o pipefail; go test ./internal/sim/ -run TestGoldWindow -v 2>&1 | tail -3 && go test ./internal/sim/ 2>&1 | tail -2`
Expected: PASS.

```bash
git add internal/sim/
git commit -m "Track each miner's gold strikes over a rolling day"
```

---

### Task 4: Stream thought state on EntityView

**Files:**
- Modify: `internal/server/protocol.go` (fields, ViewOf signature)
- Modify: `internal/server/api.go` (two call sites)
- Modify: `internal/server/protocol_test.go` (two call sites)
- Test: `internal/server/protocol_test.go`

**Interfaces:**
- Consumes: `Entity.Social`, `SeenID`, `SeenTick`, `w.GoldLast24h(e)`.
- Produces: `ViewOf(w *sim.World, e *sim.Entity) EntityView`; EntityView fields `Soc float64 json:"soc,omitempty"`, `G24 int json:"g24,omitempty"`, `SeenID int json:"seenId,omitempty"`, `SeenTick int64 json:"seenTick,omitempty"`.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/protocol_test.go` (mirror the file's existing world-building helpers):

```go
func TestViewCarriesThoughtState(t *testing.T) {
	w := newProtoWorld(t) // reuse the file's existing world helper name; adapt if it differs
	d := w.Spawn("dwarf", sim.Point{X: 2, Y: 2})
	d.Social = 6.5
	d.SeenID = 42
	d.SeenTick = 1234
	d.GoldStrikes = append(d.GoldStrikes, sim.GoldStrike{Tick: w.Tick, Amount: 3})
	v := ViewOf(w, d)
	if v.Soc != 6.5 || v.G24 != 3 || v.SeenID != 42 || v.SeenTick != 1234 {
		t.Fatalf("thought state lost: %+v", v)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/server/ -run TestViewCarriesThought 2>&1 | tail -3`
Expected: build FAIL (ViewOf arity, missing fields).

- [ ] **Step 3: Implement**

`internal/server/protocol.go`: add to EntityView:

```go
	Soc      float64 `json:"soc,omitempty"`
	G24      int     `json:"g24,omitempty"`
	SeenID   int     `json:"seenId,omitempty"`
	SeenTick int64   `json:"seenTick,omitempty"`
```

Change the constructor:

```go
func ViewOf(w *sim.World, e *sim.Entity) EntityView {
	...
	return EntityView{
		...existing fields...,
		Soc: e.Social, G24: w.GoldLast24h(e),
		SeenID: e.SeenID, SeenTick: e.SeenTick,
	}
}
```

Update every call site to pass the world: `protocol.go:84` and `:101` (`ViewOf(w, ...)`), `api.go:91` and `:109` (`ViewOf(s.world, e)`), and the two `protocol_test.go` calls.

- [ ] **Step 4: Run to verify pass, commit**

Run: `set -o pipefail; go test ./internal/server/ 2>&1 | tail -2 && go test ./internal/data/ ./internal/gen/ 2>&1 | tail -2`
Expected: PASS.

```bash
git add internal/server/
git commit -m "Stream social, gold window, and seen state to clients"
```

---

### Task 5: Thought bubbles and popup detail in the client

**Files:**
- Modify: `client/src/types.ts`, `client/src/render.ts`, `client/src/ui.ts`

**Interfaces:**
- Consumes: EntityView `soc`, `g24`, `seenId`, `seenTick`; EntityType json `socialSize`, `socialThreshold`, `hungerThreshold`, `stomachSize`; `world.tick`, `world.tickIntervalMs`.
- Produces: `composeThought(e: RenderEntity): string | null` exported from render.ts (exported for reuse by ui.ts popup if handy; popup uses raw numbers).

- [ ] **Step 1: Types**

`client/src/types.ts`: add to `EntityType`: `hungerThreshold: number; socialSize: number; socialThreshold: number;` and to `EntityView`: `soc?: number; g24?: number; seenId?: number; seenTick?: number;`.

- [ ] **Step 2: Thought composer and bubble rendering**

`client/src/render.ts`, above `startRender`:

```ts
// dominant thought: starving > hungry > lonely > gold today > seen > content
export function composeThought(e: import("./types").RenderEntity): string | null {
  const sp = world.types[e.s];
  if (!sp || sp.kind !== "fauna" || e.dead) return null;
  if (e.full <= 0) return "starving...";
  if (e.full < sp.hungerThreshold) return "hungry";
  if (sp.socialSize > 0 && (e.soc ?? 0) < sp.socialThreshold) return "feeling lonely";
  if ((e.g24 ?? 0) > 0) return `struck ${e.g24} gold today!`;
  const dayTicks = 86400 * (1000 / world.tickIntervalMs);
  if (e.seenTick && world.tick - e.seenTick <= dayTicks) {
    const seen = e.seenId != null ? world.entities.get(e.seenId) : undefined;
    const name = seen?.owner ?? "a dwarf";
    return `seen ${name} recently!`;
  }
  return "content";
}
```

In the `frame` loop after the entity loop (and after the owner-ring block, so bubbles sit on top), add:

```ts
      ctx.font = "9px ui-monospace, monospace";
      ctx.textAlign = "center";
      for (const e of world.entities.values()) {
        const thought = composeThought(e);
        if (!thought) continue;
        const t = Math.min(1, (now - e.movedAt) / lerpMs);
        const bx = (e.px + (e.x - e.px) * t) * TILE + TILE / 2;
        const by = (e.py + (e.y - e.py) * t) * TILE - 4;
        const w2 = ctx.measureText(thought).width / 2 + 4;
        ctx.fillStyle = "rgba(20, 17, 15, 0.85)";
        ctx.fillRect(bx - w2, by - 10, w2 * 2, 12);
        ctx.fillStyle = "#cfc9bf";
        ctx.fillText(thought, bx, by - 1);
      }
      ctx.textAlign = "start";
```

- [ ] **Step 3: Popup detail**

`client/src/ui.ts`, in `renderInspector` inside the `fauna && !dead` branch after the fullness line:

```ts
    if (sp.socialSize > 0) {
      lines.push(`social ${(e.soc ?? 0).toFixed(1)} / ${sp.socialSize}`);
    }
    lines.push(`gold today: ${e.g24 ?? 0}`);
```

- [ ] **Step 4: Build gate, commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add client/
git commit -m "Draw dominant thought bubbles over dwarves"
```

---

### Task 6: End-to-end verification and push

**Files:** throwaway scripts in the scratchpad only.

- [ ] **Step 1: Isolated server**

Copy `data/` to a scratch dir, point its `sim.toml` `save_path` into the scratch dir, build the client, run `go run ./cmd/cellarfloor -addr :8083 -data <scratch>/data -static client/dist -fresh` in the background. Confirm `lsof -i :8083` shows only your server and `curl -s localhost:8083/api/state` answers.

- [ ] **Step 2: Loneliness arc via API**

Spawn one dwarf over ws (hello + spawn with a test token). `POST /api/advance?ticks=400000` (2+ days): `GET /api/entities?type=dwarf` shows `soc` at 0. Headless Playwright: assert the canvas near the dwarf renders the "feeling lonely" text (bubble pixels or `ctx` text via screenshot; a screenshot plus a pixel-run check on the bubble background color band above the dwarf tile is acceptable evidence). Screenshot.

- [ ] **Step 3: Reunion arc**

Spawn a second dwarf with a different token. Advance in slices (~2000 ticks) until the two dwarves' positions are within 2 cells; assert one of them passed through action `seeking company` or `socializing` in `/api/entities` samples, and after enough refill ticks `soc` rises toward 10 and the thought changes away from "feeling lonely" (screenshot). If mining takes priority visibly, remember both must be below threshold 4 after the long advance, so seeking should win over mining.

- [ ] **Step 4: Gold thought**

Advance ~200000 more ticks so a lit face finishes; check `g24 > 0` on the miner via API and the bubble shows "struck N gold today!". (90% drop chance makes this near-certain; if the roll missed, advance another day.)

- [ ] **Step 5: Cleanup, gate, push**

Kill your :8083 server (match its command line, nothing else). Then:

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: all green.

```bash
git push
```

Report observed values (soc numbers, actions seen, bubble text) and screenshot paths.

---

## Self-Review Notes

- Spec coverage: data fields/rates (T1), meter+mutual refill+seen (T2 tick), seek/socialize hysteresis and priority (T2 ai/social.go), lone survivor (T2), migration + spawn seed (T2 world.go), gold window (T3), wire fields + types json (T4 + socialSize/socialThreshold json tags from T1), dominant thought order + bubble + popup (T5), e2e (T6). Out of scope respected.
- `Social` json tag uses `omitempty`; a legitimately-zero social value round-trips as absent and loads as 0, which the SetConfig migration then seeds to half. That collapses "empty meter" into "half meter" across a save/load for a fully-drained dwarf; acceptable because the migration can't distinguish absent from zero, and a drained dwarf re-drains quickly. Noted deliberately.
- `composeThought` gates on `kind === "fauna"`, keeping the renderer generic (no "dwarf" literal); the seen-name falls back to "a dwarf" as copy, which is fine as text.
- tick.go social block runs before the starve/lifespan kill checks in the same iteration; order relative to kills is irrelevant since a killed entity's Social no longer matters.
