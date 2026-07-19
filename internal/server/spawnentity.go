package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// spawnEntity drops one entity of the given type on a tile for debugging. It
// follows the open debug menu (gold, level, claims): no admin gate, since this
// server keeps only reset admin-gated. Returns a log event when it placed one,
// nil when the type or tile was rejected. Caller holds s.mu.
func (s *Server) spawnEntity(name string, x, y int) *sim.Event {
	if _, ok := s.cfg.Types[name]; !ok {
		return nil
	}
	p := sim.Point{X: x, Y: y}
	if !s.world.InBounds(p) {
		return nil
	}
	// food (flora) may be buried in mineable rock so a hungry dwarf digs to
	// reach it; fauna and structures still need open ground to stand on
	tile := s.world.At(p)
	buryable := s.cfg.Types[name].Kind == "flora" && s.world.Mineable(tile)
	if !s.world.Passable(tile) && !buryable {
		return nil
	}
	// structures cannot stack on another structure, matching torch placement
	if s.cfg.Types[name].Kind == "structure" {
		for _, id := range s.world.SortedIDs() {
			e := s.world.Entities[id]
			if !e.Dead && e.Pos == p && s.cfg.Types[e.Type].Kind == "structure" {
				return nil
			}
		}
	}
	if s.world.Spawn(name, p) == nil {
		return nil
	}
	return &sim.Event{
		Tick: s.world.Tick, Type: "placed",
		Msg: fmt.Sprintf("debug spawned a %s", s.cfg.Types[name].Name),
	}
}
