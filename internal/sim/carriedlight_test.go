package sim

import "testing"

// A dwarf that carries its own light lights the rock around it, so it can mine
// in an otherwise pitch-dark world and never flees the dark. This is what lets
// torches go away.
func TestCarriedLightMinesInTheDark(t *testing.T) {
	cfg := mineCfg()
	cfg.Types["dwarf"].LightRadius = 2 // carry a small light
	w := NewWorld(20, 20, 1, cfg)      // no sunstone: the world is dark
	face := Point{5, 4}
	w.Terrain[idx(w, face)] = Terrain(3) // rock right above the dwarf

	d := w.Spawn("dwarf", Point{5, 5})
	d.Fullness = 8 // well fed; it mines only the face assigned to it
	assignFace(d, 5, 4)

	if !w.Lit(d.Pos) {
		t.Fatal("a light-carrier must light its own tile")
	}
	if !w.Lit(face) {
		t.Fatal("the adjacent face must fall inside the carried light")
	}

	for i := 0; i < 5; i++ {
		w.Step()
		if d.Action == "fleeing the dark" {
			t.Fatalf("a light-carrier must never flee the dark (step %d)", i)
		}
	}
	if w.MineDamage[idx(w, face)] == 0 && w.At(face) == Terrain(3) {
		t.Fatalf("dwarf never mined the lit face it was standing beside; action=%q", d.Action)
	}
}

// The light must travel with the dwarf: after it moves, its old tile goes dark
// and the new one lights up.
func TestCarriedLightFollowsTheDwarf(t *testing.T) {
	cfg := mineCfg()
	cfg.Types["dwarf"].LightRadius = 2
	w := NewWorld(20, 20, 1, cfg)
	d := w.Spawn("dwarf", Point{5, 5})
	d.Fullness = 8
	start := d.Pos
	// nudge it and let a tick rebuild the light
	d.Pos = Point{12, 12}
	w.RecomputeLight()
	if w.Lit(start) {
		t.Errorf("old tile %v should be dark after the light-carrier left", start)
	}
	if !w.Lit(d.Pos) {
		t.Errorf("new tile %v should be lit by the carried light", d.Pos)
	}
}
