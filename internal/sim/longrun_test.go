package sim_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
)

func loadCfg(t *testing.T) *data.Config {
	_, f, _, _ := runtime.Caller(0)
	cfg, err := data.Load(filepath.Join(filepath.Dir(f), "testdata", "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestFiftyThousandTickStability(t *testing.T) {
	if testing.Short() {
		t.Skip("long run")
	}
	cfg := loadCfg(t)
	w := gen.Generate(2026, cfg)
	guardrailSpawns := 0
	popSum := map[string]int{}
	samples := 0
	for i := 0; i < 50000; i++ {
		for _, ev := range w.Step() {
			if ev.Type == "spawned" {
				guardrailSpawns++
			}
		}
		if i%100 == 0 {
			for sid, s := range cfg.Types {
				if s.Kind == "fauna" {
					popSum[sid] += w.CountAlive(sid)
				}
			}
			samples++
		}
	}
	if guardrailSpawns > 200 {
		t.Errorf("ecology leans on guardrails: %d floor spawns in 50k ticks", guardrailSpawns)
	}
	for sid, s := range cfg.Types {
		if s.Kind != "fauna" {
			continue
		}
		avg := float64(popSum[sid]) / float64(samples)
		t.Logf("%s avg population %.1f (floor %d cap %d)", sid, avg, s.PopFloor, s.PopCap)
		if avg < float64(s.PopFloor) {
			t.Errorf("%s average %.1f below floor %d", sid, avg, s.PopFloor)
		}
		if n := w.CountAlive(sid); n > s.PopCap {
			t.Errorf("%s final population %d exceeds cap %d", sid, n, s.PopCap)
		}
	}
}

func TestWorldDeterminism(t *testing.T) {
	cfg := loadCfg(t)
	a, b := gen.Generate(99, cfg), gen.Generate(99, cfg)
	for i := 0; i < 2000; i++ {
		a.Step()
		b.Step()
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("same seed diverged after 2000 ticks")
	}
}
