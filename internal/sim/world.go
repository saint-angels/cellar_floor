package sim

import (
	"sort"

	"cellarfloor/internal/data"
)

type Terrain uint8

const (
	TerrainGrass Terrain = iota
	TerrainDirt
	TerrainWater
	TerrainRock
	TerrainFloor // mined-out stone
	TerrainGold  // gold vein, mineable like rock
)

var terrainNames = [...]string{"grass", "dirt", "water", "rock", "floor", "gold"}

func TerrainName(t Terrain) string { return terrainNames[t] }
func Passable(t Terrain) bool {
	return t == TerrainGrass || t == TerrainDirt || t == TerrainFloor
}
func Mineable(t Terrain) bool { return t == TerrainRock || t == TerrainGold }

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Entity struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Pos         Point          `json:"pos"`
	Produces    []data.Produce `json:"produces"`
	Fullness    float64        `json:"fullness"`
	StarvingFor int            `json:"starvingFor"`
	Age         int            `json:"age"`
	Home        *Point         `json:"home,omitempty"`
	Dead        bool           `json:"dead"`
	DecayLeft   int            `json:"decayLeft"`
	Action      string         `json:"action"`
	MoveAcc     float64        `json:"moveAcc"`
	MineTarget  *Point         `json:"mineTarget,omitempty"`
}

type World struct {
	Width    int             `json:"width"`
	Height   int             `json:"height"`
	Terrain  []Terrain       `json:"terrain"`
	Entities map[int]*Entity `json:"entities"`
	NextID   int             `json:"nextId"`
	Tick     int64           `json:"tick"`
	Rng      uint64          `json:"rng"`
	Removed  []int           `json:"-"`

	Gold         int             `json:"gold"`
	MineProgress map[int]float64 `json:"mineProgress,omitempty"`

	cfg          *data.Config
	dirty        map[int]bool
	terrainDirty []int
	diedThisTick map[int]bool
	occ          map[Point]int
	counts       map[string]int
	sortedCache  []int
	sortedDirty  bool
	lit          []bool
}

func NewWorld(w, h int, seed uint64, cfg *data.Config) *World {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &World{
		Width: w, Height: h,
		Terrain:      make([]Terrain, w*h),
		Entities:     map[int]*Entity{},
		NextID:       1,
		Rng:          seed,
		MineProgress: map[int]float64{},
		cfg:          cfg,
		dirty:        map[int]bool{},
		occ:          map[Point]int{},
		counts:       map[string]int{},
	}
}

func (w *World) SetConfig(cfg *data.Config) {
	w.cfg = cfg
	if w.dirty == nil {
		w.dirty = map[int]bool{}
	}
	if w.MineProgress == nil {
		w.MineProgress = map[int]float64{}
	}
	w.rebuildOcc()
	w.rebuildCounts()
	w.RecomputeLight()
}

// RecomputeLight rebuilds the derived lit bitfield from living light
// sources. Called on load and whenever a light source spawns or dies.
func (w *World) RecomputeLight() {
	w.lit = make([]bool, w.Width*w.Height)
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if e.Dead {
			continue
		}
		s, ok := w.cfg.Types[e.Type]
		if !ok || s.LightRadius <= 0 {
			continue
		}
		r := s.LightRadius
		for y := maxInt(0, e.Pos.Y-r); y <= minInt(w.Height-1, e.Pos.Y+r); y++ {
			for x := maxInt(0, e.Pos.X-r); x <= minInt(w.Width-1, e.Pos.X+r); x++ {
				dx, dy := x-e.Pos.X, y-e.Pos.Y
				if dx*dx+dy*dy <= r*r {
					w.lit[y*w.Width+x] = true
				}
			}
		}
	}
}

func (w *World) Lit(p Point) bool {
	if w.lit == nil {
		return false
	}
	return w.lit[p.Y*w.Width+p.X]
}

func (w *World) rebuildCounts() {
	w.counts = map[string]int{}
	for _, e := range w.Entities {
		if !e.Dead {
			w.counts[e.Type]++
		}
	}
}

func (w *World) rebuildOcc() {
	w.occ = map[Point]int{}
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if e.Dead {
			continue
		}
		if s, ok := w.cfg.Types[e.Type]; ok && s.Kind == "fauna" {
			w.occ[e.Pos] = id
		}
	}
}

func (w *World) Cfg() *data.Config { return w.cfg }

func (w *World) rand() uint64 {
	x := w.Rng
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	w.Rng = x
	return x * 0x2545F4914F6CDD1D
}

func (w *World) RandFloat() float64 { return float64(w.rand()>>11) / (1 << 53) }
func (w *World) RandN(n int) int    { return int(w.rand() % uint64(n)) }

func (w *World) InBounds(p Point) bool {
	return p.X >= 0 && p.X < w.Width && p.Y >= 0 && p.Y < w.Height
}
func (w *World) At(p Point) Terrain { return w.Terrain[p.Y*w.Width+p.X] }

// SetTerrain mutates a cell and records it for the next tick's terrain diff.
func (w *World) SetTerrain(p Point, t Terrain) {
	i := p.Y*w.Width + p.X
	if w.Terrain[i] == t {
		return
	}
	w.Terrain[i] = t
	w.terrainDirty = append(w.terrainDirty, i)
}

// TerrainDirtyAndReset returns cell indexes changed since the last call.
func (w *World) TerrainDirtyAndReset() []int {
	d := w.terrainDirty
	w.terrainDirty = nil
	return d
}

func (w *World) FaunaAt(p Point) *Entity {
	id, ok := w.occ[p]
	if !ok {
		return nil
	}
	e := w.Entities[id]
	if e == nil || e.Dead {
		return nil
	}
	return e
}

func (w *World) SortedIDs() []int {
	if w.sortedDirty || w.sortedCache == nil {
		ids := make([]int, 0, len(w.Entities))
		for id := range w.Entities {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		w.sortedCache = ids
		w.sortedDirty = false
	}
	return w.sortedCache
}

func (w *World) CountAlive(typeID string) int {
	return w.counts[typeID]
}

func (w *World) Spawn(typeID string, p Point) *Entity {
	s, ok := w.cfg.Types[typeID]
	if !ok {
		return nil
	}
	e := &Entity{
		ID:       w.NextID,
		Type:     typeID,
		Pos:      p,
		Produces: append([]data.Produce(nil), s.Produces...),
	}
	if s.Kind == "fauna" {
		e.Fullness = s.StomachSize / 2
	}
	w.NextID++
	w.Entities[e.ID] = e
	w.sortedDirty = true
	w.counts[typeID]++
	if s.Kind == "fauna" {
		w.occ[p] = e.ID
	}
	w.dirty[e.ID] = true
	if s.LightRadius > 0 {
		w.RecomputeLight()
	}
	return e
}

func Dist(a, b Point) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
