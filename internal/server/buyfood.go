package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// buyFood is the player-facing, gold-costing food placement: a living player
// spends colony gold to plant one edible flora on a tile. Unlike the free
// debug spawner, it charges Cost and requires a live dwarf. Food may be
// buried in mineable rock so a hungry dwarf senses it and tunnels over —
// planting into the dark is how you steer dwarves. Caller holds s.mu.
func (s *Server) buyFood(token, name string, x, y int) PlayerMsg {
	pm := s.playerMsg(token)
	if pm.State != "alive" {
		pm.Error = "you need a living dwarf"
		return pm
	}
	sp, ok := s.cfg.Types[name]
	if !ok || sp.Kind != "flora" || sp.Cost <= 0 {
		pm.Error = "that isn't for sale"
		return pm
	}
	if s.world.Gold < sp.Cost {
		pm.Error = "not enough gold"
		return pm
	}
	p := sim.Point{X: x, Y: y}
	if !s.world.InBounds(p) {
		pm.Error = "out of bounds"
		return pm
	}
	tile := s.world.At(p)
	// open ground, or buried in mineable rock for a dwarf to dig toward
	if !s.world.Passable(tile) && !s.world.Mineable(tile) {
		pm.Error = "can't plant there"
		return pm
	}
	if s.world.Spawn(name, p) == nil {
		pm.Error = "can't plant there"
		return pm
	}
	s.world.Gold -= sp.Cost
	s.pending = append(s.pending, sim.Event{
		Tick: s.world.Tick, Type: "placed",
		Msg: fmt.Sprintf("%s planted a %s", s.players[token].Name, sp.Name),
	})
	return pm
}
