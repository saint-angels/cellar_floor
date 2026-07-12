package data

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Produce struct {
	Resource   string  `toml:"resource" json:"resource"`
	Amount     float64 `toml:"amount" json:"amount"`
	Max        float64 `toml:"max" json:"max"`
	Regrow     float64 `toml:"-" json:"regrow"`
	RegrowDays float64 `toml:"regrow_days" json:"-"`
}

type Desire struct {
	Resource string  `toml:"resource" json:"resource"`
	Amount   float64 `toml:"amount" json:"amount"`
	Aversion bool    `toml:"aversion" json:"aversion"`
}

// Thought is one bubble rule: the first list entry whose condition
// matches wins. Conditions come from a fixed vocabulary; text may use
// {gold} and {name} placeholders, substituted client-side.
type Thought struct {
	When string `toml:"when" json:"when"`
	Text string `toml:"text" json:"text"`
}

var validThoughtConditions = map[string]bool{
	"starving": true, "hungry": true, "lonely": true,
	"struck_gold": true, "seen_recently": true, "always": true,
}

type EntityType struct {
	ID                string    `toml:"-" json:"id"`
	Name              string    `toml:"name" json:"name"`
	Kind              string    `toml:"kind" json:"kind"`
	Color             string    `toml:"color" json:"color"`
	Produces          []Produce `toml:"produces" json:"produces"`
	Eats              []string  `toml:"eats" json:"eats"`
	Shelters          []string  `toml:"shelters" json:"shelters"`
	Desires           []Desire  `toml:"desires" json:"desires"`
	BiteSize          float64   `toml:"bite_size" json:"biteSize"`
	StomachSize       float64   `toml:"stomach_size" json:"stomachSize"`
	HungerThreshold   float64   `toml:"hunger_threshold" json:"hungerThreshold"`
	Metabolism        float64   `toml:"-" json:"metabolism"`
	StomachDrainHours float64   `toml:"stomach_drain_hours" json:"-"`
	StarveTicks       int       `toml:"-" json:"starveTicks"`
	StarveHours       float64   `toml:"starve_hours" json:"-"`
	FearRadius        int       `toml:"fear_radius" json:"fearRadius"`
	Speed             float64   `toml:"-" json:"speed"`
	CellsPerSecond    float64   `toml:"cells_per_second" json:"-"`
	WanderChance      float64   `toml:"wander_chance" json:"-"`
	HomeRange         int       `toml:"home_range" json:"homeRange"`
	Lifespan          int       `toml:"-" json:"lifespan"`
	LifespanDays      float64   `toml:"lifespan_days" json:"-"`
	MatureAge         int       `toml:"-" json:"matureAge"`
	MatureDays        float64   `toml:"mature_days" json:"-"`
	ReproThreshold    float64   `toml:"repro_threshold" json:"reproThreshold"`
	ReproChance       float64   `toml:"repro_chance" json:"reproChance"`
	ReproCost         float64   `toml:"repro_cost" json:"reproCost"`
	PopFloor          int       `toml:"pop_floor" json:"popFloor"`
	PopCap            int       `toml:"pop_cap" json:"popCap"`
	DecayTicks        int       `toml:"-" json:"decayTicks"`
	DecayHours        float64   `toml:"decay_hours" json:"-"`
	MineDamage        int       `toml:"mine_damage" json:"mineDamage"`
	SocialSize        float64   `toml:"social_size" json:"socialSize"`
	SocialThreshold   float64   `toml:"social_threshold" json:"socialThreshold"`
	SocialRadius      int       `toml:"social_radius" json:"-"`
	SocialDrainDays   float64   `toml:"social_drain_days" json:"-"`
	SocialRefillHours float64   `toml:"social_refill_hours" json:"-"`
	SocialDrain       float64   `toml:"-" json:"-"`
	SocialRefill      float64   `toml:"-" json:"-"`
	LightRadius       int       `toml:"light_radius" json:"lightRadius"`
	Thoughts          []Thought `toml:"thoughts" json:"thoughts,omitempty"`
}

type SimConfig struct {
	TickRate        float64 `toml:"tick_rate"`
	AutosaveMinutes int     `toml:"autosave_minutes"`
	SavePath        string  `toml:"save_path"`
	GoldMin         int     `toml:"gold_min"`
	GoldMax         int     `toml:"gold_max"`
}

type ScatterRule struct {
	Type    string  `toml:"type"`
	Terrain string  `toml:"terrain"`
	Chance  float64 `toml:"chance"`
}

type VeinRule struct {
	Terrain string `toml:"terrain"`
	Seeds   int    `toml:"seeds"`
	Size    int    `toml:"size"`
}

type GenConfig struct {
	Width          int           `toml:"width"`
	Height         int           `toml:"height"`
	NoiseScale     float64       `toml:"noise_scale"`
	NoiseOctaves   int           `toml:"noise_octaves"`
	WaterBelow     float64       `toml:"water_below"`
	DirtAbove      float64       `toml:"dirt_above"`
	RockAbove      float64       `toml:"rock_above"`
	ClearingRadius int           `toml:"clearing_radius"`
	Center         string        `toml:"center"`
	Crust          string        `toml:"crust"`
	CrustChance    float64       `toml:"crust_chance"`
	Scatter        []ScatterRule `toml:"scatter"`
	Veins          []VeinRule    `toml:"veins"`
}

type TerrainType struct {
	ID            string  `toml:"id" json:"id"`
	Color         string  `toml:"color" json:"color"`
	Passable      bool    `toml:"passable" json:"passable"`
	Mineable      bool    `toml:"mineable" json:"mineable"`
	HitPoints     int     `toml:"hit_points" json:"hitPoints"`
	GoldChance    float64 `toml:"gold_chance" json:"goldChance"`
	SpreadMinutes float64 `toml:"spread_minutes" json:"-"`
	SpreadChance  float64 `toml:"-" json:"-"`
	SproutMinutes float64 `toml:"sprout_minutes" json:"-"`
	SproutChance  float64 `toml:"-" json:"-"`
}

type Upgrade struct {
	Name     string `toml:"name" json:"name"`
	Kind     string `toml:"kind" json:"kind"`
	Amount   int    `toml:"amount" json:"amount"`
	Max      int    `toml:"max" json:"max"`
	Color    string `toml:"color" json:"color"`
	Radius   int    `toml:"radius" json:"radius"`
	PeriodMs int    `toml:"period_ms" json:"periodMs"`
}

type Config struct {
	Sim         SimConfig
	Gen         GenConfig
	Terrain     []TerrainType
	Types       map[string]*EntityType
	Upgrades    []Upgrade
	LevelBase   float64
	LevelGrowth float64
}

// CanonicalTerrain returns the five pinned base types in wire order.
func CanonicalTerrain() []TerrainType {
	return []TerrainType{
		{ID: "grass", Color: "#3d5a36", Passable: true},
		{ID: "dirt", Color: "#6b5537", Passable: true},
		{ID: "water", Color: "#2b4a63"},
		{ID: "rock", Color: "#3a3a3a", Mineable: true, HitPoints: 172800},
		{ID: "floor", Color: "#26221e", Passable: true},
	}
}

func (c *Config) TerrainIndex(id string) (int, bool) {
	for i, t := range c.Terrain {
		if t.ID == id {
			return i, true
		}
	}
	return 0, false
}

func Load(dir string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(filepath.Join(dir, "sim.toml"), &cfg.Sim); err != nil {
		return nil, fmt.Errorf("sim.toml: %w", err)
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "gen.toml"), &cfg.Gen); err != nil {
		return nil, fmt.Errorf("gen.toml: %w", err)
	}
	var tt struct {
		Terrain []TerrainType `toml:"terrain"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "terrain.toml"), &tt); err != nil {
		return nil, fmt.Errorf("terrain.toml: %w", err)
	}
	cfg.Terrain = tt.Terrain
	var up struct {
		LevelBase   float64   `toml:"level_base"`
		LevelGrowth float64   `toml:"level_growth"`
		Upgrade     []Upgrade `toml:"upgrade"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "upgrades.toml"), &up); err != nil {
		return nil, fmt.Errorf("upgrades.toml: %w", err)
	}
	cfg.Upgrades = up.Upgrade
	cfg.LevelBase = up.LevelBase
	cfg.LevelGrowth = up.LevelGrowth
	var et struct {
		Types map[string]*EntityType `toml:"type"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "entities.toml"), &et); err != nil {
		return nil, fmt.Errorf("entities.toml: %w", err)
	}
	cfg.Types = et.Types
	for id, t := range cfg.Types {
		t.ID = id
	}
	if cfg.Sim.TickRate <= 0 {
		return nil, fmt.Errorf("sim: tick_rate must be positive")
	}
	cfg.resolveTimes()
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolveTimes converts the unit-named data fields into internal ticks
// and per-tick rates using the sim tick rate.
func (c *Config) resolveTimes() {
	tr := c.Sim.TickRate
	hours := func(h float64) int { return int(math.Round(h * 3600 * tr)) }
	days := func(d float64) int { return int(math.Round(d * 86400 * tr)) }
	for _, t := range c.Types {
		t.StarveTicks = hours(t.StarveHours)
		t.DecayTicks = hours(t.DecayHours)
		t.Lifespan = days(t.LifespanDays)
		t.MatureAge = days(t.MatureDays)
		if t.StomachDrainHours > 0 {
			t.Metabolism = t.StomachSize / (t.StomachDrainHours * 3600 * tr)
		}
		if t.SocialSize > 0 {
			t.SocialDrain = t.SocialSize / (t.SocialDrainDays * 86400 * tr)
			t.SocialRefill = t.SocialSize / (t.SocialRefillHours * 3600 * tr)
		}
		t.Speed = t.CellsPerSecond / tr
		for i := range t.Produces {
			p := &t.Produces[i]
			if p.RegrowDays > 0 {
				p.Regrow = p.Max / (p.RegrowDays * 86400 * tr)
			}
		}
	}
	for i := range c.Terrain {
		tt := &c.Terrain[i]
		if tt.SpreadMinutes > 0 {
			tt.SpreadChance = 1.0 / (tt.SpreadMinutes * 60 * tr)
		}
		if tt.SproutMinutes > 0 {
			tt.SproutChance = 1.0 / (tt.SproutMinutes * 60 * tr)
		}
	}
}

func Validate(cfg *Config) error {
	canon := CanonicalTerrain()
	if len(cfg.Terrain) < len(canon) {
		return fmt.Errorf("terrain: table needs at least the %d canonical types", len(canon))
	}
	seen := map[string]bool{}
	for i, tt := range cfg.Terrain {
		if tt.ID == "" || tt.Color == "" {
			return fmt.Errorf("terrain[%d]: id and color are required", i)
		}
		if seen[tt.ID] {
			return fmt.Errorf("terrain: duplicate id %q", tt.ID)
		}
		seen[tt.ID] = true
		if i < len(canon) && tt.ID != canon[i].ID {
			return fmt.Errorf("terrain[%d] must be %q (saves store indices; append only), got %q", i, canon[i].ID, tt.ID)
		}
		if tt.Mineable && tt.HitPoints <= 0 {
			return fmt.Errorf("terrain %s: mineable needs positive hit_points", tt.ID)
		}
		if tt.Mineable && tt.Passable {
			return fmt.Errorf("terrain %s: cannot be both passable and mineable", tt.ID)
		}
		if tt.GoldChance < 0 || tt.GoldChance > 1 {
			return fmt.Errorf("terrain %s: gold_chance must be between 0 and 1", tt.ID)
		}
		if tt.SpreadMinutes < 0 {
			return fmt.Errorf("terrain %s: spread_minutes must be non-negative", tt.ID)
		}
		if tt.SproutMinutes < 0 {
			return fmt.Errorf("terrain %s: sprout_minutes must be non-negative", tt.ID)
		}
	}
	if len(cfg.Upgrades) > 0 {
		if cfg.LevelBase <= 0 {
			return fmt.Errorf("upgrades: level_base must be positive")
		}
		if cfg.LevelGrowth <= 1 {
			return fmt.Errorf("upgrades: level_growth must exceed 1")
		}
	}
	upNames := map[string]bool{}
	validKinds := map[string]bool{"damage": true, "luck": true, "weapon": true, "beam": true, "missile": true}
	for i, u := range cfg.Upgrades {
		if u.Name == "" {
			return fmt.Errorf("upgrade[%d]: name is required", i)
		}
		if upNames[u.Name] {
			return fmt.Errorf("upgrade: duplicate name %q", u.Name)
		}
		upNames[u.Name] = true
		if !validKinds[u.Kind] {
			return fmt.Errorf("upgrade %s: unknown kind %q", u.Name, u.Kind)
		}
		if u.Amount <= 0 {
			return fmt.Errorf("upgrade %s: amount must be positive", u.Name)
		}
		if u.Max < 0 {
			return fmt.Errorf("upgrade %s: max must be non-negative", u.Name)
		}
		if (u.Kind == "weapon" || u.Kind == "beam" || u.Kind == "missile") && (u.Color == "" || u.Radius <= 0 || u.PeriodMs <= 0) {
			return fmt.Errorf("upgrade %s: weapons, beams and missiles need color, radius, period_ms", u.Name)
		}
	}
	produced := map[string]bool{}
	for _, s := range cfg.Types {
		for _, p := range s.Produces {
			produced[p.Resource] = true
		}
	}
	for id, s := range cfg.Types {
		if s.Kind != "flora" && s.Kind != "fauna" && s.Kind != "structure" {
			return fmt.Errorf("type %s: kind must be flora, fauna or structure, got %q", id, s.Kind)
		}
		if s.Name == "" || s.Color == "" {
			return fmt.Errorf("type %s: name and color are required", id)
		}
		if s.LightRadius < 0 {
			return fmt.Errorf("type %s: light_radius must be non-negative", id)
		}
		if s.StarveHours < 0 || s.DecayHours < 0 ||
			s.LifespanDays < 0 || s.MatureDays < 0 || s.StomachDrainHours < 0 ||
			s.CellsPerSecond < 0 {
			return fmt.Errorf("type %s: time fields must be non-negative", id)
		}
		if s.WanderChance < 0 || s.WanderChance > 1 {
			return fmt.Errorf("type %s: wander_chance must be between 0 and 1", id)
		}
		if s.SocialSize < 0 || s.SocialThreshold < 0 || s.SocialRadius < 0 ||
			s.SocialDrainDays < 0 || s.SocialRefillHours < 0 {
			return fmt.Errorf("type %s: social fields must be non-negative", id)
		}
		if s.SocialSize > 0 && (s.SocialDrainDays <= 0 || s.SocialRefillHours <= 0 || s.SocialRadius <= 0) {
			return fmt.Errorf("type %s: social_size needs positive social_drain_days, social_refill_hours, social_radius", id)
		}
		for _, p := range s.Produces {
			if p.RegrowDays < 0 {
				return fmt.Errorf("type %s: regrow_days must be non-negative", id)
			}
		}
		for _, th := range s.Thoughts {
			if !validThoughtConditions[th.When] {
				return fmt.Errorf("type %s: unknown thought condition %q", id, th.When)
			}
			if th.Text == "" {
				return fmt.Errorf("type %s: thought %q needs text", id, th.When)
			}
		}
		if s.Kind == "structure" && s.Lifespan > 0 && s.DecayTicks <= 0 {
			return fmt.Errorf("type %s: a structure with a lifespan needs positive decay_ticks", id)
		}
		for _, r := range s.Eats {
			if !produced[r] {
				return fmt.Errorf("type %s eats %q which nothing produces", id, r)
			}
		}
		for _, r := range s.Shelters {
			if !produced[r] {
				return fmt.Errorf("type %s shelters in %q which nothing produces", id, r)
			}
		}
		if s.Kind == "fauna" {
			if s.StomachSize <= 0 || s.BiteSize <= 0 || s.Speed <= 0 ||
				s.Metabolism <= 0 || s.StarveTicks <= 0 || s.DecayTicks <= 0 ||
				s.Lifespan <= 0 || s.PopCap <= 0 {
				return fmt.Errorf("type %s: fauna requires positive stomach_size, bite_size, cells_per_second, stomach_drain_hours, starve_hours, decay_hours, lifespan_days, pop_cap", id)
			}
			if len(s.Eats) == 0 {
				return fmt.Errorf("type %s: fauna must eat something", id)
			}
			if s.MineDamage < 0 {
				return fmt.Errorf("type %s: mine_damage must be non-negative", id)
			}
		}
	}
	if cfg.Sim.TickRate <= 0 {
		return fmt.Errorf("sim: tick_rate must be positive")
	}
	anyGold := false
	for _, tt := range cfg.Terrain {
		if tt.GoldChance > 0 {
			anyGold = true
		}
	}
	if anyGold && (cfg.Sim.GoldMin < 1 || cfg.Sim.GoldMax < cfg.Sim.GoldMin) {
		return fmt.Errorf("sim: gold drop needs 1 <= gold_min <= gold_max")
	}
	if cfg.Gen.Width <= 0 || cfg.Gen.Height <= 0 {
		return fmt.Errorf("gen: width and height must be positive")
	}
	if cfg.Gen.Center != "" {
		if _, ok := cfg.Types[cfg.Gen.Center]; !ok {
			return fmt.Errorf("gen: center references unknown type %q", cfg.Gen.Center)
		}
	}
	if cfg.Gen.Crust != "" {
		idx, ok := cfg.TerrainIndex(cfg.Gen.Crust)
		if !ok {
			return fmt.Errorf("gen: crust references unknown terrain %q", cfg.Gen.Crust)
		}
		if !cfg.Terrain[idx].Mineable {
			return fmt.Errorf("gen: crust terrain %q must be mineable", cfg.Gen.Crust)
		}
		if cfg.Gen.CrustChance < 0 || cfg.Gen.CrustChance > 1 {
			return fmt.Errorf("gen: crust_chance must be between 0 and 1")
		}
	}
	for _, r := range cfg.Gen.Scatter {
		if _, ok := cfg.Types[r.Type]; !ok {
			return fmt.Errorf("scatter rule references unknown type %q", r.Type)
		}
		if _, ok := cfg.TerrainIndex(r.Terrain); !ok {
			return fmt.Errorf("scatter rule references unknown terrain %q", r.Terrain)
		}
	}
	for _, v := range cfg.Gen.Veins {
		idx, ok := cfg.TerrainIndex(v.Terrain)
		if !ok {
			return fmt.Errorf("vein rule references unknown terrain %q", v.Terrain)
		}
		if !cfg.Terrain[idx].Mineable {
			return fmt.Errorf("vein terrain %q must be mineable", v.Terrain)
		}
		if v.Seeds < 0 || v.Size < 1 {
			return fmt.Errorf("vein %q needs non-negative seeds and size >= 1", v.Terrain)
		}
	}
	return nil
}
