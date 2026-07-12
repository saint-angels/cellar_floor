package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// buyUpgrade purchases the next pick tier with colony gold. Caller holds s.mu.
func (s *Server) buyUpgrade(token string) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if s.world.UpgradeLevel >= len(s.cfg.Upgrades) {
		pm.Error = "nothing left to forge"
		return pm
	}
	tier := s.cfg.Upgrades[s.world.UpgradeLevel]
	if s.world.Gold < tier.Cost {
		pm.Error = "not enough gold"
		return pm
	}
	s.world.Gold -= tier.Cost
	s.world.UpgradeLevel++
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "forged",
		Msg: fmt.Sprintf("%s forged %s", s.players[token].Name, tier.Name),
	})
	return pm
}
