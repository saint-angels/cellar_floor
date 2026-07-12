package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cellarfloor/internal/gen"
)

func TestAdminOK(t *testing.T) {
	s := newPlayerServer(t)
	if !s.adminOK("") || !s.adminOK("anything") {
		t.Fatal("a server without a token must allow everyone")
	}
	s.admin = "sekret"
	if s.adminOK("") || s.adminOK("wrong") {
		t.Fatal("a token server must reject missing or wrong tokens")
	}
	if !s.adminOK("sekret") {
		t.Fatal("a token server must accept the token")
	}
}

func TestAdvanceRequiresAdmin(t *testing.T) {
	cfg := loadCfg(t)
	s := &Server{cfg: cfg, world: gen.Generate(7, cfg), hub: NewHub(), admin: "sekret"}
	s.scale.Store(1)
	mux := http.NewServeMux()
	s.registerAPI(mux)

	post := func(path string, hdr map[string]string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post("/api/advance?ticks=5", nil); code != http.StatusForbidden {
		t.Fatalf("no token: got %d, want 403", code)
	}
	if code := post("/api/advance?ticks=5&admin=wrong", nil); code != http.StatusForbidden {
		t.Fatalf("wrong token: got %d, want 403", code)
	}
	if code := post("/api/advance?ticks=5&admin=sekret", nil); code != http.StatusOK {
		t.Fatalf("query token: got %d, want 200", code)
	}
	if code := post("/api/advance?ticks=5", map[string]string{"X-Admin-Token": "sekret"}); code != http.StatusOK {
		t.Fatalf("header token: got %d, want 200", code)
	}
}
