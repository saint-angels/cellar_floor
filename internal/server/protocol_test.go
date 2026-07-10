package server

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

func TestSnapshotAndTickMessages(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	snap := BuildSnapshot(w, 1)
	if snap.Type != "snapshot" || snap.Width != 64 || len(snap.Entities) == 0 {
		t.Fatalf("bad snapshot: %+v", snap.Type)
	}
	if _, ok := snap.Species["dwarf"]; !ok {
		t.Error("snapshot missing species table")
	}
	b, err := json.Marshal(snap)
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}

	w.Spawn("dwarf", sim.Point{X: 32, Y: 32}) // clearing center is dirt
	evs := w.Step()
	tick := BuildTick(w, evs, 1)
	if tick.Type != "tick" || tick.Tick != w.Tick {
		t.Fatalf("bad tick msg")
	}
	if len(tick.Changed) == 0 {
		t.Error("expected changed entities on first tick")
	}
	if tick.Pops["dwarf"] == 0 {
		t.Error("pops missing")
	}
	// diff semantics: after building, dirty set is drained
	tick2 := BuildTick(w, nil, 1)
	if len(tick2.Changed) != 0 {
		t.Error("dirty set not drained by BuildTick")
	}
}

func TestTickCarriesMiningState(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	w.Gold = 3
	w.MineProgress[5] = 0.25
	w.SetTerrain(sim.Point{X: 1, Y: 0}, sim.TerrainFloor)

	snap := BuildSnapshot(w, 1)
	if snap.Gold != 3 || snap.Mining[5] != 0.25 {
		t.Errorf("snapshot missing mining state: gold=%d mining=%v", snap.Gold, snap.Mining)
	}

	tick := BuildTick(w, nil, 1)
	if tick.Gold != 3 || tick.Mining[5] != 0.25 {
		t.Errorf("tick missing mining state: gold=%d mining=%v", tick.Gold, tick.Mining)
	}
	if len(tick.Terrain) != 1 || tick.Terrain[0].I != 1 || tick.Terrain[0].T != uint8(sim.TerrainFloor) {
		t.Errorf("terrain diff = %+v", tick.Terrain)
	}
	tick2 := BuildTick(w, nil, 1)
	if len(tick2.Terrain) != 0 {
		t.Error("terrain dirty set not drained")
	}
}
