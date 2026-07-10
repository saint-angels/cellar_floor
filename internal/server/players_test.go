package server

import (
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
	if e == nil || e.Species != "dwarf" {
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
		if c := s.world.Entities[id]; c.Species == "mushroom" {
			for i := range c.Produces {
				c.Produces[i].Amount = 0 // nothing to eat, starvation must win
			}
		}
	}
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
	popCap := s.cfg.Species["dwarf"].PopCap
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
