# Debug API Design

Date: 2026-07-10

## Purpose

Read-only JSON endpoints on the existing Go server so a human (curl + jq) or an agent can inspect the live simulation without scraping canvas pixels. This replaces pixel-scanning as the primary inspection method during development and verification.

## Endpoints

All responses are `application/json`. All handlers take `s.mu` while reading world state, same as the WebSocket snapshot path.

### GET /api/state

World overview:

```json
{
  "tick": 48108,
  "timeScale": 1,
  "width": 64,
  "height": 64,
  "pops": { "rabbit": 9, "wolf": 2 },
  "entities": 143
}
```

`pops` counts alive fauna per species (same computation as the tick message). `entities` is the total entity count including flora and corpses.

### GET /api/entities

Array of entities in the existing `EntityView` shape used by the WebSocket protocol (`id`, `s`, `x`, `y`, `dead`, `full`, `action`, `home`, `res`), sorted by id.

Query parameters, combinable:

- `species=<id>` - only that species (e.g. `?species=rabbit`)
- `alive=true` / `alive=false` - filter by dead flag

### GET /api/entities/{id}

Single `EntityView` by numeric id. `404` with `{"error":"not found"}` if the id does not exist; `400` if the id is not a number.

## Approach

Add the three handlers to the mux in `internal/server/server.go` (Go 1.26 method+path patterns). Reuse `ViewOf` and `CountAlive`; no new packages, no changes to the sim or the client. A small `writeJSON` helper sets the content type and encodes.

## Out of scope (YAGNI)

- Write operations (spawn, set timescale) - revisit if agent-driven tuning is wanted, possibly behind an MCP wrapper.
- Event history, terrain dumps (available via /ws snapshot if ever needed).
- Auth: the server is localhost-only for development.

## Testing

Table-driven handler test in `internal/server` using `httptest` and a small generated world: status codes, filter behavior, 404/400 cases. Manual curl check against the running server.
