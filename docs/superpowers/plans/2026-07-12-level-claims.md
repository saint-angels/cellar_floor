# Level Bar and Claimed Upgrades Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A gold-fed level bar draws random upgrades into a pending queue that players must see and Claim before anything applies; weapons add extra orbiting tools. Supersedes the forge tier system.

**Architecture:** upgrades.toml reshapes into a level curve + draw pool. The sim earns levels from GoldMined thresholds, draws uniformly from non-maxed entries into `World.Pending` (inert), and applies effects only from `World.Claims`. The server swaps the `upgrade` intent for `claim` (FIFO pop) and streams level/pending/claims. The client replaces the forge button with a level bar plus a claim card, and renders claimed weapons as additional orbits.

**Tech Stack:** Go stdlib + BurntSushi/toml, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-12-level-claims-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on Go tasks; `set -o pipefail` when piping `go test`.
- Nothing applies unclaimed: effects read `Claims` only; `Pending` is inert. Draws are uniform over entries where claimed + already-pending occurrences < Max (Max 0 = uncapped); draws use the world RNG inside the tick for determinism.
- Exact data file per the spec's Data section (curve 2.0 / 1.6; Sharper Picks damage 1 max 0; Lucky Veins luck 1 max 2; Chisel weapon 1 max 1 #e8d44d r10 1100ms; Hammer weapon 2 max 1 #b87333 r18 2300ms).
- Level N cumulative target = sum of level_base * level_growth^(k-1), k=1..N, float math against GoldMined.
- The forge system is REMOVED in the same commits that replace it (buyUpgrade, upgrade intent, forge button, UpgradeLevel field and its wire fields); the repo builds at every task boundary EXCEPT between Tasks 2 and 3 as noted.
- Never touch the live server (port 8080) or canonical saves; e2e on :8083, scratch data dir REFRESHED (upgrades.toml changes shape), launched from the repo root. Do not push until Task 5.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: Curve and pool in the data layer

**Files:**
- Modify: `internal/data/data.go`, `data/upgrades.toml`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: `data.Upgrade{Name string toml:"name" json:"name"; Kind string toml:"kind" json:"kind"; Amount int toml:"amount" json:"amount"; Max int toml:"max" json:"max"; Color string toml:"color" json:"color"; Radius int toml:"radius" json:"radius"; PeriodMs int toml:"period_ms" json:"periodMs"}` REPLACING the old {Name, Cost, Damage} shape; `Config.LevelBase, LevelGrowth float64` decoded from top-level keys of upgrades.toml.

- [ ] **Step 1: Write the failing tests**

REPLACE `TestUpgradesParse` and `TestUpgradeValidation` in `internal/data/data_test.go` with:

```go
func TestUpgradePoolParses(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LevelBase != 2.0 || cfg.LevelGrowth != 1.6 {
		t.Fatalf("curve = %v %v, want 2.0 1.6", cfg.LevelBase, cfg.LevelGrowth)
	}
	if len(cfg.Upgrades) != 4 {
		t.Fatalf("pool = %d entries, want 4", len(cfg.Upgrades))
	}
	sharper := cfg.Upgrades[0]
	if sharper.Name != "Sharper Picks" || sharper.Kind != "damage" || sharper.Amount != 1 || sharper.Max != 0 {
		t.Fatalf("sharper = %+v", sharper)
	}
	chisel := cfg.Upgrades[2]
	if chisel.Kind != "weapon" || chisel.Color != "#e8d44d" || chisel.Radius != 10 || chisel.PeriodMs != 1100 {
		t.Fatalf("chisel = %+v", chisel)
	}
}

func TestUpgradePoolValidation(t *testing.T) {
	base := func() *Config {
		cfg := minimalConfig()
		cfg.LevelBase, cfg.LevelGrowth = 2, 1.6
		cfg.Upgrades = []Upgrade{{Name: "Picks", Kind: "damage", Amount: 1}}
		return cfg
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("valid pool rejected: %v", err)
	}
	cfg := base()
	cfg.Upgrades[0].Kind = "haste"
	if err := Validate(cfg); err == nil {
		t.Fatal("unknown kind must fail")
	}
	cfg = base()
	cfg.Upgrades[0].Amount = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("non-positive amount must fail")
	}
	cfg = base()
	cfg.Upgrades = append(cfg.Upgrades, Upgrade{Name: "Chisel", Kind: "weapon", Amount: 1, Max: 1})
	if err := Validate(cfg); err == nil {
		t.Fatal("weapon without color radius period must fail")
	}
	cfg = base()
	cfg.LevelGrowth = 1.0
	if err := Validate(cfg); err == nil {
		t.Fatal("level_growth must exceed 1")
	}
	cfg = base()
	cfg.LevelBase = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("level_base must be positive")
	}
	cfg = base()
	cfg.Upgrades = append(cfg.Upgrades, Upgrade{Name: "Picks", Kind: "damage", Amount: 1})
	if err := Validate(cfg); err == nil {
		t.Fatal("duplicate name must fail")
	}
}
```

NOTE the curve fields must be legal absent (in-code configs and the legacy fixture have no upgrades and no curve): validation of LevelBase/LevelGrowth applies ONLY when the pool is non-empty.

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/data/ -run TestUpgradePool 2>&1 | tail -3`
Expected: build FAIL (LevelBase undefined; Upgrade fields changed).

- [ ] **Step 3: Implement**

`internal/data/data.go`:
- Replace the Upgrade struct with the new shape (Interfaces above).
- `Config` gains `LevelBase, LevelGrowth float64`.
- The upgrades.toml decode block becomes:

```go
	var up struct {
		LevelBase   float64   `toml:"level_base"`
		LevelGrowth float64   `toml:"level_growth"`
		Upgrade     []Upgrade `toml:"upgrade"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "upgrades.toml"), &up); err != nil {
		return nil, fmt.Errorf("upgrades.toml: %w", err)
	}
	cfg.Upgrades = up.Upgrade
	cfg.LevelBase = up.LevelBase
	cfg.LevelGrowth = up.LevelGrowth
```

- Replace the upgrade validation block:

```go
	if len(cfg.Upgrades) > 0 {
		if cfg.LevelBase <= 0 {
			return fmt.Errorf("upgrades: level_base must be positive")
		}
		if cfg.LevelGrowth <= 1 {
			return fmt.Errorf("upgrades: level_growth must exceed 1")
		}
	}
	upNames := map[string]bool{}
	validKinds := map[string]bool{"damage": true, "luck": true, "weapon": true}
	for i, u := range cfg.Upgrades {
		if u.Name == "" {
			return fmt.Errorf("upgrade[%d]: name is required", i)
		}
		if upNames[u.Name] {
			return fmt.Errorf("upgrade: duplicate name %q", u.Name)
		}
		upNames[u.Name] = true
		if !validKinds[u.Kind] {
			return fmt.Errorf("upgrade %s: unknown kind %q", u.Name, u.Kind)
		}
		if u.Amount <= 0 {
			return fmt.Errorf("upgrade %s: amount must be positive", u.Name)
		}
		if u.Max < 0 {
			return fmt.Errorf("upgrade %s: max must be non-negative", u.Name)
		}
		if u.Kind == "weapon" && (u.Color == "" || u.Radius <= 0 || u.PeriodMs <= 0) {
			return fmt.Errorf("upgrade %s: weapons need color, radius, period_ms", u.Name)
		}
	}
```

- `data/upgrades.toml`: replace the whole file with the spec's Data section verbatim (curve comment + two knobs + four entries).
- Legacy fixture upgrades.toml stays a comment-only empty file (valid: empty pool skips curve checks).

- [ ] **Step 4: Run to verify pass**

Run: `set -o pipefail; go test ./internal/data/ 2>&1 | tail -2`
Expected: PASS. `go build ./...` FAILS at internal/server (buyUpgrade uses .Cost/.Damage) until Task 3; do not gate on it; commit data only.

- [ ] **Step 5: Commit**

```bash
git add internal/data/ data/upgrades.toml
git commit -m "Reshape the upgrade data into a level curve and draw pool"
```

---

### Task 2: Levels, draws, pending, and claimed effects in the sim

**Files:**
- Modify: `internal/sim/world.go`, `internal/sim/mine.go`, `internal/sim/tick.go`
- Test: `internal/sim/level_test.go` (new), `internal/sim/mine_test.go` (bonus test rewrite)

**Interfaces:**
- Consumes: `cfg.Upgrades` (new shape), `cfg.LevelBase/LevelGrowth`, `GoldMined`.
- Produces: World fields `Level int json:"level"`, `Pending []string json:"pending,omitempty"`, `Claims map[string]int json:"claims,omitempty"` (init in NewWorld/SetConfig like MineDamage); `UpgradeLevel` field DELETED; `(w *World) NextLevelGold() int` (cumulative float target for Level+1, rounded up); `(w *World) MineBonus() int` (claims of damage+weapon kinds: sum Amount*count); `(w *World) LuckBonus() int` (claims of luck); `levelStep()` in the tick after spreadStep: while GoldMined >= NextLevelGold, Level++ and draw into Pending with the eligibility rule, emitting the events from the spec; mine.go gold roll uses `sc.GoldMin + w.LuckBonus()` and `sc.GoldMax + w.LuckBonus()`.

- [ ] **Step 1: Write the failing tests**

Create `internal/sim/level_test.go`:

```go
package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

func levelCfg() *data.Config {
	cfg := mineCfg() // fast mining world helpers
	cfg.LevelBase = 2
	cfg.LevelGrowth = 2
	cfg.Upgrades = []data.Upgrade{
		{Name: "Sharper", Kind: "damage", Amount: 1, Max: 0},
		{Name: "Lucky", Kind: "luck", Amount: 1, Max: 1},
	}
	return cfg
}

func TestLevelTargetsEscalate(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	if got := w.NextLevelGold(); got != 2 {
		t.Fatalf("level 1 target = %d, want 2", got)
	}
	w.Level = 1
	if got := w.NextLevelGold(); got != 6 { // 2 + 4
		t.Fatalf("level 2 target = %d, want 6", got)
	}
	w.Level = 2
	if got := w.NextLevelGold(); got != 14 { // 2 + 4 + 8
		t.Fatalf("level 3 target = %d, want 14", got)
	}
}

func TestCrossingDrawsIntoPending(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.GoldMined = 7 // covers targets 2 and 6
	evs := w.Step()
	if w.Level != 2 {
		t.Fatalf("level = %d, want 2", w.Level)
	}
	if len(w.Pending) != 2 {
		t.Fatalf("pending = %v, want two draws", w.Pending)
	}
	named := 0
	for _, ev := range evs {
		if ev.Type == "level" {
			named++
		}
	}
	if named != 2 {
		t.Fatalf("level events = %d, want 2", named)
	}
	// determinism
	w2 := NewWorld(5, 5, 1, levelCfg())
	w2.GoldMined = 7
	w2.Step()
	for i := range w.Pending {
		if w.Pending[i] != w2.Pending[i] {
			t.Fatal("draws not deterministic")
		}
	}
}

func TestPendingIsInertUntilClaimed(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Pending = []string{"Sharper", "Sharper"}
	if w.MineBonus() != 0 {
		t.Fatal("pending upgrades must not add damage")
	}
	w.Claims = map[string]int{"Sharper": 3}
	if w.MineBonus() != 3 {
		t.Fatalf("MineBonus = %d, want 3", w.MineBonus())
	}
}

func TestCappedEntriesLeaveTheDrawPool(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Claims = map[string]int{"Lucky": 1} // Lucky at max
	w.GoldMined = 1000
	for i := 0; i < 5; i++ {
		w.Step()
	}
	for _, name := range w.Pending {
		if name == "Lucky" {
			t.Fatal("capped entry drawn")
		}
	}
	if w.Level == 0 || len(w.Pending) == 0 {
		t.Fatalf("levels should still accrue: level %d pending %d", w.Level, len(w.Pending))
	}
}

func TestLuckRaisesDropBounds(t *testing.T) {
	w := newMineWorld(t) // chance 1, min=max=2
	w.Cfg().Upgrades = []data.Upgrade{{Name: "Lucky", Kind: "luck", Amount: 1, Max: 2}}
	w.Claims = map[string]int{"Lucky": 2}
	e := w.Spawn("miner", Point{2, 2})
	_ = e
	for i := 0; i < 30; i++ {
		w.Step()
	}
	if w.Gold != 4 { // 2 + luck 2, min == max keeps it exact
		t.Fatalf("gold = %d, want 4 with +2 luck", w.Gold)
	}
}

func TestLevelStateSurvivesSaveLoad(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Level = 3
	w.Pending = []string{"Sharper"}
	w.Claims = map[string]int{"Lucky": 1}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(levelCfg())
	if w2.Level != 3 || len(w2.Pending) != 1 || w2.Claims["Lucky"] != 1 {
		t.Fatalf("state lost: %d %v %v", w2.Level, w2.Pending, w2.Claims)
	}
}
```

Also in mine_test.go: REWRITE `TestMineBonusSpeedsMining` for the new model (delete the UpgradeLevel usage): set `cfg.Upgrades = []data.Upgrade{{Name: "Sharper", Kind: "damage", Amount: 1, Max: 0}}` on mineCfg, `w.Claims = map[string]int{"Sharper": 1}`, assert damage 2 per tick and `w.MineBonus() == 1`.

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestLevel|TestCrossing|TestPendingIsInert|TestCapped|TestLuck' 2>&1 | tail -3`
Expected: build FAIL (fields undefined; UpgradeLevel references from the old test).

- [ ] **Step 3: Implement**

`internal/sim/world.go`:
- Replace `UpgradeLevel int json:"upgradeLevel"` with:

```go
	Level   int            `json:"level"`
	Pending []string       `json:"pending,omitempty"`
	Claims  map[string]int `json:"claims,omitempty"`
```

- Init `Claims` in NewWorld and nil-guard in SetConfig (like MineDamage).
- Replace MineBonus and add helpers:

```go
// MineBonus is the summed damage of CLAIMED upgrades; pending draws are inert.
func (w *World) MineBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "damage" || u.Kind == "weapon" {
			bonus += u.Amount * w.Claims[u.Name]
		}
	}
	return bonus
}

// LuckBonus widens gold drops from claimed luck upgrades.
func (w *World) LuckBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "luck" {
			bonus += u.Amount * w.Claims[u.Name]
		}
	}
	return bonus
}

// NextLevelGold is the cumulative mined gold required for the next level.
func (w *World) NextLevelGold() int {
	if len(w.cfg.Upgrades) == 0 {
		return 1 << 30
	}
	target := 0.0
	step := w.cfg.LevelBase
	for k := 0; k <= w.Level; k++ {
		target += step
		step *= w.cfg.LevelGrowth
	}
	return int(math.Ceil(target))
}
```

(import math in world.go.)

`internal/sim/tick.go`, after `w.spreadStep()`:

```go
	// 7. level ups: mined gold fills the bar; each crossing draws a
	// pending upgrade that stays inert until a player claims it
	events = append(events, w.levelStep()...)
```

and:

```go
func (w *World) levelStep() []Event {
	var evs []Event
	for len(w.cfg.Upgrades) > 0 && w.GoldMined >= w.NextLevelGold() {
		w.Level++
		pendingCount := map[string]int{}
		for _, name := range w.Pending {
			pendingCount[name]++
		}
		var eligible []data.Upgrade
		for _, u := range w.cfg.Upgrades {
			if u.Max == 0 || w.Claims[u.Name]+pendingCount[u.Name] < u.Max {
				eligible = append(eligible, u)
			}
		}
		if len(eligible) == 0 {
			evs = append(evs, Event{Tick: w.Tick, Type: "level",
				Msg: fmt.Sprintf("the colony reached level %d", w.Level)})
			continue
		}
		pick := eligible[w.RandN(len(eligible))]
		w.Pending = append(w.Pending, pick.Name)
		evs = append(evs, Event{Tick: w.Tick, Type: "level",
			Msg: fmt.Sprintf("the colony reached level %d: %s awaits", w.Level, pick.Name)})
	}
	return evs
}
```

(import data in tick.go if not present.)

`internal/sim/mine.go` gold roll: `amt := sc.GoldMin + w.LuckBonus()` and the spread uses `sc.GoldMax + w.LuckBonus() - amt + 1`... keep it simple and explicit:

```go
			lo := sc.GoldMin + w.LuckBonus()
			hi := sc.GoldMax + w.LuckBonus()
			amt := lo
			if hi > lo {
				amt += w.RandN(hi - lo + 1)
			}
```

- [ ] **Step 4: Full suite**

Run: `set -o pipefail; go vet ./internal/sim/... 2>/dev/null; go test -count=1 ./internal/sim/ ./internal/data/ ./internal/gen/ 2>&1 | tail -4`
Expected: PASS (internal/server still broken until Task 3; exclude it here).

- [ ] **Step 5: Commit**

```bash
git add internal/sim/
git commit -m "Earn levels from mined gold and queue inert upgrade draws"
```

---

### Task 3: Claim intent and wire on the server

**Files:**
- Delete: `internal/server/upgrade.go`, `internal/server/upgrade_test.go` content replaced
- Modify: `internal/server/server.go`, `internal/server/protocol.go`, `internal/server/players_ws_test.go`
- Test: rewrite `internal/server/upgrade_test.go` as claim tests

**Interfaces:**
- Consumes: `w.Pending`, `w.Claims`, `w.Level`, `w.NextLevelGold`, `w.GoldMined`.
- Produces: `{type:"claim", player}` -> `(s *Server) claimUpgrade(token) PlayerMsg` (errors "you need a living dwarf" / "nothing to claim"; success pops Pending[0], `Claims[name]++`, pending event type "claimed", msg `"<player> claimed <name>"`); reader case replaces the `upgrade` case; SnapshotMsg: REPLACE `UpgradeLevel` with `Level int json:"level"`, `GoldMined int json:"goldMined"`, `NextLevelGold int json:"nextLevelGold"`, `Pending []string json:"pending"`, `Claims map[string]int json:"claims"` (upgrades pool field stays); TickMsg: REPLACE `UpgradeLevel` with the same five (pending/claims always included; they are tiny).

- [ ] **Step 1: Rewrite the failing tests**

Replace `internal/server/upgrade_test.go` with:

```go
package server

import "testing"

func TestClaimUpgrade(t *testing.T) {
	s := newPlayerServer(t)
	if res := s.claimUpgrade("ghost"); res.Error == "" {
		t.Fatal("no dwarf: must error")
	}
	s.spawnDwarf("tok", "Misha")
	if res := s.claimUpgrade("tok"); res.Error != "nothing to claim" {
		t.Fatalf("got %q, want nothing to claim", res.Error)
	}
	s.world.Pending = []string{"Sharper Picks", "Chisel"}
	if res := s.claimUpgrade("tok"); res.Error != "" {
		t.Fatalf("claim failed: %v", res.Error)
	}
	if s.world.Claims["Sharper Picks"] != 1 {
		t.Fatalf("claims = %v", s.world.Claims)
	}
	if len(s.world.Pending) != 1 || s.world.Pending[0] != "Chisel" {
		t.Fatalf("pending after claim = %v, want FIFO pop", s.world.Pending)
	}
	if len(s.pending) != 1 || s.pending[0].Type != "claimed" {
		t.Fatalf("events = %+v", s.pending)
	}
}
```

Run: `set -o pipefail; go test ./internal/server/ -run TestClaimUpgrade 2>&1 | tail -3`
Expected: build FAIL (claimUpgrade undefined; buyUpgrade references dead fields).

- [ ] **Step 2: Implement**

Replace `internal/server/upgrade.go` wholesale:

```go
package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// claimUpgrade applies the oldest pending upgrade draw. Caller holds s.mu.
func (s *Server) claimUpgrade(token string) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if len(s.world.Pending) == 0 {
		pm.Error = "nothing to claim"
		return pm
	}
	name := s.world.Pending[0]
	s.world.Pending = s.world.Pending[1:]
	if s.world.Claims == nil {
		s.world.Claims = map[string]int{}
	}
	s.world.Claims[name]++
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "claimed",
		Msg: fmt.Sprintf("%s claimed %s", s.players[token].Name, name),
	})
	return pm
}
```

server.go reader: rename the `upgrade` case to `claim` calling claimUpgrade (same shape).

protocol.go: swap the fields per Interfaces; BuildSnapshot sets `Level: w.Level, GoldMined: w.GoldMined, NextLevelGold: w.NextLevelGold(), Pending: w.Pending, Claims: w.Claims`; BuildTick sets the same five.

players_ws_test.go: the ws upgrade round-trip test becomes a claim round-trip (seed `s.world.Pending` under lock, send claim, assert Claims and FIFO under lock). Any protocol test asserting upgradeLevel updates to the new fields.

- [ ] **Step 3: Full Go suite, commit**

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... 2>&1 | tail -5`
Expected: PASS everywhere (repo builds again).

```bash
git add internal/server/
git commit -m "Replace the forge intent with pending upgrade claims"
```

---

### Task 4: Level bar, claim card, and weapon orbits in the client

**Files:**
- Modify: `client/src/types.ts`, `client/src/world.ts`, `client/src/net.ts`, `client/src/ui.ts`, `client/src/fx.ts`, `client/index.html`

**Interfaces:**
- Consumes: snapshot/tick `level`, `goldMined`, `nextLevelGold`, `pending`, `claims`, pool `upgrades` (new shape with kind/color/radius/periodMs); claim intent.
- Produces: `sendClaim()`; world fields `level`, `goldMined`, `nextLevelGold`, `pending: string[]`, `claims: Record<string, number>`; level bar UI replacing the forge button; claim card; extra weapon orbits in fx.

- [ ] **Step 1: Types, world, net**

types.ts: `Upgrade` becomes `{name: string; kind: string; amount: number; max: number; color: string; radius: number; periodMs: number}`; SnapshotMsg swaps `upgradeLevel` for `level: number; goldMined: number; nextLevelGold: number; pending: string[]; claims: Record<string, number>;` (upgrades stays); TickMsg likewise (all five).

world.ts: fields `level = 0; goldMined = 0; nextLevelGold = 1; pending: string[] = []; claims: Record<string, number> = {};` set in applySnapshot (`??` defaults) and applyTick (always overwrite from the message; they are always present). Remove `upgradeLevel`.

net.ts: replace `sendUpgrade` with:

```ts
export function sendClaim() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "claim", player: playerToken() }));
}
```

- [ ] **Step 2: Level bar and claim card**

index.html: replace `#forge-btn` with:

```html
      <div id="levelbox"><span id="levellabel">Lv 0</span><div id="levelbar"><div id="levelfill"></div></div></div>
```

css: `#levelbox { display: flex; align-items: center; gap: 8px; margin-top: 6px; } #levelbar { flex: 1; height: 10px; background: #1a1815; border: 1px solid #444; } #levelfill { height: 100%; width: 0%; background: #4a7c3f; }`.

Add the claim card inside #map after #recap:

```html
    <div id="claimcard"><div id="claimtext"></div><button id="claim-btn">Claim</button><div id="claimmore"></div></div>
```

css: `#claimcard { position: absolute; top: 48px; left: 50%; transform: translateX(-50%); display: none; background: #1c1815; border: 1px solid #6a5a2e; padding: 12px 16px; text-align: center; } #claimcard button { background: #7a5a1e; color: #fff; border: none; padding: 6px 14px; margin-top: 8px; cursor: pointer; font: inherit; } #claimmore { color: #8d857a; margin-top: 4px; }`.

ui.ts: replace initForge with:

```ts
function initLevel() {
  const label = document.getElementById("levellabel")!;
  const fill = document.getElementById("levelfill")!;
  const card = document.getElementById("claimcard")!;
  const text = document.getElementById("claimtext")!;
  const more = document.getElementById("claimmore")!;
  const btn = document.getElementById("claim-btn") as HTMLButtonElement;
  btn.onclick = () => sendClaim();
  world.onChange(() => {
    label.textContent = `Lv ${world.level}`;
    const prev = world.level === 0 ? 0 : world.nextLevelGold - levelSpan();
    const span = world.nextLevelGold - prev;
    const frac = span > 0 ? (world.goldMined - prev) / span : 0;
    fill.style.width = `${Math.max(0, Math.min(1, frac)) * 100}%`;
    const next = world.pending[0];
    if (!next) {
      card.style.display = "none";
      return;
    }
    text.textContent = `Level ${world.level - world.pending.length + 1} reached: ${next}`;
    more.textContent = world.pending.length > 1 ? `+${world.pending.length - 1} more waiting` : "";
    btn.disabled = world.playerState !== "alive";
    card.style.display = "block";
  });
}

// levelSpan is the gold width of the current level segment, derived from
// the curve the server already resolved into nextLevelGold: the previous
// threshold is nextLevelGold minus base*growth^level, mirrored from data
function levelSpan(): number {
  const base = 2.0, growth = 1.6; // must match upgrades.toml
  return Math.ceil(base * Math.pow(growth, world.level));
}
```

WAIT, hardcoding the curve client-side violates the data-first rule. Instead have the wire carry the segment: ADD `prevLevelGold int json:"prevLevelGold"` to both messages server-side in Task 3 (target already passed for the current level: `w.NextLevelGold()` computed at `Level-1`... implement server-side as a second helper value: BuildSnapshot/BuildTick set `PrevLevelGold: w.PrevLevelGold()` where `PrevLevelGold()` returns the cumulative target for the CURRENT level (0 when Level == 0)). Task 3's implementer adds this sixth field; Task 4 then computes `frac = (goldMined - prevLevelGold) / (nextLevelGold - prevLevelGold)`. THIS PARAGRAPH IS BINDING FOR BOTH TASKS; the levelSpan() sketch above must NOT be implemented.

- [ ] **Step 3: Weapon orbits in fx.ts**

In the orbit loop, after drawing the base tool, iterate claimed weapons:

```ts
    for (const u of world.upgrades) {
      if (u.kind !== "weapon" || !(world.claims[u.name] > 0)) continue;
      const wAngle = (fxClock / u.periodMs) * Math.PI * 2 + e.id * 2.4 + u.radius;
      const wx = cx + Math.cos(wAngle) * u.radius;
      const wy = cy + Math.sin(wAngle) * u.radius;
      ctx.fillStyle = u.color;
      ctx.fillRect(wx - TOOL_SIZE / 2, wy - TOOL_SIZE / 2, TOOL_SIZE, TOOL_SIZE);
      const wcx = Math.floor(wx / TILE);
      const wcy = Math.floor(wy / TILE);
      const inW = wcx >= 0 && wcy >= 0 && wcx < world.width && wcy < world.height;
      const wcell = inW ? wcy * world.width + wcx : -1;
      const wprev = toolCell.get(e.id * 131 + u.radius) ?? -1;
      const wmine = wcell >= 0 && (world.terrainTypes[world.terrain[wcell]]?.mineable ?? false);
      if (wmine && wcell !== wprev && running) {
        spawnDebris(wx, wy, cx, cy, DEBRIS_COLOR);
        shakes.set(wcell, now);
      }
      toolCell.set(e.id * 131 + u.radius, wcell);
    }
```

(weapon strikes rattle and spray debris; damage numbers stay driven by the primary tool's strikes on tracked cells to avoid double-counting pops; the `e.id * 131 + u.radius` key namespaces per-weapon tracking away from the base tool's `e.id` key.)

- [ ] **Step 4: Recap line**

ui.ts initRecap text: after the parts join, append pending info:

```ts
    const claimsLine = world.pending.length
      ? ` ${world.pending.length} upgrade${world.pending.length > 1 ? "s" : ""} await your claim!`
      : "";
    box.textContent = `While you were away (${dur}): ${parts.join(", ")}.${claimsLine}`;
```

- [ ] **Step 5: Build gate, commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add client/ 
git commit -m "Show the level bar claim card and weapon orbits"
```

---

### Task 5: End-to-end verification, docs, push

**Files:** throwaway scripts in the scratchpad; `.claude/skills/verify/SKILL.md`.

- [ ] **Step 1: First level live**

Fresh scratch server (REFRESH data dir; upgrades.toml is reshaped). Spawn, torch the crust, watch the bar: with level_base 2, the first two mined gold fill it; assert the claim card appears with one of the four pool names, the level events hit the feed, and pending stays INERT (ws snapshot: claims empty, damage deltas still +1/tick) until you click Claim; after claiming, verify the effect (damage delta +2/tick if Sharper/weapon; wider drops if Lucky; report which was drawn). Screenshots of bar, card, and post-claim.

- [ ] **Step 2: Away stacking**

Disconnect; `POST /api/advance?ticks=200000` (a chunk of mining); reconnect: multiple pending drawn while away, the claim card shows the oldest with "+N more waiting", the recap toast includes the "upgrades await" line. Claim through two of them and watch the card advance FIFO. Screenshots.

- [ ] **Step 3: Weapon orbit visual (seed permitting)**

If any claim yielded Chisel or Hammer, screenshot a mining dwarf with two orbiting tools (colors #e8d44d / #b87333). If the seed never drew a weapon within your claims, force it: set `world.Pending` via... there is no debug write endpoint, so instead keep claiming through advances until a weapon comes up (uniform over 4 entries; a handful of levels suffices); report how many draws it took.

- [ ] **Step 4: Docs, gate, push**

`.claude/skills/verify/SKILL.md`: replace the forge-button note with the level bar (`#levelbox`, fills from goldMined between prevLevelGold/nextLevelGold), the claim card (`#claimcard`/`#claim-btn`, FIFO pending queue, effects only after claim), and weapon orbits (extra tool sprites from claimed weapon upgrades).

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: green. Kill your :8083 server. Commit docs, push everything.

```bash
git add -A
git commit -m "Verify level claims end to end and refresh docs"
git push
```

Relay: server restart + reload; upgrades.toml changed shape; forged tiers from the superseded system are dropped (young world, fast refund via the curve).

---

## Self-Review Notes

- Spec coverage: curve+pool+validation (T1), levels/draws/inert-pending/effects/luck (T2), claim intent + wire (T3), bar/card/weapons/recap line (T4), e2e (T5).
- BINDING cross-task fix baked in: `prevLevelGold` rides the wire (T3) so the client never hardcodes the curve (T4's levelSpan sketch is explicitly forbidden).
- Draw eligibility counts pending occurrences so two Chisels can never coexist in the queue.
- The forge system's removal is spread across T1 (data shape), T2 (UpgradeLevel field), T3 (intent), T4 (button); repo intentionally broken between T1 and T3 for internal/server, gated per task accordingly.
