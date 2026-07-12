# Mold Design

Date: 2026-07-12

## Purpose

Two problems, one organism. New dwarves lock onto day-long rock the
moment the game starts; and dug-out tunnels are inert forever. Mold is a
very weak block (3 seconds to destroy) that worldgen wraps around the
clearing as a full crust, giving the first minutes visible pop-pop-pop
progress, and that spreads through DARK passable space afterward, slowly
growing abandoned tunnels shut. Torch light suppresses it, which turns
torch upkeep into defense.

Decisions from brainstorming: spread only in darkness; the crust fully
rings the clearing; mold drops gold at 10%.

## Terrain data

`terrain.toml` appends (append-only table; mold becomes index 6):

```toml
[[terrain]]
id = "mold"
color = "#7a8a4d"
mineable = true
hit_points = 6       # 3 seconds at 1 damage per tick
gold_chance = 0.1
spread_minutes = 20  # each block converts one dark neighbor per ~20 min
```

## Gold moves per terrain

`gold_chance` leaves `sim.toml` and becomes a per-terrain field
(`TerrainType.GoldChance`, toml `gold_chance`): rock 0.9, soft_rock 0.9,
mold 0.1. Without this, a 3-second block at the global 90% would be a
money printer. `gold_min`/`gold_max` stay in sim.toml. The completion
roll in `mineStep` reads the terrain's chance, captured before the cell
flips to floor. Validation: per-terrain 0 <= gold_chance <= 1;
`SimConfig.GoldChance` and its validation are deleted; the min/max rule
applies when any terrain has a positive chance.

## Spread

New generic engine rule in the tick (a late step, after corpse decay):
every cell whose terrain has `SpreadChance > 0` (derived per tick in
resolveTimes: `1 / (spread_minutes * 60 * tick_rate)`) rolls the world
RNG; on success it picks a random 8-neighbor that is passable, UNLIT,
and free of fauna, and converts it to the spreading terrain via
SetTerrain (streams as a normal terrain diff). Scan in cell-index order
for determinism; a cell converted mid-pass may itself roll later in the
same pass, which is deterministic and accepted. Lit cells are immune,
so the campfire ring and torch-lit corridors never mold over; live
torches keep their own cell lit by definition. Dead torch stubs can be
molded under; they decay anyway.

`spread_minutes >= 0` validated; 0 means the terrain does not spread.

## Crust generation

`gen.toml` gains `crust = "mold"` and `crust_chance = 1.0`
(GenConfig.Crust string, CrustChance float64; validated against the
terrain table, chance 0..1). After the underground fill and before
veins: every rock cell that is 8-adjacent to a dirt cell rolls
`crust_chance` to become the crust terrain. At 1.0 the clearing is
fully ringed. Veins only replace plain rock, so they never overwrite
the crust. The crust sits inside the campfire's lit radius, so day-one
dwarves immediately chew through 3-second blocks with damage numbers
flying before reaching real rock.

Seeded crust cannot spread at generation time even in principle: spread
targets passable cells, and at t0 no dark passable space exists.

## Client

Nothing. Color, bars, floating numbers, debris, and AOE all derive from
the terrain table and the mining map.

## Compatibility

Terrain append keeps old saves loading. The user's existing world gains
mold only after a reset: crust is gen-time, and spread grows FROM
existing mold blocks, so a world with no mold has no source and spread
never starts there. (E2e also showed sources are scarce in practice:
the crust sits inside the campfire's light and gets mined away early,
and dwarves only dig lit tunnels, so dark floor bordering live mold is
rare. A follow-up idea is spontaneous sprouting on dark floor; not in
this feature.) sim.toml loses its gold_chance line in the same commit
that adds the per-terrain ones.

## Out of scope

Mold damaging or slowing dwarves, mold spreading over structures'
cells while lit, spore visuals, mold-specific thoughts, cure items.

## Testing

- data: mold entry parses (hp 6, gold 0.1, spread 20); per-terrain
  gold_chance validation; SimConfig.GoldChance gone; spread_minutes
  non-negative; crust fields validated against the table.
- sim: a mold cell converts an adjacent dark floor cell (seeded RNG,
  deterministic); a lit neighbor is never converted; fauna-occupied
  cells skipped; per-terrain gold: chance-1 terrain always drops,
  chance-0 terrain never drops (event types verified); mold mined out
  in 6 damage.
- gen: every rock cell adjacent to dirt becomes mold at crust_chance 1;
  veins never overwrite crust; determinism per seed.
- e2e: fresh world shows the mold ring (color scan); first floor cell
  appears within the first ~100 ticks of a dwarf reaching the crust
  (versus 43200+ before); after digging and letting a tunnel go dark,
  advance shows mold cells appearing in it; screenshot the ring and a
  reclaimed tunnel.
