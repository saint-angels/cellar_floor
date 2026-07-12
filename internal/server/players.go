package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"cellarfloor/internal/sim"
)

// firstSpawnGold is the colony gold a brand-new player brings along,
// enough for a few torches to direct their dwarf from the start.
const firstSpawnGold = 5

// Player ties an anonymous browser token to a dwarf. Ownership is a server
// concern; the sim engine knows nothing about players.
type Player struct {
	Name    string `json:"name"`
	DwarfID int    `json:"dwarfId"`
	// Seen* snapshot the world counters as of this player's last hello, so a
	// returning player can be told what changed while they were away.
	SeenTick   int64 `json:"seenTick"`
	SeenBlocks int   `json:"seenBlocks"`
	SeenGold   int   `json:"seenGold"`
	SeenMold   int   `json:"seenMold"`
}

type RecapMsg struct {
	Type   string `json:"type"`
	Ticks  int64  `json:"ticks"`
	Blocks int    `json:"blocks"`
	Gold   int    `json:"gold"`
	Mold   int    `json:"mold"`
}

// max0 clamps a delta at zero so a stale snapshot (for instance one left over
// from a world that was reset to lower counters) never reports negative.
func max0[T int | int64](v T) T {
	if v < 0 {
		return 0
	}
	return v
}

// recapFor builds the away-summary for a known token and advances its
// snapshot. Nil for unknown tokens. Caller holds s.mu.
func (s *Server) recapFor(token string) *RecapMsg {
	p, ok := s.players[token]
	if !ok {
		return nil
	}
	r := &RecapMsg{
		Type:   "recap",
		Ticks:  max0(s.world.Tick - p.SeenTick),
		Blocks: max0(s.world.BlocksMined - p.SeenBlocks),
		Gold:   max0(s.world.GoldMined - p.SeenGold),
		Mold:   max0(s.world.MoldGrown - p.SeenMold),
	}
	p.SeenTick = s.world.Tick
	p.SeenBlocks = s.world.BlocksMined
	p.SeenGold = s.world.GoldMined
	p.SeenMold = s.world.MoldGrown
	return r
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
	// entity ids restart when the world is reset, so the stored id can
	// collide with an unrelated entity; only a living dwarf counts
	if e, exists := s.world.Entities[p.DwarfID]; exists && !e.Dead && e.Type == "dwarf" {
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
	if s.world.CountAlive("dwarf") >= s.cfg.Types["dwarf"].PopCap {
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
	prev, returning := s.players[token]
	e := s.world.Spawn("dwarf", pos)
	np := &Player{Name: name, DwarfID: e.ID}
	if returning {
		// a respawn after death must not fake a recap: carry the old
		// snapshot forward so nothing looks earned during the away time
		np.SeenTick = prev.SeenTick
		np.SeenBlocks = prev.SeenBlocks
		np.SeenGold = prev.SeenGold
		np.SeenMold = prev.SeenMold
	} else {
		// a brand new token starts even with the world: its first recap is
		// empty, only genuine future absence counts
		np.SeenTick = s.world.Tick
		np.SeenBlocks = s.world.BlocksMined
		np.SeenGold = s.world.GoldMined
		np.SeenMold = s.world.MoldGrown
	}
	s.players[token] = np
	// the purse arrives with a player's first spawn in each world: brand
	// new tokens, and returning players after a reset (which zeroes their
	// DwarfID). Death respawns keep the old id, so dying farms nothing.
	if !returning || prev.DwarfID == 0 {
		s.world.Gold += firstSpawnGold
	}
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
