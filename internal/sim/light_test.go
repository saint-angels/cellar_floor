package sim

import (
	"encoding/json"
	"testing"
)

// roundTripThroughJSON marshals and unmarshals a world the way persist.go
// does on load, then reapplies the config so derived state is rebuilt.
func roundTripThroughJSON(t *testing.T, w *World) *World {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w2.SetConfig(structCfg())
	return &w2
}

func TestLightCircle(t *testing.T) {
	w := newStructWorld(t)
	w.Spawn("campfire", Point{10, 10})                  // light_radius 8
	if !w.Lit(Point{10, 10}) || !w.Lit(Point{15, 15}) { // 5*5+5*5=50 <= 64
		t.Fatal("cells inside the radius must be lit")
	}
	if w.Lit(Point{16, 16}) { // 6*6+6*6=72 > 64
		t.Fatal("cells outside the euclidean circle must be dark")
	}
}

func TestLightDiesWithSource(t *testing.T) {
	w := newStructWorld(t)
	torch := w.Spawn("torch", Point{3, 3}) // light_radius 3, lifespan 10
	if !w.Lit(Point{3, 3}) {
		t.Fatal("torch should light its cell")
	}
	for i := 0; i < 11; i++ {
		w.Step()
	}
	_ = torch
	if w.Lit(Point{3, 3}) {
		t.Fatal("light must go out when the torch burns out")
	}
}

func TestLightRebuiltOnSetConfig(t *testing.T) {
	w := newStructWorld(t)
	w.Spawn("campfire", Point{5, 5})
	w2 := roundTripThroughJSON(t, w) // marshal/unmarshal like persist does, then SetConfig
	if !w2.Lit(Point{5, 5}) {
		t.Fatal("lit field must be rebuilt on load")
	}
}
