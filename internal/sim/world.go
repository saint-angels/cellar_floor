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
)

var terrainNames = [...]string{"grass", "dirt", "water", "rock"}

func TerrainName(t Terrain) string { return terrainNames[t] }
func Passable(t Terrain) bool      { return t == TerrainGrass || t == TerrainDirt }

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Entity struct {
	ID          int            `json:"id"`
	Species     string         `json:"species"`
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
}

type World struct {
	Width    int             `json:"width"`
	Height   int             `json:"height"`
	Terrain  []Terrain       `json:"terrain"`
	Entities map[int]*Entity `json:"entities"`
	NextID   int             `json:"nextId"`
	Tick     int64           `json:"tick"`
	Rng      uint64          `json:"rng"`

	cfg   *data.Config
	dirty map[int]bool
}

func NewWorld(w, h int, seed uint64, cfg *data.Config) *World {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &World{
		Width: w, Height: h,
		Terrain:  make([]Terrain, w*h),
		Entities: map[int]*Entity{},
		NextID:   1,
		Rng:      seed,
		cfg:      cfg,
		dirty:    map[int]bool{},
	}
}

func (w *World) SetConfig(cfg *data.Config) {
	w.cfg = cfg
	if w.dirty == nil {
		w.dirty = map[int]bool{}
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

func (w *World) FaunaAt(p Point) *Entity {
	for _, id := range w.SortedIDs() {
		e := w.Entities[id]
		if e.Pos == p && w.cfg.Species[e.Species].Kind == "fauna" && !e.Dead {
			return e
		}
	}
	return nil
}

func (w *World) SortedIDs() []int {
	ids := make([]int, 0, len(w.Entities))
	for id := range w.Entities {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (w *World) CountAlive(speciesID string) int {
	n := 0
	for _, e := range w.Entities {
		if e.Species == speciesID && !e.Dead {
			n++
		}
	}
	return n
}

func (w *World) Spawn(speciesID string, p Point) *Entity {
	s, ok := w.cfg.Species[speciesID]
	if !ok {
		return nil
	}
	e := &Entity{
		ID:       w.NextID,
		Species:  speciesID,
		Pos:      p,
		Produces: append([]data.Produce(nil), s.Produces...),
	}
	if s.Kind == "fauna" {
		e.Fullness = s.StomachSize / 2
	}
	w.NextID++
	w.Entities[e.ID] = e
	w.dirty[e.ID] = true
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
