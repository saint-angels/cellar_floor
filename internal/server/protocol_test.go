package server

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/gen"
)

func TestSnapshotAndTickMessages(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	snap := BuildSnapshot(w, 1)
	if snap.Type != "snapshot" || snap.Width != 64 || len(snap.Entities) == 0 {
		t.Fatalf("bad snapshot: %+v", snap.Type)
	}
	if _, ok := snap.Species["rabbit"]; !ok {
		t.Error("snapshot missing species table")
	}
	b, err := json.Marshal(snap)
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}

	evs := w.Step()
	tick := BuildTick(w, evs, 1)
	if tick.Type != "tick" || tick.Tick != w.Tick {
		t.Fatalf("bad tick msg")
	}
	if len(tick.Changed) == 0 {
		t.Error("expected changed entities on first tick")
	}
	if tick.Pops["rabbit"] == 0 {
		t.Error("pops missing")
	}
	// diff semantics: after building, dirty set is drained
	tick2 := BuildTick(w, nil, 1)
	if len(tick2.Changed) != 0 {
		t.Error("dirty set not drained by BuildTick")
	}
}
