package gen

import (
	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

// lattice returns a deterministic pseudo-random float in [0,1) for a lattice point.
func lattice(seed int64, x, y int) float64 {
	h := uint64(seed)*0x9E3779B97F4A7C15 + uint64(x)*0xBF58476D1CE4E5B9 + uint64(y)*0x94D049BB133111EB
	h ^= h >> 30
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 27
	return float64(h>>11) / (1 << 53)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func noiseAt(seed int64, fx, fy float64) float64 {
	x0, y0 := int(fx), int(fy)
	tx, ty := smooth(fx-float64(x0)), smooth(fy-float64(y0))
	top := lerp(lattice(seed, x0, y0), lattice(seed, x0+1, y0), tx)
	bot := lerp(lattice(seed, x0, y0+1), lattice(seed, x0+1, y0+1), tx)
	return lerp(top, bot, ty)
}

func fractal(seed int64, x, y int, scale float64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	freq := 1.0 / scale
	for o := 0; o < octaves; o++ {
		sum += amp * noiseAt(seed+int64(o)*7919, float64(x)*freq, float64(y)*freq)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

func Generate(seed int64, cfg *data.Config) *sim.World {
	g := cfg.Gen
	w := sim.NewWorld(g.Width, g.Height, uint64(seed), cfg)

	if g.ClearingRadius > 0 {
		cx, cy := g.Width/2, g.Height/2
		r2 := g.ClearingRadius * g.ClearingRadius
		for y := 0; y < g.Height; y++ {
			for x := 0; x < g.Width; x++ {
				dx, dy := x-cx, y-cy
				var t sim.Terrain
				switch {
				case dx*dx+dy*dy <= r2:
					t = sim.TerrainDirt
				case w.RandFloat() < g.GoldChance:
					t = sim.TerrainGold
				default:
					t = sim.TerrainRock
				}
				w.Terrain[y*g.Width+x] = t
			}
		}
		if g.Center != "" {
			w.Spawn(g.Center, sim.Point{X: g.Width / 2, Y: g.Height / 2})
		}
	} else {
		for y := 0; y < g.Height; y++ {
			for x := 0; x < g.Width; x++ {
				v := fractal(seed, x, y, g.NoiseScale, g.NoiseOctaves)
				var t sim.Terrain
				switch {
				case v < g.WaterBelow:
					t = sim.TerrainWater
				case v > g.RockAbove:
					t = sim.TerrainRock
				case v > g.DirtAbove:
					t = sim.TerrainDirt
				default:
					t = sim.TerrainGrass
				}
				w.Terrain[y*g.Width+x] = t
			}
		}
	}

	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := sim.Point{X: x, Y: y}
			tname := sim.TerrainName(w.At(p))
			for _, rule := range g.Scatter {
				if rule.Terrain != tname {
					continue
				}
				if w.RandFloat() >= rule.Chance {
					continue
				}
				s := cfg.Types[rule.Type]
				if s.Kind == "fauna" {
					if !sim.Passable(w.At(p)) || w.FaunaAt(p) != nil {
						continue
					}
				}
				w.Spawn(rule.Type, p)
			}
		}
	}
	w.DirtyAndReset()
	return w
}
