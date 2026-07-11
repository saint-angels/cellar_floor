# Human-Readable Durations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Data files express every duration and rate in unit-named numeric fields (`mine_hours = 24`, `stomach_drain_hours = 24`, `cells_per_second = 1.0`) converted to internal ticks/rates once at load.

**Architecture:** `data.EntityType`'s internal tick/rate fields lose their TOML tags; new unit-named float fields carry them. A `resolveTimes` step in `data.Load` converts unit fields to the internal ones using `tick_rate`. The sim engine, save format, wire format, and client are untouched. In-code test configs that set internal fields directly (all of `internal/sim`'s helpers) are unaffected because they bypass `Load`.

**Tech Stack:** Go stdlib + BurntSushi/toml (already a dependency).

**Spec:** `docs/superpowers/specs/2026-07-11-human-durations-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD: failing test first, see it fail, implement, see it pass, commit.
- When piping `go test` output, use `set -o pipefail`.
- Conversion formulas verbatim from the spec: tick counts `round(hours*3600*tr)` / `round(days*86400*tr)`; rates derived directly with no intermediate tick rounding: `Metabolism = StomachSize/(stomach_drain_hours*3600*tr)` when positive else 0, `Regrow = Max/(regrow_days*86400*tr)` when positive else 0, `Speed = cells_per_second/tr`.
- Zero keeps its "off" meaning: `lifespan_days 0` immortal, `regrow_days 0` never regrows, `mine_hours 0` not a miner, `stomach_drain_hours 0` only legal for non-fauna.
- The legacy fixture must reproduce its exact historical tick values after conversion (values pinned in Task 1 Step 6).
- Never touch the user's live server (port 8080) or the canonical world.json/players.json.
- All Go commands from the repo root /Users/michael/cellar-floor.

---

### Task 1: Unit-named duration fields in the data layer and both data sets

**Files:**
- Modify: `internal/data/data.go`
- Modify: `data/entities.toml`
- Modify: `internal/sim/testdata/legacy/entities.toml`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Consumes: existing `data.Config`, `EntityType`, `Produce`, `Load`, `Validate`.
- Produces: `EntityType` TOML fields `mine_hours`, `starve_hours`, `decay_hours`, `lifespan_days`, `mature_days`, `stomach_drain_hours`, `cells_per_second` (float64, json:"-") feeding the unchanged internal fields `MineTicks`, `StarveTicks`, `DecayTicks`, `Lifespan`, `MatureAge`, `Metabolism`, `Speed`; `Produce` TOML field `regrow_days` feeding `Regrow`. Internal fields all become `toml:"-"`. Unexported `(c *Config) resolveTimes()` called inside `Load`.

- [ ] **Step 1: Write the failing conversion test**

Append to `internal/data/data_test.go` (the file already has a `write` helper pattern inside TestMiningFieldsParse; this test carries its own):

```go
func TestUnitFieldsConvertToTicks(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sim.toml", "tick_rate = 2.0\nautosave_minutes = 0\nsave_path = \"w.json\"\n")
	write("gen.toml", "width = 8\nheight = 8\nclearing_radius = 3\nscatter = []\n")
	write("entities.toml", `
[type.shroom]
name = "Shroom"
kind = "flora"
color = "#fff"
produces = [{ resource = "shroom", amount = 6, max = 6, regrow_days = 1.75 }]

[type.digger]
name = "Digger"
kind = "fauna"
color = "#fff"
eats = ["shroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
stomach_drain_hours = 24
starve_hours = 48
cells_per_second = 1.0
lifespan_days = 58
mature_days = 6
pop_cap = 10
decay_hours = 24
mine_hours = 24

[type.lamp]
name = "Lamp"
kind = "structure"
color = "#fff"
light_radius = 5
lifespan_days = 1
decay_hours = 0.5
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["digger"]
	if d.MineTicks != 172800 || d.StarveTicks != 345600 || d.DecayTicks != 172800 {
		t.Errorf("hour fields: mine %d starve %d decay %d", d.MineTicks, d.StarveTicks, d.DecayTicks)
	}
	if d.Lifespan != 10022400 || d.MatureAge != 1036800 {
		t.Errorf("day fields: lifespan %d mature %d", d.Lifespan, d.MatureAge)
	}
	if want := 10.0 / 172800; d.Metabolism != want {
		t.Errorf("metabolism = %v, want %v", d.Metabolism, want)
	}
	if d.Speed != 0.5 {
		t.Errorf("speed = %v, want 0.5", d.Speed)
	}
	sh := cfg.Types["shroom"]
	if want := 6.0 / 302400; sh.Produces[0].Regrow != want {
		t.Errorf("regrow = %v, want %v", sh.Produces[0].Regrow, want)
	}
	lamp := cfg.Types["lamp"]
	if lamp.Lifespan != 172800 || lamp.DecayTicks != 3600 {
		t.Errorf("lamp: lifespan %d decay %d", lamp.Lifespan, lamp.DecayTicks)
	}
}

func TestNegativeUnitFieldRejected(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["shroom"].StarveHours = -1
	if err := Validate(cfg); err == nil {
		t.Fatal("negative starve_hours must fail validation")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `set -o pipefail; go test ./internal/data/ -run 'TestUnitFields|TestNegativeUnitField' -v`
Expected: FAIL. TestUnitFieldsConvertToTicks fails on validation ("fauna requires positive ...", since the old tags decode nothing and internals stay zero) or on the tick assertions; TestNegativeUnitFieldRejected fails to compile until `StarveHours` exists, so expect a build error first; that is the RED signal.

- [ ] **Step 3: Implement the fields and conversion**

In `internal/data/data.go`:

`Produce` becomes:

```go
type Produce struct {
	Resource   string  `toml:"resource" json:"resource"`
	Amount     float64 `toml:"amount" json:"amount"`
	Max        float64 `toml:"max" json:"max"`
	Regrow     float64 `toml:"-" json:"regrow"`
	RegrowDays float64 `toml:"regrow_days" json:"-"`
}
```

In `EntityType`, retag the internal fields and add the unit fields (keep field order tidy, units next to their internal partner):

```go
	Metabolism        float64 `toml:"-" json:"metabolism"`
	StomachDrainHours float64 `toml:"stomach_drain_hours" json:"-"`
	StarveTicks       int     `toml:"-" json:"starveTicks"`
	StarveHours       float64 `toml:"starve_hours" json:"-"`
	FearRadius        int     `toml:"fear_radius" json:"fearRadius"`
	Speed             float64 `toml:"-" json:"speed"`
	CellsPerSecond    float64 `toml:"cells_per_second" json:"-"`
	HomeRange         int     `toml:"home_range" json:"homeRange"`
	Lifespan          int     `toml:"-" json:"lifespan"`
	LifespanDays      float64 `toml:"lifespan_days" json:"-"`
	MatureAge         int     `toml:"-" json:"matureAge"`
	MatureDays        float64 `toml:"mature_days" json:"-"`
	...
	DecayTicks        int     `toml:"-" json:"decayTicks"`
	DecayHours        float64 `toml:"decay_hours" json:"-"`
	MineTicks         int     `toml:"-" json:"mineTicks"`
	MineHours         float64 `toml:"mine_hours" json:"-"`
```

(Only the listed fields change; BiteSize, StomachSize, HungerThreshold, ReproThreshold, ReproChance, ReproCost, PopFloor, PopCap, LightRadius keep their current tags. The `...` above stands for those untouched lines, not for omitted work.)

Add the conversion (import `math`):

```go
// resolveTimes converts the unit-named data fields into internal ticks
// and per-tick rates using the sim tick rate.
func (c *Config) resolveTimes() {
	tr := c.Sim.TickRate
	hours := func(h float64) int { return int(math.Round(h * 3600 * tr)) }
	days := func(d float64) int { return int(math.Round(d * 86400 * tr)) }
	for _, t := range c.Types {
		t.MineTicks = hours(t.MineHours)
		t.StarveTicks = hours(t.StarveHours)
		t.DecayTicks = hours(t.DecayHours)
		t.Lifespan = days(t.LifespanDays)
		t.MatureAge = days(t.MatureDays)
		if t.StomachDrainHours > 0 {
			t.Metabolism = t.StomachSize / (t.StomachDrainHours * 3600 * tr)
		}
		t.Speed = t.CellsPerSecond / tr
		for i := range t.Produces {
			p := &t.Produces[i]
			if p.RegrowDays > 0 {
				p.Regrow = p.Max / (p.RegrowDays * 86400 * tr)
			}
		}
	}
}
```

In `Load`, after the entities decode and ID assignment, before `Validate`:

```go
	if cfg.Sim.TickRate <= 0 {
		return nil, fmt.Errorf("sim: tick_rate must be positive")
	}
	cfg.resolveTimes()
```

(The same check stays in `Validate` for configs built in code.)

In `Validate`:
- Add a units sanity block inside the per-type loop, before the kind branch:

```go
		if t.MineHours < 0 || t.StarveHours < 0 || t.DecayHours < 0 ||
			t.LifespanDays < 0 || t.MatureDays < 0 || t.StomachDrainHours < 0 ||
			t.CellsPerSecond < 0 {
			return fmt.Errorf("type %s: time fields must be non-negative", id)
		}
		for _, p := range t.Produces {
			if p.RegrowDays < 0 {
				return fmt.Errorf("type %s: regrow_days must be non-negative", id)
			}
		}
```

(rename the loop variable from `s` to `t` only if that is a trivial local change; otherwise keep `s`.)
- Reword the fauna requirement message to the new names: `"type %s: fauna requires positive stomach_size, bite_size, cells_per_second, stomach_drain_hours, starve_hours, decay_hours, lifespan_days, pop_cap"` (the checks themselves keep testing the internal fields, which are now derived).
- Reword the mine message: `"type %s: mine_hours must be non-negative"` (check stays on `MineTicks < 0`, which can no longer go negative after the units block, but keep it as belt and braces).

- [ ] **Step 4: Convert the live data file**

`data/entities.toml` mushroom produces line:

```toml
produces = [{ resource = "mushroom", amount = 6, max = 6, regrow_days = 1.75 }]
```

Dwarf block, replacing the old duration/rate lines (other lines unchanged):

```toml
stomach_drain_hours = 24
starve_hours = 48
cells_per_second = 1.0
lifespan_days = 58
mature_days = 6
decay_hours = 24
mine_hours = 24
```

(delete `metabolism`, `starve_ticks`, `speed`, `lifespan`, `mature_age`, `decay_ticks`, `mine_ticks` and the speed comment.)

Torch and campfire:

```toml
[type.torch]
name = "Torch"
kind = "structure"
color = "#ffb347"
light_radius = 5
lifespan_days = 1
decay_hours = 0.5

[type.campfire]
name = "Campfire"
kind = "structure"
color = "#e25822"
light_radius = 8
lifespan_days = 0   # never burns out
```

- [ ] **Step 5: Run the conversion test to verify it passes**

Run: `set -o pipefail; go test ./internal/data/ -run 'TestUnitFields|TestNegativeUnitField' -v`
Expected: PASS. Also update `TestMiningFieldsParse` in the same file: its inline `entities.toml` swaps `mine_ticks = 500` for `mine_hours = 0.06944444444444445` (500 ticks), `metabolism = 0.0001` for `stomach_drain_hours = 13.88888888888889` (10/0.0001=100000 ticks = 50000 s), `starve_ticks = 1000` for `starve_hours = 0.1388888888888889`, `speed = 0.5` for `cells_per_second = 1.0`, `lifespan = 100000` for `lifespan_days = 0.5787037037037037`, `decay_ticks = 100` for `decay_hours = 0.013888888888888888`, and regrow `0.001` for `regrow_days = 0.03472222222222222` (6/0.001=6000 ticks); its assertion keeps checking `d.MineTicks != 500`.

- [ ] **Step 6: Write the failing legacy pinning test**

Append to `internal/data/data_test.go`:

```go
// The legacy fixture keeps the pre-pivot rabbit/wolf balance; its unit
// fields must reproduce the exact historical tick values or the long-run
// regression drifts.
func TestLegacyFixtureKeepsHistoricalTicks(t *testing.T) {
	cfg, err := Load("../sim/testdata/legacy")
	if err != nil {
		t.Fatal(err)
	}
	r, w := cfg.Types["rabbit"], cfg.Types["wolf"]
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"rabbit starve", r.StarveTicks, 600},
		{"rabbit lifespan", r.Lifespan, 8000},
		{"rabbit mature", r.MatureAge, 800},
		{"rabbit decay", r.DecayTicks, 400},
		{"wolf starve", w.StarveTicks, 1400},
		{"wolf lifespan", w.Lifespan, 10000},
		{"wolf mature", w.MatureAge, 1000},
		{"wolf decay", w.DecayTicks, 400},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if r.Speed != 0.5 || w.Speed != 0.6 {
		t.Errorf("speeds %v %v, want 0.5 0.6", r.Speed, w.Speed)
	}
	const eps = 1e-9
	near := func(got, want float64) bool { d := got - want; return d < eps && d > -eps }
	if !near(r.Metabolism, 0.02) || !near(w.Metabolism, 0.012) {
		t.Errorf("metabolisms %v %v, want ~0.02 ~0.012", r.Metabolism, w.Metabolism)
	}
	grass := cfg.Types["grass"].Produces[0]
	bush := cfg.Types["bush"].Produces[0]
	if !near(grass.Regrow, 0.09) || !near(bush.Regrow, 0.01) {
		t.Errorf("regrows %v %v, want ~0.09 ~0.01", grass.Regrow, bush.Regrow)
	}
	if tree := cfg.Types["tree"].Produces[0]; tree.Regrow != 0 {
		t.Errorf("tree regrow = %v, want 0", tree.Regrow)
	}
}
```

Run: `set -o pipefail; go test ./internal/data/ -run TestLegacyFixture -v`
Expected: FAIL (fixture still uses old field names, internals decode to zero).

- [ ] **Step 7: Convert the legacy fixture**

In `internal/sim/testdata/legacy/entities.toml` (tick_rate 2.0; hours = ticks/7200, days = ticks/172800, drain_hours = stomach/(metabolism*7200), regrow_days = max/(regrow*172800), cells_per_second = speed*2):

- grass produces: `regrow = 0.09` becomes `regrow_days = 0.0003215020576131687`
- bush produces: `regrow = 0.01` becomes `regrow_days = 0.004629629629629629`
- tree produces: `regrow = 0` becomes `regrow_days = 0`
- rabbit and wolf meat/fur produces: `regrow = 0` becomes `regrow_days = 0`
- rabbit:
  - `metabolism = 0.02` becomes `stomach_drain_hours = 0.06944444444444445`
  - `starve_ticks = 600` becomes `starve_hours = 0.08333333333333333`
  - `speed = 0.5` becomes `cells_per_second = 1.0`
  - `lifespan = 8000` becomes `lifespan_days = 0.046296296296296294`
  - `mature_age = 800` becomes `mature_days = 0.004629629629629629`
  - `decay_ticks = 400` becomes `decay_hours = 0.05555555555555555`
- wolf:
  - `metabolism = 0.012` becomes `stomach_drain_hours = 0.18518518518518517`
  - `starve_ticks = 1400` becomes `starve_hours = 0.19444444444444445`
  - `speed = 0.6` becomes `cells_per_second = 1.2`
  - `lifespan = 10000` becomes `lifespan_days = 0.05787037037037037`
  - `mature_age = 1000` becomes `mature_days = 0.005787037037037037`
  - `decay_ticks = 400` becomes `decay_hours = 0.05555555555555555`

- [ ] **Step 8: Run the pinning test to verify it passes**

Run: `set -o pipefail; go test ./internal/data/ -run TestLegacyFixture -v`
Expected: PASS. If any tick assertion is off by one, adjust that fixture float by appending digits from the exact decimal expansion (compute with `python3 -c "print(600/7200)"` style checks) rather than loosening the test.

- [ ] **Step 9: Full suite**

Run: `set -o pipefail; go test -count=1 ./...`
Expected: PASS, including internal/sim (the legacy fixture feeds its engine tests; the 50k longrun soak takes ~1 minute). Any sim failure here means a legacy value converted inexactly; fix the fixture float, not the sim test.

- [ ] **Step 10: Commit**

```bash
git add internal/data/ data/entities.toml internal/sim/testdata/legacy/entities.toml
git commit -m "Express data durations in unit-named fields"
```

---

### Task 2: Docs sweep and final gate

**Files:**
- Modify: `.claude/skills/verify/SKILL.md` (only if it names old fields)
- Modify: `README.md` (only if it names old fields)

**Interfaces:**
- Consumes: Task 1's field names.

- [ ] **Step 1: Sweep docs for old field names**

Run: `rg -n "mine_ticks|starve_ticks|decay_ticks|mature_age|metabolism|regrow[^_]|speed =" --glob '!docs/superpowers/**' --glob '!.superpowers' --glob '!internal' --glob '!client' --glob '!data'`
Expected: hits only in files that genuinely discuss the old names. Update wording to the new field names where found (the verify skill mentions data file facts; keep its pacing claims, fix field names). If there are no hits, say so and skip the edit.

- [ ] **Step 2: Full gate**

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: all green (client untouched; build is just the gate).

- [ ] **Step 3: Commit and push**

```bash
git add -A
git commit -m "Update docs for unit-named duration fields"
git push
```

If Step 1 changed nothing, push Task 1's commit instead: `git push`.

---

## Self-Review Notes

- Spec coverage: field mapping + formulas (Task 1 Steps 3-4), value roundings (Step 4 values), legacy exactness + pinning test (Steps 6-8), validation renames (Step 3), docs (Task 2), longrun acceptance (Step 9 and Task 2 Step 2). Out of scope respected: no duration strings, tick_rate/autosave untouched.
- The rates use direct derivation (no tick rounding), so legacy metabolism/regrow land within 1e-15 of historical values; the pinning test uses eps 1e-9 and the longrun test asserts ranges, which absorb that.
- In-code sim test configs set internal fields directly and never call Load, so they need no changes; only file-based TOML changes shape.
