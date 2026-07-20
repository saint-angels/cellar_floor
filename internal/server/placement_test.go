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
			if w.Passable(w.At(p)) && !occupied[p] {
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
			if !w.Passable(w.At(p)) {
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
