# Cellar Floor

A little world living on the floor of a dark cellar. A persistent ecology
of grass, bushes, trees, rabbits and wolves, simulated on a Go server and
watched live from the browser.

Spec: docs/superpowers/specs/2026-07-08-cellar-floor-design.md

## Run

    cd client && npm install && npm run build && cd ..
    go run ./cmd/cellarfloor

Open http://localhost:8080. Flags: -addr, -seed, -fresh, -data, -static.

## Develop

    go test ./...                  # includes the 50k-tick stability test
    go test ./... -short           # skip the long run
    cd client && npm run dev       # Vite dev server proxying /ws to :8080

All species and balance live in data/*.toml. The engine knows nothing
about rabbits.
