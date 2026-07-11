---
name: verify
description: Build, run, and drive Cellar Floor to verify client or server changes end to end
---

# Verifying Cellar Floor changes

## Build and launch

```bash
cd client && npm run build && cd ..   # tsc + vite build into client/dist
go run ./cmd/cellarfloor              # serves client/dist and /ws on :8080, run in background
```

The server loads the persisted world from `world.json` (log line: "loaded world at tick N"). Use `-fresh` for a new world, `-seed` to control generation. Check readiness with `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/`.

## Inspecting sim state (prefer this over pixel-scanning)

Read-only JSON endpoints on the running server:

```bash
curl -s localhost:8080/api/state                          # tick, timeScale, pops, entity count
curl -s 'localhost:8080/api/entities?type=dwarf&alive=true'   # EntityView list, filters combinable
curl -s 'localhost:8080/api/entities?type=torch'              # type= matches any entity type, structures too (campfire, torch, dwarf)
curl -s localhost:8080/api/entities/1481                  # one entity; 404 unknown id, 400 non-numeric
curl -s -X POST 'localhost:8080/api/advance?ticks=200000' # fast-forward ~a day; broadcasts a snapshot
```

Entity JSON matches the ws EntityView shape (`id`, `s`, `x`, `y`, `dead`, `full`, `action`, `home`, `res`). Use these to find a creature's tile before clicking it (tile * 12 canvas px), to confirm liveness (tick advances), or to watch populations. Pixel-scanning is only needed to verify what is actually drawn.

## Driving the UI

The Claude Chrome extension may not be connected; headless Playwright against installed Chrome works:

```bash
cd <scratchpad> && PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm install playwright
# then: chromium.launch({ channel: 'chrome', headless: true })
```

The whole map is one `<canvas>` (no DOM per entity). Prefer finding entities via `/api/entities` and converting tile to canvas px (tile * 12); pixel-scanning via `getImageData` also works with data-driven colors. Terrain colors come from `data/terrain.toml` (grass `#3d5a36`, dirt `#6b5537`, water `#2b4a63`, rock `#3a3a3a`, floor `#26221e`, soft rock `#575049`); the snapshot ships this table as `terrainTypes` and the client renders each cell from it. Soft rock (`#575049`) is a faster-mined vein (43200 hit points vs 172800 for plain rock in `data/terrain.toml`) that worldgen grows as connected blobs; it only appears in newly generated worlds. Entity and structure colors come from `data/entities.toml` (dwarf `#d9a066`, mushroom `#c4b5d9`; structures: campfire `#e25822`, torch `#ffb347`; mining bar track `#1a1815`, fill `#ffb347`; dead `#443c38`, selection box `#ffd75e`). Unlit cells are covered by a darkness veil (`rgba(0, 0, 0, 0.75)` drawn over the terrain), so only cells inside a light circle read at full brightness; pixel-scan lit cells (near a campfire or torch) when checking terrain colors. Tiles are 12 canvas px; the canvas is CSS-scaled, so convert with `canvas.getBoundingClientRect()` before `page.mouse.click`.

Useful DOM handles: `#popup` (entity inspector popup inside `#map`), `#pops` (population labels), `#events` (event feed with tick numbers), `#timescale button:text-is("64x")` (speed buttons: pause/1x/8x/64x).

## Gotchas

- The world is deliberately slow (one rock cell takes ~1 real day, dwarves step once per ~2 minutes at 1x). Use `POST /api/advance?ticks=N` to fast-forward instead of waiting at 64x; connected clients receive a fresh snapshot afterwards.
- `data/terrain.toml` colors are authoritative for terrain, `data/entities.toml` for entities and structures; update the color list above if either changes.
- Mining is integer damage against terrain hit points: the ws snapshot `mining` map carries int damage per cell, and the bar fraction is damage / the terrain's `hitPoints` from `terrainTypes` (rock 172800, soft rock 43200 in `data/terrain.toml`). Dwarf `mine_damage` (1 per tick) lives in `data/entities.toml`.
- Floating damage numbers (`#e8e2d8`, 9px, ~1s lifetime: 400ms rise then 600ms fade) pop just above a struck face on each tool-orbit strike (~every 1.6s at 1x, faster at higher scales). The first strike on a cell sets a silent baseline, so numbers appear from the second strike onward; expect ~3 at 1x and ~60 at 64x per pop.
- Mining a rock cell rolls for gold, tuned by `data/sim.toml`: `gold_chance` (0.9), `gold_min` (1), `gold_max` (3). A finished cell has a `gold_chance` chance to drop `gold_min`..`gold_max` colony gold, so gold count runs ahead of cells mined. Placing a torch spends 1 colony gold.
- Thought bubbles over dwarves are intermittent by design: visible ~3s out of every 60s per dwarf (phase-offset by entity id). A missing bubble is usually just the cadence window; sample for at least 60s or check the entity's soc/g24/action via the API instead. Thought copy and conditions live in the `thoughts` list in `data/entities.toml`.
- Entity tick data arrives over `/ws`; confirm liveness by counting websocket frames or watching `#events` tick numbers, not by expecting positions to change.
- `/favicon.ico` 404s in the console; pre-existing, ignore.
- "64x" yields only a few ticks/sec observed client-side, not 128/s; don't treat slow ticks as breakage.
