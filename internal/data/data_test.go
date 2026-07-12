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
	if d.Kind != "fauna" || d.ID != "dwarf" || len(d.Eats) != 1 || d.MineDamage != 1 {
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
	write("terrain.toml", minimalTerrainTOML)
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
mine_damage = 1
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if d.MineDamage != 1 {
		t.Errorf("mining fields: %d", d.MineDamage)
	}
	if cfg.Gen.ClearingRadius != 3 {
		t.Errorf("gen fields: %d", cfg.Gen.ClearingRadius)
	}
}

func minimalConfig() *Config {
	return &Config{
		Sim:     SimConfig{TickRate: 2},
		Gen:     GenConfig{Width: 8, Height: 8},
		Terrain: CanonicalTerrain(),
		Types: map[string]*EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []Produce{{Resource: "shroom", Amount: 6, Max: 6, Regrow: 0.001}}},
		},
	}
}

// minimalTerrainTOML is the canonical five-type terrain table used by
// temp-dir fixtures so their loaded configs pass terrain validation.
const minimalTerrainTOML = `
[[terrain]]
id = "grass"
color = "#3d5a36"
passable = true

[[terrain]]
id = "dirt"
color = "#6b5537"
passable = true

[[terrain]]
id = "water"
color = "#2b4a63"

[[terrain]]
id = "rock"
color = "#3a3a3a"
mineable = true
hit_points = 172800

[[terrain]]
id = "floor"
color = "#26221e"
passable = true
`

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
	write("terrain.toml", minimalTerrainTOML)
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
mine_damage = 1

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
	if d.StarveTicks != 345600 || d.DecayTicks != 172800 {
		t.Errorf("hour fields: starve %d decay %d", d.StarveTicks, d.DecayTicks)
	}
	if d.MineDamage != 1 {
		t.Errorf("mine_damage = %d, want 1", d.MineDamage)
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

func TestSocialFieldsConvert(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if d.SocialSize != 10 || d.SocialThreshold != 4 || d.SocialRadius != 3 {
		t.Fatalf("social basics: %v %v %d", d.SocialSize, d.SocialThreshold, d.SocialRadius)
	}
	if want := 10.0 / (d.SocialDrainDays * 86400 * 2); d.SocialDrain != want {
		t.Errorf("drain = %v, want %v", d.SocialDrain, want)
	}
	if want := 10.0 / (d.SocialRefillHours * 3600 * 2); d.SocialRefill != want {
		t.Errorf("refill = %v, want %v", d.SocialRefill, want)
	}
	if m := cfg.Types["mushroom"]; m.SocialSize != 0 || m.SocialDrain != 0 {
		t.Errorf("mushroom must have no social: %v %v", m.SocialSize, m.SocialDrain)
	}
}

func TestSocialValidation(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["shroom"].SocialSize = 5
	if err := Validate(cfg); err == nil {
		t.Fatal("social_size without drain and refill times must fail")
	}
	cfg.Types["shroom"].SocialDrainDays = 2
	cfg.Types["shroom"].SocialRefillHours = 1
	cfg.Types["shroom"].SocialRadius = 3
	if err := Validate(cfg); err != nil {
		t.Fatalf("complete social block should validate: %v", err)
	}
}

func TestThoughtsParseAndValidate(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Types["dwarf"]
	if len(d.Thoughts) != 6 {
		t.Fatalf("dwarf thoughts = %d, want 6", len(d.Thoughts))
	}
	if d.Thoughts[0].When != "starving" || d.Thoughts[5].When != "always" {
		t.Fatalf("thought order wrong: first %q last %q", d.Thoughts[0].When, d.Thoughts[5].When)
	}
	if m := cfg.Types["mushroom"]; len(m.Thoughts) != 0 {
		t.Fatal("mushroom should have no thoughts")
	}
}

func TestThoughtValidationRejectsBadRules(t *testing.T) {
	cfg := minimalConfig()
	cfg.Types["shroom"].Thoughts = []Thought{{When: "moody", Text: "hmm"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("unknown thought condition must fail validation")
	}
	cfg.Types["shroom"].Thoughts = []Thought{{When: "always", Text: ""}}
	if err := Validate(cfg); err == nil {
		t.Fatal("empty thought text must fail validation")
	}
	cfg.Types["shroom"].Thoughts = []Thought{{When: "always", Text: "just a mushroom"}}
	if err := Validate(cfg); err != nil {
		t.Fatalf("valid thought rejected: %v", err)
	}
}

func TestTerrainTableParses(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Terrain) != 7 {
		t.Fatalf("terrain types = %d, want 7", len(cfg.Terrain))
	}
	want := []string{"grass", "dirt", "water", "rock", "floor", "soft_rock"}
	for i, id := range want {
		if cfg.Terrain[i].ID != id {
			t.Fatalf("terrain[%d] = %q, want %q", i, cfg.Terrain[i].ID, id)
		}
	}
	soft := cfg.Terrain[5]
	if !soft.Mineable || soft.Passable || soft.HitPoints != 43200 || soft.Color != "#575049" {
		t.Fatalf("soft_rock wrong: %+v", soft)
	}
	if i, ok := cfg.TerrainIndex("soft_rock"); !ok || i != 5 {
		t.Fatalf("TerrainIndex soft_rock = %d %v", i, ok)
	}
	if len(cfg.Gen.Veins) != 1 || cfg.Gen.Veins[0].Terrain != "soft_rock" ||
		cfg.Gen.Veins[0].Seeds != 10 || cfg.Gen.Veins[0].Size != 14 {
		t.Fatalf("veins wrong: %+v", cfg.Gen.Veins)
	}
}

func TestHitPointsAndDamageParse(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if hp := cfg.Terrain[3].HitPoints; hp != 172800 {
		t.Fatalf("rock hp = %d, want 172800", hp)
	}
	if hp := cfg.Terrain[5].HitPoints; hp != 43200 {
		t.Fatalf("soft rock hp = %d, want 43200", hp)
	}
	if d := cfg.Types["dwarf"].MineDamage; d != 1 {
		t.Fatalf("dwarf mine_damage = %d, want 1", d)
	}
}

func TestMineableNeedsHitPoints(t *testing.T) {
	cfg := minimalConfig()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("mineable without positive hit_points must fail")
	}
	cfg.Terrain[len(cfg.Terrain)-1].HitPoints = 100
	if err := Validate(cfg); err != nil {
		t.Fatalf("mineable with hp should validate: %v", err)
	}
}

func TestTerrainTableValidation(t *testing.T) {
	base := func() *Config {
		cfg := minimalConfig()
		cfg.Terrain = CanonicalTerrain()
		return cfg
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("canonical table should validate: %v", err)
	}
	cfg := base()
	cfg.Terrain[0], cfg.Terrain[1] = cfg.Terrain[1], cfg.Terrain[0]
	if err := Validate(cfg); err == nil {
		t.Fatal("reordered canonical terrain must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "rock", Color: "#111"})
	if err := Validate(cfg); err == nil {
		t.Fatal("duplicate id must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Mineable: true, HitPoints: 100})
	if err := Validate(cfg); err == nil {
		t.Fatal("missing color must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("mineable without positive hit_points must fail")
	}
	cfg = base()
	cfg.Terrain = append(cfg.Terrain, TerrainType{ID: "ore", Color: "#111", Mineable: true, HitPoints: 100, Passable: true})
	if err := Validate(cfg); err == nil {
		t.Fatal("passable and mineable together must fail")
	}
	cfg = base()
	cfg.Gen.Veins = []VeinRule{{Terrain: "unobtanium", Seeds: 1, Size: 2}}
	if err := Validate(cfg); err == nil {
		t.Fatal("vein referencing unknown terrain must fail")
	}
}

func TestMoldTerrainParses(t *testing.T) {
	cfg, err := Load(dataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Terrain) != 7 || cfg.Terrain[6].ID != "mold" {
		t.Fatalf("terrain[6] should be mold, got %+v", cfg.Terrain)
	}
	m := cfg.Terrain[6]
	if !m.Mineable || m.HitPoints != 6 || m.GoldChance != 0.1 || m.SpreadMinutes != 20 {
		t.Fatalf("mold wrong: %+v", m)
	}
	if want := 1.0 / (20 * 60 * 2); m.SpreadChance != want {
		t.Fatalf("spread chance = %v, want %v", m.SpreadChance, want)
	}
	if cfg.Terrain[3].GoldChance != 0.9 || cfg.Terrain[5].GoldChance != 0.9 {
		t.Fatalf("rock/soft gold_chance: %v %v", cfg.Terrain[3].GoldChance, cfg.Terrain[5].GoldChance)
	}
	if cfg.Gen.Crust != "mold" || cfg.Gen.CrustChance != 1.0 {
		t.Fatalf("crust: %q %v", cfg.Gen.Crust, cfg.Gen.CrustChance)
	}
}

func TestTerrainGoldAndSpreadValidation(t *testing.T) {
	cfg := minimalConfig()
	cfg.Terrain = CanonicalTerrain()
	cfg.Terrain[3].GoldChance = 1.5
	if err := Validate(cfg); err == nil {
		t.Fatal("gold_chance above 1 must fail")
	}
	cfg = minimalConfig()
	cfg.Terrain = append(CanonicalTerrain(), TerrainType{ID: "goo", Color: "#111", Mineable: true, HitPoints: 6, SpreadMinutes: -1})
	if err := Validate(cfg); err == nil {
		t.Fatal("negative spread_minutes must fail")
	}
	cfg = minimalConfig()
	cfg.Gen.Crust = "unobtanium"
	cfg.Gen.CrustChance = 1
	if err := Validate(cfg); err == nil {
		t.Fatal("crust referencing unknown terrain must fail")
	}
}
