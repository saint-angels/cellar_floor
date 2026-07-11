package server

import (
	"testing"

	"cellarfloor/internal/sim"
)

// findFreeDirt returns a passable cell that holds no structure.
func findFreeDirt(t *testing.T, s *Server) sim.Point {
	t.Helper()
	w := s.world
	occupied := map[sim.Point]bool{}
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if !e.Dead && s.cfg.Types[e.Type].Kind == "structure" {
			occupied[e.Pos] = true
		}
	}
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if sim.Passable(w.At(p)) && !occupied[p] {
				return p
			}
		}
	}
	t.Fatal("no free passable cell found")
	return sim.Point{}
}

// findRock returns an impassable rock cell.
func findRock(t *testing.T, s *Server) sim.Point {
	t.Helper()
	w := s.world
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if !sim.Passable(w.At(p)) {
				return p
			}
		}
	}
	t.Fatal("no impassable cell found")
	return sim.Point{}
}

// findStructure returns the cell of an existing structure (the gen campfire).
func findStructure(t *testing.T, s *Server) sim.Point {
	t.Helper()
	w := s.world
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if !e.Dead && s.cfg.Types[e.Type].Kind == "structure" {
			return e.Pos
		}
	}
	t.Fatal("no structure found in generated world")
	return sim.Point{}
}

func TestPlaceTorchHappyPath(t *testing.T) {
	s := newPlayerServer(t)
	pm := s.spawnDwarf("tok", "Misha")
	if pm.Error != "" {
		t.Fatalf("spawn: %v", pm.Error)
	}
	s.world.Gold = 5
	p := findFreeDirt(t, s)
	res := s.placeTorch("tok", p.X, p.Y)
	if res.Error != "" {
		t.Fatalf("placeTorch: %v", res.Error)
	}
	if s.world.Gold != 4 {
		t.Fatalf("gold = %d, want 4", s.world.Gold)
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
	if len(s.pending) != 1 || s.pending[0].Type != "placed" {
		t.Fatal("expected one pending placed event")
	}
}

func TestPlaceTorchValidation(t *testing.T) {
	s := newPlayerServer(t)
	if res := s.placeTorch("ghost", 1, 1); res.Error == "" {
		t.Fatal("no dwarf: must error")
	}
	s.spawnDwarf("tok", "Misha")
	s.world.Gold = 0
	free := findFreeDirt(t, s)
	if res := s.placeTorch("tok", free.X, free.Y); res.Error != "not enough gold" {
		t.Fatalf("got %q, want not enough gold", res.Error)
	}
	s.world.Gold = 5
	if res := s.placeTorch("tok", -1, 0); res.Error == "" {
		t.Fatal("out of bounds: must error")
	}
	rock := findRock(t, s)
	if res := s.placeTorch("tok", rock.X, rock.Y); res.Error == "" {
		t.Fatal("impassable cell: must error")
	}
	structPos := findStructure(t, s)
	if res := s.placeTorch("tok", structPos.X, structPos.Y); res.Error == "" {
		t.Fatal("cell holding a structure: must error")
	}
	if s.world.Gold != 5 {
		t.Fatalf("failed placements must not spend gold, gold = %d", s.world.Gold)
	}
}

func TestPlaceTorchOnFaunaCellAllowed(t *testing.T) {
	s := newPlayerServer(t)
	s.spawnDwarf("tok", "Misha")
	s.world.Gold = 5
	// find a fauna and place a torch on its cell; allowed since occ is fauna-only
	var faunaPos sim.Point
	found := false
	for _, id := range s.world.SortedIDs() {
		e := s.world.Entities[id]
		if !e.Dead && s.cfg.Types[e.Type].Kind == "fauna" && sim.Passable(s.world.At(e.Pos)) {
			faunaPos = e.Pos
			found = true
			break
		}
	}
	if !found {
		t.Skip("no fauna in generated world")
	}
	res := s.placeTorch("tok", faunaPos.X, faunaPos.Y)
	if res.Error != "" {
		t.Fatalf("placing on a fauna cell should be allowed, got %q", res.Error)
	}
}
