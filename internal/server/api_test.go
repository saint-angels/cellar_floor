package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestAPIEntities(t *testing.T) {
	mux, w := newTestAPI(t)
	var deadID int
	for _, id := range w.SortedIDs() {
		if w.Entities[id].Species == "rabbit" {
			w.Entities[id].Dead = true
			deadID = id
			break
		}
	}

	var all []EntityView
	if err := json.Unmarshal(apiGet(t, mux, "/api/entities").Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != len(w.Entities) {
		t.Errorf("got %d entities, want %d", len(all), len(w.Entities))
	}

	var rabbits []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?species=rabbit").Body.Bytes(), &rabbits)
	if len(rabbits) == 0 || len(rabbits) >= len(all) {
		t.Errorf("species filter broken: %d of %d", len(rabbits), len(all))
	}
	for _, e := range rabbits {
		if e.S != "rabbit" {
			t.Errorf("filter leaked species %q", e.S)
		}
	}

	var deadRabbits []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?species=rabbit&alive=false").Body.Bytes(), &deadRabbits)
	if len(deadRabbits) != 1 || deadRabbits[0].ID != deadID {
		t.Errorf("alive=false filter broken: %+v", deadRabbits)
	}

	var aliveRabbits []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?species=rabbit&alive=true").Body.Bytes(), &aliveRabbits)
	if len(aliveRabbits) != len(rabbits)-1 {
		t.Errorf("alive=true filter broken: %d, want %d", len(aliveRabbits), len(rabbits)-1)
	}
}

func TestAPIEntityByID(t *testing.T) {
	mux, w := newTestAPI(t)
	id := w.SortedIDs()[0]

	rec := apiGet(t, mux, "/api/entities/"+strconv.Itoa(id))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var e EntityView
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.ID != id {
		t.Errorf("got id %d, want %d", e.ID, id)
	}

	if rec := apiGet(t, mux, "/api/entities/999999"); rec.Code != http.StatusNotFound {
		t.Errorf("missing id: status %d, want 404", rec.Code)
	}
	if rec := apiGet(t, mux, "/api/entities/abc"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id: status %d, want 400", rec.Code)
	}
}
