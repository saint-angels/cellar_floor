package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestWSTorch(t *testing.T) {
	s, ts := newWSServer(t)
	c := dialWS(t, ts)

	send(t, c, map[string]any{"type": "spawn", "player": "tok1", "name": "Misha"})
	pm := readPlayerMsg(t, c)
	if pm.State != "alive" {
		t.Fatalf("spawn: %+v", pm)
	}

	s.mu.Lock()
	s.world.Gold = 3
	p := findFreeDirt(t, s)
	s.mu.Unlock()

	send(t, c, map[string]any{"type": "torch", "player": "tok1", "x": p.X, "y": p.Y})
	if res := readPlayerMsg(t, c); res.Error != "" {
		t.Fatalf("torch reply error: %q", res.Error)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.world.Gold != 2 {
		t.Fatalf("gold = %d, want 2", s.world.Gold)
	}
	found := false
	for _, e := range s.world.Entities {
		if e.Type == "torch" && e.Pos == p {
			found = true
		}
	}
	if !found {
		t.Fatal("torch entity not spawned")
	}
}

func TestWSUpgrade(t *testing.T) {
	s, ts := newWSServer(t)
	c := dialWS(t, ts)

	send(t, c, map[string]any{"type": "spawn", "player": "tok1", "name": "Misha"})
	if pm := readPlayerMsg(t, c); pm.State != "alive" {
		t.Fatalf("spawn: %+v", pm)
	}

	s.mu.Lock()
	s.world.Gold = 100
	s.mu.Unlock()

	send(t, c, map[string]any{"type": "upgrade", "player": "tok1"})
	if res := readPlayerMsg(t, c); res.Error != "" {
		t.Fatalf("upgrade reply error: %q", res.Error)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.world.UpgradeLevel != 1 {
		t.Fatalf("upgrade level = %d, want 1", s.world.UpgradeLevel)
	}
}

func TestWSSnapshotCarriesUpgrades(t *testing.T) {
	s, ts := newWSServer(t)
	s.mu.Lock()
	s.world.UpgradeLevel = 2
	s.mu.Unlock()
	c := dialWS(t, ts)

	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, b, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("no snapshot arrived: %v", err)
		}
		var snap SnapshotMsg
		if json.Unmarshal(b, &snap) != nil || snap.Type != "snapshot" {
			continue
		}
		if len(snap.Upgrades) != len(s.cfg.Upgrades) {
			t.Fatalf("snapshot upgrades = %d, want %d", len(snap.Upgrades), len(s.cfg.Upgrades))
		}
		if snap.UpgradeLevel != 2 {
			t.Fatalf("snapshot upgradeLevel = %d, want 2", snap.UpgradeLevel)
		}
		break
	}
}

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
