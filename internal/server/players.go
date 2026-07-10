package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"cellarfloor/internal/sim"
)

// Player ties an anonymous browser token to a dwarf. Ownership is a server
// concern; the sim engine knows nothing about players.
type Player struct {
	Name    string `json:"name"`
	DwarfID int    `json:"dwarfId"`
}

type PlayerMsg struct {
	Type    string `json:"type"`
	State   string `json:"state"` // none | alive | dead
	DwarfID int    `json:"dwarfId,omitempty"`
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
}

// playerMsg reports the current state for a token. Caller holds s.mu.
func (s *Server) playerMsg(token string) PlayerMsg {
	p, ok := s.players[token]
	if !ok {
		return PlayerMsg{Type: "player", State: "none"}
	}
	if e, exists := s.world.Entities[p.DwarfID]; exists && !e.Dead {
		return PlayerMsg{Type: "player", State: "alive", DwarfID: p.DwarfID, Name: p.Name}
	}
	return PlayerMsg{Type: "player", State: "dead", Name: p.Name}
}

// spawnDwarf spawns a dwarf for the token unless one is already alive.
// Caller holds s.mu.
func (s *Server) spawnDwarf(token, name string) PlayerMsg {
	if cur := s.playerMsg(token); cur.State == "alive" {
		return cur
	}
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 24 {
		name = string(r[:24])
	}
	if name == "" {
		pm := s.playerMsg(token)
		pm.Error = "name required"
		return pm
	}
	if s.world.CountAlive("dwarf") >= s.cfg.Species["dwarf"].PopCap {
		pm := s.playerMsg(token)
		pm.Error = "the cellar is crowded"
		return pm
	}
	pos, ok := s.freeDirtTile()
	if !ok {
		pm := s.playerMsg(token)
		pm.Error = "no room in the clearing"
		return pm
	}
	e := s.world.Spawn("dwarf", pos)
	s.players[token] = &Player{Name: name, DwarfID: e.ID}
	return PlayerMsg{Type: "player", State: "alive", DwarfID: e.ID, Name: name}
}

// owners maps dwarf entity id to owning player name. Caller holds s.mu.
func (s *Server) owners() map[int]string {
	m := make(map[int]string, len(s.players))
	for _, p := range s.players {
		m[p.DwarfID] = p.Name
	}
	return m
}

// freeDirtTile picks a random unoccupied dirt tile (the clearing).
// Caller holds s.mu.
func (s *Server) freeDirtTile() (sim.Point, bool) {
	w := s.world
	var cands []sim.Point
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			p := sim.Point{X: x, Y: y}
			if w.At(p) == sim.TerrainDirt && w.FaunaAt(p) == nil {
				cands = append(cands, p)
			}
		}
	}
	if len(cands) == 0 {
		return sim.Point{}, false
	}
	return cands[w.RandN(len(cands))], true
}

func playersPath(savePath string) string {
	return filepath.Join(filepath.Dir(savePath), "players.json")
}

func SavePlayers(players map[string]*Player, path string) error {
	b, err := json.Marshal(players)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadPlayers(path string) (map[string]*Player, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]*Player{}, nil
	}
	if err != nil {
		return nil, err
	}
	players := map[string]*Player{}
	if err := json.Unmarshal(b, &players); err != nil {
		return nil, err
	}
	return players, nil
}
