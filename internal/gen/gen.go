package gen

import (
	"sort"

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

// randomRockCell picks a uniformly random plain-rock cell, or reports none.
func randomRockCell(w *sim.World) (sim.Point, bool) {
	for i := 0; i < 200; i++ {
		p := sim.Point{X: w.RandN(w.Width), Y: w.RandN(w.Height)}
		if w.At(p) == sim.TerrainRock {
			return p, true
		}
	}
	return sim.Point{}, false
}

// randomRockNeighbor picks a random 8-neighbor of p that is still plain rock.
func randomRockNeighbor(w *sim.World, p sim.Point) (sim.Point, bool) {
	start := w.RandN(8)
	dirs := [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
	for i := 0; i < 8; i++ {
		d := dirs[(start+i)%8]
		q := sim.Point{X: p.X + d[0], Y: p.Y + d[1]}
		if w.InBounds(q) && w.At(q) == sim.TerrainRock {
			return q, true
		}
	}
	return sim.Point{}, false
}

// marketTile scans rings of increasing Chebyshev radius outward from
// the origin (ox, oy) and returns the first passable tile, visiting the
// cells of each ring in ascending cell-index order for determinism.
func marketTile(w *sim.World, ox, oy int) (sim.Point, bool) {
	for r := 1; r <= w.Width+w.Height; r++ {
		var ring []sim.Point
		for y := oy - r; y <= oy+r; y++ {
			for x := ox - r; x <= ox+r; x++ {
				if max(abs(x-ox), abs(y-oy)) != r {
					continue
				}
				p := sim.Point{X: x, Y: y}
				if w.InBounds(p) {
					ring = append(ring, p)
				}
			}
		}
		sort.Slice(ring, func(i, j int) bool {
			return ring[i].Y*w.Width+ring[i].X < ring[j].Y*w.Width+ring[j].X
		})
		for _, p := range ring {
			if w.Passable(w.At(p)) {
				return p, true
			}
		}
	}
	return sim.Point{}, false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
				default:
					t = sim.TerrainRock
				}
				w.Terrain[y*g.Width+x] = t
			}
		}
		if g.Crust != "" && g.CrustChance > 0 {
			if idx, ok := cfg.TerrainIndex(g.Crust); ok {
				ct := sim.Terrain(idx)
				for y := 0; y < g.Height; y++ {
					for x := 0; x < g.Width; x++ {
						p := sim.Point{X: x, Y: y}
						if w.At(p) != sim.TerrainRock {
							continue
						}
						touchesDirt := false
						for _, d := range [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
							q := sim.Point{X: x + d[0], Y: y + d[1]}
							if w.InBounds(q) && w.At(q) == sim.TerrainDirt {
								touchesDirt = true
								break
							}
						}
						if touchesDirt && w.RandFloat() < g.CrustChance {
							w.Terrain[y*g.Width+x] = ct
						}
					}
				}
			}
		}
		for _, vein := range g.Veins {
			idx, ok := cfg.TerrainIndex(vein.Terrain)
			if !ok {
				continue // validated at load; belt and braces
			}
			vt := sim.Terrain(idx)
			for s := 0; s < vein.Seeds; s++ {
				seed, found := randomRockCell(w)
				if !found {
					break
				}
				blob := []sim.Point{seed}
				w.Terrain[seed.Y*g.Width+seed.X] = vt
				for tries := 0; len(blob) < vein.Size && tries < vein.Size*20; tries++ {
					p := blob[w.RandN(len(blob))]
					q, ok := randomRockNeighbor(w, p)
					if !ok {
						continue
					}
					w.Terrain[q.Y*g.Width+q.X] = vt
					blob = append(blob, q)
					if len(blob) >= vein.Size {
						break
					}
				}
			}
		}
		if g.Center != "" {
			w.Spawn(g.Center, sim.Point{X: g.Width / 2, Y: g.Height / 2})
		}
		if g.Market != "" {
			if p, ok := marketTile(w, g.Width/2+2, g.Height/2); ok {
				w.Spawn(g.Market, p)
			}
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
			tname := w.TerrainName(w.At(p))
			for _, rule := range g.Scatter {
				if rule.Terrain != tname {
					continue
				}
				if w.RandFloat() >= rule.Chance {
					continue
				}
				s := cfg.Types[rule.Type]
				if s.Kind == "fauna" {
					if !w.Passable(w.At(p)) || w.FaunaAt(p) != nil {
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
