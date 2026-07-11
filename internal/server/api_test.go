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
	w.Spawn("dwarf", sim.Point{X: 30, Y: 32})
	w.Spawn("dwarf", sim.Point{X: 34, Y: 32})
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
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
	if st.Pops["dwarf"] == 0 {
		t.Error("pops missing dwarf")
	}
	if _, ok := st.Pops["mushroom"]; ok {
		t.Error("pops must only contain fauna")
	}

	w.Gold = 9
	var st2 struct {
		Gold int `json:"gold"`
	}
	if err := json.Unmarshal(apiGet(t, mux, "/api/state").Body.Bytes(), &st2); err != nil {
		t.Fatal(err)
	}
	if st2.Gold != 9 {
		t.Errorf("gold = %d, want 9", st2.Gold)
	}
}

func TestAPIEntities(t *testing.T) {
	mux, w := newTestAPI(t)
	var deadID int
	for _, id := range w.SortedIDs() {
		if w.Entities[id].Type == "dwarf" {
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

	var dwarfs []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?type=dwarf").Body.Bytes(), &dwarfs)
	if len(dwarfs) == 0 || len(dwarfs) >= len(all) {
		t.Errorf("type filter broken: %d of %d", len(dwarfs), len(all))
	}
	for _, e := range dwarfs {
		if e.S != "dwarf" {
			t.Errorf("filter leaked type %q", e.S)
		}
	}

	var deadDwarfs []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?type=dwarf&alive=false").Body.Bytes(), &deadDwarfs)
	if len(deadDwarfs) != 1 || deadDwarfs[0].ID != deadID {
		t.Errorf("alive=false filter broken: %+v", deadDwarfs)
	}

	var aliveDwarfs []EntityView
	json.Unmarshal(apiGet(t, mux, "/api/entities?type=dwarf&alive=true").Body.Bytes(), &aliveDwarfs)
	if len(aliveDwarfs) != len(dwarfs)-1 {
		t.Errorf("alive=true filter broken: %d, want %d", len(aliveDwarfs), len(dwarfs)-1)
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

func TestAPIAdvance(t *testing.T) {
	mux, w := newTestAPI(t)
	before := w.Tick

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/advance?ticks=100", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var resp struct {
		Advanced int   `json:"advanced"`
		Tick     int64 `json:"tick"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Advanced != 100 || resp.Tick != before+100 || w.Tick != before+100 {
		t.Errorf("advance: %+v, world tick %d", resp, w.Tick)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/advance?ticks=0", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ticks=0: status %d, want 400", rec.Code)
	}
	if rec := apiGet(t, mux, "/api/advance?ticks=5"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET advance: status %d, want 405", rec.Code)
	}
}

func TestAPIEntityOwner(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	s := &Server{cfg: cfg, world: w, hub: NewHub(), players: map[string]*Player{}}
	s.scale.Store(1)
	pm := s.spawnDwarf("tok1", "Misha")
	mux := http.NewServeMux()
	s.registerAPI(mux)

	var e EntityView
	if err := json.Unmarshal(apiGet(t, mux, "/api/entities/"+strconv.Itoa(pm.DwarfID)).Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Owner != "Misha" {
		t.Errorf("owner = %q, want Misha", e.Owner)
	}
}
