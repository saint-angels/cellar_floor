package server

import (
	"encoding/json"
	"strings"
	"testing"

	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

func TestSnapshotAndTickMessages(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	snap := BuildSnapshot(w, 1, nil)
	if snap.Type != "snapshot" || snap.Width != 64 || len(snap.Entities) == 0 {
		t.Fatalf("bad snapshot: %+v", snap.Type)
	}
	if _, ok := snap.Types["dwarf"]; !ok {
		t.Error("snapshot missing types table")
	}
	b, err := json.Marshal(snap)
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}

	w.Spawn("dwarf", sim.Point{X: 32, Y: 32}) // clearing center is dirt
	evs := w.Step()
	tick := BuildTick(w, evs, 1, nil)
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
	tick2 := BuildTick(w, nil, 1, nil)
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

	snap := BuildSnapshot(w, 1, nil)
	if snap.Gold != 3 || snap.Mining[5] != 0.25 {
		t.Errorf("snapshot missing mining state: gold=%d mining=%v", snap.Gold, snap.Mining)
	}

	tick := BuildTick(w, nil, 1, nil)
	if tick.Gold != 3 || tick.Mining[5] != 0.25 {
		t.Errorf("tick missing mining state: gold=%d mining=%v", tick.Gold, tick.Mining)
	}
	if len(tick.Terrain) != 1 || tick.Terrain[0].I != 1 || tick.Terrain[0].T != uint8(sim.TerrainFloor) {
		t.Errorf("terrain diff = %+v", tick.Terrain)
	}
	tick2 := BuildTick(w, nil, 1, nil)
	if len(tick2.Terrain) != 0 {
		t.Error("terrain dirty set not drained")
	}
}

func TestViewCarriesMineTarget(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	d := w.Spawn("dwarf", sim.Point{X: 32, Y: 32})
	if v := ViewOf(d); v.MT != nil {
		t.Errorf("mt should be nil without a target, got %v", v.MT)
	}
	target := sim.Point{X: 40, Y: 32}
	d.MineTarget = &target
	v := ViewOf(d)
	if v.MT == nil || *v.MT != target {
		t.Errorf("mt = %v, want %v", v.MT, target)
	}
	b, err := json.Marshal(v)
	if err != nil || !strings.Contains(string(b), `"mt":{"x":40,"y":32}`) {
		t.Errorf("marshal: %s %v", b, err)
	}
}

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

	evs := []sim.Event{{Actor: d.ID, ActorType: "dwarf", Msg: "Dwarf struck gold"}}
	tick := BuildTick(w, evs, 1, owners)
	if tick.Events[0].Msg != "Misha's dwarf struck gold" {
		t.Errorf("decorated msg = %q", tick.Events[0].Msg)
	}
	if evs[0].Msg != "Dwarf struck gold" {
		t.Error("decoration mutated the caller's slice")
	}

	// unowned actors stay untouched
	evs2 := []sim.Event{{Actor: 999999, ActorType: "dwarf", Msg: "Dwarf starved"}}
	if got := BuildTick(w, evs2, 1, owners); got.Events[0].Msg != "Dwarf starved" {
		t.Errorf("unowned msg changed: %q", got.Events[0].Msg)
	}
}
