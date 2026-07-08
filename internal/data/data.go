package data

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Produce struct {
	Resource string  `toml:"resource" json:"resource"`
	Amount   float64 `toml:"amount" json:"amount"`
	Max      float64 `toml:"max" json:"max"`
	Regrow   float64 `toml:"regrow" json:"regrow"`
}

type Desire struct {
	Resource string  `toml:"resource" json:"resource"`
	Amount   float64 `toml:"amount" json:"amount"`
	Aversion bool    `toml:"aversion" json:"aversion"`
}

type Species struct {
	ID              string    `toml:"-" json:"id"`
	Name            string    `toml:"name" json:"name"`
	Kind            string    `toml:"kind" json:"kind"`
	Color           string    `toml:"color" json:"color"`
	Produces        []Produce `toml:"produces" json:"produces"`
	Eats            []string  `toml:"eats" json:"eats"`
	Shelters        []string  `toml:"shelters" json:"shelters"`
	Desires         []Desire  `toml:"desires" json:"desires"`
	BiteSize        float64   `toml:"bite_size" json:"biteSize"`
	StomachSize     float64   `toml:"stomach_size" json:"stomachSize"`
	HungerThreshold float64   `toml:"hunger_threshold" json:"hungerThreshold"`
	Metabolism      float64   `toml:"metabolism" json:"metabolism"`
	StarveTicks     int       `toml:"starve_ticks" json:"starveTicks"`
	FearRadius      int       `toml:"fear_radius" json:"fearRadius"`
	Speed           float64   `toml:"speed" json:"speed"`
	HomeRange       int       `toml:"home_range" json:"homeRange"`
	Lifespan        int       `toml:"lifespan" json:"lifespan"`
	MatureAge       int       `toml:"mature_age" json:"matureAge"`
	ReproThreshold  float64   `toml:"repro_threshold" json:"reproThreshold"`
	ReproChance     float64   `toml:"repro_chance" json:"reproChance"`
	ReproCost       float64   `toml:"repro_cost" json:"reproCost"`
	PopFloor        int       `toml:"pop_floor" json:"popFloor"`
	PopCap          int       `toml:"pop_cap" json:"popCap"`
	DecayTicks      int       `toml:"decay_ticks" json:"decayTicks"`
}

type SimConfig struct {
	TickRate        float64 `toml:"tick_rate"`
	AutosaveMinutes int     `toml:"autosave_minutes"`
	SavePath        string  `toml:"save_path"`
}

type ScatterRule struct {
	Species string  `toml:"species"`
	Terrain string  `toml:"terrain"`
	Chance  float64 `toml:"chance"`
}

type GenConfig struct {
	Width        int           `toml:"width"`
	Height       int           `toml:"height"`
	NoiseScale   float64       `toml:"noise_scale"`
	NoiseOctaves int           `toml:"noise_octaves"`
	WaterBelow   float64       `toml:"water_below"`
	DirtAbove    float64       `toml:"dirt_above"`
	RockAbove    float64       `toml:"rock_above"`
	Scatter      []ScatterRule `toml:"scatter"`
}

type Config struct {
	Sim     SimConfig
	Gen     GenConfig
	Species map[string]*Species
}

var validTerrains = map[string]bool{"grass": true, "dirt": true, "water": true, "rock": true}

func Load(dir string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(filepath.Join(dir, "sim.toml"), &cfg.Sim); err != nil {
		return nil, fmt.Errorf("sim.toml: %w", err)
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "gen.toml"), &cfg.Gen); err != nil {
		return nil, fmt.Errorf("gen.toml: %w", err)
	}
	var sp struct {
		Species map[string]*Species `toml:"species"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "species.toml"), &sp); err != nil {
		return nil, fmt.Errorf("species.toml: %w", err)
	}
	cfg.Species = sp.Species
	for id, s := range cfg.Species {
		s.ID = id
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Validate(cfg *Config) error {
	produced := map[string]bool{}
	for _, s := range cfg.Species {
		for _, p := range s.Produces {
			produced[p.Resource] = true
		}
	}
	for id, s := range cfg.Species {
		if s.Kind != "flora" && s.Kind != "fauna" {
			return fmt.Errorf("species %s: kind must be flora or fauna, got %q", id, s.Kind)
		}
		if s.Name == "" || s.Color == "" {
			return fmt.Errorf("species %s: name and color are required", id)
		}
		for _, r := range s.Eats {
			if !produced[r] {
				return fmt.Errorf("species %s eats %q which nothing produces", id, r)
			}
		}
		for _, r := range s.Shelters {
			if !produced[r] {
				return fmt.Errorf("species %s shelters in %q which nothing produces", id, r)
			}
		}
		if s.Kind == "fauna" {
			if s.StomachSize <= 0 || s.BiteSize <= 0 || s.Speed <= 0 ||
				s.Metabolism <= 0 || s.StarveTicks <= 0 || s.DecayTicks <= 0 ||
				s.Lifespan <= 0 || s.PopCap <= 0 {
				return fmt.Errorf("species %s: fauna requires positive stomach_size, bite_size, speed, metabolism, starve_ticks, decay_ticks, lifespan, pop_cap", id)
			}
			if len(s.Eats) == 0 {
				return fmt.Errorf("species %s: fauna must eat something", id)
			}
		}
	}
	if cfg.Sim.TickRate <= 0 {
		return fmt.Errorf("sim: tick_rate must be positive")
	}
	if cfg.Gen.Width <= 0 || cfg.Gen.Height <= 0 {
		return fmt.Errorf("gen: width and height must be positive")
	}
	for _, r := range cfg.Gen.Scatter {
		if _, ok := cfg.Species[r.Species]; !ok {
			return fmt.Errorf("scatter rule references unknown species %q", r.Species)
		}
		if !validTerrains[r.Terrain] {
			return fmt.Errorf("scatter rule references unknown terrain %q", r.Terrain)
		}
	}
	return nil
}
