package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/entities", s.handleEntities)
	mux.HandleFunc("GET /api/entities/{id}", s.handleEntity)
	mux.HandleFunc("POST /api/advance", s.handleAdvance)
}

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	json.NewEncoder(rw).Encode(v)
}

type stateResp struct {
	Tick      int64          `json:"tick"`
	TimeScale int            `json:"timeScale"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Pops      map[string]int `json:"pops"`
	Entities  int            `json:"entities"`
	Gold      int            `json:"gold"`
}

func (s *Server) handleState(rw http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	resp := stateResp{
		Tick: s.world.Tick, TimeScale: int(s.scale.Load()),
		Width: s.world.Width, Height: s.world.Height,
		Pops: map[string]int{}, Entities: len(s.world.Entities),
		Gold: s.world.Gold,
	}
	for sid, sp := range s.world.Cfg().Species {
		if sp.Kind == "fauna" {
			resp.Pops[sid] = s.world.CountAlive(sid)
		}
	}
	s.mu.Unlock()
	writeJSON(rw, http.StatusOK, resp)
}

// handleAdvance is a dev tool: it fast-forwards the world so slow
// hours-scale behavior can be tested without waiting.
func (s *Server) handleAdvance(rw http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("ticks"))
	if err != nil || n < 1 {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "ticks must be a positive integer"})
		return
	}
	if n > 1000000 {
		n = 1000000
	}
	s.mu.Lock()
	for i := 0; i < n; i++ {
		s.world.Step()
	}
	// the snapshot below carries the full state; drop pending diffs
	s.world.DirtyAndReset()
	s.world.TerrainDirtyAndReset()
	snap, merr := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners()))
	tick := s.world.Tick
	s.mu.Unlock()
	if merr == nil {
		s.hub.Broadcast(snap)
	}
	writeJSON(rw, http.StatusOK, map[string]any{"advanced": n, "tick": tick})
}

func (s *Server) handleEntities(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	species := q.Get("species")
	alive := q.Get("alive")
	s.mu.Lock()
	owners := s.owners()
	views := []EntityView{}
	for _, id := range s.world.SortedIDs() {
		e := s.world.Entities[id]
		if species != "" && e.Species != species {
			continue
		}
		if alive == "true" && e.Dead || alive == "false" && !e.Dead {
			continue
		}
		v := ViewOf(e)
		v.Owner = owners[e.ID]
		views = append(views, v)
	}
	s.mu.Unlock()
	writeJSON(rw, http.StatusOK, views)
}

func (s *Server) handleEntity(rw http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	s.mu.Lock()
	e, ok := s.world.Entities[id]
	var view EntityView
	if ok {
		view = ViewOf(e)
		view.Owner = s.owners()[e.ID]
	}
	s.mu.Unlock()
	if !ok {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(rw, http.StatusOK, view)
}
