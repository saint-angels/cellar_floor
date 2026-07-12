package server

import "testing"

func TestDebugActions(t *testing.T) {
	s := newPlayerServer(t)

	s.world.Gold = 3
	s.debugAction(ClientMsg{Action: "gold", N: 10})
	if s.world.Gold != 13 {
		t.Fatalf("gold = %d, want 13", s.world.Gold)
	}
	s.debugAction(ClientMsg{Action: "gold", N: -100})
	if s.world.Gold != 0 {
		t.Fatalf("gold = %d, want clamp to 0", s.world.Gold)
	}

	if len(s.cfg.Upgrades) == 0 {
		t.Fatal("fixture config must have an upgrade pool")
	}
	name := s.cfg.Upgrades[0].Name
	s.debugAction(ClientMsg{Action: "claims", Name: name, N: 2})
	if s.world.Claims[name] != 2 {
		t.Fatalf("claims[%s] = %d, want 2", name, s.world.Claims[name])
	}
	s.debugAction(ClientMsg{Action: "claims", Name: name, N: -5})
	if _, ok := s.world.Claims[name]; ok {
		t.Fatalf("claims[%s] must clamp away, got %v", name, s.world.Claims)
	}
	s.debugAction(ClientMsg{Action: "claims", Name: "No Such Upgrade", N: 1})
	if _, ok := s.world.Claims["No Such Upgrade"]; ok {
		t.Fatal("unknown names must not enter claims")
	}

	before := s.world.Level
	s.debugAction(ClientMsg{Action: "level"})
	s.world.Step()
	if s.world.Level != before+1 {
		t.Fatalf("level = %d, want %d after debug level", s.world.Level, before+1)
	}
	if s.world.PendingLevels == 0 || len(s.world.Offer) == 0 {
		t.Fatal("completing a level must queue a choice and roll an offer")
	}
}
