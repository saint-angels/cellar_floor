package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// claimUpgrade picks one upgrade from the current offer. Caller holds s.mu.
func (s *Server) claimUpgrade(token, name string) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if s.world.PendingLevels == 0 {
		pm.Error = "nothing to claim"
		return pm
	}
	if !s.world.ClaimOffer(name) {
		pm.Error = "not on offer"
		return pm
	}
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "claimed",
		Msg: fmt.Sprintf("%s claimed %s", s.players[token].Name, name),
	})
	return pm
}
