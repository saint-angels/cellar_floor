package data

import (
	"os"
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
	d, ok := cfg.Types["dwarf"]
	if !ok {
		t.Fatal("no dwarf type")
	}
	if d.Kind != "fauna" || d.ID != "dwarf" || len(d.Eats) != 1 || d.MineTicks != 172800 {
		t.Errorf("dwarf mis-parsed: %+v", d)
	}
	if cfg.Gen.Width != 64 || len(cfg.Gen.Scatter) == 0 {
		t.Errorf("gen mis-parsed: %+v", cfg.Gen)
	}
}

func TestMiningFieldsParse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sim.toml", "tick_rate = 2.0\nautosave_minutes = 0\nsave_path = \"w.json\"\n")
	write("gen.toml", "width = 8\nheight = 8\nclearing_radius = 3\nscatter = []\n")
	write("entities.toml", `
[type.shroom]
name = "Shroom"
kind = "flora"
color = "#fff"
produces = [{ resource = "shroom", amount = 6, max = 6, regrow_days = 0.034722222222222224 }]

[type.dwarf]
name = "Dwarf"
kind = "fauna"
color = "#d9a066"
eats = ["shroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
stomach_drain_hours = 13.88888888888889
starve_hours = 0.1388888888888889
cells_per_second = 1.0
lifespan_days = 0.5787037037037037
pop_cap = 10
decay_hours = 0.013888888888888888
mine_hours = 0.06944444444444445
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if d.MineTicks != 500 {
		t.Errorf("mining fields: %d", d.MineTicks)
	}
	if cfg.Gen.ClearingRadius != 3 {
		t.Errorf("gen fields: %d", cfg.Gen.ClearingRadius)
	}
}

func minimalConfig() *Config {
	return &Config{
		Sim: SimConfig{TickRate: 2},
		Gen: GenConfig{Width: 8, Height: 8},
		Types: map[string]*EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.001}}},
		},
	}
}

func TestStructureKindValidates(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["torch"] = &EntityType{ID: "torch", Name: "Torch", Kind: "structure",
		Color: "#ffb347", LightRadius: 5, Lifespan: 100, DecayTicks: 10}
	if err := Validate(cfg); err != nil {
		t.Fatalf("structure should validate: %v", err)
	}
	cfg.Types["torch"].LightRadius = -1
	if err := Validate(cfg); err == nil {
		t.Fatal("negative light_radius must fail validation")
	}
}

func TestValidationRejectsUnknownResource(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Types["dwarf"].Eats = append(cfg.Types["dwarf"].Eats, "plutonium")
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unknown eaten resource")
	}
}

func TestValidationRejectsBadFauna(t *testing.T) {
	cfg, _ := Load(dataDir(t))
	cfg.Types["dwarf"].StomachSize = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for zero stomach_size")
	}
}

func TestUnitFieldsConvertToTicks(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sim.toml", "tick_rate = 2.0\nautosave_minutes = 0\nsave_path = \"w.json\"\n")
	write("gen.toml", "width = 8\nheight = 8\nclearing_radius = 3\nscatter = []\n")
	write("entities.toml", `
[type.shroom]
name = "Shroom"
kind = "flora"
color = "#fff"
produces = [{ resource = "shroom", amount = 6, max = 6, regrow_days = 1.75 }]

[type.digger]
name = "Digger"
kind = "fauna"
color = "#fff"
eats = ["shroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
stomach_drain_hours = 24
starve_hours = 48
cells_per_second = 1.0
lifespan_days = 58
mature_days = 6
pop_cap = 10
decay_hours = 24
mine_hours = 24

[type.lamp]
name = "Lamp"
kind = "structure"
color = "#fff"
light_radius = 5
lifespan_days = 1
decay_hours = 0.5
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["digger"]
	if d.MineTicks != 172800 || d.StarveTicks != 345600 || d.DecayTicks != 172800 {
		t.Errorf("hour fields: mine %d starve %d decay %d", d.MineTicks, d.StarveTicks, d.DecayTicks)
	}
	if d.Lifespan != 10022400 || d.MatureAge != 1036800 {
		t.Errorf("day fields: lifespan %d mature %d", d.Lifespan, d.MatureAge)
	}
	if want := 10.0 / 172800; d.Metabolism != want {
		t.Errorf("metabolism = %v, want %v", d.Metabolism, want)
	}
	if d.Speed != 0.5 {
		t.Errorf("speed = %v, want 0.5", d.Speed)
	}
	sh := cfg.Types["shroom"]
	if want := 6.0 / 302400; sh.Produces[0].Regrow != want {
		t.Errorf("regrow = %v, want %v", sh.Produces[0].Regrow, want)
	}
	lamp := cfg.Types["lamp"]
	if lamp.Lifespan != 172800 || lamp.DecayTicks != 3600 {
		t.Errorf("lamp: lifespan %d decay %d", lamp.Lifespan, lamp.DecayTicks)
	}
}

func TestNegativeUnitFieldRejected(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["shroom"].StarveHours = -1
	if err := Validate(cfg); err == nil {
		t.Fatal("negative starve_hours must fail validation")
	}
}

// The legacy fixture keeps the pre-pivot rabbit/wolf balance; its unit
// fields must reproduce the exact historical tick values or the long-run
// regression drifts.
func TestLegacyFixtureKeepsHistoricalTicks(t *testing.T) {
	cfg, err := Load("../sim/testdata/legacy")
	if err != nil {
		t.Fatal(err)
	}
	r, w := cfg.Types["rabbit"], cfg.Types["wolf"]
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"rabbit starve", r.StarveTicks, 600},
		{"rabbit lifespan", r.Lifespan, 8000},
		{"rabbit mature", r.MatureAge, 800},
		{"rabbit decay", r.DecayTicks, 400},
		{"wolf starve", w.StarveTicks, 1400},
		{"wolf lifespan", w.Lifespan, 10000},
		{"wolf mature", w.MatureAge, 1000},
		{"wolf decay", w.DecayTicks, 400},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if r.Speed != 0.5 || w.Speed != 0.6 {
		t.Errorf("speeds %v %v, want 0.5 0.6", r.Speed, w.Speed)
	}
	const eps = 1e-9
	near := func(got, want float64) bool { d := got - want; return d < eps && d > -eps }
	if !near(r.Metabolism, 0.02) || !near(w.Metabolism, 0.012) {
		t.Errorf("metabolisms %v %v, want ~0.02 ~0.012", r.Metabolism, w.Metabolism)
	}
	grass := cfg.Types["grass"].Produces[0]
	bush := cfg.Types["bush"].Produces[0]
	if !near(grass.Regrow, 0.09) || !near(bush.Regrow, 0.01) {
		t.Errorf("regrows %v %v, want ~0.09 ~0.01", grass.Regrow, bush.Regrow)
	}
	if tree := cfg.Types["tree"].Produces[0]; tree.Regrow != 0 {
		t.Errorf("tree regrow = %v, want 0", tree.Regrow)
	}
}
