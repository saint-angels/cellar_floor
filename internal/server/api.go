package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleState)
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
}

func (s *Server) handleState(rw http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	resp := stateResp{
		Tick: s.world.Tick, TimeScale: int(s.scale.Load()),
		Width: s.world.Width, Height: s.world.Height,
		Pops: map[string]int{}, Entities: len(s.world.Entities),
	}
	for sid, sp := range s.world.Cfg().Species {
		if sp.Kind == "fauna" {
			resp.Pops[sid] = s.world.CountAlive(sid)
		}
	}
	s.mu.Unlock()
	writeJSON(rw, http.StatusOK, resp)
}
