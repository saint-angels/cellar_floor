package sim

import (
	"fmt"

	"cellarfloor/internal/data"
)

// typeEatsProduceOf reports whether eater's diet covers anything victim
// produces. Both lists are a handful of entries, so it scans them directly:
// fleeStep calls this for every entity pair every tick and the set it used to
// build allocated a map each time.
func typeEatsProduceOf(eater, victim *data.EntityType) bool {
	if eater.ID == victim.ID || eater.Kind != "fauna" {
		return false
	}
	for _, r := range eater.Eats {
		for _, p := range victim.Produces {
			if p.Resource == r {
				return true
			}
		}
	}
	return false
}

var neighbors = []Point{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func adjacent(a, b Point) bool { return Dist(a, b) <= 1 }

func (w *World) aiStep(e *Entity) []Event {
	s := w.spec(e)

	// 0. darkness: a creature caught in the dark panics back toward light
	if w.darkStep(e) {
		return nil
	}

	// 1. danger (implemented in Task 5)
	if evs, fled := w.fleeStep(e); fled {
		w.setTarget(e, 0)
		return evs
	}

	// 2. food — greedy: any beacon that catches a creature pulls it, full or
	// not; it eats the source clean and moves on, so food never accumulates.
	// Each item is a consumable command token (garden the cave), and eating
	// resets the starve clock. Live prey stays hunger-gated inside findFood.
	hungry := e.Fullness < s.HungerThreshold
	if food := w.findFood(e); food != nil {
		w.setTarget(e, food.ID)
		if adjacent(e.Pos, food.Pos) {
			return w.eatFrom(e, food)
		}
		e.Action = "seeking food"
		w.pathToward(e, food.Pos)
		return nil
	}
	// no walk-reachable food: tunnel toward the nearest beacon reaching
	// through the rock (food planted behind a wall makes the dwarf dig to it)
	if evs, dug := w.digFoodStep(e); dug {
		return evs
	}
	if hungry {
		// starving with no beacon in range: wander and hope to cross one
		e.Action = "searching"
		w.setTarget(e, 0)
		w.wander(e)
		return nil
	}

	// 3. company
	if w.socialStep(e) {
		return nil
	}

	// 4. a full bag heads to the market before mining more
	if evs, hauled := w.haulStep(e, true); hauled {
		return evs
	}

	// 5. finish an assigned dig. mineStep never picks a face on its own now;
	// it only breaks one already assigned by food-digging, so a fed dwarf
	// with no buried food to reach never mines (food is the only driver).
	if evs, mined := w.mineStep(e); mined {
		return evs
	}

	// 6. nothing left to mine but ore in the bag: sell the rest
	if evs, hauled := w.haulStep(e, false); hauled {
		return evs
	}

	// 7. shelter
	if w.shelterStep(e) {
		return nil
	}

	// 8. wander
	w.setTarget(e, 0)
	e.Action = "idle"
	if w.RandFloat() < s.WanderChance {
		w.wander(e)
	}
	return nil
}

func (w *World) findFood(e *Entity) *Entity {
	// Eats holds a couple of entries, so linear scans beat building sets:
	// greedy eaters run this every tick for every creature.
	s := w.spec(e)
	hungry := e.Fullness < s.HungerThreshold
	var edibles []*Entity
	var nearest *Entity
	bestD := 1 << 30
	for _, c := range w.entities() {
		if c.ID == e.ID || c.Type == e.Type {
			continue
		}
		// live prey is only worth killing when hungry; flora and corpses
		// (passive beacons) pull a greedy eater regardless of fullness
		if !hungry && !c.Dead && w.spec(c).Kind == "fauna" {
			continue
		}
		if !edibleTo(c, s.Eats, s.BiteSize) {
			continue
		}
		// beacon model: food is sensed only within ITS OWN radius (a property
		// of the food, not the eater). Anything farther simply does not exist
		// to a hungry creature — it wanders until a beacon catches it.
		if Dist(e.Pos, c.Pos) > w.spec(c).SenseRadius {
			continue
		}
		edibles = append(edibles, c)
		if d := Dist(e.Pos, c.Pos); d < bestD {
			nearest, bestD = c, d
		}
	}
	if nearest == nil {
		return nil
	}
	// fast path: the straight-line nearest is usually reachable; the BFS
	// probe exits early on success, so this stays cheap
	if adjacent(e.Pos, nearest.Pos) {
		return nearest
	}
	if _, ok := w.nextStepToward(e.Pos, nearest.Pos); ok {
		return nearest
	}
	// nearest is walled off (mold pockets): flood once and take the
	// closest actually reachable meal
	dist := w.reachableDist(e.Pos)
	var best *Entity
	bestC := 1 << 30
	for _, c := range edibles {
		if d := w.reachCost(dist, c.Pos); d >= 0 && d < bestC {
			best, bestC = c, d
		}
	}
	return best
}

// edibleTo reports whether c offers a bite worth taking (see biteWorthy) of a
// resource in eats. Shared by walk- and dig-food seeking.
func edibleTo(c *Entity, eats []string, biteSize float64) bool {
	for i := range c.Produces {
		p := &c.Produces[i]
		if !biteWorthy(p, biteSize) {
			continue
		}
		for _, r := range eats {
			if r == p.Resource {
				return true
			}
		}
	}
	return false
}

// biteWorthy: at least half a bite left, or a finishable remnant of a
// non-regrowing food — the last sliver kills the token. The regrow guard stops
// an eater camping a regrowing stub to sip its per-tick trickle.
func biteWorthy(p *data.Produce, biteSize float64) bool {
	if p.Amount <= 0 {
		return false
	}
	return p.Amount >= biteSize*0.5 || p.Regrow <= 0
}

// spent reports whether a food has nothing left in any produce and no way to
// regrow; such a husk is dead weight and its beacon must die with it.
func spent(food *Entity) bool {
	for i := range food.Produces {
		p := &food.Produces[i]
		if p.Amount > 0 || p.Regrow > 0 {
			return false
		}
	}
	return true
}

// nearestSensedFood returns the closest edible whose own sense radius covers
// the eater, measured in a straight line so a beacon reaches through rock.
// Distinct from findFood, which only sees food it can already walk to.
func (w *World) nearestSensedFood(e *Entity) *Entity {
	s := w.spec(e)
	hungry := e.Fullness < s.HungerThreshold
	var best *Entity
	bestD := 1 << 30
	for _, c := range w.entities() {
		if c.ID == e.ID || c.Type == e.Type {
			continue
		}
		// same prey rule as findFood: live fauna is only food when hungry
		if !hungry && !c.Dead && w.spec(c).Kind == "fauna" {
			continue
		}
		if !edibleTo(c, s.Eats, s.BiteSize) {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d <= w.spec(c).SenseRadius && d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// stepTowardBuried greedily heads one tile toward a walled-off target: the
// neighbor that most reduces distance, walking open tiles when possible and
// otherwise naming a rock face to mine. ok is false at a dead end (only water
// or backward moves remain), which greedy digging cannot route around.
func (w *World) stepTowardBuried(from, to Point) (step Point, isDig, ok bool) {
	cur := Dist(from, to)
	var walk, dig Point
	bw, bd := cur, cur
	haveWalk, haveDig := false, false
	for _, n := range neighbors {
		p := Point{from.X + n.X, from.Y + n.Y}
		if !w.InBounds(p) {
			continue
		}
		d := Dist(p, to)
		if d >= cur {
			continue // only tiles that get us closer
		}
		t := w.At(p)
		switch {
		case w.Passable(t) && w.FaunaAt(p) == nil:
			if !haveWalk || d < bw {
				walk, bw, haveWalk = p, d, true
			}
		case w.Mineable(t):
			if !haveDig || d < bd {
				dig, bd, haveDig = p, d, true
			}
		}
	}
	if haveWalk {
		return walk, false, true // prefer an open detour over digging
	}
	if haveDig {
		return dig, true, true
	}
	return Point{}, false, false
}

// digFoodStep is the core of food-directed digging: a dwarf with no
// walk-reachable food commits to the nearest food whose beacon radius reaches
// it and tunnels toward it, mining the rock in the way. Returns (events, true)
// when it spent the tick on this.
func (w *World) digFoodStep(e *Entity) ([]Event, bool) {
	s := w.spec(e)
	// sensing range lives on the food (its beacon radius), so the only
	// eater-side requirement is being able to dig at all
	if s.MineDamage <= 0 {
		return nil, false
	}
	target := w.nearestSensedFood(e)
	if target == nil {
		return nil, false
	}
	w.setTarget(e, target.ID)
	if adjacent(e.Pos, target.Pos) {
		return w.eatFrom(e, target), true
	}
	step, isDig, ok := w.stepTowardBuried(e.Pos, target.Pos)
	if !ok {
		return nil, false
	}
	if isDig {
		// hand the face to the miner, which breaks it over ticks; the dwarf
		// advances into the opened tile on a later step
		e.MineTarget = &step
		w.markDirty(e.ID)
		evs, mined := w.mineStep(e)
		// mineStep clears TargetID once it starts breaking the face (the face
		// rides MineTarget). Re-assert the food commitment so it persists
		// across the whole dig and the client can draw a line to the buried
		// food the dwarf is tunnelling toward.
		if mined {
			w.setTarget(e, target.ID)
		}
		return evs, mined
	}
	// walk the open leg toward the food, respecting move speed
	e.Action = "digging to food"
	e.MoveAcc += w.moveSpeed(e)
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target.Pos) {
		e.MoveAcc--
		st, dig, ok2 := w.stepTowardBuried(e.Pos, target.Pos)
		if !ok2 || dig {
			break // stop at a rock face; next tick mines it
		}
		if !w.walkStep(e, st, target.Pos) {
			break
		}
	}
	if adjacent(e.Pos, target.Pos) {
		return w.eatFrom(e, target), true
	}
	return nil, true
}

func (w *World) eatFrom(e *Entity, food *Entity) []Event {
	s := w.cfg.Types[e.Type]
	// Live fauna prey is killed first (Task 5 covers the hunt event path).
	if !food.Dead && w.cfg.Types[food.Type].Kind == "fauna" {
		return w.huntStrike(e, food)
	}
	eats := map[string]bool{}
	for _, r := range s.Eats {
		eats[r] = true
	}
	for i := range food.Produces {
		p := &food.Produces[i]
		if !eats[p.Resource] || !biteWorthy(p, s.BiteSize) {
			continue
		}
		// greedy: bite regardless of stomach room. Overeating is wasted, but
		// the source still empties — each food is a consumable token, and
		// eating (even while full) holds the starve clock at bay.
		bite := s.BiteSize
		if p.Amount < bite {
			bite = p.Amount // finish the sliver so the token dies
		}
		p.Amount -= bite
		e.Fullness += bite
		if e.Fullness > s.StomachSize {
			e.Fullness = s.StomachSize
		}
		e.Action = "eating"
		w.markDirty(e.ID)
		w.markDirty(food.ID)
		evs := []Event{{
			Tick: w.Tick, Type: "ate",
			Actor: e.ID, ActorType: e.Type,
			Target: food.ID, TargetType: food.Type,
			Msg: fmt.Sprintf("%s ate %s from %s", s.Name, p.Resource, w.cfg.Types[food.Type].Name),
		}}
		// a flora eaten clean with nothing regrowing is spent: it dies, and
		// its beacon dies with it
		if !food.Dead && w.cfg.Types[food.Type].Kind == "flora" && spent(food) {
			evs = append(evs, w.kill(food, "consumed",
				fmt.Sprintf("%s was eaten clean", w.cfg.Types[food.Type].Name)))
		}
		return evs
	}
	// nothing worth eating left here; release the meal
	if e.Action == "eating" {
		e.Action = "idle"
		w.markDirty(e.ID)
	}
	return nil
}

func (w *World) moveToward(e *Entity, target Point) { w.move(e, target, false) }
func (w *World) moveAway(e *Entity, from Point)     { w.move(e, from, true) }

// reachableDist floods passable cells from start and returns BFS
// distances per cell index, -1 for unreachable. Array-based for speed:
// hungry fauna flood every tick.
func (w *World) reachableDist(start Point) []int32 {
	dist := make([]int32, w.Width*w.Height)
	for i := range dist {
		dist[i] = -1
	}
	s0 := int32(start.Y*w.Width + start.X)
	dist[s0] = 0
	queue := make([]int32, 0, 256)
	queue = append(queue, s0)
	for qi := 0; qi < len(queue); qi++ {
		p := queue[qi]
		px, py := int(p)%w.Width, int(p)/w.Width
		for _, n := range neighbors {
			x, y := px+n.X, py+n.Y
			if x < 0 || y < 0 || x >= w.Width || y >= w.Height {
				continue
			}
			i := y*w.Width + x
			if dist[i] >= 0 {
				continue
			}
			if !w.Passable(w.Terrain[i]) {
				continue
			}
			dist[i] = dist[p] + 1
			queue = append(queue, int32(i))
		}
	}
	return dist
}

// reachCost is the BFS cost to stand next to p (or on it), or -1 when
// no adjacent cell is reachable. Works even when p itself is impassable,
// like a mushroom molded under.
func (w *World) reachCost(dist []int32, p Point) int {
	best := int32(-1)
	if w.InBounds(p) {
		best = dist[p.Y*w.Width+p.X]
	}
	for _, n := range neighbors {
		q := Point{p.X + n.X, p.Y + n.Y}
		if !w.InBounds(q) {
			continue
		}
		if d := dist[q.Y*w.Width+q.X]; d >= 0 && (best < 0 || d < best) {
			best = d
		}
	}
	return int(best)
}

// sidestep picks a detour when the BFS step is blocked by another creature:
// the nearest free neighbor that does not walk away from target. The BFS
// ignores occupancy and is deterministic, so two creatures wedged head-on
// each sit on the cell the other's path demands and both wait forever;
// stepping aside breaks that symmetry. Reports false when hemmed in.
func (w *World) sidestep(e *Entity, target Point) (Point, bool) {
	cur := Dist(e.Pos, target)
	var best Point
	bestD := 0
	found := false
	for _, n := range neighbors {
		p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
		if !w.InBounds(p) || !w.Passable(w.At(p)) || w.FaunaAt(p) != nil {
			continue
		}
		d := Dist(p, target)
		if d > cur {
			continue // never retreat from the target to dodge
		}
		if !found || d < bestD {
			best, bestD, found = p, d, true
		}
	}
	return best, found
}

// walkStep advances e one cell toward target along the BFS path, sidestepping
// a blocker when the path cell is taken. Reports false when the entity cannot
// move at all this step.
func (w *World) walkStep(e *Entity, next, target Point) bool {
	step := next
	if w.FaunaAt(step) != nil {
		alt, ok := w.sidestep(e, target)
		if !ok {
			return false // hemmed in, wait
		}
		step = alt
	}
	delete(w.occ, e.Pos)
	e.Pos = step
	w.occ[e.Pos] = e.ID
	w.markDirty(e.ID)
	return true
}

// pathToward walks the entity along BFS shortest paths, stopping when
// adjacent to the target. Unlike the greedy move it routes around
// obstacles such as mold pockets.
func (w *World) pathToward(e *Entity, target Point) {
	e.MoveAcc += w.moveSpeed(e)
	for e.MoveAcc >= 1 && !adjacent(e.Pos, target) {
		e.MoveAcc--
		next, ok := w.nextStepToward(e.Pos, target)
		if !ok {
			return
		}
		if !w.walkStep(e, next, target) {
			return
		}
	}
}

func (w *World) move(e *Entity, ref Point, away bool) {
	e.MoveAcc += w.moveSpeed(e)
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		best := e.Pos
		bestD := Dist(e.Pos, ref)
		for _, n := range neighbors {
			p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
			if !w.InBounds(p) || !w.Passable(w.At(p)) || w.FaunaAt(p) != nil {
				continue
			}
			d := Dist(p, ref)
			if (!away && d < bestD) || (away && d > bestD) {
				best, bestD = p, d
			}
		}
		if best == e.Pos {
			return
		}
		delete(w.occ, e.Pos)
		e.Pos = best
		w.occ[best] = e.ID
		w.markDirty(e.ID)
	}
}

func (w *World) wander(e *Entity) {
	e.MoveAcc += w.moveSpeed(e)
	for e.MoveAcc >= 1 {
		e.MoveAcc--
		n := neighbors[w.RandN(len(neighbors))]
		p := Point{e.Pos.X + n.X, e.Pos.Y + n.Y}
		if w.InBounds(p) && w.Passable(w.At(p)) && w.FaunaAt(p) == nil {
			delete(w.occ, e.Pos)
			e.Pos = p
			w.occ[p] = e.ID
			w.markDirty(e.ID)
		}
	}
}

// darkStep sends a creature standing in an unlit cell toward the nearest
// living light source. In a world with no light at all it does nothing.
func (w *World) darkStep(e *Entity) bool {
	if w.spec(e).LightRadius > 0 {
		return false // carries its own light; never caught in the dark
	}
	if w.Lit(e.Pos) {
		return false
	}
	var light *Entity
	bestD := 1 << 30
	for _, c := range w.entities() {
		if c.Dead {
			continue
		}
		s := w.spec(c)
		if s == nil || s.LightRadius <= 0 {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			light, bestD = c, d
		}
	}
	if light == nil {
		return false
	}
	e.Action = "fleeing the dark"
	w.pathToward(e, light.Pos)
	return true
}

func (w *World) fleeStep(e *Entity) ([]Event, bool) {
	me := w.spec(e)
	if me.FearRadius <= 0 {
		return nil, false
	}
	var threat *Entity
	bestD := me.FearRadius + 1
	for _, c := range w.entities() {
		if c.Dead || c.ID == e.ID {
			continue
		}
		cs := w.spec(c)
		if !typeEatsProduceOf(cs, me) {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			threat, bestD = c, d
		}
	}
	if threat == nil {
		return nil, false
	}
	var evs []Event
	if e.Action != "fleeing" {
		evs = append(evs, Event{
			Tick: w.Tick, Type: "fled",
			Actor: e.ID, ActorType: e.Type,
			Target: threat.ID, TargetType: threat.Type,
			Msg: fmt.Sprintf("%s fled from %s", me.Name, w.cfg.Types[threat.Type].Name),
		})
	}
	e.Action = "fleeing"
	w.moveAway(e, threat.Pos)
	return evs, true
}

func (w *World) huntStrike(e *Entity, prey *Entity) []Event {
	s := w.cfg.Types[e.Type]
	ev := w.kill(prey, "killed", fmt.Sprintf("%s was killed by %s", w.cfg.Types[prey.Type].Name, s.Name))
	ev.Target = e.ID
	ev.TargetType = e.Type
	e.Action = "hunting"
	w.markDirty(e.ID)
	hunt := Event{
		Tick: w.Tick, Type: "hunted",
		Actor: e.ID, ActorType: e.Type,
		Target: prey.ID, TargetType: prey.Type,
		Msg: fmt.Sprintf("%s hunted down %s", s.Name, w.cfg.Types[prey.Type].Name),
	}
	return []Event{hunt, ev}
}

func (w *World) shelterStep(e *Entity) bool {
	s := w.cfg.Types[e.Type]
	if len(s.Shelters) == 0 {
		return false
	}
	if e.Home == nil {
		want := map[string]bool{}
		for _, r := range s.Shelters {
			want[r] = true
		}
		var best *Entity
		bestD := 1 << 30
		for _, id := range w.SortedIDs() {
			c := w.Entities[id]
			if c.ID == e.ID || c.Dead {
				continue
			}
			ok := false
			for _, p := range c.Produces {
				if want[p.Resource] {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			if d := Dist(e.Pos, c.Pos); d < bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			return false
		}
		h := best.Pos
		e.Home = &h
		w.markDirty(e.ID)
	}
	if Dist(e.Pos, *e.Home) > s.HomeRange {
		e.Action = "going home"
		w.moveToward(e, *e.Home)
		return true
	}
	return false
}

// haulStep carries mined ore to the nearest living market and sells it for
// colony gold. With fullOnly the dwarf only sets out once the bag is full;
// otherwise it dumps whatever it has left. Returns (events, true) when the
// tick was spent hauling or depositing. A missing market returns false so
// the dwarf keeps mining and its ore simply accumulates harmlessly.
func (w *World) haulStep(e *Entity, fullOnly bool) ([]Event, bool) {
	s := w.cfg.Types[e.Type]
	if s.CarryCapacity <= 0 || e.Ore <= 0 {
		return nil, false
	}
	if fullOnly && e.Ore < s.CarryCapacity {
		return nil, false
	}
	var market *Entity
	bestD := 1 << 30
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.Dead {
			continue
		}
		cs, ok := w.cfg.Types[c.Type]
		if !ok || !cs.Market {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			market, bestD = c, d
		}
	}
	if market == nil {
		return nil, false
	}
	if adjacent(e.Pos, market.Pos) {
		n := e.Ore
		w.Gold += n
		w.GoldMined += n // the level bar moves at the market, not the rock
		e.GoldStrikes = append(e.GoldStrikes, GoldStrike{Tick: w.Tick, Amount: n})
		w.GoldLast24h(e)
		e.Ore = 0
		e.Action = "selling"
		w.setTarget(e, 0)
		w.markDirty(e.ID)
		return []Event{{
			Tick: w.Tick, Type: "sold", Actor: e.ID, ActorType: e.Type,
			Amount: n,
			Msg:    fmt.Sprintf("%s sold %d ore", s.Name, n),
		}}, true
	}
	w.setTarget(e, market.ID) // the client ring shows where the haul is headed
	e.Action = "hauling ore"
	w.pathToward(e, market.Pos)
	return nil, true
}
