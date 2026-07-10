package server

import (
	"strings"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

type EntityView struct {
	ID     int                `json:"id"`
	S      string             `json:"s"`
	X      int                `json:"x"`
	Y      int                `json:"y"`
	Dead   bool               `json:"dead,omitempty"`
	Full   float64            `json:"full"`
	Action string             `json:"action,omitempty"`
	Home   *sim.Point         `json:"home,omitempty"`
	Res    map[string]float64 `json:"res,omitempty"`
	Owner  string             `json:"owner,omitempty"`
	MT     *sim.Point         `json:"mt,omitempty"`
}

func ViewOf(e *sim.Entity) EntityView {
	res := map[string]float64{}
	for _, p := range e.Produces {
		res[p.Resource] = p.Amount
	}
	return EntityView{
		ID: e.ID, S: e.Species, X: e.Pos.X, Y: e.Pos.Y,
		Dead: e.Dead, Full: e.Fullness, Action: e.Action, Home: e.Home, Res: res,
		MT: e.MineTarget,
	}
}

type SnapshotMsg struct {
	Type      string                   `json:"type"`
	Tick      int64                    `json:"tick"`
	Width     int                      `json:"width"`
	Height    int                      `json:"height"`
	Terrain   []uint8                  `json:"terrain"`
	Species   map[string]*data.Species `json:"species"`
	Entities  []EntityView             `json:"entities"`
	TimeScale int                      `json:"timeScale"`
	Gold      int                      `json:"gold"`
	Mining    map[int]float64          `json:"mining,omitempty"`
}

// TerrainDiff is one mutated cell in a tick message.
type TerrainDiff struct {
	I int   `json:"i"`
	T uint8 `json:"t"`
}

type TickMsg struct {
	Type      string          `json:"type"`
	Tick      int64           `json:"tick"`
	TimeScale int             `json:"timeScale"`
	Changed   []EntityView    `json:"changed"`
	Removed   []int           `json:"removed"`
	Events    []sim.Event     `json:"events"`
	Pops      map[string]int  `json:"pops"`
	Gold      int             `json:"gold"`
	Mining    map[int]float64 `json:"mining,omitempty"`
	Terrain   []TerrainDiff   `json:"terrain,omitempty"`
}

type ClientMsg struct {
	Type   string `json:"type"`
	Scale  int    `json:"scale"`
	Player string `json:"player"`
	Name   string `json:"name"`
}

func BuildSnapshot(w *sim.World, scale int, owners map[int]string) SnapshotMsg {
	terrain := make([]uint8, len(w.Terrain))
	for i, t := range w.Terrain {
		terrain[i] = uint8(t)
	}
	ents := make([]EntityView, 0, len(w.Entities))
	for _, id := range w.SortedIDs() {
		v := ViewOf(w.Entities[id])
		v.Owner = owners[id]
		ents = append(ents, v)
	}
	return SnapshotMsg{
		Type: "snapshot", Tick: w.Tick,
		Width: w.Width, Height: w.Height,
		Terrain: terrain, Species: w.Cfg().Species,
		Entities: ents, TimeScale: scale,
		Gold: w.Gold, Mining: w.MineProgress,
	}
}

func BuildTick(w *sim.World, events []sim.Event, scale int, owners map[int]string) TickMsg {
	changed := []EntityView{}
	for _, id := range w.DirtyAndReset() {
		if e, ok := w.Entities[id]; ok {
			v := ViewOf(e)
			v.Owner = owners[id]
			changed = append(changed, v)
		}
	}
	pops := map[string]int{}
	for sid, s := range w.Cfg().Species {
		if s.Kind == "fauna" {
			pops[sid] = w.CountAlive(sid)
		}
	}
	if events == nil {
		events = []sim.Event{}
	}
	// name owned actors in event messages: "Dwarf struck gold" becomes
	// "Misha's dwarf struck gold"
	decorated := make([]sim.Event, len(events))
	copy(decorated, events)
	for i := range decorated {
		name := owners[decorated[i].Actor]
		if name == "" {
			continue
		}
		if sp := w.Cfg().Species[decorated[i].ActorSpecies]; sp != nil {
			decorated[i].Msg = strings.Replace(decorated[i].Msg, sp.Name, name+"'s dwarf", 1)
		}
	}
	events = decorated
	removed := append([]int{}, w.Removed...)
	var tdiffs []TerrainDiff
	for _, i := range w.TerrainDirtyAndReset() {
		tdiffs = append(tdiffs, TerrainDiff{I: i, T: uint8(w.Terrain[i])})
	}
	return TickMsg{
		Type: "tick", Tick: w.Tick, TimeScale: scale,
		Changed: changed, Removed: removed, Events: events, Pops: pops,
		Gold: w.Gold, Mining: w.MineProgress, Terrain: tdiffs,
	}
}
