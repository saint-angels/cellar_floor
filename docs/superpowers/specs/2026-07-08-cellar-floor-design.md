# Cellar Floor: Design (v1 "Terrarium")

Date: 2026-07-08
Status: Draft for review

## Vision

Cellar Floor is a persistent, server-hosted ecology observed live from the browser. Creatures live, eat, shelter, reproduce, and die according to a systemic resource-relationship model inspired by Ultima Online's unshipped resource system (production / food / shelter / desire). The long-term game is tamagotchi + dwarf fortress + idle: each player has a character that is just another creature in the ecology, keeps living while the player is offline, can permanently die, and is influenced only indirectly by shaping the environment.

v1 deliberately builds none of the player layer. v1 is the terrarium: a procedurally generated world where grass, berry bushes, rabbits, and wolves live together, and any number of browsers can connect and watch.

### v1 success criteria

- The ecology is alive and interesting to watch.
- Rabbit and wolf populations oscillate (predator-prey waves) rather than collapse or explode, verified by a long-run headless test and by the population graphs in the client.
- Multiple browsers can watch the same world simultaneously.

### Explicitly deferred (later phases, kept unblocked by design)

- Player characters (they will be ordinary creatures with personality parameters, so nothing in v1 special-cases them).
- Environment-shaping verbs and their resource economy.
- Permadeath, login/identity, offline recap ("while you were away").
- Fancy biomes, rivers, weather, seasons.

## Data-driven principle

Everything that defines behavior or balance lives in data files, not Go code. The engine knows about the four relationship sets, the Maslow loop, and terrain; it knows nothing about rabbits. Concretely, a `data/` directory of TOML files (comments matter for tuning) defines:

- **Species**: for each entity type, its produces / eats / shelters / desires sets, AI parameters (hunger threshold, fear radius, speed, stomach size, lifespan, reproduction rule), population floor and cap, and client presentation (color or sprite id, display name).
- **Simulation config**: tick rate, autosave interval.
- **Generation config**: map size, terrain noise thresholds, per-species scatter weights by terrain.

Adding a new species means adding a data entry and zero engine code. The server sends the loaded species table to the client in the snapshot, so the client hardcodes no species either; it renders whatever presentation data it is given. Data files are validated at load with clear errors (unknown resource names, missing fields). Hot reload of data files is a nice-to-have for tuning, not a v1 requirement; restart-to-reload is acceptable.

## World model

- Single fixed tile grid, 64x64 to start.
- Each tile has a terrain type: grass, dirt, water, rock.
- Entities occupy tiles: flora (grass patch, berry bush, tree) and fauna (rabbit, wolf). One fauna entity per tile; flora and fauna may share a tile.

### Procedural generation

`Generate(seed) -> World`. Simple noise for terrain, then scatter flora and initial fauna weighted by terrain. Regenerating from a new seed is the primary tuning tool. The generator can grow (biomes, rivers) without touching the sim, since the sim only sees the resulting World.

## Resource schema

Every entity type declares up to four relationship sets:

- **Produces**: resources it is made of, each with current amount, max, and regrowth rate. A berry bush produces berries and regrows them. A rabbit produces meat and fur, harvestable only by killing it.
- **Eats**: which resources satisfy hunger and how much per bite.
- **Shelters in**: which resource producers it wants to stay near, with a remembered home location.
- **Desires**: optional wants beyond survival, with an aversion flag for fears. Mostly a hook for future player characters; no v1 species uses desires.

Predation is emergent: a wolf's "eats meat" plus a rabbit's "produces meat" equals hunting. Fear is emergent by the same rule: an entity treats as a predator any creature whose Eats includes a resource it Produces. No special cases per species pair, and no explicit predator lists.

### v1 species table

This table is the human-readable summary of what ships in the v1 `data/` files:

| Entity | Produces | Eats | Shelters in | Notes |
| --- | --- | --- | --- | --- |
| Grass patch | grass (regrows) | - | - | ground cover food |
| Berry bush | berries (regrows) | - | - | also shelter for rabbits |
| Tree | wood (static in v1) | - | - | shelter for wolves |
| Rabbit | meat, fur | grass, berries | bush | flees predators |
| Wolf | meat, fur | meat | tree | hunts rabbits |

## AI

One shared decision loop, Maslow-ordered, identical for all fauna:

1. If hunger past threshold: seek food. Pathfind to nearest edible producer; eat it, or hunt it if it is alive.
2. Else if in danger (predator within fear radius) or far from home: seek shelter / flee.
3. Else if it has unmet desires: pursue them.
4. Else wander or idle.

Per-entity parameters (hunger threshold, fear radius, speed) come from the species data files, so a future player character is just a fauna entry with personalized parameters.

Pathfinding: greedy step toward target with simple obstacle avoidance is acceptable for v1; upgrade to A* only if behavior looks dumb in practice.

## Simulation loop

- Single-threaded deterministic tick loop. Base rate 2 ticks/second, scaled by a global time control (pause, 1x, 8x, 64x) that affects all connected clients.
- Each tick: regrow resources, run each entity's AI one step, resolve eating / combat / deaths / births, emit events, broadcast diffs.
- Determinism: all randomness from a seeded RNG owned by the world, so a seed + tick count reproduces a run.
- Population guardrails, crude on purpose: per-species floor (spawn one at map edge when below it) and cap (suppress births at it). Starting values: rabbits floor 4 cap 60, wolves floor 2 cap 15; tune from there. These prevent extinction spirals and explosions while tuning; the goal is that tuned dynamics rarely hit them.
- Every meaningful occurrence (ate, fled, hunted, killed, starved, born) is appended to a world event log. This log later powers the offline recap feature; in v1 it feeds the client event feed.

## Client (spectator)

TypeScript canvas app, no framework requirements beyond what is convenient.

- Renders the tile map with simple colored shapes or minimal sprites. Interpolates entity movement between ticks so 2 Hz does not look like teleporting.
- Click any creature to inspect: needs meters (hunger, fear), current action ("seeking food: berry bush at 12,34"), home location, parameters. This is both the debugging tool and the seed of the future tamagotchi UI.
- Scrolling world event feed.
- Per-species population counters with small sparkline history graphs, to make predator-prey waves visible.
- Time control UI (pause / 1x / 8x / 64x), global for all viewers.

## Architecture

Go server, four packages with clean seams:

- `data`: loads and validates the TOML data files into typed config structs (species table, sim config, gen config). Everything downstream consumes these structs.
- `sim`: pure simulation. World state, entity components (the four relationship sets), AI step, tick function. No networking, no I/O, no species knowledge beyond the loaded species table. `Tick(world) -> events`.
- `gen`: procedural generation. `Generate(seed, genConfig, speciesTable) -> World`.
- `server`: serves static client files over HTTP plus a WebSocket endpoint. Owns the tick loop (one goroutine, ticker scaled by time control), applies sim ticks, broadcasts to clients.

The purity of `sim` is what makes it testable and later portable.

### Protocol

JSON over WebSocket. Readability over efficiency at this scale.

- On connect: full world snapshot, including the species table with presentation data.
- Per tick: diff (entities moved / changed / died / born) plus that tick's events.
- Client to server in v1: time-scale commands only. Inspection reads local state.
- Reconnect: client re-requests a snapshot.

### Persistence

Serialize the whole world to a single file (JSON or gob) every 5 minutes and on shutdown; load on boot, else generate from seed. No database. A corrupt or incompatible save means regenerating the world, which is acceptable for a terrarium.

## Error handling

- The tick loop never dies: recover from panics per tick, log, continue. A creature with a broken brain skips its turn rather than killing the world.
- WebSocket clients that error or disconnect are dropped without affecting the sim.
- Malformed client messages are logged and ignored.

## Testing

The pure, deterministic `sim` package is the strategy:

- Unit tests: hungry rabbit adjacent to bush eats it; wolf paths to and kills nearest rabbit; bush regrows at its rate; fleeing rabbit moves away from wolf.
- Long-run stability test: run 50k ticks from a fixed seed headlessly; assert no species goes extinct or exceeds its cap. This is the permanent ecology-stability regression check.
- Client untested in v1 beyond manual eyeballing.

## Name

"Cellar Floor": a little world living on the floor of a dark cellar. Repo: cellar-floor.
