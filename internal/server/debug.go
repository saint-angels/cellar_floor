package server

import (
	"fmt"

	"cellarfloor/internal/sim"
)

// debugAction applies an admin-only tweak to live progression state so the
// debug menu can exercise leveling and upgrades without waiting for mining.
// Caller holds s.mu.
func (s *Server) debugAction(m ClientMsg) {
	w := s.world
	switch m.Action {
	case "clear":
		// sweep every living entity of a type from the world (stale food,
		// debug spawns); the removals surface as one summary event
		if n := w.ClearType(m.Name); n > 0 {
			s.pending = append(s.pending, sim.Event{
				Tick: w.Tick, Type: "removed",
				Msg: fmt.Sprintf("debug cleared %d %s", n, m.Name),
			})
		}
	case "gold":
		if w.Gold += m.N; w.Gold < 0 {
			w.Gold = 0
		}
	case "level":
		// fill the bar; levelStep levels up and draws on the next tick
		if t := w.NextLevelGold(); t < 1<<29 && w.GoldMined < t {
			w.GoldMined = t
		}
	case "claims":
		for _, u := range s.cfg.Upgrades {
			if u.Name != m.Name {
				continue
			}
			if w.Claims == nil {
				w.Claims = map[string]int{}
			}
			if c := w.Claims[m.Name] + m.N; c > 0 {
				w.Claims[m.Name] = c
			} else {
				delete(w.Claims, m.Name)
			}
		}
	}
}
