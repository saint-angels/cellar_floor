# Cellar Floor

Multiplayer idle colony sim: Go server and sim, TypeScript canvas client,
TOML data. All game content and balance live in data/*.toml; the engine
knows nothing about specific creatures.

## Commands

- Fast tests while iterating: `go test ./... -short` (skips the ~75s
  ecology soak).
- Full gate before any push: `gofmt -l internal/` (must print nothing),
  `go vet ./...`, `go test -count=1 ./...`, and
  `cd client && npx tsc --noEmit && npm run build`.
- Run locally: `./run.sh` (builds client if needed, serves client/dist
  and /ws on :8080).

## Hard rules

- data/terrain.toml is ordered and append-only: the entry index is the
  save and wire byte. Never reorder or delete entries.
- The sim must stay deterministic per seed: randomness only through the
  world RNG inside Step, never wall-clock time.
- Wire changes (internal/server/protocol.go) require a server restart
  and a client hard reload to take effect anywhere.
- Pushing to master IS a production deploy: a GitHub webhook rebuilds
  and restarts the public server at https://cellar-door.zettelwerk.app.
  Only push a fully gated, working tree.

## Verification

Use the verify skill (.claude/skills/verify/SKILL.md) to drive the game
end to end. E2E runs go on an isolated port (:8083) with a scratch data
dir and their own save_path. NEVER run tests against the live :8080
server or the repo's world.json/players.json.
