package server

import "testing"

func TestBuyUpgrade(t *testing.T) {
	s := newPlayerServer(t)
	if res := s.buyUpgrade("ghost"); res.Error == "" {
		t.Fatal("no dwarf: must error")
	}
	s.spawnDwarf("tok", "Misha")
	s.world.Gold = 2
	if res := s.buyUpgrade("tok"); res.Error != "not enough gold" {
		t.Fatalf("got %q, want not enough gold", res.Error)
	}
	s.world.Gold = 10
	if res := s.buyUpgrade("tok"); res.Error != "" {
		t.Fatalf("buy failed: %v", res.Error)
	}
	if s.world.UpgradeLevel != 1 {
		t.Fatalf("level = %d, want 1", s.world.UpgradeLevel)
	}
	if s.world.Gold != 10-s.cfg.Upgrades[0].Cost {
		t.Fatalf("gold = %d after buying %+v", s.world.Gold, s.cfg.Upgrades[0])
	}
	if len(s.pending) != 1 || s.pending[0].Type != "forged" {
		t.Fatalf("pending = %+v, want one forged event", s.pending)
	}
	// exhaust the track
	s.world.Gold = 100000
	for s.world.UpgradeLevel < len(s.cfg.Upgrades) {
		if res := s.buyUpgrade("tok"); res.Error != "" {
			t.Fatalf("buy at level %d: %v", s.world.UpgradeLevel, res.Error)
		}
	}
	if res := s.buyUpgrade("tok"); res.Error != "nothing left to forge" {
		t.Fatalf("got %q, want nothing left to forge", res.Error)
	}
}

func TestRecapDeltasAndSnapshotAdvance(t *testing.T) {
	s := newPlayerServer(t)
	s.spawnDwarf("tok", "Misha")
	// simulate progress since the spawn-time snapshot
	s.world.Tick += 5000
	s.world.BlocksMined += 7
	s.world.GoldMined += 4
	s.world.MoldGrown += 2
	r := s.recapFor("tok")
	if r == nil || r.Ticks != 5000 || r.Blocks != 7 || r.Gold != 4 || r.Mold != 2 {
		t.Fatalf("recap = %+v", r)
	}
	if again := s.recapFor("tok"); again == nil || again.Blocks != 0 || again.Ticks != 0 {
		t.Fatalf("snapshot must advance after a recap, got %+v", again)
	}
	if s.recapFor("stranger") != nil {
		t.Fatal("unknown tokens get no recap")
	}
	// a snapshot ahead of the live counters (a stale record from a past
	// world) must clamp to zero, never report a negative recap
	s.players["tok"].SeenTick = s.world.Tick + 999
	s.players["tok"].SeenBlocks = s.world.BlocksMined + 999
	s.players["tok"].SeenGold = s.world.GoldMined + 999
	s.players["tok"].SeenMold = s.world.MoldGrown + 999
	clamped := s.recapFor("tok")
	if clamped == nil || clamped.Ticks != 0 || clamped.Blocks != 0 || clamped.Gold != 0 || clamped.Mold != 0 {
		t.Fatalf("recap must clamp at zero, got %+v", clamped)
	}
}
