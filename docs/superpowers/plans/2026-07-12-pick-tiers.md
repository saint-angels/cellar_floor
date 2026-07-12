# Pick Tiers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Colony-wide pick upgrades (data-driven tier track, bought with shared gold, additive integer damage) plus a welcome-back recap toast, closing the mine-collect-improve loop with the first purchase inside the opening minute.

**Architecture:** `data/upgrades.toml` loads into `Config.Upgrades`. `World.UpgradeLevel` counts purchased tiers; `MineBonus()` sums their damage and `mineStep` adds it per strike. Recap counters (`BlocksMined`, `GoldMined`, `MoldGrown`) increment at the source. The server gains an `upgrade` intent (torch-intent pattern) and sends a `recap` message on hello with deltas against a per-player snapshot stored in players.json. The client gains a forge button and a recap toast.

**Tech Stack:** Go stdlib + BurntSushi/toml, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-12-pick-tiers-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on Go tasks; `set -o pipefail` when piping `go test`.
- Exact tier data (data/upgrades.toml): Copper 3g +1, Iron 8g +1, Steel 20g +2, Mithril 50g +3, Adamant 120g +5, in that order; purchases strictly in order.
- Effective damage = `mine_damage + MineBonus()`; all ints; UpgradeLevel and the three counters persist in world.json and zero naturally on reset (fresh world).
- Recap only for known players, deltas since the stored snapshot, snapshot advanced after sending; client toast only when `ticks >= 120` and something is nonzero.
- The engine stays generic; no entity-type or tier names in internal/sim.
- Never touch the live server (port 8080) or canonical saves; e2e on :8083, scratch data dir (REFRESH it: upgrades.toml is a new required file), launched from the repo root.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: Upgrade table in the data layer

**Files:**
- Create: `data/upgrades.toml`, `internal/sim/testdata/legacy/upgrades.toml` (empty file with a comment; Load requires the file)
- Modify: `internal/data/data.go`
- Test: `internal/data/data_test.go`

**Interfaces:**
- Produces: `type Upgrade struct { Name string toml:"name" json:"name"; Cost int toml:"cost" json:"cost"; Damage int toml:"damage" json:"damage" }`; `Config.Upgrades []Upgrade` decoded from `upgrades.toml` (`[[upgrade]]` array; a file with no entries yields an empty, valid list).

- [ ] **Step 1: Write the failing tests**

Append to `internal/data/data_test.go`:

```go
func TestUpgradesParse(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []Upgrade{
		{Name: "Copper Picks", Cost: 3, Damage: 1},
		{Name: "Iron Picks", Cost: 8, Damage: 1},
		{Name: "Steel Picks", Cost: 20, Damage: 2},
		{Name: "Mithril Picks", Cost: 50, Damage: 3},
		{Name: "Adamant Picks", Cost: 120, Damage: 5},
	}
	if len(cfg.Upgrades) != len(want) {
		t.Fatalf("upgrades = %d, want %d", len(cfg.Upgrades), len(want))
	}
	for i, u := range want {
		if cfg.Upgrades[i] != u {
			t.Fatalf("upgrade[%d] = %+v, want %+v", i, cfg.Upgrades[i], u)
		}
	}
}

func TestUpgradeValidation(t *testing.T) {
	base := func() *Config {
		cfg := minimalConfig()
		cfg.Upgrades = []Upgrade{{Name: "Copper", Cost: 3, Damage: 1}}
		return cfg
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("valid upgrades rejected: %v", err)
	}
	cfg := base()
	cfg.Upgrades = append(cfg.Upgrades, Upgrade{Name: "Copper", Cost: 5, Damage: 1})
	if err := Validate(cfg); err == nil {
		t.Fatal("duplicate name must fail")
	}
	cfg = base()
	cfg.Upgrades[0].Cost = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("non-positive cost must fail")
	}
	cfg = base()
	cfg.Upgrades[0].Damage = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("non-positive damage must fail")
	}
	cfg = base()
	cfg.Upgrades[0].Name = ""
	if err := Validate(cfg); err == nil {
		t.Fatal("empty name must fail")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/data/ -run TestUpgrade 2>&1 | tail -3`
Expected: build FAIL (Upgrade undefined).

- [ ] **Step 3: Implement**

`internal/data/data.go`:
- Add the `Upgrade` struct per Interfaces; `Config` gains `Upgrades []Upgrade`.
- In `Load`, decode after terrain.toml:

```go
	var up struct {
		Upgrade []Upgrade `toml:"upgrade"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "upgrades.toml"), &up); err != nil {
		return nil, fmt.Errorf("upgrades.toml: %w", err)
	}
	cfg.Upgrades = up.Upgrade
```

- In `Validate`, before the per-type loop:

```go
	upNames := map[string]bool{}
	for i, u := range cfg.Upgrades {
		if u.Name == "" {
			return fmt.Errorf("upgrade[%d]: name is required", i)
		}
		if upNames[u.Name] {
			return fmt.Errorf("upgrade: duplicate name %q", u.Name)
		}
		upNames[u.Name] = true
		if u.Cost <= 0 {
			return fmt.Errorf("upgrade %s: cost must be positive", u.Name)
		}
		if u.Damage <= 0 {
			return fmt.Errorf("upgrade %s: damage must be positive", u.Name)
		}
	}
```

- Create `data/upgrades.toml` exactly per the spec's Upgrades data section (with the leading comment line).
- Create `internal/sim/testdata/legacy/upgrades.toml` containing only `# no upgrades in the legacy era` (empty list keeps legacy behavior identical).
- The temp-dir fixtures in data_test.go (TestMiningFieldsParse, TestUnitFieldsConvertToTicks) write their own data dirs and now need `write("upgrades.toml", "")` (empty file) or Load fails; add that line to each. Also check internal/server and internal/gen helpers that Load from scratch dirs; loadCfg uses the real data dir (fine) and legacy (covered).

- [ ] **Step 4: Run to verify pass, full data package + build**

Run: `set -o pipefail; go test ./internal/data/ 2>&1 | tail -2 && go build ./... && go test ./internal/sim/ -run TestSpawnCopies -count=1 2>&1 | tail -2`
Expected: PASS (the sim spot-check proves the legacy fixture still loads).

- [ ] **Step 5: Commit**

```bash
git add internal/data/ data/upgrades.toml internal/sim/testdata/legacy/upgrades.toml
git commit -m "Load the colony pick upgrade track from data"
```

---

### Task 2: Upgrade level, mine bonus, and recap counters in the sim

**Files:**
- Modify: `internal/sim/world.go`, `internal/sim/mine.go`, `internal/sim/tick.go`
- Test: `internal/sim/mine_test.go`, `internal/sim/spread_test.go`

**Interfaces:**
- Consumes: `cfg.Upgrades`.
- Produces: `World.UpgradeLevel int` (json `upgradeLevel`), `World.BlocksMined int` (json `blocksMined`), `World.GoldMined int` (json `goldMined`), `World.MoldGrown int` (json `moldGrown`); `(w *World) MineBonus() int` (sum of first UpgradeLevel entries' Damage, level clamped to table length); mineStep deals `s.MineDamage + w.MineBonus()`; counters increment: BlocksMined per completed cell, GoldMined += amt per drop, MoldGrown per spread conversion AND per sprout.

- [ ] **Step 1: Write the failing tests**

Append to `internal/sim/mine_test.go`:

```go
func TestMineBonusSpeedsMining(t *testing.T) {
	cfg := mineCfg()
	cfg.Upgrades = []data.Upgrade{{Name: "Copper", Cost: 3, Damage: 1}, {Name: "Iron", Cost: 8, Damage: 1}}
	w := NewWorld(5, 5, 1, cfg)
	w.Spawn("sunstone", Point{0, 0})
	w.UpgradeLevel = 1 // Copper only: damage 1+1=2 against the 10 hp rock
	face := Point{3, 2}
	w.Terrain[idx(w, face)] = TerrainRock
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	w.Step()
	if got := w.MineDamage[idx(w, face)]; got != 2 {
		t.Fatalf("damage per tick = %d, want 2 at upgrade level 1", got)
	}
	if w.MineBonus() != 1 {
		t.Fatalf("MineBonus = %d, want 1", w.MineBonus())
	}
	w.UpgradeLevel = 99 // clamped to the table
	if w.MineBonus() != 2 {
		t.Fatalf("MineBonus clamped = %d, want 2", w.MineBonus())
	}
}

func TestRecapCountersTrack(t *testing.T) {
	w := newMineWorld(t) // chance 1.0, drop exactly 2
	e := w.Spawn("miner", Point{2, 2})
	_ = e
	for i := 0; i < 30; i++ {
		w.Step()
	}
	if w.BlocksMined != 1 {
		t.Fatalf("BlocksMined = %d, want 1", w.BlocksMined)
	}
	if w.GoldMined != 2 {
		t.Fatalf("GoldMined = %d, want 2", w.GoldMined)
	}
}
```

Append to `internal/sim/spread_test.go`:

```go
func TestMoldGrownCounter(t *testing.T) {
	w := NewWorld(9, 9, 1, spreadCfg())
	w.Terrain[idx(w, Point{4, 4})] = Terrain(5) // goo spreads at chance 1
	w.Step()
	if w.MoldGrown == 0 {
		t.Fatal("spread conversions must count")
	}
	w2 := NewWorld(9, 9, 1, sproutCfg())
	w2.Step()
	if w2.MoldGrown == 0 {
		t.Fatal("sprouts must count")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestMineBonus|TestRecapCounters|TestMoldGrown' 2>&1 | tail -3`
Expected: build FAIL (UpgradeLevel undefined).

- [ ] **Step 3: Implement**

`internal/sim/world.go`: World gains, next to Gold:

```go
	UpgradeLevel int `json:"upgradeLevel"`
	BlocksMined  int `json:"blocksMined"`
	GoldMined    int `json:"goldMined"`
	MoldGrown    int `json:"moldGrown"`
```

and:

```go
// MineBonus is the summed damage of purchased pick tiers.
func (w *World) MineBonus() int {
	lvl := w.UpgradeLevel
	if lvl > len(w.cfg.Upgrades) {
		lvl = len(w.cfg.Upgrades)
	}
	bonus := 0
	for _, u := range w.cfg.Upgrades[:lvl] {
		bonus += u.Damage
	}
	return bonus
}
```

`internal/sim/mine.go`, in the AOE loop: `dmg := s.MineDamage + w.MineBonus()` hoisted before the cells loop; `w.MineDamage[i] += dmg`; after `w.SetTerrain(p, TerrainFloor)` add `w.BlocksMined++`; in the gold branch after `w.Gold += amt` add `w.GoldMined += amt`.

`internal/sim/tick.go` spreadStep: after each spread `w.SetTerrain(q, tr)` add `w.MoldGrown++`; after each sprout `w.SetTerrain(p, st)` add `w.MoldGrown++`.

- [ ] **Step 4: Full suite, commit**

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... 2>&1 | tail -5`
Expected: PASS (legacy has no upgrades: MineBonus 0, counters harmless; soak ~70s).

```bash
git add internal/sim/
git commit -m "Add pick bonus damage and recap counters to the world"
```

---

### Task 3: Upgrade intent and recap on the server

**Files:**
- Create: `internal/server/upgrade.go`
- Modify: `internal/server/server.go` (reader case), `internal/server/players.go` (Player snapshot fields, recap builder), `internal/server/protocol.go` (wire fields)
- Test: `internal/server/upgrade_test.go` (new), `internal/server/players_ws_test.go`

**Interfaces:**
- Consumes: `w.UpgradeLevel`, `w.MineBonus`, `cfg.Upgrades`, counters, pending-events queue, torch-intent pattern.
- Produces: ws intent `{type:"upgrade", player}` handled by `(s *Server) buyUpgrade(token string) PlayerMsg` (errors: "you need a living dwarf", "nothing left to forge", "not enough gold"; success: gold -= cost, UpgradeLevel++, pending event `"<player> forged <tier>"`); `Player` gains `SeenTick int64 json:"seenTick"`, `SeenBlocks int json:"seenBlocks"`, `SeenGold int json:"seenGold"`, `SeenMold int json:"seenMold"`; `RecapMsg{Type:"recap", Ticks int64 json:"ticks", Blocks, Gold, Mold int}` sent on hello for KNOWN players (before the player msg is fine), snapshot then advanced; `SnapshotMsg` gains `Upgrades []data.Upgrade json:"upgrades"` and `UpgradeLevel int json:"upgradeLevel"`; `TickMsg` gains `UpgradeLevel int json:"upgradeLevel"`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/upgrade_test.go`:

```go
package server

import "testing"

func TestBuyUpgrade(t *testing.T) {
	s := newPlayerServer(t)
	if res := s.buyUpgrade("ghost"); res.Error == "" {
		t.Fatal("no dwarf: must error")
	}
	s.spawnDwarf("tok", "Misha")
	s.world.Gold = 2
	if res := s.buyUpgrade("tok"); res.Error != "not enough gold" {
		t.Fatalf("got %q, want not enough gold", res.Error)
	}
	s.world.Gold = 10
	if res := s.buyUpgrade("tok"); res.Error != "" {
		t.Fatalf("buy failed: %v", res.Error)
	}
	if s.world.UpgradeLevel != 1 {
		t.Fatalf("level = %d, want 1", s.world.UpgradeLevel)
	}
	if s.world.Gold != 10-s.cfg.Upgrades[0].Cost {
		t.Fatalf("gold = %d after buying %+v", s.world.Gold, s.cfg.Upgrades[0])
	}
	if len(s.pending) != 1 || s.pending[0].Type != "forged" {
		t.Fatalf("pending = %+v, want one forged event", s.pending)
	}
	// exhaust the track
	s.world.Gold = 100000
	for s.world.UpgradeLevel < len(s.cfg.Upgrades) {
		if res := s.buyUpgrade("tok"); res.Error != "" {
			t.Fatalf("buy at level %d: %v", s.world.UpgradeLevel, res.Error)
		}
	}
	if res := s.buyUpgrade("tok"); res.Error != "nothing left to forge" {
		t.Fatalf("got %q, want nothing left to forge", res.Error)
	}
}

func TestRecapDeltasAndSnapshotAdvance(t *testing.T) {
	s := newPlayerServer(t)
	s.spawnDwarf("tok", "Misha")
	// simulate progress since the spawn-time snapshot
	s.world.Tick += 5000
	s.world.BlocksMined += 7
	s.world.GoldMined += 4
	s.world.MoldGrown += 2
	r := s.recapFor("tok")
	if r == nil || r.Ticks != 5000 || r.Blocks != 7 || r.Gold != 4 || r.Mold != 2 {
		t.Fatalf("recap = %+v", r)
	}
	if again := s.recapFor("tok"); again == nil || again.Blocks != 0 || again.Ticks != 0 {
		t.Fatalf("snapshot must advance after a recap, got %+v", again)
	}
	if s.recapFor("stranger") != nil {
		t.Fatal("unknown tokens get no recap")
	}
}
```

(`recapFor` returns `*RecapMsg` and is the caller-holds-mu builder used by the hello case; spawnDwarf initializes the Seen* snapshot to the current values so a fresh player's first recap is empty.)

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/server/ -run 'TestBuyUpgrade|TestRecapDeltas' 2>&1 | tail -3`
Expected: build FAIL.

- [ ] **Step 3: Implement**

`internal/server/upgrade.go`:

```go
package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// buyUpgrade purchases the next pick tier with colony gold. Caller holds s.mu.
func (s *Server) buyUpgrade(token string) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if s.world.UpgradeLevel >= len(s.cfg.Upgrades) {
		pm.Error = "nothing left to forge"
		return pm
	}
	tier := s.cfg.Upgrades[s.world.UpgradeLevel]
	if s.world.Gold < tier.Cost {
		pm.Error = "not enough gold"
		return pm
	}
	s.world.Gold -= tier.Cost
	s.world.UpgradeLevel++
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "forged",
		Msg: fmt.Sprintf("%s forged %s", s.players[token].Name, tier.Name),
	})
	return pm
}
```

`internal/server/players.go`:
- `Player` gains the four Seen fields per Interfaces.
- `spawnDwarf` sets them to current world values when creating the record (`SeenTick: s.world.Tick, SeenBlocks: s.world.BlocksMined, ...`) so a new player's next hello recaps only genuine absence. IMPORTANT: preserve them across respawn: when overwriting the record for an existing token, carry the previous Seen values over (respawning must not eat the recap).
- Add:

```go
type RecapMsg struct {
	Type   string `json:"type"`
	Ticks  int64  `json:"ticks"`
	Blocks int    `json:"blocks"`
	Gold   int    `json:"gold"`
	Mold   int    `json:"mold"`
}

// recapFor builds the away-summary for a known token and advances its
// snapshot. Nil for unknown tokens. Caller holds s.mu.
func (s *Server) recapFor(token string) *RecapMsg {
	p, ok := s.players[token]
	if !ok {
		return nil
	}
	r := &RecapMsg{
		Type:   "recap",
		Ticks:  s.world.Tick - p.SeenTick,
		Blocks: s.world.BlocksMined - p.SeenBlocks,
		Gold:   s.world.GoldMined - p.SeenGold,
		Mold:   s.world.MoldGrown - p.SeenMold,
	}
	p.SeenTick = s.world.Tick
	p.SeenBlocks = s.world.BlocksMined
	p.SeenGold = s.world.GoldMined
	p.SeenMold = s.world.MoldGrown
	return r
}
```

`internal/server/server.go` reader: in the hello branch, after sending the player msg, build `recapFor(m.Player)` under the same lock and send it on `c.send` when non-nil (same select-with-default pattern); add the upgrade case mirroring torch:

```go
			case m.Type == "upgrade" && m.Player != "":
				s.mu.Lock()
				pm := s.buyUpgrade(m.Player)
				s.mu.Unlock()
				if b, err := json.Marshal(pm); err == nil {
					select {
					case c.send <- b:
					default:
					}
				}
```

NOTE the hello case currently computes `pm` under lock then sends; extend it to also compute the recap under the SAME lock hold, send player msg first, recap second.

`internal/server/protocol.go`: SnapshotMsg gains `Upgrades []data.Upgrade json:"upgrades"` (set from `w.Cfg().Upgrades` in BuildSnapshot) and `UpgradeLevel int json:"upgradeLevel"` (from `w.UpgradeLevel`); TickMsg gains `UpgradeLevel int json:"upgradeLevel"` (set in BuildTick).

Add a ws round-trip in players_ws_test.go: spawn, set gold, send `{"type":"upgrade","player":tok}`, read reply (no error), assert level 1 under lock. Also extend an existing snapshot test (or the ws snapshot read) to assert `upgrades` and `upgradeLevel` arrive.

- [ ] **Step 4: Full server package, commit**

Run: `set -o pipefail; go test ./internal/server/ -count=1 2>&1 | tail -2 && go build ./...`
Expected: PASS.

```bash
git add internal/server/
git commit -m "Sell pick tiers over ws and recap returning players"
```

---

### Task 4: Forge button and recap toast in the client

**Files:**
- Modify: `client/src/types.ts`, `client/src/world.ts`, `client/src/net.ts`, `client/src/ui.ts`, `client/index.html`

**Interfaces:**
- Consumes: snapshot `upgrades` + `upgradeLevel`, tick `upgradeLevel`, recap message, upgrade intent.
- Produces: `sendUpgrade()` in net.ts; `world.upgrades: Upgrade[]`, `world.upgradeLevel: number`, `world.recap: RecapMsg | null` (set by applyRecap, cleared by dismiss); forge button `#forge-btn`; toast `#recap`.

- [ ] **Step 1: Types and world**

`client/src/types.ts`:

```ts
export interface Upgrade {
  name: string;
  cost: number;
  damage: number;
}

export interface RecapMsg {
  type: "recap";
  ticks: number;
  blocks: number;
  gold: number;
  mold: number;
}
```

`SnapshotMsg` gains `upgrades: Upgrade[]; upgradeLevel: number;`; `TickMsg` gains `upgradeLevel: number;`.

`client/src/world.ts`: fields `upgrades: Upgrade[] = []; upgradeLevel = 0; recap: RecapMsg | null = null;`. applySnapshot sets `this.upgrades = m.upgrades ?? []; this.upgradeLevel = m.upgradeLevel ?? 0;`. applyTick sets `this.upgradeLevel = m.upgradeLevel ?? this.upgradeLevel;`. New `applyRecap(m: RecapMsg)`: if `m.ticks >= 120 && (m.blocks || m.gold || m.mold)` set `this.recap = m` and fireChange.

`client/src/net.ts`: route `msg.type === "recap"` to `world.applyRecap(msg)`; add:

```ts
export function sendUpgrade() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "upgrade", player: playerToken() }));
}
```

- [ ] **Step 2: Forge button**

`client/index.html`, inside the Gold block under the torch button:

```html
      <button id="forge-btn" disabled>forge</button>
```

(reuse `#torch-btn`'s css by extending the selector: `#torch-btn, #forge-btn { ... }` and the disabled rule likewise).

`client/src/ui.ts`, in `initTorch`'s onChange (or a sibling initForge called from initUI):

```ts
function initForge() {
  const btn = document.getElementById("forge-btn") as HTMLButtonElement;
  btn.onclick = () => sendUpgrade();
  world.onChange(() => {
    const next = world.upgrades[world.upgradeLevel];
    if (!next) {
      btn.textContent = "picks maxed";
      btn.disabled = true;
      return;
    }
    btn.textContent = `${next.name} (${next.cost}g)`;
    btn.disabled = !(world.playerState === "alive" && world.gold >= next.cost);
  });
}
```

- [ ] **Step 3: Recap toast**

`client/index.html`, inside `#map` after the popup div:

```html
    <div id="recap"></div>
```

css: `#recap { position: absolute; top: 12px; left: 50%; transform: translateX(-50%); display: none; background: rgba(20, 17, 15, 0.94); border: 1px solid #4a443c; padding: 10px 16px; cursor: pointer; max-width: 80%; }`.

`client/src/ui.ts`, initRecap (called from initUI):

```ts
function initRecap() {
  const box = document.getElementById("recap")!;
  let hideAt = 0;
  box.onclick = () => {
    world.recap = null;
    box.style.display = "none";
  };
  world.onChange(() => {
    const r = world.recap;
    if (!r) {
      box.style.display = "none";
      return;
    }
    const secs = r.ticks / 2;
    const dur = secs >= 3600
      ? `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
      : `${Math.max(1, Math.floor(secs / 60))}m`;
    const parts = [];
    if (r.blocks) parts.push(`${r.blocks} blocks mined`);
    if (r.gold) parts.push(`${r.gold} gold mined`);
    if (r.mold) parts.push(`${r.mold} tunnels molded over`);
    box.textContent = `While you were away (${dur}): ${parts.join(", ")}`;
    box.style.display = "block";
    hideAt = Date.now() + 12000;
    setTimeout(() => {
      if (Date.now() >= hideAt) {
        world.recap = null;
        box.style.display = "none";
      }
    }, 12100);
  });
}
```

- [ ] **Step 4: Build gate, commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add client/
git commit -m "Add the forge button and welcome back recap toast"
```

---

### Task 5: End-to-end verification, docs, push

**Files:** throwaway scripts in the scratchpad; `.claude/skills/verify/SKILL.md`.

- [ ] **Step 1: Micro-loop live**

Fresh scratch server (:8083, REFRESHED data dir including upgrades.toml, repo root, -fresh). Via the browser: spawn (purse 5), place a torch on the crust, confirm floats pop; then click the forge button (Copper 3g), assert via a ws snapshot that upgradeLevel is 1 and gold dropped by 3, and that subsequent float deltas roughly double (sample band values before/after at 1x; screenshots). Confirm the whole spawn-torch-forge sequence is doable inside ~20s of wall time (report the actual time you took).

- [ ] **Step 2: Recap live**

Close the page (disconnect), `POST /api/advance?ticks=5000`, reopen the page with the same browser profile/localStorage token: assert the recap toast appears with plausible numbers and dismisses on click (screenshots). Reload immediately again: no toast (snapshot advanced, under the 120-tick floor).

- [ ] **Step 3: Docs, gate, push**

`.claude/skills/verify/SKILL.md`: add the forge button (`#forge-btn`, shows next tier + cost), the upgrades data file, upgradeLevel on the wire, and the `#recap` toast (appears on reconnect after absence >= 60s; click to dismiss).

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: green. Kill your :8083 server. Commit docs, push everything.

```bash
git add -A
git commit -m "Verify pick tiers end to end and refresh docs"
git push
```

Relay: server restart needed (new required data file + wire fields); existing world keeps its save (UpgradeLevel starts at 0).

---

## Self-Review Notes

- Spec coverage: table + validation (T1), bonus + counters (T2), intent + recap + wire (T3), button + toast (T4), micro-loop + recap e2e (T5).
- Respawn preserves Seen* so death does not fake a recap; reset zeroes World counters AND resetWorld zeroes DwarfIDs but Player Seen* still reference old-world counter values HIGHER than the fresh world's zeros, making deltas negative. T3's recapFor must clamp negatives to zero OR resetWorld must reset Seen* snapshots; simplest correct: clamp each delta with a max(0, x) helper in recapFor AND have resetWorld reset each player's Seen* to zero alongside DwarfID. Do BOTH (belt and braces); add a line to TestResetWorld asserting Seen* zeroed. THIS IS BINDING for T3 despite being discovered in self-review.
- upgrades.toml is a new REQUIRED file: every scratch data dir and temp fixture must include it (T1 covers the known ones; e2e refreshes its scratch copy).
- Client toast uses wall-clock auto-hide with a dismiss guard; recap only renders when world.recap set, so reconnect floods cannot stack toasts.
