# AOE Mining Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A mining dwarf damages every mineable, lit cell adjacent to it each tick at full mine_damage; the pickaxe orbit pops numbers and debris on every face it sweeps, not just the claimed target.

**Architecture:** `mineStep`'s adjacent branch collects the mineable+lit 8-neighbors of the dwarf (sorted by cell index), damages each, and completes each independently (floor + gold roll + event, clearing the claim if the target was among them). fx.ts swaps the target-rect strike edge for a which-cell-is-the-tool-inside tracker so debris and damage pops fire per struck face. No data or wire changes.

**Tech Stack:** Go stdlib sim, TypeScript canvas client, headless Playwright e2e.

**Spec:** `docs/superpowers/specs/2026-07-11-aoe-mining-design.md`

## Global Constraints

- Commit messages: one sentence, under 70 characters, no Claude attribution, no em or en dashes anywhere in text or code comments.
- TDD on the sim task; `set -o pipefail` when piping `go test`.
- Full damage to every adjacent mineable lit cell; unlit or out-of-range neighbors take zero; completion order is cell-index order; gold rolls stay per completed cell through the world RNG.
- The claim system is untouched: MineTarget still picked, validated, and steers pathing; clear it when the target cell completes.
- Never touch the live server (port 8080) or canonical world.json/players.json; e2e uses :8083 with a scratch data dir owning its save_path, launched FROM THE REPO ROOT.
- All Go commands from repo root /Users/michael/cellar-floor; client commands from client/.

---

### Task 1: AOE damage in the sim

**Files:**
- Modify: `internal/sim/mine.go`
- Test: `internal/sim/mine_test.go`

**Interfaces:**
- Consumes: `w.Mineable`, `w.Lit`, `w.terrainAt(...).HitPoints`, `w.MineDamage`, `neighbors`, `sortInts` (tick.go), gold-roll block.
- Produces: the mining branch of `mineStep` damages all adjacent mineable lit cells; signature and callers unchanged.

- [ ] **Step 1: Write the failing tests**

Append to `internal/sim/mine_test.go`:

```go
func TestAOEDamagesAllAdjacentLitFaces(t *testing.T) {
	w := mineWorld(7, 7)
	// three faces around the dwarf's cell {2,2}: north, east, northeast
	faces := []Point{{2, 1}, {3, 2}, {3, 1}}
	for _, f := range faces {
		w.Terrain[idx(w, f)] = TerrainRock
	}
	d := w.Spawn("dwarf", Point{2, 2})
	d.Fullness = 10
	w.Step()
	for _, f := range faces {
		if got := w.MineDamage[idx(w, f)]; got != 1 {
			t.Fatalf("face %v damage = %d, want 1", f, got)
		}
	}
	// all three finish together on the 10th damage tick, three events at once
	var events []Event
	for i := 0; i < 11; i++ {
		events = append(events, w.Step()...)
	}
	for _, f := range faces {
		if w.At(f) != TerrainFloor {
			t.Fatalf("face %v never completed", f)
		}
	}
	n := 0
	lastIdx := -1
	for _, ev := range events {
		if ev.Type == "mined" || ev.Type == "gold" {
			n++
		}
	}
	_ = lastIdx
	if n != 3 {
		t.Fatalf("completion events = %d, want 3", n)
	}
}

func TestAOESkipsUnlitFaces(t *testing.T) {
	w := NewWorld(20, 20, 1, mineCfg())
	// pinpoint campfire lights only the miner's cell {2,2}; the target face
	// {3,2} is lit by a torch, the second face {2,1} stays dark
	w.Terrain[idx(w, Point{3, 2})] = TerrainRock
	w.Terrain[idx(w, Point{2, 1})] = TerrainRock
	w.Spawn("campfire", Point{2, 2})
	w.Spawn("torch", Point{4, 2}) // radius 3: lights {3,2} but not... verify below
	e := w.Spawn("miner", Point{2, 2})
	_ = e
	w.Step()
	if got := w.MineDamage[idx(w, Point{3, 2})]; got == 0 {
		t.Fatal("lit face should take damage")
	}
	if got := w.MineDamage[idx(w, Point{2, 1})]; got != 0 {
		t.Fatalf("unlit face took %d damage; must take zero", got)
	}
}
```

IMPORTANT geometry check before trusting the second test: the torch at {4,2} has LightRadius 3 in mineCfg, and {2,1} is at distance sqrt(4+1)=2.24 <= 3 EUCLIDEAN, so it WOULD be lit. Move the torch farther or shrink the light: place the torch at {6,2} (distance to {3,2} is 3, lit exactly at r=3 since 9 <= 9; distance to {2,1} is sqrt(16+1)=4.12, dark). Verify with the light math (dx*dx+dy*dy <= r*r) and adjust so the target face is lit and the off-face is dark; assert both conditions in the test itself via `w.Lit(...)` guards at the top:

```go
	if !w.Lit(Point{3, 2}) || w.Lit(Point{2, 1}) {
		t.Fatal("test geometry wrong: want {3,2} lit and {2,1} dark")
	}
```

(The miner's own cell must also be lit by the campfire or it flees; campfire radius 1 covers {2,2} and {3,2}... radius 1 means dx*dx+dy*dy <= 1: covers {2,1} orthogonally too! That breaks the setup. Fix the geometry: give the test its own config instead: copy mineCfg and set campfire LightRadius to 0? Validation forbids light_radius 0 on... no, light_radius 0 is fine (no light). A radius-0 campfire emits nothing, and the miner then flees the dark. Cleanest layout: use the torch as the ONLY light: torch at {3,3} radius 3 lights the miner {2,2} (2 <= 9), target {3,2} (1 <= 9), and {2,1} (dx=1,dy=2: 5 <= 9) STILL lit. Geometry on a radius-3 light is too generous for adjacent cells. SOLUTION: make the dark face non-adjacent to the light by distance: put the light far and the faces at the edge of its circle: torch {5,2} r=3: {3,2} dx2=4 lit; {2,2} dx3=9 lit exactly; {2,1} dx=3,dy=1 -> 10 > 9 dark. Use torch at {5,2}, no campfire, miner {2,2}. Assert the three Lit conditions first. Note for the implementer: this exact layout works; keep the assertions so any light-model change fails loudly.)

Final layout for TestAOESkipsUnlitFaces: faces {3,2} and {2,1}, torch at {5,2}, miner at {2,2}, no campfire; Lit assertions guard the geometry.

- [ ] **Step 2: Run to verify failure**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestAOE' 2>&1 | tail -4`
Expected: FAIL (only the target face takes damage today; second face damage 0).

- [ ] **Step 3: Implement**

In `internal/sim/mine.go`, replace the adjacent-branch body (from `e.Action = "mining"` through the completion block's `return`) with:

```go
	if adjacent(e.Pos, target) {
		e.Action = "mining"
		w.markDirty(e.ID)
		cells := make([]int, 0, 8)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !w.Mineable(w.At(p)) || !w.Lit(p) {
				continue
			}
			cells = append(cells, p.Y*w.Width+p.X)
		}
		sortInts(cells)
		var evs []Event
		for _, i := range cells {
			w.MineDamage[i] += s.MineDamage
			hp := 0
			if tt := w.terrainAt(w.Terrain[i]); tt != nil {
				hp = tt.HitPoints
			}
			if w.MineDamage[i] < hp {
				continue
			}
			p := Point{X: i % w.Width, Y: i / w.Width}
			delete(w.MineDamage, i)
			w.SetTerrain(p, TerrainFloor)
			if p == target {
				e.MineTarget = nil
			}
			sc := w.cfg.Sim
			if sc.GoldChance > 0 && w.RandFloat() < sc.GoldChance {
				amt := sc.GoldMin
				if sc.GoldMax > sc.GoldMin {
					amt += w.RandN(sc.GoldMax - sc.GoldMin + 1)
				}
				w.Gold += amt
				e.GoldStrikes = append(e.GoldStrikes, GoldStrike{Tick: w.Tick, Amount: amt})
				w.GoldLast24h(e)
				evs = append(evs, Event{
					Tick: w.Tick, Type: "gold", Actor: e.ID, ActorType: e.Type,
					Msg: fmt.Sprintf("%s struck gold", s.Name),
				})
			} else {
				evs = append(evs, Event{
					Tick: w.Tick, Type: "mined", Actor: e.ID, ActorType: e.Type,
					Msg: fmt.Sprintf("%s mined out a rock", s.Name),
				})
			}
		}
		return evs, true
	}
```

Notes: the claimed target is adjacent and (post-validation) mineable and lit, so it is always among `cells`; a dark or gone target was already cleared by the validation at the top of mineStep before this branch.

- [ ] **Step 4: Run to verify pass, full suite**

Run: `set -o pipefail; go test ./internal/sim/ -run 'TestAOE' -v 2>&1 | tail -5 && go vet ./... && go test -count=1 ./... 2>&1 | tail -5`
Expected: PASS everywhere. Existing single-face tests keep passing (one face adjacent = identical behavior). If TestSocializesUntilFullThenReturnsToWork or gate tests wobble because a second face nearby now takes damage, inspect the world layouts (they use single faces; they should not wobble; report if they do rather than adjusting them).

- [ ] **Step 5: Commit**

```bash
git add internal/sim/
git commit -m "Mining damages every adjacent lit face each tick"
```

---

### Task 2: Per-face strikes in fx.ts

**Files:**
- Modify: `client/src/fx.ts`

**Interfaces:**
- Consumes: `world.terrain`, `world.terrainTypes[].mineable/.hitPoints`, `world.mining`, existing `spawnDebris`, `spawnFloat`, `shownDamage` record logic.
- Produces: strike detection keyed by which cell the tool tip is inside; the old `wasInside` boolean map is replaced by `toolCell: Map<number, number>` (dwarf id to current cell index, -1 when none).

- [ ] **Step 1: Implement**

In `client/src/fx.ts`:
- Replace `const wasInside = new Map<number, boolean>();` with `const toolCell = new Map<number, number>();` and clear it in the snapshotVersion block alongside the others.
- In the orbit loop, replace the `inside`/`wasInside` block with:

```ts
    const cx2 = Math.floor(tx / TILE);
    const cy2 = Math.floor(ty / TILE);
    const inWorld = cx2 >= 0 && cy2 >= 0 && cx2 < world.width && cy2 < world.height;
    const cell = inWorld ? cy2 * world.width + cx2 : -1;
    const prev = toolCell.get(e.id) ?? -1;
    const mineable = cell >= 0 && (world.terrainTypes[world.terrain[cell]]?.mineable ?? false);
    if (mineable && cell !== prev && running) {
      spawnDebris(tx, ty, cx, cy, DEBRIS_COLOR);
      const dealt = world.mining[cell] ?? 0;
      const rec = shownDamage.get(cell);
      if (rec == null) {
        const hp = world.terrainTypes[world.terrain[cell]]?.hitPoints ?? 0;
        shownDamage.set(cell, { shown: dealt, hp });
      } else if (dealt > rec.shown) {
        spawnFloat(cx2, cy2, String(dealt - rec.shown));
        rec.shown = dealt;
      }
    }
    toolCell.set(e.id, cell);
```

- The entity-loop guard keeps `e.action !== "mining" || !e.mt` (the orbit still only shows while mining); also keep the `toolCell.delete(e.id)` behavior for non-mining entities where the old code deleted from wasInside.
- The completion sweep and float drawing are untouched.

- [ ] **Step 2: Build gate and commit**

Run: `cd client && npx tsc --noEmit && npm run build`
Expected: clean.

```bash
git add client/src/fx.ts
git commit -m "Strike every face the pickaxe sweeps through"
```

---

### Task 3: End-to-end verification, docs, push

**Files:** throwaway scripts in the scratchpad; `.claude/skills/verify/SKILL.md`.

- [ ] **Step 1: Isolated server, corner miner**

Scratch data copy with own save_path, client built, :8083 from the repo root with `-fresh` and a seed where the campfire ring offers a corner (most seeds do; the clearing edge is a ring of rock). Spawn a dwarf, advance ~4000 ticks, then query `/api/entities?type=dwarf` for its target and read a ws snapshot's `mining` map: EXPECT MULTIPLE CELLS accruing damage when the dwarf sits in a concave spot (if the first dwarf found a flat wall with only 1-2 adjacent faces, spawn a second dwarf or advance more; report the face count observed).

- [ ] **Step 2: Visual proof**

Screenshot the mining area at 8x: multiple progress bars filling side by side around one dwarf, and over ~4s of sampling, float pops above at least two different cells (band pixel checks like prior e2e runs). Screenshot for the record.

- [ ] **Step 3: Docs, gate, push**

`.claude/skills/verify/SKILL.md` mining gotcha line: mining now damages every adjacent lit face each tick (AOE), so several bars fill at once around a miner and gold events can arrive in bursts.

Run: `set -o pipefail; go vet ./... && go test -count=1 ./... && (cd client && npm run build)`
Expected: green. Kill your :8083 server. Commit docs, push.

```bash
git add -A
git commit -m "Verify AOE mining end to end and refresh docs"
git push
```

Relay: server restart needed to pick up the sim change; mining throughput and gold income roughly triple at multi-face spots by design.

---

## Self-Review Notes

- Spec coverage: full-damage AOE with lit gate and index-order determinism (T1), per-face strikes/debris/pops (T2), e2e + docs (T3). Claims untouched; gold per completed cell.
- The tricky light-geometry for TestAOESkipsUnlitFaces is worked out in the plan text (torch {5,2} r3: miner lit at exactly r, target lit, off-face dark) and guarded by explicit Lit assertions so the layout cannot rot silently.
- The old single-target code path is a strict subset (one adjacent face = same behavior), so existing tests should pass unmodified; the plan tells the implementer to report rather than adjust if any wobble.
