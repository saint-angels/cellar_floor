# Idle Pivot: Dwarves, Rock, and Gold

Date: 2026-07-10

## Vision and the time-scale principle

The cellar floor is a slow underground world. **Most things happen on a scale of hours or real days.** A dwarf mines one rock cell in about a real day, gets hungry a couple of times a day, and crossing the map is an outing measured in hours. The canonical world runs at 1x wall-clock; the 8x/64x buttons remain as dev and observation tools. Watching live shows an almost-still scene. The payoff of the game is checking back later: new tunnels, a bigger gold counter.

The previous ecology (rabbits, wolves, grass, bushes, trees) worked on a scale of seconds and is removed. The engine's species machinery (hunger, eating, starvation, population floor) stays and is reused.

Roadmap after this increment, each as its own spec:

1. Player-placeable torches (first client-to-server world write).
2. Light field; dwarves prefer lit areas and avoid the dark.

## The map

Generation changes from noise-based landscape to underground:

- Nearly the whole map is `rock` terrain (impassable, mineable).
- A small dirt clearing (radius ~6, roughly centered, `clearing_radius` in `gen.toml`) is the only open space. Dwarves and mushrooms spawn there.
- Rare gold cells (`TerrainGold`, `gold_chance` per rock cell, ~0.005) are scattered through the rock. Impassable and mineable like rock.
- A new passable terrain `floor` is what mining leaves behind. Rendered visibly darker than dirt so tunnels read as carved.
- Water and the old grass terrain are gone from generation (the terrain enum keeps its existing values for wire compatibility; `floor` and `gold` are appended).

The old `world.json` references removed species; the pivot starts a fresh world (`-fresh` once, or delete `world.json`).

## Species

`species.toml` drops grass, bush, tree, rabbit, wolf and defines:

- **Mushroom** (`kind = "flora"`): dwarf food, scattered in the clearing (`chance` ~0.15). Produces `mushroom` (amount 6, max 6) with regrow tuned so a bitten-out mushroom recovers over ~2 real days (regrow ~0.00002 per tick).
- **Dwarf** (`kind = "fauna"`): eats `["mushroom"]`. Retuned to the day scale at `tick_rate = 2.0` (172,800 ticks per real day): `metabolism ~0.00006` (a full stomach of 10 drains in ~1 day), `speed ~0.004` (one tile per ~2 minutes, map crossing in ~2 hours), `starve_ticks ~350000` (~2 days on empty), `pop_floor = 3` (the colony never dies out), `repro_chance = 0` for now. New species fields: `mine_ticks = 172800` (~1 real day per cell) and `gold_sense = 8` (tiles).

All numbers live in TOML and are tuning, not code.

## Mining

New AI step for species with `mine_ticks > 0`, priority after food, before wander:

1. **Pick a target face.** A face is a rock or gold cell adjacent to walkable terrain. If any gold cell (exposed or buried) lies within `gold_sense` tiles of the dwarf, target the face that most reduces distance to that gold; an exposed gold face beats everything. Otherwise target the nearest reachable face. One dwarf per face; a claimed face is skipped by others.
2. **Walk there.** BFS pathfinding over walkable cells (the greedy step-toward movement gets stuck in tunnels; BFS on 64x64 is cheap). Movement still spends `speed` accumulator per tick, so travel takes hours.
3. **Dig.** Each tick adjacent to the face adds `1/mine_ticks` to that cell's progress. Progress is stored in `map[cellIndex]float64` on the world, persists in `world.json`, and survives the dwarf wandering off to eat.
4. **Break through.** At progress >= 1 the cell becomes `floor`. Rock yields nothing; the tunnel is the product. Gold increments the global `Gold` counter by 1 and fires an event ("Dwarf struck gold"). Rock completion fires "Dwarf mined out a rock".

Hunger interrupts mining (existing priority order); the dwarf walks back to the clearing, eats, and returns.

## Protocol and client

Snapshot and tick messages gain:

- `terrain` diffs on ticks: `[{"i": cellIndex, "t": terrainType}]` for cells that changed.
- `mining`: `{cellIndex: progress}` for cells with progress > 0.
- `gold`: the global counter.

Client changes:

- Colors for `floor` (dark carved stone) and `gold` (warm glint) cells; dwarf color from TOML.
- **Mining progress bar**: any cell present in `mining` draws a small bar on top of the cell (2px track across the 12px tile, amber fill proportional to progress). Visible at all times, not only while a dwarf stands there.
- Gold counter in the side panel where populations show today; pops section now lists dwarves.
- Existing popup inspector works unchanged for dwarves (name, position, fullness, action such as "mining").

## Testing

Sim tests tick the world directly (no real-time waits): dwarf claims the nearest reachable face; BFS routes around corners; progress accumulates at 1/mine_ticks and completion converts the cell; gold completion increments the counter and rock does not; gold within `gold_sense` redirects digging; a hungry dwarf abandons the face, eats, and resumes. Protocol tests cover the new tick fields. A time-scale sanity test asserts one cell takes exactly `mine_ticks` adjacent ticks.

Live verification via the debug API (`/api/state` gold and pops, dwarf `action`) and a 64x visual check of the progress bar. At 64x a 1-day cell still needs ~22 minutes, so live checks assert progress accumulating, not completion.

## Out of scope (YAGNI)

- Torches, light, and dark-avoidance (next two specs).
- Stone as a resource, hauling, item entities.
- Dwarf reproduction, sleep, homes; day/night cycle.
- Terrain regeneration; mined is mined forever.
