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
	TargetID    int            `json:"targetId,omitempty"`
	Social      float64        `json:"social,omitempty"`
	SeenID      int            `json:"seenId,omitempty"`
	SeenTick    int64          `json:"seenTick,omitempty"`
	GoldStrikes []GoldStrike   `json:"goldStrikes,omitempty"`
	Ore         int            `json:"ore,omitempty"`

	// spec caches cfg.Types[Type]. Unexported, so it never serializes and a
	// loaded world re-resolves it on first use. The per-tick scans hash this
	// string key thousands of times otherwise.
	spec *data.EntityType
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

	Gold          int            `json:"gold"`
	Level         int            `json:"level"`
	Pending       []string       `json:"pending,omitempty"` // legacy queue, migrated in SetConfig
	PendingLevels int            `json:"pendingLevels,omitempty"`
	Offer         []string       `json:"offer,omitempty"`
	Claims        map[string]int `json:"claims,omitempty"`
	BlocksMined   int            `json:"blocksMined"`
	GoldMined     int            `json:"goldMined"`
	MoldGrown     int            `json:"moldGrown"`
	MineDamage    map[int]int    `json:"mineDamage,omitempty"`

	cfg          *data.Config
	dirty        map[int]bool
	terrainDirty []int
	diedThisTick map[int]bool
	occ          map[Point]int
	counts       map[string]int
	sortedCache  []int
	entCache     []*Entity
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
	// the cached specs point into the old config's table
	for _, e := range w.Entities {
		e.spec = nil
	}
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
	// legacy saves queued drawn NAMES; each becomes a level worth of choice
	if len(w.Pending) > 0 {
		w.PendingLevels += len(w.Pending)
		w.Pending = nil
	}
	if w.PendingLevels > 0 && len(w.Offer) == 0 {
		w.rollOffer()
	}
	w.rebuildOcc()
	w.rebuildCounts()
	w.RecomputeLight()
	// older saves predate the market; give them one by the campfire so the
	// ore economy works without a world reset. Runs after the rebuilds so
	// Spawn's counts/occ maps are live. Never duplicated.
	w.ensureMarket()
}

// ensureMarket spawns a single market entity next to the world center when
// the config defines a Market type but no living market exists yet. It is a
// no-op when a market already lives or the config has none, so it stays safe
// to run on every load without duplicating.
func (w *World) ensureMarket() {
	marketType := ""
	for _, id := range w.sortedTypeIDs() {
		if w.cfg.Types[id].Market {
			marketType = id
			break
		}
	}
	if marketType == "" {
		return
	}
	for _, e := range w.Entities {
		if e.Dead {
			continue
		}
		if s, ok := w.cfg.Types[e.Type]; ok && s.Market {
			return
		}
	}
	if p, ok := w.marketRingTile(w.Width/2, w.Height/2); ok {
		w.Spawn(marketType, p)
	}
}

// sortedTypeIDs lists the config's type ids in deterministic order.
func (w *World) sortedTypeIDs() []string {
	ids := make([]string, 0, len(w.cfg.Types))
	for id := range w.cfg.Types {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// marketRingTile scans Chebyshev rings outward from (ox, oy) and returns the
// first passable tile, visiting each ring in ascending cell-index order for
// determinism. Mirrors gen.marketTile, which sim cannot import.
func (w *World) marketRingTile(ox, oy int) (Point, bool) {
	origin := Point{X: ox, Y: oy}
	for r := 1; r <= w.Width+w.Height; r++ {
		var ring []Point
		for y := oy - r; y <= oy+r; y++ {
			for x := ox - r; x <= ox+r; x++ {
				p := Point{X: x, Y: y}
				if Dist(p, origin) != r || !w.InBounds(p) {
					continue
				}
				ring = append(ring, p)
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
	return Point{}, false
}

// rollOffer draws up to three distinct eligible upgrades for the current
// pending level. When nothing is eligible the queued levels are dropped;
// claims only grow, so eligibility never comes back.
func (w *World) rollOffer() {
	w.Offer = nil
	if w.PendingLevels <= 0 {
		return
	}
	var eligible []string
	for _, u := range w.cfg.Upgrades {
		if u.Max == 0 || w.Claims[u.Name] < u.Max {
			eligible = append(eligible, u.Name)
		}
	}
	if len(eligible) == 0 {
		w.PendingLevels = 0
		return
	}
	for i := 0; i < len(eligible)-1 && i < 3; i++ {
		j := i + w.RandN(len(eligible)-i)
		eligible[i], eligible[j] = eligible[j], eligible[i]
	}
	if len(eligible) > 3 {
		eligible = eligible[:3]
	}
	w.Offer = eligible
}

// ClaimOffer applies one upgrade from the current offer, consumes the
// level, and rolls the next offer. False when the name is not on offer.
func (w *World) ClaimOffer(name string) bool {
	if w.PendingLevels <= 0 {
		return false
	}
	ok := false
	for _, n := range w.Offer {
		if n == name {
			ok = true
		}
	}
	if !ok {
		return false
	}
	if w.Claims == nil {
		w.Claims = map[string]int{}
	}
	w.Claims[name]++
	w.PendingLevels--
	w.rollOffer()
	return true
}

// RecomputeLight rebuilds the derived lit bitfield from living light
// sources. Called on load and whenever a light source spawns or dies.
func (w *World) RecomputeLight() {
	w.lit = make([]bool, w.Width*w.Height)
	for _, e := range w.entities() {
		if e.Dead {
			continue
		}
		s := w.spec(e)
		if s == nil || s.LightRadius <= 0 {
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
	for _, e := range w.entities() {
		if e.Dead {
			continue
		}
		if s := w.spec(e); s != nil && s.Kind == "fauna" {
			w.occ[e.Pos] = e.ID
		}
	}
}

// setTarget records which entity e is currently interacting with (food,
// companion); zero means none. Dirty only on change.
func (w *World) setTarget(e *Entity, id int) {
	if e.TargetID != id {
		e.TargetID = id
		w.markDirty(e.ID)
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

// BeamBonus is the summed damage of claimed target-focused weapons (beams
// and missiles); unlike the AOE MineBonus it lands only on the miner's
// chosen target face.
func (w *World) BeamBonus() int {
	bonus := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "beam" || u.Kind == "missile" {
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

// SpeedFactor is the movement multiplier from claimed speed upgrades:
// 1 + sum(amount*claims)/100. No speed claims yields 1.0, so every walk
// is unchanged until Swift Boots is claimed.
func (w *World) SpeedFactor() float64 {
	sum := 0
	for _, u := range w.cfg.Upgrades {
		if u.Kind == "speed" {
			sum += u.Amount * w.Claims[u.Name]
		}
	}
	return 1 + float64(sum)/100
}

// moveSpeed is a type's base speed scaled by the colony speed factor; the
// per-tick MoveAcc increment for every walk so speed upgrades apply
// everywhere a creature moves.
func (w *World) moveSpeed(e *Entity) float64 {
	return w.spec(e).Speed * w.SpeedFactor()
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
	w.refreshOrder()
	return w.sortedCache
}

// entities lists every entity in ascending ID order, the same order as
// SortedIDs, so per-tick scans keep their deterministic sweep without paying
// a map lookup per id.
func (w *World) entities() []*Entity {
	w.refreshOrder()
	return w.entCache
}

func (w *World) refreshOrder() {
	if !w.sortedDirty && w.sortedCache != nil {
		return
	}
	ids := make([]int, 0, len(w.Entities))
	for id := range w.Entities {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	ents := make([]*Entity, len(ids))
	for i, id := range ids {
		ents[i] = w.Entities[id]
	}
	w.sortedCache = ids
	w.entCache = ents
	w.sortedDirty = false
}

// spec is cfg.Types[e.Type], cached on the entity. Returns nil for a type the
// config does not define, matching the comma-ok lookups it replaces.
func (w *World) spec(e *Entity) *data.EntityType {
	if e.spec == nil {
		e.spec = w.cfg.Types[e.Type]
	}
	return e.spec
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
