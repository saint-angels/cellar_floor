package server

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

func newPlayerServer(t *testing.T) *Server {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
	s.scale.Store(1)
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
	if e == nil || e.Type != "dwarf" {
		t.Fatal("no dwarf entity spawned")
	}
	if !s.world.InBounds(e.Pos) || s.world.At(e.Pos) != sim.TerrainDirt {
		t.Fatalf("spawned off the clearing: %v", e.Pos)
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
	for _, id := range s.world.SortedIDs() {
		if c := s.world.Entities[id]; c.Type == "mushroom" {
			for i := range c.Produces {
				c.Produces[i].Amount = 0 // nothing to eat, starvation must win
			}
		}
	}
	e := s.world.Entities[pm.DwarfID]
	e.Fullness = 0
	e.StarvingFor = s.cfg.Types["dwarf"].StarveTicks + 1
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
	popCap := s.cfg.Types["dwarf"].PopCap
	for i := 0; i < popCap; i++ {
		if pm := s.spawnDwarf(fmt.Sprintf("tok%d", i), "P"); pm.State != "alive" {
			t.Fatalf("spawn %d failed: %+v", i, pm)
		}
	}
	pm := s.spawnDwarf("late", "P")
	if pm.Error != "the cellar is crowded" || pm.State != "none" {
		t.Fatalf("crowded reply %+v", pm)
	}
}

func TestResetWorld(t *testing.T) {
	s := newPlayerServer(t)
	s.cfg.Sim.SavePath = filepath.Join(t.TempDir(), "world.json")
	pm := s.spawnDwarf("tok1", "Misha")
	if pm.State != "alive" {
		t.Fatalf("spawn: %+v", pm)
	}
	s.world.Gold = 5
	s.world.MineProgress[42] = 0.5

	snap := s.resetWorld()
	if snap == nil {
		t.Fatal("resetWorld returned nil snapshot")
	}
	if s.world.Tick != 0 {
		t.Errorf("world not fresh: tick %d", s.world.Tick)
	}
	if s.world.CountAlive("dwarf") != 0 {
		t.Error("dwarves survived the reset")
	}
	// stale ids from the old world must not linger: entity ids restart on
	// reset, so a kept DwarfID could collide with a new world's entity and
	// make owners() flip names between players
	for tok, p := range s.players {
		if p.DwarfID != 0 {
			t.Errorf("player %s kept stale DwarfID %d after reset", tok, p.DwarfID)
		}
	}
	if s.world.Gold != 0 || len(s.world.MineProgress) != 0 {
		t.Error("gold or mining progress survived the reset")
	}
	if got := s.playerMsg("tok1"); got.State != "dead" || got.Name != "Misha" {
		t.Errorf("player after reset: %+v, want dead with name", got)
	}
	var parsed SnapshotMsg
	if err := json.Unmarshal(snap, &parsed); err != nil || parsed.Type != "snapshot" {
		t.Fatalf("bad snapshot payload: %v", err)
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

func TestFirstSpawnGrantsOneGold(t *testing.T) {
	s := newPlayerServer(t)
	start := s.world.Gold
	if pm := s.spawnDwarf("tok1", "Misha"); pm.Error != "" {
		t.Fatalf("spawn: %v", pm.Error)
	}
	if s.world.Gold != start+1 {
		t.Fatalf("gold = %d, want %d after a new player's first spawn", s.world.Gold, start+1)
	}
	// a second new player brings another coin
	if pm := s.spawnDwarf("tok2", "Sasha"); pm.Error != "" {
		t.Fatalf("spawn: %v", pm.Error)
	}
	if s.world.Gold != start+2 {
		t.Fatalf("gold = %d, want %d after a second new player", s.world.Gold, start+2)
	}
	// mark tok1's dwarf dead; the respawn must not grant again
	// (flipping Dead is enough for spawnDwarf's alive check; pop cap is far off)
	s.world.Entities[s.players["tok1"].DwarfID].Dead = true
	if pm := s.spawnDwarf("tok1", "Misha"); pm.Error != "" {
		t.Fatalf("respawn: %v", pm.Error)
	}
	if s.world.Gold != start+2 {
		t.Fatalf("gold = %d, want %d; respawn must not farm gold", s.world.Gold, start+2)
	}
}
