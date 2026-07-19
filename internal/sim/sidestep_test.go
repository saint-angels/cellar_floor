package sim

import (
	"testing"

	"cellarfloor/internal/data"
)

// wedgeWorld reproduces the head-on wedge seen live: a miner walking to a rock
// face and a hauler walking to market, each standing on the exact cell the
// other's path demands. nextStepToward BFSes over terrain only and is
// deterministic, so the geometry below is stable: the miner at {4,3} paths
// through {5,2} and the hauler at {5,2} paths back through {4,3}.
//
// Live, this pinned dwarf 66 ("heading to mine") and dwarf 68 ("hauling ore")
// for ~700 ticks. Hunger did not break it either: both had a mushroom in
// reach, so they ate in place and re-wedged.
func wedgeWorld(t *testing.T) (*World, *Entity, *Entity) {
	t.Helper()
	cfg := mineCfg()
	cfg.Types["dwarf"].CarryCapacity = 3
	cfg.Types["market"] = &data.EntityType{
		ID: "market", Name: "Market", Kind: "structure", Color: "#fff", Market: true,
	}
	w := NewWorld(20, 20, 1, cfg)
	w.Spawn("sunstone", Point{0, 0}) // flood the world with light
	// The sole face, hardish (10000 hp) so it never mines out mid-test and the
	// miner keeps wanting it.
	w.Terrain[idx(w, Point{8, 2})] = Terrain(6)
	w.Spawn("market", Point{1, 6})

	miner := w.Spawn("dwarf", Point{4, 3}) // ore 0 -> walks east to the face
	hauler := w.Spawn("dwarf", Point{5, 2})
	hauler.Ore = cfg.Types["dwarf"].CarryCapacity // full bag -> walks west to market
	return w, miner, hauler
}

func TestWedgedDwarvesDoNotDeadlock(t *testing.T) {
	w, miner, hauler := wedgeWorld(t)

	// the wedge must actually be set up: each path step lands on the other
	if n, ok := w.nextStepToward(miner.Pos, Point{8, 2}); !ok || n != hauler.Pos {
		t.Fatalf("precondition: miner's step = %v (ok=%v), want the hauler's cell %v", n, ok, hauler.Pos)
	}
	if n, ok := w.nextStepToward(hauler.Pos, Point{1, 6}); !ok || n != miner.Pos {
		t.Fatalf("precondition: hauler's step = %v (ok=%v), want the miner's cell %v", n, ok, miner.Pos)
	}

	startMiner, startHauler := miner.Pos, hauler.Pos
	for i := 0; i < 40; i++ {
		w.Step()
	}
	if miner.Pos == startMiner && hauler.Pos == startHauler {
		t.Fatalf("deadlock: miner still at %v and hauler still at %v after 40 steps",
			miner.Pos, hauler.Pos)
	}
	// both must finish their errand, not merely jitter loose
	if !adjacent(miner.Pos, Point{8, 2}) {
		t.Errorf("miner at %v never reached the face {8,2}", miner.Pos)
	}
	if hauler.Ore != 0 {
		t.Errorf("hauler still holds %d ore at %v: never reached the market", hauler.Ore, hauler.Pos)
	}
}
