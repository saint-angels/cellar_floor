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

func (s *Server) handleEntities(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	species := q.Get("species")
	alive := q.Get("alive")
	s.mu.Lock()
	views := []EntityView{}
	for _, id := range s.world.SortedIDs() {
		e := s.world.Entities[id]
		if species != "" && e.Species != species {
			continue
		}
		if alive == "true" && e.Dead || alive == "false" && !e.Dead {
			continue
		}
		views = append(views, ViewOf(e))
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
	}
	s.mu.Unlock()
	if !ok {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(rw, http.StatusOK, view)
}
