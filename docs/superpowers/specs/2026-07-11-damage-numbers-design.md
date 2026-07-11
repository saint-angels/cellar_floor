# Damage Numbers Design

Date: 2026-07-11

## Purpose

Vampire Survivors style floating damage numbers when a dwarf's tool
strikes a rock face, backed by an integer hit-point model: rock health
becomes `hit_points` (int) in terrain data and dwarves deal `mine_damage`
(int) per tick, replacing the float progress fraction.

Decision from brainstorming: tick-granular HP. Rock 172800 hp, soft rock
43200 hp, dwarf damage 1 per tick, so mining time is unchanged and the
number shown per visual strike is the damage accrued since the last
strike: about 3 at 1x, 13 at 8x, 60 at 64x.

## Data

- `terrain.toml`: mineable entries swap `mine_factor` for
  `hit_points` (int, required positive when mineable), with a comment
  explaining the time math (hp at 1 damage per tick, 2 ticks per second:
  172800 = 24h, 43200 = 6h). `TerrainType.MineFactor` is deleted;
  `HitPoints int` (toml `hit_points`, json `hitPoints`) replaces it.
  `data.CanonicalTerrain()` gives rock 172800.
- `entities.toml`: dwarf swaps `mine_hours = 24` for `mine_damage = 1`.
  `EntityType.MineTicks`/`MineHours` are deleted; `MineDamage int`
  (toml `mine_damage`, json `mineDamage`) replaces them, non-negative,
  and `resolveTimes` drops the mine conversion. Mining capability now
  gates on `MineDamage > 0`.

## Sim

- `World.MineProgress map[int]float64` becomes
  `MineDamage map[int]int` (json `mineDamage,omitempty`): damage dealt
  per cell. Completion when `damage >= hit_points` of the cell's
  terrain. The per-tick accrual in `mineStep` adds `miner.MineDamage`.
- Everything around it is untouched: target picking, lit-face gate,
  claims, gold roll on completion, decay of progress is still "none"
  (damage persists on a cell until mined out).
- Save compatibility: the old `mineProgress` float map is dropped on
  load (field renamed); at most a day of partial progress on actively
  mined cells is lost, once. Terrain and entity data are unaffected.

## Wire

The `mining` map in SnapshotMsg and TickMsg carries int damage instead
of a float fraction (same json key). `terrainTypes` already streams;
`hitPoints` rides along, so the client computes the progress bar as
`damage / hitPoints` (bars look identical) and damage deltas exactly.

## Client

- Progress bar: fraction = `mining[i] / terrainTypes[terrain[i]].hitPoints`.
- Floating numbers in fx.ts:
  - Trigger: the existing orbit-strike edge detection on the miner's
    target cell. On each strike, pop the damage accrued on that cell
    since the last pop (`mining[cell] - lastShown[cell]`), an int.
  - First observation of a cell sets the baseline silently (no giant
    number after reloads); pops start from the second strike.
  - Completion: when a tracked cell leaves the mining map and its
    terrain is floor, pop the remainder (`hitPoints - lastShown`), then
    forget the cell.
  - Animation: spawns centered above the rock cell; phase 1 rises ~8 px
    over 400 ms with alpha easing 0 to 1 (ease-in quad on both position
    and alpha); phase 2 holds position and fades alpha 1 to 0 over
    600 ms. 9 px monospace, white (#e8e2d8), pooled with a cap (40
    concurrent) alongside the particle system; clock pauses with the
    existing fx pause behavior.

## Out of scope

Tool upgrades that raise mine_damage (this feature just makes them
possible), damage variance or crits, numbers for events other than
mining, sound.

## Testing

- data: hit_points parses and validates (mineable without positive hp
  rejected); mine_damage parses; old field names absent from data files.
- sim: damage accrual completes a 10 hp test rock in 10 ticks at damage
  1; a 5 hp soft cell finishes first; completion still rolls gold and
  fires events; save round-trip keeps MineDamage; a mid-mine cell keeps
  its damage when the miner is interrupted.
- server: mining map carries ints.
- client: build clean; headless e2e on an isolated port: advance while a
  dwarf mines, assert a damage number appears above the face (pixel or
  screenshot evidence), assert the progress bar still fills, and at a
  higher timescale the popped number is larger.
