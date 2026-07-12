package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// claimUpgrade applies the oldest pending upgrade draw. Caller holds s.mu.
func (s *Server) claimUpgrade(token string) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if len(s.world.Pending) == 0 {
		pm.Error = "nothing to claim"
		return pm
	}
	name := s.world.Pending[0]
	s.world.Pending = s.world.Pending[1:]
	if s.world.Claims == nil {
		s.world.Claims = map[string]int{}
	}
	s.world.Claims[name]++
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "claimed",
		Msg: fmt.Sprintf("%s claimed %s", s.players[token].Name, name),
	})
	return pm
}
