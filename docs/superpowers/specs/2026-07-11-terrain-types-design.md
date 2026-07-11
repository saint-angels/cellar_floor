# Terrain Types Design

Date: 2026-07-11

## Purpose

Rock hardness becomes content: terrain moves into `data/terrain.toml`,
and a new soft rock generates as connected blob veins that mine in a
quarter of the time. Terrain colors travel to the client like entity
colors already do.

Decisions from brainstorming: blob veins (not speckle or depth rings);
two rock tiers to start (rock at 1x, soft rock at 0.25x); per-terrain
`mine_factor` multiplies the miner's `mine_hours`.

## Data shape

`data/terrain.toml`, an ordered `[[terrain]]` list where the position is
the byte stored in saves and sent on the wire:

```toml
# order defines the saved terrain value: append new types, never reorder
[[terrain]]
id = "grass"
color = "#3d5a36"
passable = true

[[terrain]]
id = "dirt"
color = "#6b5537"
passable = true

[[terrain]]
id = "water"
color = "#2b4a63"

[[terrain]]
id = "rock"
color = "#3a3a3a"
mineable = true
mine_factor = 1.0

[[terrain]]
id = "floor"
color = "#26221e"
passable = true

[[terrain]]
id = "soft_rock"
color = "#575049"
mineable = true
mine_factor = 0.25   # quarter of the miner's mine_hours: ~6h
```

Fields: `id`, `color`, `passable` (default false), `mineable` (default
false), `mine_factor` (required positive when mineable).

## Validation (load fails loudly)

- The first five entries must be exactly grass, dirt, water, rock,
  floor in that order: pins today's Go constants and every existing
  save. Appending is the only allowed evolution.
- Unique non-empty ids; color required; `mine_factor > 0` when
  mineable; a terrain may not be both passable and mineable.
- Every terrain reference elsewhere (scatter rules, gen vein rules)
  must name an id in the table; the old hardcoded `validTerrains` map
  dies.

## Engine

- `data.Config` gains `Terrain []TerrainType` and
  `TerrainIndex(id string) (int, bool)`.
- `sim.Passable/Mineable/TerrainName` stop being package functions and
  become `World` methods reading the table; out-of-range values are
  impassable, unmineable, named "unknown". The Go constants
  (TerrainGrass..TerrainFloor) stay for the pinned five.
- Mining: a cell's total time is `miner.MineTicks * terrain.MineFactor`
  (progress step `1 / that`). Progress bars and MineProgress keep their
  0..1 semantics unchanged.

## Generation

`data/gen.toml` gains vein rules applied after the base underground
fill (dirt clearing, rock elsewhere), before scatter and the campfire:

```toml
veins = [
  { terrain = "soft_rock", seeds = 10, size = 14 },
]
```

Each seed picks a random rock cell, then grows a connected blob: repeat
`size - 1` times, pick a random cell already in the blob, then a random
neighbor that is still plain rock, convert it. Deterministic through
the world RNG; veins only ever replace rock, never the clearing, water,
or each other.

## Wire and client

`SnapshotMsg` gains `terrainTypes` (the table, json id/color/passable/
mineable/mineFactor). The client stores it on `world` and renders
terrain colors from it; the hardcoded `TERRAIN_COLORS` array in
render.ts is deleted. Unknown indices render black as before.

## Compatibility

Old saves keep working with no reset: indices 0-4 are pinned and
soft_rock is appended as 5. The legacy test fixture directory gains a
terrain.toml with only the canonical five.

## Out of scope

Per-terrain gold odds or drop tables, terrain-specific events, noise
or depth-based hardness, more rock tiers (pure data later), walkable
speed modifiers per terrain.

## Testing

- data: table parses; pinned-order violation rejected; duplicate id,
  missing color, non-positive mine_factor on mineable, passable AND
  mineable each rejected; vein/scatter references validated.
- sim: World Passable/Mineable/TerrainName read the table including an
  appended sixth type; a mine_factor 0.5 face completes in half the
  ticks of a factor 1 face under the same miner.
- gen: veins produce exactly the seeded number of connected blobs of
  the requested size (subject to running out of rock), replace only
  rock, deterministic per seed.
- server: snapshot carries terrainTypes.
- client: build clean; e2e on an isolated port: two rock grays visible
  in the world, a soft-rock face reaches floor after ~0.25 day of
  advance while an adjacent plain-rock face has not; screenshot.
