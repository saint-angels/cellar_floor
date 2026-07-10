# Player Dwarves Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every dwarf is owned by a player: anonymous localStorage-token identity, hello/spawn over the existing WebSocket, a death screen with manual respawn, and visible ownership (popup, events, own-dwarf marker and status line).

**Architecture:** Ownership lives entirely in the server layer: a new `internal/server/players.go` holds the `Players` map (token -> name + dwarf id), the hello/spawn state machine, and `players.json` persistence saved under the same lock as `world.json`. The sim is untouched except data (`pop_floor = 0`, no dwarf scatter): players are the only source of dwarves. The client keeps a token in localStorage, drives a three-state overlay (welcome / dead / hidden) from `player` messages plus its own death detection, and renders an owner ring and status line.

**Tech Stack:** Go stdlib + gorilla/websocket (existing), TypeScript/Vite client (existing). No new dependencies.

## Global Constraints

- The engine knows no players; ownership is server-layer only (spec, DESIGN.md).
- `pop_floor = 0` for dwarves; the world never spawns them on its own (spec).
- Name: trimmed, capped at 24 characters (runes), empty rejected (spec).
- Pop-cap rejection error text is exactly `the cellar is crowded` (spec).
- Event decoration replaces the first occurrence of the species display name: "Dwarf struck gold" becomes "Misha's dwarf struck gold" (spec).
- Client sends only small intents: `hello`, `spawn`, `timescale` (DESIGN.md).
- Commit messages: one sentence, under 70 characters, no Claude attribution (user CLAUDE.md).

---

### Task 1: Player records, spawn/hello logic, persistence, data flip

**Files:**
- Create: `internal/server/players.go`
- Modify: `internal/server/server.go:25-46` (Server struct + Run + save), `data/species.toml` (pop_floor), `data/gen.toml` (remove dwarf scatter), `internal/gen/gen_test.go` (dwarf expectations), `.gitignore`
- Test: `internal/server/players_test.go` (create)

**Interfaces:**
- Consumes: `Server{cfg, world, mu, players}`, `sim.World.Spawn/CountAlive/Entities/At/FaunaAt/RandN`, `loadCfg(t)` (persist_test.go), `gen.Generate`.
- Produces: `type Player struct{Name string; DwarfID int}`; `type PlayerMsg struct{Type, State string; DwarfID int; Name, Error string}` (json `type/state/dwarfId/name/error`, omitempty on the last three); `(s *Server) playerMsg(token string) PlayerMsg`, `(s *Server) spawnDwarf(token, name string) PlayerMsg`, `(s *Server) owners() map[int]string` (all require `s.mu` held); `SavePlayers(players map[string]*Player, path string) error`, `LoadPlayers(path string) (map[string]*Player, error)` (missing file yields empty map), `playersPath(savePath string) string`; `Server.players map[string]*Player` field.

- [ ] **Step 1: Flip the data**

In `data/species.toml` change `pop_floor = 3` to `pop_floor = 0`.
In `data/gen.toml` remove the dwarf scatter line, leaving:

```toml
scatter = [
  { species = "mushroom", terrain = "dirt", chance = 0.15 },
]
```

In `internal/gen/gen_test.go` `TestGenerateContents`, replace the block after the mushroom check (the comment, `w.Step()`, and the pop-floor assertion) with:

```go
	if counts["dwarf"] != 0 {
		t.Error("dwarves must not generate; players spawn them")
	}
```

Append `players.json` to the Project section of `.gitignore`.

- [ ] **Step 2: Write the failing tests**

Create `internal/server/players_test.go`:

```go
package server

import (
	"net/http"
	"path/filepath"
	"testing"

	"cellarfloor/internal/gen"
)

func newPlayerServer(t *testing.T) *Server {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
	s.scale.Store(1)
	_ = http.StatusOK // keep import used if handlers move
	return s
}

func TestHelloUnknownToken(t *testing.T) {
	s := newPlayerServer(t)
	pm := s.playerMsg("tok1")
	if pm.State != "none" || pm.Type != "player" {
		t.Fatalf("got %+v, want state none", pm)
	}
}

func TestSpawnAndHelloAlive(t *testing.T) {
	s := newPlayerServer(t)
	pm := s.spawnDwarf("tok1", "  Misha  ")
	if pm.State != "alive" || pm.Name != "Misha" || pm.DwarfID == 0 {
		t.Fatalf("spawn reply %+v", pm)
	}
	e := s.world.Entities[pm.DwarfID]
	if e == nil || e.Species != "dwarf" {
		t.Fatal("no dwarf entity spawned")
	}
	if s.world.At(e.Pos).String() == "" { // touch At to assert pos valid
		t.Fatal("bad position")
	}
	// spawning again while alive is a no-op returning the same dwarf
	pm2 := s.spawnDwarf("tok1", "Misha")
	if pm2.DwarfID != pm.DwarfID || pm2.State != "alive" {
		t.Fatalf("second spawn %+v, want same dwarf", pm2)
	}
	if got := s.playerMsg("tok1"); got.State != "alive" || got.DwarfID != pm.DwarfID {
		t.Fatalf("hello after spawn %+v", got)
	}
}

func TestSpawnRejectsBadName(t *testing.T) {
	s := newPlayerServer(t)
	if pm := s.spawnDwarf("tok1", "   "); pm.Error == "" || pm.State != "none" {
		t.Fatalf("empty name accepted: %+v", pm)
	}
	long := "abcdefghijklmnopqrstuvwxyz" // 26 chars
	pm := s.spawnDwarf("tok2", long)
	if len([]rune(pm.Name)) != 24 {
		t.Fatalf("name not capped: %q", pm.Name)
	}
}

func TestDeathAndRespawn(t *testing.T) {
	s := newPlayerServer(t)
	pm := s.spawnDwarf("tok1", "Misha")
	e := s.world.Entities[pm.DwarfID]
	e.Fullness = 0
	e.StarvingFor = s.cfg.Species["dwarf"].StarveTicks + 1
	s.world.Step()
	if got := s.playerMsg("tok1"); got.State != "dead" || got.Name != "Misha" {
		t.Fatalf("after death %+v, want dead", got)
	}
	pm2 := s.spawnDwarf("tok1", "Misha")
	if pm2.State != "alive" || pm2.DwarfID == pm.DwarfID {
		t.Fatalf("respawn %+v", pm2)
	}
}

func TestSpawnCrowded(t *testing.T) {
	s := newPlayerServer(t)
	cap := s.cfg.Species["dwarf"].PopCap
	for i := 0; i < cap; i++ {
		if pm := s.spawnDwarf(fmt.Sprintf("tok%d", i), "P"); pm.State != "alive" {
			t.Fatalf("spawn %d failed: %+v", i, pm)
		}
	}
	pm := s.spawnDwarf("late", "P")
	if pm.Error != "the cellar is crowded" || pm.State != "none" {
		t.Fatalf("crowded reply %+v", pm)
	}
}

func TestPlayersPersistRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "players.json")
	in := map[string]*Player{"tok1": {Name: "Misha", DwarfID: 7}}
	if err := SavePlayers(in, path); err != nil {
		t.Fatal(err)
	}
	out, err := LoadPlayers(path)
	if err != nil {
		t.Fatal(err)
	}
	if out["tok1"] == nil || out["tok1"].Name != "Misha" || out["tok1"].DwarfID != 7 {
		t.Fatalf("round trip lost data: %+v", out)
	}
	empty, err := LoadPlayers(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("missing file should yield empty map, got %v %v", empty, err)
	}
}
```

Add `"fmt"` to the imports. Remove the `_ = http.StatusOK` line and the `"net/http"` import (they are placeholders in this listing only if unused; keep imports minimal and let the compiler guide).

Note: `sim.Terrain` has no `String()`; replace the position sanity check with `if !s.world.InBounds(e.Pos) || s.world.At(e.Pos) != sim.TerrainDirt { t.Fatalf("spawned off the clearing: %v", e.Pos) }` and import `"cellarfloor/internal/sim"`.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestHello|TestSpawn|TestDeath|TestPlayers' -v`
Expected: FAIL to compile ("undefined: Player")

- [ ] **Step 4: Implement players.go**

```go
package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"cellarfloor/internal/sim"
)

// Player ties an anonymous browser token to a dwarf. Ownership is a server
// concern; the sim engine knows nothing about players.
type Player struct {
	Name    string `json:"name"`
	DwarfID int    `json:"dwarfId"`
}

type PlayerMsg struct {
	Type    string `json:"type"`
	State   string `json:"state"` // none | alive | dead
	DwarfID int    `json:"dwarfId,omitempty"`
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
}

// playerMsg reports the current state for a token. Caller holds s.mu.
func (s *Server) playerMsg(token string) PlayerMsg {
	p, ok := s.players[token]
	if !ok {
		return PlayerMsg{Type: "player", State: "none"}
	}
	if e, exists := s.world.Entities[p.DwarfID]; exists && !e.Dead {
		return PlayerMsg{Type: "player", State: "alive", DwarfID: p.DwarfID, Name: p.Name}
	}
	return PlayerMsg{Type: "player", State: "dead", Name: p.Name}
}

// spawnDwarf spawns a dwarf for the token unless one is already alive.
// Caller holds s.mu.
func (s *Server) spawnDwarf(token, name string) PlayerMsg {
	if cur := s.playerMsg(token); cur.State == "alive" {
		return cur
	}
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 24 {
		name = string(r[:24])
	}
	if name == "" {
		pm := s.playerMsg(token)
		pm.Error = "name required"
		return pm
	}
	if s.world.CountAlive("dwarf") >= s.cfg.Species["dwarf"].PopCap {
		pm := s.playerMsg(token)
		pm.Error = "the cellar is crowded"
		return pm
	}
	pos, ok := s.freeDirtTile()
	if !ok {
		pm := s.playerMsg(token)
		pm.Error = "no room in the clearing"
		return pm
	}
	e := s.world.Spawn("dwarf", pos)
	s.players[token] = &Player{Name: name, DwarfID: e.ID}
	return PlayerMsg{Type: "player", State: "alive", DwarfID: e.ID, Name: name}
}

// owners maps dwarf entity id to owning player name. Caller holds s.mu.
func (s *Server) owners() map[int]string {
	m := make(map[int]string, len(s.players))
	for _, p := range s.players {
		m[p.DwarfID] = p.Name
	}
	return m
}

// freeDirtTile picks a random unoccupied dirt tile (the clearing).
// Caller holds s.mu.
func (s *Server) freeDirtTile() (sim.Point, bool) {
	w := s.world
	var cands []sim.Point
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if w.At(p) == sim.TerrainDirt && w.FaunaAt(p) == nil {
				cands = append(cands, p)
			}
		}
	}
	if len(cands) == 0 {
		return sim.Point{}, false
	}
	return cands[w.RandN(len(cands))], true
}

func playersPath(savePath string) string {
	return filepath.Join(filepath.Dir(savePath), "players.json")
}

func SavePlayers(players map[string]*Player, path string) error {
	b, err := json.Marshal(players)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadPlayers(path string) (map[string]*Player, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]*Player{}, nil
	}
	if err != nil {
		return nil, err
	}
	players := map[string]*Player{}
	if err := json.Unmarshal(b, &players); err != nil {
		return nil, err
	}
	return players, nil
}
```

- [ ] **Step 5: Wire the Server struct, Run, and save**

In `internal/server/server.go` add the field to the struct:

```go
	players map[string]*Player // guarded by mu
```

In `Run`, after constructing `s`:

```go
	players, err := LoadPlayers(playersPath(cfg.Sim.SavePath))
	if err != nil {
		log.Printf("load players: %v (starting empty)", err)
		players = map[string]*Player{}
	}
	s.players = players
```

In `save()`, after the world save:

```go
	if err := SavePlayers(s.players, playersPath(s.cfg.Sim.SavePath)); err != nil {
		log.Printf("save players: %v", err)
	}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/server/ -v -run 'TestHello|TestSpawn|TestDeath|TestPlayers' && go test ./... -short && go vet ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/players.go internal/server/players_test.go internal/server/server.go data/species.toml data/gen.toml internal/gen/gen_test.go .gitignore
git commit -m "Add player records with spawn, death state, and persistence"
```

---

### Task 2: hello and spawn over the WebSocket

**Files:**
- Modify: `internal/server/protocol.go:52-55` (ClientMsg), `internal/server/server.go:143-159` (reader goroutine)
- Test: `internal/server/players_ws_test.go` (create)

**Interfaces:**
- Consumes: `playerMsg`, `spawnDwarf` (Task 1), `handleWS`, `Client.send`.
- Produces: `ClientMsg` gains `Player string \`json:"player"\`` and `Name string \`json:"name"\``; `handleWS` answers `hello`/`spawn` messages with a marshaled `PlayerMsg` on that connection only.

- [ ] **Step 1: Write the failing test**

Create `internal/server/players_ws_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"cellarfloor/internal/gen"
)

func newWSServer(t *testing.T) (*Server, *httptest.Server) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
	s.scale.Store(1)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return s, ts
}

func dialWS(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// readPlayerMsg skips snapshot/tick frames until a player message arrives.
func readPlayerMsg(t *testing.T, c *websocket.Conn) PlayerMsg {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, b, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var probe struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &probe) != nil || probe.Type != "player" {
			continue
		}
		var pm PlayerMsg
		if err := json.Unmarshal(b, &pm); err != nil {
			t.Fatal(err)
		}
		return pm
	}
}

func send(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	if err := c.WriteJSON(v); err != nil {
		t.Fatal(err)
	}
}

func TestWSHelloSpawnFlow(t *testing.T) {
	s, ts := newWSServer(t)
	c := dialWS(t, ts)

	send(t, c, map[string]any{"type": "hello", "player": "tok1"})
	if pm := readPlayerMsg(t, c); pm.State != "none" {
		t.Fatalf("hello: %+v, want none", pm)
	}

	send(t, c, map[string]any{"type": "spawn", "player": "tok1", "name": "Misha"})
	pm := readPlayerMsg(t, c)
	if pm.State != "alive" || pm.Name != "Misha" || pm.DwarfID == 0 {
		t.Fatalf("spawn: %+v", pm)
	}

	s.mu.Lock()
	if e := s.world.Entities[pm.DwarfID]; e == nil || e.Dead {
		s.mu.Unlock()
		t.Fatal("spawned dwarf missing in world")
	}
	s.mu.Unlock()

	// a second connection with the same token is already alive
	c2 := dialWS(t, ts)
	send(t, c2, map[string]any{"type": "hello", "player": "tok1"})
	if pm2 := readPlayerMsg(t, c2); pm2.State != "alive" || pm2.DwarfID != pm.DwarfID {
		t.Fatalf("second hello: %+v", pm2)
	}
}

func TestWSHelloWithoutTokenIgnored(t *testing.T) {
	_, ts := newWSServer(t)
	c := dialWS(t, ts)
	send(t, c, map[string]any{"type": "hello"})
	send(t, c, map[string]any{"type": "hello", "player": "tok9"})
	// the first (tokenless) hello must not produce a reply; we still get
	// exactly one player message, for tok9
	if pm := readPlayerMsg(t, c); pm.State != "none" {
		t.Fatalf("got %+v", pm)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestWSHello -v`
Expected: FAIL with read deadline exceeded (server never answers hello)

- [ ] **Step 3: Implement**

In `internal/server/protocol.go` extend `ClientMsg`:

```go
type ClientMsg struct {
	Type   string `json:"type"`
	Scale  int    `json:"scale"`
	Player string `json:"player"`
	Name   string `json:"name"`
}
```

In `internal/server/server.go`, replace the timescale handling in the reader goroutine with a switch:

```go
			switch {
			case m.Type == "timescale" && validScales[m.Scale]:
				s.scale.Store(int64(m.Scale))
			case (m.Type == "hello" || m.Type == "spawn") && m.Player != "":
				s.mu.Lock()
				var pm PlayerMsg
				if m.Type == "hello" {
					pm = s.playerMsg(m.Player)
				} else {
					pm = s.spawnDwarf(m.Player, m.Name)
				}
				s.mu.Unlock()
				if b, err := json.Marshal(pm); err == nil {
					select {
					case c.send <- b:
					default:
					}
				}
			}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v && go test ./... -short`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/protocol.go internal/server/server.go internal/server/players_ws_test.go
git commit -m "Handle hello and spawn messages over the WebSocket"
```

---

### Task 3: Visible ownership in views, events, and the debug API

**Files:**
- Modify: `internal/server/protocol.go` (EntityView, BuildSnapshot, BuildTick), `internal/server/server.go:74,124` (call sites), `internal/server/api.go` (owner in entity responses)
- Test: `internal/server/protocol_test.go` (append), `internal/server/api_test.go` (append)

**Interfaces:**
- Consumes: `owners()` (Task 1).
- Produces: `EntityView.Owner string \`json:"owner,omitempty"\``; `BuildSnapshot(w *sim.World, scale int, owners map[int]string) SnapshotMsg`; `BuildTick(w *sim.World, events []sim.Event, scale int, owners map[int]string) TickMsg` (events decorated: first occurrence of the actor species display name replaced with `<owner>'s dwarf`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/server/protocol_test.go`:

```go
func TestOwnerDecoration(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	d := w.Spawn("dwarf", sim.Point{X: 32, Y: 32})
	owners := map[int]string{d.ID: "Misha"}

	snap := BuildSnapshot(w, 1, owners)
	found := false
	for _, ev := range snap.Entities {
		if ev.ID == d.ID {
			found = true
			if ev.Owner != "Misha" {
				t.Errorf("owner = %q", ev.Owner)
			}
		}
	}
	if !found {
		t.Fatal("dwarf not in snapshot")
	}

	evs := []sim.Event{{Actor: d.ID, ActorSpecies: "dwarf", Msg: "Dwarf struck gold"}}
	tick := BuildTick(w, evs, 1, owners)
	if tick.Events[0].Msg != "Misha's dwarf struck gold" {
		t.Errorf("decorated msg = %q", tick.Events[0].Msg)
	}
	if evs[0].Msg != "Dwarf struck gold" {
		t.Error("decoration mutated the caller's slice")
	}

	// unowned actors stay untouched
	evs2 := []sim.Event{{Actor: 999999, ActorSpecies: "dwarf", Msg: "Dwarf starved"}}
	if got := BuildTick(w, evs2, 1, owners); got.Events[0].Msg != "Dwarf starved" {
		t.Errorf("unowned msg changed: %q", got.Events[0].Msg)
	}
}
```

Append to `internal/server/api_test.go`:

```go
func TestAPIEntityOwner(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
	s.scale.Store(1)
	pm := s.spawnDwarf("tok1", "Misha")
	mux := http.NewServeMux()
	s.registerAPI(mux)

	var e EntityView
	if err := json.Unmarshal(apiGet(t, mux, "/api/entities/"+strconv.Itoa(pm.DwarfID)).Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Owner != "Misha" {
		t.Errorf("owner = %q, want Misha", e.Owner)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run 'TestOwnerDecoration|TestAPIEntityOwner' -v`
Expected: FAIL to compile (BuildSnapshot takes 2 args; EntityView has no Owner)

- [ ] **Step 3: Implement**

In `internal/server/protocol.go`:

Add to `EntityView` (after `Res`):

```go
	Owner  string             `json:"owner,omitempty"`
```

Change `BuildSnapshot` signature and entity loop:

```go
func BuildSnapshot(w *sim.World, scale int, owners map[int]string) SnapshotMsg {
	...
	for _, id := range w.SortedIDs() {
		v := ViewOf(w.Entities[id])
		v.Owner = owners[id]
		ents = append(ents, v)
	}
```

Change `BuildTick` signature, entity loop, and decorate events (import `"strings"`):

```go
func BuildTick(w *sim.World, events []sim.Event, scale int, owners map[int]string) TickMsg {
	changed := []EntityView{}
	for _, id := range w.DirtyAndReset() {
		if e, ok := w.Entities[id]; ok {
			v := ViewOf(e)
			v.Owner = owners[id]
			changed = append(changed, v)
		}
	}
	...
	if events == nil {
		events = []sim.Event{}
	}
	decorated := make([]sim.Event, len(events))
	copy(decorated, events)
	for i := range decorated {
		name := owners[decorated[i].Actor]
		if name == "" {
			continue
		}
		if sp := w.Cfg().Species[decorated[i].ActorSpecies]; sp != nil {
			decorated[i].Msg = strings.Replace(decorated[i].Msg, sp.Name, name+"'s dwarf", 1)
		}
	}
```

and pass `Events: decorated` in the returned literal.

In `internal/server/server.go` update the two call sites:

```go
	msg := BuildTick(s.world, events, scale, s.owners())      // in safeTick
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners())) // in handleWS
```

(Both already run under `s.mu`.)

In `internal/server/api.go`, set owners in both entity handlers while holding the lock:

```go
	// in handleEntities, before the loop:
	owners := s.owners()
	// inside the loop, after ViewOf:
	v := ViewOf(e)
	v.Owner = owners[e.ID]
	views = append(views, v)
```

```go
	// in handleEntity, when found:
	view = ViewOf(e)
	view.Owner = s.owners()[e.ID]
```

Fix the two existing `BuildSnapshot(w, 1)` / `BuildTick(w, ...)` calls in `protocol_test.go` to pass `nil` owners.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v && go test ./... -short && go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/protocol.go internal/server/server.go internal/server/api.go internal/server/protocol_test.go internal/server/api_test.go
git commit -m "Expose dwarf ownership in views, events, and debug API"
```

---

### Task 4: Time-advance debug endpoint

**Files:**
- Modify: `internal/server/api.go` (route + handler), `.claude/skills/verify/SKILL.md` (document it)
- Test: `internal/server/api_test.go` (append)

**Interfaces:**
- Consumes: `s.mu`, `s.world.Step()`, `BuildSnapshot(w, scale, owners)` (Task 3), `s.hub.Broadcast`, `writeJSON`.
- Produces: `POST /api/advance?ticks=N` — steps the sim N ticks (clamped to 1,000,000) under the world lock, drains the dirty sets, broadcasts a fresh snapshot to all clients, returns `{"advanced": N, "tick": <now>}`. `400` for missing or non-positive ticks; GET is 405 via the method pattern.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/api_test.go`:

```go
func TestAPIAdvance(t *testing.T) {
	mux, w := newTestAPI(t)
	before := w.Tick

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/advance?ticks=100", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var resp struct {
		Advanced int   `json:"advanced"`
		Tick     int64 `json:"tick"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Advanced != 100 || resp.Tick != before+100 || w.Tick != before+100 {
		t.Errorf("advance: %+v, world tick %d", resp, w.Tick)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/advance?ticks=0", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ticks=0: status %d, want 400", rec.Code)
	}
	if rec := apiGet(t, mux, "/api/advance?ticks=5"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET advance: status %d, want 405", rec.Code)
	}
}
```

Note: `newTestAPI` must construct the Server with `players: map[string]*Player{}` (added in Task 1's flow) so `s.owners()` works; update the helper if it does not already.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestAPIAdvance -v`
Expected: FAIL (404: route not registered)

- [ ] **Step 3: Implement**

In `internal/server/api.go` register the route:

```go
	mux.HandleFunc("POST /api/advance", s.handleAdvance)
```

Add the handler:

```go
// handleAdvance is a dev tool: it fast-forwards the world so slow
// hours-scale behavior can be tested without waiting.
func (s *Server) handleAdvance(rw http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("ticks"))
	if err != nil || n < 1 {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "ticks must be a positive integer"})
		return
	}
	if n > 1000000 {
		n = 1000000
	}
	s.mu.Lock()
	for i := 0; i < n; i++ {
		s.world.Step()
	}
	// the snapshot below carries the full state; drop pending diffs
	s.world.DirtyAndReset()
	s.world.TerrainDirtyAndReset()
	snap, merr := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners()))
	tick := s.world.Tick
	s.mu.Unlock()
	if merr == nil {
		s.hub.Broadcast(snap)
	}
	writeJSON(rw, http.StatusOK, map[string]any{"advanced": n, "tick": tick})
}
```

- [ ] **Step 4: Document in the verify skill**

In `.claude/skills/verify/SKILL.md`, add to the debug-endpoints code block:

```bash
curl -s -X POST 'localhost:8080/api/advance?ticks=200000'  # fast-forward ~a day; broadcasts a snapshot
```

and in Gotchas, note that `/api/advance` replaces waiting at 64x for completions.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -v && go test ./... -short && go vet ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/api.go internal/server/api_test.go .claude/skills/verify/SKILL.md
git commit -m "Add POST /api/advance to fast-forward the world for testing"
```

---

### Task 5: Client identity, overlay, owner marker, and status line

**Files:**
- Modify: `client/src/types.ts`, `client/src/net.ts`, `client/src/world.ts`, `client/src/ui.ts`, `client/src/render.ts`, `client/index.html`

**Interfaces:**
- Consumes: wire messages from Tasks 2-3 (`player` message, `EntityView.owner`).
- Produces: localStorage token (`cellar-player-token`); `world.playerState/playerDwarfId/playerName/playerError`; `sendSpawn(name)`; overlay elements `#overlay`, `#overlay-title`, `#overlay-text`, `#player-name`, `#spawn-btn`, `#overlay-error`, `#watch-link`; side-panel `#mydwarf`.

- [ ] **Step 1: Types**

In `client/src/types.ts` add:

```ts
export interface PlayerMsg {
  type: "player";
  state: "none" | "alive" | "dead";
  dwarfId?: number;
  name?: string;
  error?: string;
}
```

and add to `EntityView`:

```ts
  owner?: string;
```

- [ ] **Step 2: Identity and messages in net.ts**

Replace `client/src/net.ts` content:

```ts
import { world } from "./world";
import type { PlayerMsg, SnapshotMsg, TickMsg } from "./types";

let ws: WebSocket | null = null;

const TOKEN_KEY = "cellar-player-token";

function playerToken(): string {
  let t = localStorage.getItem(TOKEN_KEY);
  if (!t) {
    t = crypto.randomUUID();
    localStorage.setItem(TOKEN_KEY, t);
  }
  return t;
}

export function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => ws?.send(JSON.stringify({ type: "hello", player: playerToken() }));
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data) as SnapshotMsg | TickMsg | PlayerMsg;
    if (msg.type === "snapshot") world.applySnapshot(msg);
    else if (msg.type === "tick") world.applyTick(msg);
    else if (msg.type === "player") world.applyPlayer(msg);
  };
  ws.onclose = () => setTimeout(connect, 1000);
}

export function sendTimescale(scale: number) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "timescale", scale }));
}

export function sendSpawn(name: string) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "spawn", player: playerToken(), name }));
}
```

- [ ] **Step 3: Player state in world.ts**

Add fields after `terrainVersion`:

```ts
  playerState: "unknown" | "none" | "alive" | "dead" = "unknown";
  playerDwarfId: number | null = null;
  playerName = "";
  playerError = "";
```

Add the method (import `PlayerMsg` in the type import):

```ts
  applyPlayer(m: PlayerMsg) {
    this.playerState = m.state;
    this.playerDwarfId = m.dwarfId ?? null;
    if (m.name) this.playerName = m.name;
    this.playerError = m.error ?? "";
    this.fireChange();
  }
```

In `applyTick`, after the removed loop and before the pops loop, detect the owner's death live:

```ts
    if (this.playerState === "alive" && this.playerDwarfId != null) {
      const mine = this.entities.get(this.playerDwarfId);
      if (!mine || mine.dead) {
        this.playerState = "dead";
        this.playerDwarfId = null;
      }
    }
```

- [ ] **Step 4: Overlay markup and styles in index.html**

Inside `#map`, after the popup div:

```html
    <div id="overlay">
      <div id="overlay-card">
        <h2 id="overlay-title">A dwarf awaits</h2>
        <p id="overlay-text"></p>
        <input id="player-name" maxlength="24" placeholder="your name" />
        <button id="spawn-btn">Spawn a dwarf</button>
        <div id="overlay-error"></div>
        <a id="watch-link" href="#">just watch</a>
      </div>
    </div>
```

In the stylesheet:

```css
    #overlay { position: absolute; inset: 0; display: none; align-items: center; justify-content: center; background: rgba(10, 8, 7, 0.72); }
    #overlay-card { background: #1c1815; border: 1px solid #4a443c; padding: 20px 24px; display: flex; flex-direction: column; gap: 10px; width: 260px; }
    #overlay-card input { background: #262220; color: #cfc9bf; border: 1px solid #444; padding: 6px 8px; font: inherit; }
    #overlay-card button { background: #4a7c3f; color: #fff; border: none; padding: 8px 10px; cursor: pointer; font: inherit; }
    #overlay-error { color: #d9724a; min-height: 1em; }
    #watch-link { color: #8d857a; text-align: center; }
```

In `#side`, after the Gold div:

```html
    <div><h2>Your Dwarf</h2><div id="mydwarf">none</div></div>
```

- [ ] **Step 5: Overlay logic and status line in ui.ts**

Import `sendSpawn` alongside `sendTimescale`. In `initUI` add `initOverlay();` and `world.onChange(renderOverlay);` and `world.onChange(renderMyDwarf);`. Add:

```ts
let spectating = false;

function initOverlay() {
  const btn = document.getElementById("spawn-btn") as HTMLButtonElement;
  const input = document.getElementById("player-name") as HTMLInputElement;
  const watch = document.getElementById("watch-link")!;
  btn.onclick = () => sendSpawn(input.value);
  input.onkeydown = (e) => { if (e.key === "Enter") sendSpawn(input.value); };
  watch.addEventListener("click", (e) => {
    e.preventDefault();
    spectating = true;
    renderOverlay();
  });
}

function renderOverlay() {
  const overlay = document.getElementById("overlay")!;
  const title = document.getElementById("overlay-title")!;
  const text = document.getElementById("overlay-text")!;
  const input = document.getElementById("player-name") as HTMLInputElement;
  const btn = document.getElementById("spawn-btn")!;
  const errBox = document.getElementById("overlay-error")!;
  const st = world.playerState;
  const show = st === "dead" || (st === "none" && !spectating);
  overlay.style.display = show ? "flex" : "none";
  if (!show) return;
  if (st === "dead") {
    title.textContent = "Your dwarf has died";
    text.textContent = "The cellar is unforgiving. Send another?";
    btn.textContent = "Spawn a new dwarf";
  } else {
    title.textContent = "A dwarf awaits";
    text.textContent = "Name yourself and send a dwarf to dig for gold.";
    btn.textContent = "Spawn a dwarf";
  }
  if (!input.value && world.playerName) input.value = world.playerName;
  errBox.textContent = world.playerError;
}

function renderMyDwarf() {
  const box = document.getElementById("mydwarf")!;
  if (world.playerState === "alive" && world.playerDwarfId != null) {
    const e = world.entities.get(world.playerDwarfId);
    if (e) {
      const cap = world.species[e.s]?.stomachSize ?? 0;
      box.textContent = `#${e.id}, ${e.action || "idle"}, fullness ${e.full.toFixed(1)} / ${cap}`;
      return;
    }
  }
  box.textContent = world.playerState === "dead" ? "dead" : "none";
}
```

In `renderInspector`, change the first line to include the owner:

```ts
    `${sp?.name ?? e.s} #${e.id}${e.owner ? ` (${e.owner})` : ""}${e.dead ? " (dead)" : ""}`,
```

- [ ] **Step 6: Owner ring in render.ts**

In `frame`, after the entity loop (before the mining bars):

```ts
      if (world.playerDwarfId != null) {
        const me = world.entities.get(world.playerDwarfId);
        if (me && !me.dead) {
          const mt = Math.min(1, (now - me.movedAt) / lerpMs);
          const mx = (me.px + (me.x - me.px) * mt) * TILE + TILE / 2;
          const my = (me.py + (me.y - me.py) * mt) * TILE + TILE / 2;
          ctx.strokeStyle = "rgba(255, 255, 255, 0.65)";
          ctx.lineWidth = 1.5;
          ctx.beginPath();
          ctx.arc(mx, my, TILE / 2 + 2.5, 0, Math.PI * 2);
          ctx.stroke();
        }
      }
```

- [ ] **Step 7: Build**

Run: `cd client && npm run build && cd ..`
Expected: clean tsc + vite build

- [ ] **Step 8: End-to-end verification (verify skill recipes)**

Normal-speed flow against a fresh world:

```bash
go run ./cmd/cellarfloor -fresh   # background
```

Playwright (channel 'chrome', headless): load page → welcome overlay visible; type "Misha", click Spawn → overlay hides, `/api/state` pops.dwarf 1, `/api/entities?species=dwarf` shows `"owner":"Misha"`; `#mydwarf` shows `#N, ...`; white ring pixels near the dwarf; click the dwarf → popup first line `Dwarf #N (Misha)`; reload the page → no overlay (state alive via stored token). Stop the server.

Death flow against a fast-starvation copy of the data:

```bash
SCRATCH=<scratchpad>/fastdeath && mkdir -p $SCRATCH && cp data/*.toml $SCRATCH/
# edit $SCRATCH/species.toml: metabolism = 0.5, starve_ticks = 10, hunger_threshold = 0.1
# (hunger_threshold near zero so the dwarf does not eat mushrooms)
go run ./cmd/cellarfloor -data $SCRATCH -fresh -addr :8081
```

Playwright against :8081: spawn, wait ~15s at 1x (or click 64x), death overlay appears with "Your dwarf has died"; click spawn again → alive with a new dwarf id. Stop the server.

- [ ] **Step 9: Commit**

```bash
git add client/src/types.ts client/src/net.ts client/src/world.ts client/src/ui.ts client/src/render.ts client/index.html
git commit -m "Add player identity, spawn overlay, and owner markers to client"
```
