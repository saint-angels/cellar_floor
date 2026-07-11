package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// placeTorch validates and spawns a player-funded torch. Caller holds s.mu.
func (s *Server) placeTorch(token string, x, y int) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	if s.world.Gold < 1 {
		pm.Error = "not enough gold"
		return pm
	}
	p := sim.Point{X: x, Y: y}
	if !s.world.InBounds(p) || !s.world.Passable(s.world.At(p)) {
		pm.Error = "can't place a torch there"
		return pm
	}
	for _, id := range s.world.SortedIDs() {
		e := s.world.Entities[id]
		if !e.Dead && e.Pos == p && s.cfg.Types[e.Type].Kind == "structure" {
			pm.Error = "something already stands there"
			return pm
		}
	}
	s.world.Gold--
	s.world.Spawn("torch", p)
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "placed",
		Msg: fmt.Sprintf("%s placed a torch", s.players[token].Name),
	})
	return pm
}
