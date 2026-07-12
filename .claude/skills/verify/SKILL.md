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

The whole map is one `<canvas>` (no DOM per entity). Prefer finding entities via `/api/entities` and converting tile to canvas px (tile * 12); pixel-scanning via `getImageData` also works with data-driven colors. Terrain colors come from `data/terrain.toml` (grass `#3d5a36`, dirt `#6b5537`, water `#2b4a63`, rock `#3a3a3a`, floor `#26221e`, soft rock `#575049`, mold `#7a8a4d`); the snapshot ships this table as `terrainTypes` and the client renders each cell from it. Soft rock (`#575049`) is a faster-mined vein (43200 hit points vs 172800 for plain rock in `data/terrain.toml`) that worldgen grows as connected blobs; it only appears in newly generated worlds. Mold (`#7a8a4d`, index 6) is a very weak block (6 hit points, ~3 seconds to mine) that worldgen lays as a full crust ring around the clearing (every rock cell 8-adjacent to the clearing's dirt becomes mold at gen time, `crust`/`crust_chance` in `data/gen.toml`), so day-one dwarves chew through it fast; it drops gold only 10% of the time (vs 90% for rock). The clearing dirt reaches tile radius ~12 from the center (32,32) and the mold crust ring sits just outside it at radius ~12-13, beyond the campfire's radius-8 light, so the crust starts DARK and un-mineable; a torch (light radius 5) placed on a passable dirt tile within 5 of a crust cell lights it and gives a spawned dwarf its first mineable face (this is the day-one loop: spawn, torch the crust, watch the dwarf dig). Mold also has a spread rule (`spread_minutes = 20`): each mold block has a small per-tick chance to convert one random 8-neighbor that is passable, UNLIT, and fauna-free into mold, so mold creeps into dark passable space and torch/campfire light suppresses it. The crust and spread only appear in worlds generated after these config changes (a server restart plus a world reset), never in an old save loaded from `world.json`. Entity and structure colors come from `data/entities.toml` (dwarf `#d9a066`, mushroom `#c4b5d9`; structures: campfire `#e25822`, torch `#ffb347`; mining bar track `#1a1815`, fill `#ffb347`; dead `#443c38`, selection box `#ffd75e`). Unlit cells are covered by a darkness veil (`rgba(0, 0, 0, 0.75)` drawn over the terrain), so only cells inside a light circle read at full brightness; pixel-scan lit cells (near a campfire or torch) when checking terrain colors. Tiles are 12 canvas px; the canvas is CSS-scaled, so convert with `canvas.getBoundingClientRect()` before `page.mouse.click`.

Useful DOM handles: `#popup` (entity inspector popup inside `#map`), `#pops` (population labels), `#events` (event feed with tick numbers), `#timescale button:text-is("64x")` (speed buttons: pause/1x/8x/64x), `#levelbox` (the colony level bar: `#levellabel` "Lv N" + `#levelfill`), `#claimcard` (the pending-upgrade card: `#claimtext`, `#claimmore`, `#claim-btn`), `#recap` (the welcome-back toast).

## Colony levels and claimed upgrades

`data/upgrades.toml` is now a level CURVE plus a random draw POOL (a NEW REQUIRED data file: a data dir missing it fails to boot; refresh every scratch/fixture dir from `data/`). Two scalars define the curve: `level_base` (mined gold for the first level) and `level_growth` (each level costs `level_growth`x the last, rounded up). Each `[[upgrade]]` is a pool entry: `name`, `kind` (`damage`, `luck`, or `weapon`), `amount`, `max` (0 = unlimited), and weapons add `color`/`radius`/`period_ms`. Cumulative MINED gold (`GoldMined`, lifetime, never spent) drives levels; the ws snapshot and every tick ship `level`, `goldMined`, `prevLevelGold`, `nextLevelGold`, `pending` (a FIFO string list), `claims` (name->count), and the whole `upgrades` table. `prevLevelGold`/`nextLevelGold` ride the wire so the client never hardcodes the curve. Each level reached appends ONE random ELIGIBLE draw to `pending` (eligibility counts pending+claimed occurrences against `max`, so a maxed upgrade never re-draws) and fires a "level" event.

Level bar `#levelbox` (`#levellabel` "Lv N" + `#levelfill`) fills from `(goldMined - prevLevelGold) / (nextLevelGold - prevLevelGold)`. Claim card `#claimcard` is hidden unless a pending draw exists AND the player has a living dwarf; it names the OLDEST pending in `#claimtext` ("Level N reached: <name>") plus `#claimmore` "+K more waiting" when >1 is queued, and `#claim-btn` sends `{type:"claim", player}`.

CRITICAL - pending draws are INERT until claimed. `claimUpgrade` pops `pending[0]` and bumps `claims[name]`; only CLAIMED upgrades feed `World.MineBonus` (summed `damage`+`weapon` amount, boosts every miner's per-tick damage) and `World.LuckBonus` (summed `luck` amount, widens the gold drop bounds `gold_min`/`gold_max`). Ground truth is the ws `mining` map at 1x (one tick message per sim tick): a mined cell's damage climbs by `mine_damage + MineBonus` per tick - +1/tick while a draw sits UNCLAIMED (claims stay empty), then +2/tick after claiming a `damage`/`weapon` amount-1 upgrade (mold's 6 hit points then break in 3 ticks instead of 6). A claimed Lucky Veins widens drops (gold_max 3 -> 4). Higher timescales batch N ticks per broadcast, so sample deltas at 1x. Claiming is FIFO: the card advances to the next oldest. Everything persists in the save; a fresh world starts at level 0 with empty pending/claims. The recap toast appends " N upgrades await your claim!" when pending is non-empty on reconnect.

Weapon orbits: a claimed `weapon` upgrade (Chisel `#e8d44d`, Hammer `#b87333`) draws an EXTRA tool sprite orbiting each MINING dwarf (`action=="mining"`) at that upgrade's `radius`/`period_ms`, alongside the base pick orbit (`#cfd6dd`, radius 14). Orbits render only while a dwarf is mining, and once MineBonus is high mold breaks in ~2 ticks so bursts are brief - pause (timescale 0) to freeze a miner mid-swing before screenshotting.

The economy is a torch-driven loop: mold only forms in DARK passable cells, and a dwarf can only mine LIT mineable faces, so each torch lights a finite mold pocket (a handful of gold at mold's 10% gold_chance) that the dwarf mines out and then idles. Sustained leveling means re-torching freshly-spread dark mold; to fast-forward, place torches on dense rock-free mold pockets then `POST /api/advance`.

## The recap toast (returning players)

The world tracks lifetime counters (`BlocksMined`, `GoldMined`, `MoldGrown`); each player record snapshots them as `Seen*` on every `hello`. On reconnect the server sends a `{type:"recap", ticks, blocks, gold, mold}` message with the deltas since that player was last seen, then advances the snapshot (deltas are clamped at zero so a post-reset stale snapshot never goes negative; `resetWorld` also scrubs `Seen*`). The client only shows the `#recap` toast when `ticks >= 120` (>= 60s of absence) AND at least one of blocks/gold/mold is non-zero, so reconnect floods never stack a toast. The toast reads "While you were away (Nm): X blocks mined, Y gold mined, Z tunnels molded over", auto-hides on a wall-clock timer (ticks cannot reset the fade), and dismisses on click. To exercise it: connect once (sets the baseline), close the page, `POST /api/advance?ticks=5000`, reopen with the SAME browser profile/localStorage token; the toast appears. Reload again immediately and it does not (the snapshot just advanced, under the 120-tick floor). A death-respawn carries the old `Seen*` forward so dying fakes no recap.

## Gotchas

- The world is deliberately slow (one rock cell takes ~1 real day, dwarves step once per ~2 minutes at 1x). Use `POST /api/advance?ticks=N` to fast-forward instead of waiting at 64x; connected clients receive a fresh snapshot afterwards.
- `data/terrain.toml` colors are authoritative for terrain, `data/entities.toml` for entities and structures; update the color list above if either changes.
- Mining is AOE: a mining dwarf damages EVERY adjacent lit mineable face each tick at full mine_damage, so several progress bars fill at once around one miner and gold events can arrive in bursts. Floating numbers pop per swept face; swept faces without a mining entry (unlit) get debris only.
- Mining is integer damage against terrain hit points: the ws snapshot `mining` map carries int damage per cell, and the bar fraction is damage / the terrain's `hitPoints` from `terrainTypes` (rock 172800, soft rock 43200 in `data/terrain.toml`). Dwarf `mine_damage` (1 per tick) lives in `data/entities.toml`.
- Floating damage numbers (`#e8e2d8`, 9px, ~1s lifetime: 400ms rise then 600ms fade) pop just above a struck face on each tool-orbit strike (~every 1.6s at 1x, faster at higher scales). The first strike on a cell sets a silent baseline, so numbers appear from the second strike onward; expect ~3 at 1x and ~60 at 64x per pop.
- Mining a cell rolls for gold. `gold_chance` is now a per-terrain field in `data/terrain.toml` (rock `0.9`, soft rock `0.9`, mold `0.1`), NOT a global knob; `data/sim.toml` keeps only `gold_min` (1) and `gold_max` (3). A finished cell rolls its terrain's `gold_chance` to drop `gold_min`..`gold_max` colony gold, so gold count runs ahead of cells mined and mold (10%) yields gold far more rarely than rock (90%). Placing a torch spends 1 colony gold.
- Thought bubbles over dwarves are intermittent by design: visible ~3s out of every 60s per dwarf (phase-offset by entity id). A missing bubble is usually just the cadence window; sample for at least 60s or check the entity's soc/g24/action via the API instead. Thought copy and conditions live in the `thoughts` list in `data/entities.toml`.
- Entity tick data arrives over `/ws`; confirm liveness by counting websocket frames or watching `#events` tick numbers, not by expecting positions to change.
- `/favicon.ico` 404s in the console; pre-existing, ignore.
- "64x" yields only a few ticks/sec observed client-side, not 128/s; don't treat slow ticks as breakage.
