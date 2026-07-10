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
curl -s 'localhost:8080/api/entities?species=rabbit&alive=true'   # EntityView list, filters combinable
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

The whole map is one `<canvas>` (no DOM per entity). Prefer finding entities via `/api/entities` and converting tile to canvas px (tile * 12); pixel-scanning via `getImageData` also works with colors from `data/species.toml` (dwarf `#d9a066`, mushroom `#c4b5d9`; terrain: dirt `#6b5537`, rock `#3a3a3a`, floor `#26221e`, gold vein `#c9a227`; mining bar track `#1a1815`, fill `#ffb347`; dead `#443c38`, selection box `#ffd75e`). Tiles are 12 canvas px; the canvas is CSS-scaled, so convert with `canvas.getBoundingClientRect()` before `page.mouse.click`.

Useful DOM handles: `#popup` (entity inspector popup inside `#map`), `#pops` (population labels), `#events` (event feed with tick numbers), `#timescale button:text-is("64x")` (speed buttons: pause/1x/8x/64x).

## Gotchas

- The world is deliberately slow (one rock cell takes ~1 real day, dwarves step once per ~2 minutes at 1x). Use `POST /api/advance?ticks=N` to fast-forward instead of waiting at 64x; connected clients receive a fresh snapshot afterwards.
- `data/species.toml` colors are authoritative; update the color list above if species change.
- Entity tick data arrives over `/ws`; confirm liveness by counting websocket frames or watching `#events` tick numbers, not by expecting positions to change.
- `/favicon.ico` 404s in the console; pre-existing, ignore.
- "64x" yields only a few ticks/sec observed client-side, not 128/s; don't treat slow ticks as breakage.
