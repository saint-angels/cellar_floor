package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

func newTestAPI(t *testing.T) (*http.ServeMux, *sim.World) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub()}
	s.scale.Store(1)
	mux := http.NewServeMux()
	s.registerAPI(mux)
	return mux, w
}

func apiGet(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestAPIState(t *testing.T) {
	mux, w := newTestAPI(t)
	rec := apiGet(t, mux, "/api/state")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type %q", ct)
	}
	var st struct {
		Tick      int64          `json:"tick"`
		TimeScale int            `json:"timeScale"`
		Width     int            `json:"width"`
		Height    int            `json:"height"`
		Pops      map[string]int `json:"pops"`
		Entities  int            `json:"entities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Width != w.Width || st.Height != w.Height || st.TimeScale != 1 {
		t.Errorf("bad dims/scale: %+v", st)
	}
	if st.Entities != len(w.Entities) {
		t.Errorf("entities %d, want %d", st.Entities, len(w.Entities))
	}
	if st.Pops["rabbit"] == 0 {
		t.Error("pops missing rabbit")
	}
	if _, ok := st.Pops["grass"]; ok {
		t.Error("pops must only contain fauna")
	}
}
