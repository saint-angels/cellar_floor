# Torches Design

Date: 2026-07-11

## Purpose

The first player tool that writes to the environment. Players spend colony
gold to place torches; light gates where dwarves mine and darkness scares
them home. Torch placement becomes the steering wheel of the colony: where
you put light is where the dwarves dig.

Decisions made in brainstorming:

- Darkness works as **fear**: dwarves can stray into dark cells but panic
  back toward light; unlit rock faces are never mined.
- Torches are bought from the **shared colony gold** at 1 gold each, by
  players with a living dwarf only.
- Torches **burn out after about one real day** (pace doc: light is upkeep,
  not a one-time build).
- Gold veins are gone; **every mined rock cell rolls a random gold drop**.
  Dwarves mine blind.

## Rename prep: species becomes entity type

A torch is not a living organism, so the "species" concept is renamed to
entity "type" everywhere before torch work starts, as its own commit:

- `data/species.toml` becomes `data/entities.toml` with `[type.dwarf]`
  style sections.
- `data.Species` becomes `data.EntityType`; `Entity.Species` becomes
  `Entity.Type`; config map `Species` becomes `Types`.
- The wire carries `types` instead of `species` in snapshots; the client
  renames its `Species` type and `e.s` stays the short entity-type id key.
- The legacy fixture in `internal/sim/testdata/legacy/` is renamed to the
  new shape too; it exists to keep engine regression tests running, not to
  preserve old field names.
- DESIGN.md's "engine knows no species" constraint is reworded to "engine
  knows no entity types".

The save file changes shape. The world is days old, so this takes a
one-time world reset, not a migration shim.

## Torch and campfire as entity types

One new generic data field and one new kind:

- `light_radius` (int, cells): any entity type may emit light. Zero means
  none.
- `kind = "structure"`: static like flora but inedible; no produces, no
  hunger, no AI. The engine skips metabolism and behavior for structures.

```toml
[type.torch]
name = "Torch"
kind = "structure"
color = "#ffb347"
light_radius = 5
lifespan = 172800   # ~1 real day at tick_rate 2.0
decay_ticks = 3600  # burnt stub lingers ~30 min, then removed

[type.campfire]
name = "Campfire"
kind = "structure"
color = "#e25822"
light_radius = 8
lifespan = 0        # never burns out
```

Worldgen places exactly one campfire at the clearing center. Radius 8 over
the radius-6 dirt clearing keeps the whole clearing plus the first ring of
rock faces lit, so mining works from day one with no torches.

Burnout reuses the existing lifespan kill and corpse decay; a dead torch
renders dark briefly and disappears.

## Light model

The world keeps a derived `lit` bitfield (not persisted, rebuilt on load):
a cell is lit when inside the Euclidean circle of any living light source,
`dx*dx + dy*dy <= r*r`, the same math the clearing generator uses.
Recomputed only when a light source spawns or dies; those are rare events.

## Dwarf behavior

- **Fear of the dark** slots in as the top AI priority, above fleeing
  predators: a dwarf standing in an unlit cell sets action "fleeing the
  dark" and greedy-moves toward the nearest living light source, reusing
  the existing move machinery. All other behaviors (wander, food seeking)
  may freely walk into darkness; the panic on the next ticks produces the
  frontier jitter we want.
- **Mining gate**: `pickMineTarget` only considers lit faces; `mineStep`
  drops a target whose face has gone dark. Partial progress stays in
  `MineProgress`, so a relit face resumes where it left off.
- `gold_sense` and the gold-biased face scoring are deleted. Target picking
  becomes: nearest lit face by BFS distance, ties by cell index.

## Economy: gold is a random drop

No gold terrain exists. Worldgen places no veins, `TerrainGold` is removed
from the enum, and `Mineable` is rock only. When a rock cell finishes
mining it rolls: with `gold_chance` the colony pot gains a uniform integer
in `[gold_min, gold_max]`, and a "struck gold" event fires ("Misha's dwarf
struck gold"). No drop means no event; the cell just becomes floor.

Knobs in `sim.toml`, all placeholder numbers for later balancing, and a
natural hook for future tool upgrades (luckier picks):

```toml
gold_chance = 0.9
gold_min = 1
gold_max = 3
```

Bootstrap: the ~50 lit rock cells around the campfire hold an expected
~90 gold at these numbers, so the colony affords its first torches within
the first mined cells and light budget is never deadlocked.

## Protocol and intents

New ws client intent:

```json
{ "type": "torch", "player": "<token>", "x": 12, "y": 40 }
```

Server validation, in order: known player with a living dwarf, gold >= 1,
cell in bounds and passable, no living structure already on the cell. On
success: gold decreases by 1, a torch entity spawns there, a "placed"
event fires ("Misha placed a torch"). On failure: an error replies on that
connection only, reusing the spawn-error path ("not enough gold"). Note
there is no lit-area restriction on placement: any passable cell works,
including deep in the dark.

No new snapshot fields: torches stream as entities, `light_radius` rides
in the types map, and the client derives light itself.

## Client

- **Darkness veil**: the client computes the same lit set from entities
  with `light_radius` and draws a translucent dark layer (about 75% black)
  over unlit cells, above terrain and mining bars but below creatures, so
  a dwarf lost in the dark stays visible. Recomputed only when an entity
  with a light radius appears, moves, or dies.
- **Torch rendering**: structures draw as a small flame-colored pixel;
  fx.ts adds a subtle flicker glow around living light sources.
- **Gold sparks**: fx.ts listens to "struck gold" events and bursts gold
  debris at that dwarf's position, replacing the old terrain-keyed gold
  debris color. Terrain color tables lose their gold entry.
- **Placement UI**: a "Torch (1 gold)" button near the gold counter,
  enabled only when the player's dwarf is alive and gold >= 1. Clicking
  arms placement (crosshair cursor); the next canvas click sends the torch
  intent; Esc or clicking the button again disarms. Server errors show in
  the existing error line.

## Testing

Go, table-driven where sensible:

- Light field: circle math, recompute on spawn/death, rebuilt on load.
- Structures: no metabolism, no AI, lifespan kill and decay for torches,
  campfire immortal.
- Fear: dwarf in an unlit cell moves toward the nearest light; dwarf in
  light never triggers it.
- Mining gate: only lit faces picked; target dropped when its light dies;
  progress resumes when relit.
- Gold drop: chance and range respected (seeded RNG), event fired, no
  gold terrain generated.
- Torch intent: each validation rule, gold decrement, event decoration.
- Rename: config loads `[type.x]` sections; legacy fixture still passes
  engine regression tests.

Client: build clean, then headless Playwright on an isolated port with a
scratch data dir: place a torch over ws, assert the veil darkens unlit
cells and lit cells stay bright, advance a day with /api/advance and
assert the torch expires, and assert a dwarf standing in darkness returns
to light. Screenshot for the record.

## Out of scope

Torch pickup or moving, per-player torch ownership, light falloff
gradients, dwarves carrying light, tool upgrades, gold milestone rewards.
