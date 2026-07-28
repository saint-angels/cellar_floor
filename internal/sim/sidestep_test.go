package sim

import (
	"testing"
)

// wedgeWorld reproduces the head-on wedge seen live: two miners walking to
// faces on opposite sides of the map, each standing on the exact cell the
// other's path demands. nextStepToward BFSes over terrain only and is
// deterministic, so the geometry below is stable: the east-bound miner at
// {4,3} paths through {5,2} and the west-bound one at {5,2} paths back
// through {4,3}.
//
// Live, this pinned dwarf 66 ("heading to mine") and dwarf 68 (then hauling
// ore to market) for ~700 ticks. Hunger did not break it either: both had a
// mushroom in reach, so they ate in place and re-wedged.
func wedgeWorld(t *testing.T) (*World, *Entity, *Entity) {
	t.Helper()
	cfg := mineCfg()
	w := NewWorld(20, 20, 1, cfg)
	w.Spawn("sunstone", Point{0, 0}) // flood the world with light
	// Both faces hardish (10000 hp) so they never mine out mid-test and the
	// miners keep wanting them.
	w.Terrain[idx(w, Point{8, 2})] = Terrain(6)
	w.Terrain[idx(w, Point{1, 6})] = Terrain(6)

	east := w.Spawn("miner", Point{4, 3}) // walks east to the face at {8,2}
	assignFace(east, 8, 2)                // dig target (food-digging assigns this live)
	west := w.Spawn("miner", Point{5, 2}) // walks west to the face at {1,6}
	assignFace(west, 1, 6)
	return w, east, west
}

func TestWedgedDwarvesDoNotDeadlock(t *testing.T) {
	w, east, west := wedgeWorld(t)

	// the wedge must actually be set up: each path step lands on the other
	if n, ok := w.nextStepToward(east.Pos, Point{8, 2}); !ok || n != west.Pos {
		t.Fatalf("precondition: east miner's step = %v (ok=%v), want the west miner's cell %v", n, ok, west.Pos)
	}
	if n, ok := w.nextStepToward(west.Pos, Point{1, 6}); !ok || n != east.Pos {
		t.Fatalf("precondition: west miner's step = %v (ok=%v), want the east miner's cell %v", n, ok, east.Pos)
	}

	startEast, startWest := east.Pos, west.Pos
	for i := 0; i < 40; i++ {
		w.Step()
	}
	if east.Pos == startEast && west.Pos == startWest {
		t.Fatalf("deadlock: miners still at %v and %v after 40 steps", east.Pos, west.Pos)
	}
	// both must finish their errand, not merely jitter loose
	if !adjacent(east.Pos, Point{8, 2}) {
		t.Errorf("east miner at %v never reached the face {8,2}", east.Pos)
	}
	if !adjacent(west.Pos, Point{1, 6}) {
		t.Errorf("west miner at %v never reached the face {1,6}", west.Pos)
	}
}
