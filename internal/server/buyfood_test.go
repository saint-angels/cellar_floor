package server

import (
	"testing"

	"cellarfloor/internal/sim"
)

// findMineable returns a mineable rock cell (food may be buried there).
func findMineable(t *testing.T, s *Server) sim.Point {
	t.Helper()
	w := s.world
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if w.Mineable(w.At(p)) {
				return p
			}
		}
	}
	t.Fatal("no mineable cell found")
	return sim.Point{}
}

func foodAt(s *Server, p sim.Point) int {
	n := 0
	for _, id := range s.world.SortedIDs() {
		e := s.world.Entities[id]
		if !e.Dead && e.Type == "mushroom" && e.Pos == p {
			n++
		}
	}
	return n
}

func TestBuyFood(t *testing.T) {
	s, _ := newWSServer(t)

	// no living dwarf: rejected, no charge
	s.world.Gold = 10
	dirt := findFreeDirt(t, s)
	if pm := s.buyFood("tok1", "mushroom", dirt.X, dirt.Y); pm.Error == "" {
		t.Fatal("buyFood without a dwarf should error")
	}
	if s.world.Gold != 10 {
		t.Fatalf("gold changed without a dwarf: %d", s.world.Gold)
	}

	// spawning grants firstSpawnGold, so pin the purse afterwards
	s.spawnDwarf("tok1", "Misha")
	s.world.Gold = 10

	// on open ground: spends the cost and plants one more mushroom
	before := foodAt(s, dirt)
	if pm := s.buyFood("tok1", "mushroom", dirt.X, dirt.Y); pm.Error != "" {
		t.Fatalf("buyFood on dirt: %q", pm.Error)
	}
	if s.world.Gold != 9 {
		t.Fatalf("gold = %d, want 9 after one mushroom", s.world.Gold)
	}
	if foodAt(s, dirt) != before+1 {
		t.Fatalf("mushroom count at %v = %d, want %d", dirt, foodAt(s, dirt), before+1)
	}

	// buried in rock: allowed, so a dwarf can dig toward it
	rock := findMineable(t, s)
	rbefore := foodAt(s, rock)
	if pm := s.buyFood("tok1", "mushroom", rock.X, rock.Y); pm.Error != "" {
		t.Fatalf("buyFood into rock: %q", pm.Error)
	}
	if foodAt(s, rock) != rbefore+1 {
		t.Fatalf("no mushroom buried at %v", rock)
	}
	if s.world.Gold != 8 {
		t.Fatalf("gold = %d, want 8", s.world.Gold)
	}

	// a non-food type is not for sale
	if pm := s.buyFood("tok1", "campfire", dirt.X, dirt.Y); pm.Error == "" {
		t.Fatal("campfire should not be buyable as food")
	}

	// not enough gold: rejected, nothing planted
	s.world.Gold = 0
	nbefore := foodAt(s, dirt)
	if pm := s.buyFood("tok1", "mushroom", dirt.X, dirt.Y); pm.Error == "" {
		t.Fatal("buyFood with no gold should error")
	}
	if foodAt(s, dirt) != nbefore {
		t.Fatal("mushroom planted despite no gold")
	}
}
