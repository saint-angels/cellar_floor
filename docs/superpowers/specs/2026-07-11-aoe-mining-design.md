# AOE Mining Design

Date: 2026-07-11

## Purpose

The orbiting pickaxe hits every block it sweeps through: a dwarf in the
mining state damages ALL mineable, lit cells adjacent to it each tick,
not just its claimed target. Full damage to every cell (decision from
brainstorming), making multi-face digging and its gold income roughly
3x when faces are available. This is the new baseline pacing.

Depends on the damage-numbers feature (integer hit points,
`World.MineDamage`, per-cell floating numbers); executes after it.

## Sim

In `mineStep`, when the dwarf is adjacent to its claimed target (the
branch that today damages only the target):

- Collect every cell within Chebyshev distance 1 of the dwarf whose
  terrain is mineable and lit, sorted by cell index for determinism.
- Each collected cell takes the full `mine_damage`:
  `MineDamage[i] += s.MineDamage`; on reaching its terrain's
  `hit_points`, the cell completes exactly like today: leave the damage
  map, become floor, roll gold, fire the mined/gold event with this
  dwarf as actor. Multiple completions in one tick are allowed and
  fire in cell-index order.
- The claimed target keeps its existing roles unchanged: pathing
  destination, claim uniqueness (one dwarf per face), and the
  drop-when-dark/unmineable validation. If splash (own or another
  dwarf's) finishes the target first, the existing validation clears
  the claim next tick.
- The lit requirement on every splashed cell keeps torches as the
  steering mechanism; unlit neighbors take nothing.
- No new data fields: AOE strength IS `mine_damage`. A future tool
  system can add a radius or falloff knob if ever needed.

## Client

- Progress bars and floating damage numbers are already per-cell
  (driven by the `mining` map) and need no changes.
- fx.ts strike detection widens: instead of watching only `e.mt`, the
  orbit strike fires for whichever mineable cell the tool is
  geometrically inside at the crossing, keyed per dwarf per cell
  (`wasInside` keyed by `id * 100000 + cell` or a nested map). Debris
  and the damage-number pop come from the struck cell. A sweep through
  three faces rattles all three per revolution.

## Out of scope

Splash falloff or radius knobs, friendly-fire on structures or fauna,
AOE for anything other than mining, tool upgrades.

## Testing

- sim: a dwarf boxed by three lit faces damages all three each tick;
  all three complete in the same tick with three events in cell-index
  order and three gold rolls; an unlit adjacent face takes zero; a
  face claimed by another dwarf still takes splash; the claim system
  still assigns distinct targets; the soak stays green (legacy fauna
  have mine_damage 0).
- client: build clean; e2e on an isolated port: dwarf mining a corner
  shows bars on multiple cells simultaneously and floating numbers over
  more than one face (screenshot); a lone face still behaves as before.
