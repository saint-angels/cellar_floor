package sim

// companionInRadius returns the nearest living same-type entity within r,
// ties broken by lowest id via the sorted scan order.
func (w *World) companionInRadius(e *Entity, r int) *Entity {
	var best *Entity
	bestD := r + 1
	for _, id := range w.SortedIDs() {
		c := w.Entities[id]
		if c.ID == e.ID || c.Dead || c.Type != e.Type {
			continue
		}
		if d := Dist(e.Pos, c.Pos); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// nearestCompanion returns the nearest living same-type entity anywhere.
func (w *World) nearestCompanion(e *Entity) *Entity {
	return w.companionInRadius(e, w.Width+w.Height)
}

// socialStep handles loneliness: seek the nearest companion when the
// meter is below threshold, then stay socializing until full. Returns
// true when the entity spent this tick on company.
func (w *World) socialStep(e *Entity) bool {
	s := w.cfg.Types[e.Type]
	if s.SocialSize <= 0 {
		return false
	}
	if c := w.companionInRadius(e, s.SocialRadius); c != nil {
		wasSocial := e.Action == "socializing" || e.Action == "seeking company"
		if e.Social < s.SocialSize && (wasSocial || e.Social < s.SocialThreshold) {
			e.Action = "socializing"
			w.markDirty(e.ID)
			return true
		}
		return false
	}
	if e.Social >= s.SocialThreshold {
		return false
	}
	target := w.nearestCompanion(e)
	if target == nil {
		return false
	}
	e.Action = "seeking company"
	w.moveToward(e, target.Pos)
	return true
}
