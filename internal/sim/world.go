package sim

import (
	"math"
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
)

// terrainAt looks a terrain value up in the config table; nil when the
// value is out of range, which callers treat as inert.
func (w *World) terrainAt(t Terrain) *data.TerrainType {
	if int(t) >= len(w.cfg.Terrain) {
		return nil
	}
	return &w.cfg.Terrain[t]
}

func (w *World) Passable(t Terrain) bool {
	tt := w.terrainAt(t)
	return tt != nil && tt.Passable
}

func (w *World) Mineable(t Terrain) bool {
	tt := w.terrainAt(t)
	return tt != nil && tt.Mineable
}

func (w *World) TerrainName(t Terrain) string {
	if tt := w.terrainAt(t); tt != nil {
		return tt.ID
	}
	return "unknown"
}

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
	Social      float64        `json:"social,omitempty"`
	SeenID      int            `json:"seenId,omitempty"`
	SeenTick    int64          `json:"seenTick,omitempty"`
	GoldStrikes []GoldStrike   `json:"goldStrikes,omitempty"`
}

// GoldStrike records one gold drop for the rolling last-24h count.
type GoldStrike struct {
	Tick   int64 `json:"tick"`
	Amount int   `json:"amount"`
}

// GoldLast24h prunes strikes older than 24 hours and sums the rest.
func (w *World) GoldLast24h(e *Entity) int {
	window := int64(86400 * w.cfg.Sim.TickRate)
	keep := e.GoldStrikes[:0]
	sum := 0
	for _, g := range e.GoldStrikes {
		if w.Tick-g.Tick <= window {
			keep = append(keep, g)
			sum += g.Amount
		}
	}
	e.GoldStrikes = keep
	return sum
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

	Gold        int            `json:"gold"`
	Level       int            `json:"level"`
	Pending     []string       `json:"pending,omitempty"`
	Claims      map[string]int `json:"claims,omitempty"`
	BlocksMined int            `json:"blocksMined"`
	GoldMined   int            `json:"goldMined"`
	MoldGrown   int            `json:"moldGrown"`
	MineDamage  map[int]int    `json:"mineDamage,omitempty"`

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
		Terrain:    make([]Terrain, w*h),
		Entities:   map[int]*Entity{},
		NextID:     1,
		Rng:        seed,
		MineDamage: map[int]int{},
		Claims:     map[string]int{},
		cfg:        cfg,
		dirty:      map[int]bool{},
		occ:        map[Point]int{},
		counts:     map[string]int{},
	}
}

func (w *World) SetConfig(cfg *data.Config) {
	w.cfg = cfg
	if w.dirty == nil {
		w.dirty = map[int]bool{}
	}
	if w.MineDamage == nil {
		w.MineDamage = map[int]int{}
	}
	if w.Claims == nil {
		w.Claims = map[string]int{}
	}
	// older saves predate the social meter; wake up half-full, like Spawn
	for _, e := range w.Entities {
		if e.Dead {
			continue
		}
		if s, ok := cfg.Types[e.Type]; ok && s.SocialSize > 0 && e.Social == 0 {
			e.Social = s.SocialSize / 2
		}
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

// MineBonus is the summed damage of CLAIMED upgrades; pending draws are inert.
func (w *World) MineBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "damage" || u.Kind == "weapon" {
			bonus += u.Amount * w.Claims[u.Name]
		}
	}
	return bonus
}

// BeamBonus is the summed damage of claimed beam upgrades; unlike the AOE
// MineBonus it lands only on the miner's chosen target face.
func (w *World) BeamBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "beam" {
			bonus += u.Amount * w.Claims[u.Name]
		}
	}
	return bonus
}

// LuckBonus widens gold drops from claimed luck upgrades.
func (w *World) LuckBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "luck" {
			bonus += u.Amount * w.Claims[u.Name]
		}
	}
	return bonus
}

// NextLevelGold is the cumulative mined gold required for the next level.
// A missing pool or a non-progressing LevelBase yields a huge sentinel so
// levelStep stays a no-op instead of looping on a zero target.
func (w *World) NextLevelGold() int {
	if len(w.cfg.Upgrades) == 0 || w.cfg.LevelBase <= 0 {
		return 1 << 30
	}
	target := 0.0
	step := w.cfg.LevelBase
	for k := 0; k <= w.Level; k++ {
		target += step
		step *= w.cfg.LevelGrowth
	}
	return int(math.Ceil(target))
}

// PrevLevelGold is the cumulative mined gold that reached the CURRENT level;
// it is NextLevelGold summing one fewer term, so 0 when Level == 0.
func (w *World) PrevLevelGold() int {
	if len(w.cfg.Upgrades) == 0 || w.cfg.LevelBase <= 0 {
		return 0
	}
	target := 0.0
	step := w.cfg.LevelBase
	for k := 0; k < w.Level; k++ {
		target += step
		step *= w.cfg.LevelGrowth
	}
	return int(math.Ceil(target))
}

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
		if s.SocialSize > 0 {
			e.Social = s.SocialSize / 2
		}
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
