# Cellar Floor

A little world living on the floor of a dark cellar. Dwarves carve tunnels
through the rock in search of gold, one cell per real day, eating mushrooms
from the clearing where they live. Most things happen on a scale of hours
or days; the world runs at 1x wall-clock on a persistent Go server and is
watched live from the browser.

Design constraints: DESIGN.md
Specs: docs/superpowers/specs/2026-07-10-idle-pivot-design.md (current),
docs/superpowers/specs/2026-07-08-cellar-floor-design.md (original engine)

## Run

    cd client && npm install && npm run build && cd ..
    go run ./cmd/cellarfloor

Open http://localhost:8080. Flags: -addr, -seed, -fresh, -data, -static.

The pivot to the underground world made old rabbit-era saves obsolete;
entities of removed species are dropped on load, but for a proper start
run once with -fresh (or delete world.json).

## Develop

    go test ./...                  # includes the 50k-tick stability test
    go test ./... -short           # skip the long run
    cd client && npm run dev       # Vite dev server proxying /ws to :8080

All species and balance live in data/*.toml. The engine knows nothing
about rabbits.
