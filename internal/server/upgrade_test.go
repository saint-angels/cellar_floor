package server

import "testing"

func TestClaimUpgrade(t *testing.T) {
	s := newPlayerServer(t)
	if res := s.claimUpgrade("ghost", "Sharper Picks"); res.Error == "" {
		t.Fatal("no dwarf: must error")
	}
	s.spawnDwarf("tok", "Misha")
	if res := s.claimUpgrade("tok", "Sharper Picks"); res.Error != "nothing to claim" {
		t.Fatalf("got %q, want nothing to claim", res.Error)
	}
	s.world.PendingLevels = 2
	s.world.Offer = []string{"Sharper Picks", "Chisel"}
	if res := s.claimUpgrade("tok", "Hammer"); res.Error != "not on offer" {
		t.Fatalf("got %q, want not on offer", res.Error)
	}
	if res := s.claimUpgrade("tok", "Sharper Picks"); res.Error != "" {
		t.Fatalf("claim failed: %v", res.Error)
	}
	if s.world.Claims["Sharper Picks"] != 1 {
		t.Fatalf("claims = %v", s.world.Claims)
	}
	if s.world.PendingLevels != 1 || len(s.world.Offer) == 0 {
		t.Fatalf("pendingLevels %d offer %v, want next offer rolled", s.world.PendingLevels, s.world.Offer)
	}
	if len(s.pending) != 1 || s.pending[0].Type != "claimed" {
		t.Fatalf("events = %+v", s.pending)
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
