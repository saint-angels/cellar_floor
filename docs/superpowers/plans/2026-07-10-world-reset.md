# World Reset Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A reset button next to the speed buttons that regenerates the world at runtime and drops every player into the death screen for a one-click rejoin.

**Architecture:** The WebSocket reader gains a `reset` case that calls a new `Server.resetWorld()`: swap `s.world` for a freshly generated one (seed `time.Now().UnixNano()`), save world and players, return a marshaled snapshot for broadcast. Player records are untouched; the existing `dead` state resolution handles everyone. Client-side, the own-dwarf death check moves into a shared method so a snapshot (not just a tick) can flip the local player to `dead`, and the timescale row gets an armed two-click reset button.

**Tech Stack:** Go stdlib + gorilla/websocket, `internal/gen` now imported by `internal/server`. TypeScript/Vite client. No new dependencies.

## Global Constraints

- Seed choice is server-layer input; sim stays deterministic per seed (spec).
- Player records are kept across reset; no new player logic (spec).
- No `confirm()` dialogs; two-click arm with a 3 second window (spec).
- The current timescale is preserved across reset (spec).
- Commit messages: one sentence, under 70 characters, no Claude attribution (user CLAUDE.md).

---

### Task 1: Server resetWorld and the reset intent

**Files:**
- Modify: `internal/server/server.go` (import gen + time, reader case, new method)
- Test: `internal/server/players_test.go` (append unit test), `internal/server/players_ws_test.go` (append ws test)

**Interfaces:**
- Consumes: `gen.Generate(seed int64, cfg *data.Config) *sim.World`, `BuildSnapshot(w, scale, owners)`, `SaveWorld`, `SavePlayers`, `playersPath`, `s.hub.Broadcast`, `ClientMsg` (already has `Type`).
- Produces: `(s *Server) resetWorld() []byte` — swaps the world, saves, returns the marshaled snapshot (nil on marshal error); reader case `m.Type == "reset"` that broadcasts it.

- [ ] **Step 1: Write the failing tests**

Append to `internal/server/players_test.go`:

```go
func TestResetWorld(t *testing.T) {
	s := newPlayerServer(t)
	pm := s.spawnDwarf("tok1", "Misha")
	s.world.Gold = 5
	s.world.MineProgress[42] = 0.5
	oldTick := s.world.Tick

	snap := s.resetWorld()
	if snap == nil {
		t.Fatal("resetWorld returned nil snapshot")
	}
	if s.world.Tick != 0 || s.world.Tick == oldTick && oldTick != 0 {
		t.Errorf("world not fresh: tick %d", s.world.Tick)
	}
	if s.world.CountAlive("dwarf") != 0 {
		t.Error("dwarves survived the reset")
	}
	if s.world.Gold != 0 || len(s.world.MineProgress) != 0 {
		t.Error("gold or mining progress survived the reset")
	}
	if got := s.playerMsg("tok1"); got.State != "dead" || got.Name != "Misha" {
		t.Errorf("player after reset: %+v, want dead with name", got)
	}
	_ = pm
	var parsed SnapshotMsg
	if err := json.Unmarshal(snap, &parsed); err != nil || parsed.Type != "snapshot" {
		t.Fatalf("bad snapshot payload: %v", err)
	}
}
```

Add `"encoding/json"` to the test file imports. Note `newPlayerServer` uses the real `data/` config whose `save_path` is `world.json`; `resetWorld` saves, which would write `world.json`/`players.json` into the repo. To keep tests hermetic, before calling `resetWorld` redirect the save path:

```go
	s.cfg.Sim.SavePath = filepath.Join(t.TempDir(), "world.json")
```

(`cfg` is loaded fresh per test via `loadCfg`, so mutating it is safe.)

Append to `internal/server/players_ws_test.go`:

```go
func TestWSReset(t *testing.T) {
	s, ts := newWSServer(t)
	s.cfg.Sim.SavePath = filepath.Join(t.TempDir(), "world.json")
	c := dialWS(t, ts)

	send(t, c, map[string]any{"type": "spawn", "player": "tok1", "name": "Misha"})
	pm := readPlayerMsg(t, c)
	if pm.State != "alive" {
		t.Fatalf("spawn: %+v", pm)
	}

	send(t, c, map[string]any{"type": "reset"})
	// a fresh snapshot must arrive on the same connection
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, b, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("no snapshot after reset: %v", err)
		}
		var probe struct {
			Type string `json:"type"`
			Tick int64  `json:"tick"`
		}
		if json.Unmarshal(b, &probe) == nil && probe.Type == "snapshot" {
			if probe.Tick != 0 {
				t.Errorf("snapshot tick %d, want 0", probe.Tick)
			}
			break
		}
	}

	send(t, c, map[string]any{"type": "hello", "player": "tok1"})
	if pm := readPlayerMsg(t, c); pm.State != "dead" || pm.Name != "Misha" {
		t.Fatalf("hello after reset: %+v, want dead", pm)
	}
}
```

Add `"path/filepath"` to that file's imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestResetWorld|TestWSReset' -v`
Expected: FAIL to compile ("s.resetWorld undefined")

- [ ] **Step 3: Implement**

In `internal/server/server.go` add imports `"time"` (already present) and `"cellarfloor/internal/gen"`. Add the method:

```go
// resetWorld swaps in a freshly generated world. Player records survive;
// their dwarves do not, so everyone resolves to the dead state and can
// rejoin from the death screen.
func (s *Server) resetWorld() []byte {
	s.mu.Lock()
	s.world = gen.Generate(time.Now().UnixNano(), s.cfg)
	log.Printf("world reset: %d entities", len(s.world.Entities))
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners()))
	s.mu.Unlock()
	s.save()
	if err != nil {
		log.Printf("marshal reset snapshot: %v", err)
		return nil
	}
	return snap
}
```

In the reader switch add:

```go
			case m.Type == "reset":
				if b := s.resetWorld(); b != nil {
					s.hub.Broadcast(b)
				}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v && go test ./... -short && go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/players_test.go internal/server/players_ws_test.go
git commit -m "Add world reset intent that regenerates and broadcasts"
```

---

### Task 2: Reset button and snapshot death detection in the client

**Files:**
- Modify: `client/src/net.ts` (sendReset), `client/src/world.ts` (shared death check), `client/src/ui.ts` (button), `client/index.html` (button style)

**Interfaces:**
- Consumes: `reset` ws intent (Task 1), existing `applySnapshot`/`applyTick`, `#timescale` row.
- Produces: `sendReset()`; armed reset button (`really?` for 3s); own-dwarf death detection running on both snapshot and tick.

- [ ] **Step 1: sendReset in net.ts**

Append:

```ts
export function sendReset() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "reset" }));
}
```

- [ ] **Step 2: Shared death check in world.ts**

Extract the block added to `applyTick` into a private method and call it from both appliers:

```ts
  private checkOwnDwarf() {
    if (this.playerState === "alive" && this.playerDwarfId != null) {
      const mine = this.entities.get(this.playerDwarfId);
      if (!mine || mine.dead) {
        this.playerState = "dead";
        this.playerDwarfId = null;
      }
    }
  }
```

In `applyTick`, replace the inline block with `this.checkOwnDwarf();`. In `applySnapshot`, call `this.checkOwnDwarf();` after the entity upsert loop (before `fireChange`).

- [ ] **Step 3: Button in ui.ts and style**

In `initTimescale`, after the speed-button loop, append:

```ts
  const reset = document.createElement("button");
  reset.textContent = "reset";
  reset.className = "reset";
  let armedAt = 0;
  reset.onclick = () => {
    if (Date.now() - armedAt < 3000) {
      sendReset();
      armedAt = 0;
      reset.textContent = "reset";
      return;
    }
    armedAt = Date.now();
    reset.textContent = "really?";
    setTimeout(() => {
      if (armedAt !== 0 && Date.now() - armedAt >= 3000) {
        armedAt = 0;
        reset.textContent = "reset";
      }
    }, 3100);
  };
  box.appendChild(reset);
```

Import `sendReset` in the net import line. In `client/index.html` add after the `#timescale button.active` rule:

```css
    #timescale button.reset { border-color: #6a3a32; color: #d9724a; margin-left: 8px; }
```

- [ ] **Step 4: Build**

Run: `cd client && npm run build && cd ..`
Expected: clean build

- [ ] **Step 5: End-to-end verification (verify skill recipes)**

Fresh server on :8080. Playwright: spawn as "Misha" (welcome overlay); `POST /api/advance?ticks=90000` so the world visibly progresses; click `reset` once and assert the label reads `really?`; click again; assert (a) `/api/state` tick is small again, (b) the death overlay is visible with title "Your dwarf has died" and the name prefilled, (c) pops show dwarf 0; click spawn and assert a new owned dwarf exists. Screenshot the armed button and the post-reset death overlay. Stop the server with SIGINT and delete `world.json`/`players.json` afterwards so no test player leaks into the user's canonical world (lesson from the last run).

- [ ] **Step 6: Commit**

```bash
git add client/src/net.ts client/src/world.ts client/src/ui.ts client/index.html
git commit -m "Add armed reset button and snapshot death detection"
```
