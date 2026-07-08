package data

import (
	"path/filepath"
	"runtime"
	"testing"
)

func dataDir(t *testing.T) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "data")
}

func TestLoadRealData(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sim.TickRate != 2.0 {
		t.Errorf("tick_rate = %v, want 2.0", cfg.Sim.TickRate)
	}
	r, ok := cfg.Species["rabbit"]
	if !ok {
		t.Fatal("no rabbit species")
	}
	if r.Kind != "fauna" || r.ID != "rabbit" || len(r.Eats) != 2 {
		t.Errorf("rabbit mis-parsed: %+v", r)
	}
	if cfg.Gen.Width != 64 || len(cfg.Gen.Scatter) == 0 {
		t.Errorf("gen mis-parsed: %+v", cfg.Gen)
	}
}

func TestValidationRejectsUnknownResource(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Species["rabbit"].Eats = append(cfg.Species["rabbit"].Eats, "plutonium")
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unknown eaten resource")
	}
}

func TestValidationRejectsBadFauna(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Species["wolf"].StomachSize = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for zero stomach_size")
	}
}
