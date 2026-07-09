package server

import (
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
}

func ViewOf(e *sim.Entity) EntityView {
	res := map[string]float64{}
	for _, p := range e.Produces {
		res[p.Resource] = p.Amount
	}
	return EntityView{
		ID: e.ID, S: e.Species, X: e.Pos.X, Y: e.Pos.Y,
		Dead: e.Dead, Full: e.Fullness, Action: e.Action, Home: e.Home, Res: res,
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
}

type TickMsg struct {
	Type      string         `json:"type"`
	Tick      int64          `json:"tick"`
	TimeScale int            `json:"timeScale"`
	Changed   []EntityView   `json:"changed"`
	Removed   []int          `json:"removed"`
	Events    []sim.Event    `json:"events"`
	Pops      map[string]int `json:"pops"`
}

type ClientMsg struct {
	Type  string `json:"type"`
	Scale int    `json:"scale"`
}

func BuildSnapshot(w *sim.World, scale int) SnapshotMsg {
	terrain := make([]uint8, len(w.Terrain))
	for i, t := range w.Terrain {
		terrain[i] = uint8(t)
	}
	ents := make([]EntityView, 0, len(w.Entities))
	for _, id := range w.SortedIDs() {
		ents = append(ents, ViewOf(w.Entities[id]))
	}
	return SnapshotMsg{
		Type: "snapshot", Tick: w.Tick,
		Width: w.Width, Height: w.Height,
		Terrain: terrain, Species: w.Cfg().Species,
		Entities: ents, TimeScale: scale,
	}
}

func BuildTick(w *sim.World, events []sim.Event, scale int) TickMsg {
	changed := []EntityView{}
	for _, id := range w.DirtyAndReset() {
		if e, ok := w.Entities[id]; ok {
			changed = append(changed, ViewOf(e))
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
	removed := append([]int{}, w.Removed...)
	return TickMsg{
		Type: "tick", Tick: w.Tick, TimeScale: scale,
		Changed: changed, Removed: removed, Events: events, Pops: pops,
	}
}
