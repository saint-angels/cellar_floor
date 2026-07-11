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
	write("gen.toml", "width = 8\nheight = 8\nclearing_radius = 3\ngold_chance = 0.01\nscatter = []\n")
	write("entities.toml", `
[type.shroom]
name = "Shroom"
kind = "flora"
color = "#fff"
produces = [{ resource = "shroom", amount = 6, max = 6, regrow = 0.001 }]

[type.dwarf]
name = "Dwarf"
kind = "fauna"
color = "#d9a066"
eats = ["shroom"]
bite_size = 2.0
stomach_size = 10.0
hunger_threshold = 4.0
metabolism = 0.0001
starve_ticks = 1000
speed = 0.5
lifespan = 100000
pop_cap = 10
decay_ticks = 100
mine_ticks = 500
gold_sense = 8
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if d.MineTicks != 500 || d.GoldSense != 8 {
		t.Errorf("mining fields: %d %d", d.MineTicks, d.GoldSense)
	}
	if cfg.Gen.ClearingRadius != 3 || cfg.Gen.GoldChance != 0.01 {
		t.Errorf("gen fields: %d %v", cfg.Gen.ClearingRadius, cfg.Gen.GoldChance)
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
