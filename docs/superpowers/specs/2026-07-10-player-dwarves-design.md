# Player Dwarves Design

Date: 2026-07-10

## Purpose

Tie each dwarf to a player. Every dwarf in the world is owned by someone;
players spawn them, watch them, and mourn them. No accounts: identity is an
anonymous browser token.

## Identity

- The client generates a token once (`crypto.randomUUID()`) and stores it in
  `localStorage`. The token is the player, scoped to that browser.
- After the WebSocket connects the client sends `{type:"hello", player: token}`.
- The server replies with a `player` message:
  `{type:"player", state: "none"|"alive"|"dead", dwarfId?: int, name?: string, error?: string}`.
  - `none`: no record of this token (or spawn rejected; see error).
  - `alive`: the player's dwarf exists and is not dead.
  - `dead`: the player has a record but the dwarf is dead or already decayed.

## Ownership lives in the server layer

Per DESIGN.md the engine knows no species and certainly no players. The sim
is unchanged except for data: `pop_floor = 0` for dwarves in
`data/species.toml`, so the engine never spawns dwarves on its own. Players
are the only source of dwarves; a fresh world is empty until someone spawns.

The Server holds `Players map[string]*Player` where `Player{Name string,
DwarfID int}`, guarded by the existing world mutex. It persists to
`players.json` next to `world.json`, written at the same moments (autosave,
SIGINT save) under the same lock. Loading tolerates a missing file (empty
map). Player records whose dwarf id no longer exists resolve to state `dead`.

## Spawning

- Client sends `{type:"spawn", player: token, name: "Misha"}`.
- Valid only when the player's state is not `alive`. The name is trimmed and
  capped at 24 characters; empty after trimming is rejected (`error`).
- If `CountAlive("dwarf") >= pop_cap` the server replies with the player
  message carrying `error: "the cellar is crowded"` and does not spawn.
- Otherwise the server places a dwarf on a random free dirt tile (the
  clearing is the only dirt) via the existing `Spawn`, records
  `Players[token] = {name, dwarfID}`, and replies with state `alive`.
  Respawning after death overwrites the same record.
- The spawned entity is broadcast to everyone through the normal tick diff.

## Death

- No auto-respawn and no ownership transfer. When the dwarf dies the player
  keeps the record pointing at a dead or removed entity.
- Online detection is client-side: the client knows its `dwarfId` and the
  tick stream delivers the entity's `dead` flag or removal; the client then
  shows the death screen.
- Offline deaths surface on the next `hello`, which replies `dead`.
- The death screen says the dwarf has died and offers one action: spawn a
  new dwarf (name prefilled with the player's previous name).

## Visible ownership

- `EntityView` gains `owner` (`json:"owner,omitempty"`): the owning player's
  name, resolved server-side via a reverse map rebuilt from `Players`.
- The popup first line becomes `Dwarf #7 (Misha)`.
- Events are decorated server-side before broadcast: if the acting entity is
  owned, "Dwarf struck gold" becomes "Misha's dwarf struck gold".
- The owner's own dwarf gets a soft ring on the canvas every frame (visually
  distinct from the yellow selection box) and a status line in the side
  panel: `Your dwarf: #7, mining, fullness 6.2 / 10`.

## Client UI

- Overlay over the map with three states:
  - welcome (`state == none`): short intro, name field, Spawn button, and a
    "just watch" link that dismisses the overlay for spectators.
  - dead (`state == dead` or detected live): "Your dwarf has died", Spawn
    button, name prefilled.
  - hidden (`state == alive` or spectating).
- Spawn button sends the `spawn` message; the reply drives the next state.
- Reconnects re-send `hello` (the existing reconnect loop calls connect()).

## Out of scope (YAGNI)

- Accounts, auth, or cross-browser identity recovery.
- Multiple dwarves per player; dwarf naming separate from player naming.
- Torches and any player world-editing (next spec).
- Spectator counts, player lists, chat.

## Testing

Server tests over a real WebSocket (httptest server + gorilla dialer):
hello with unknown token returns `none`; spawn creates an owned entity and
returns `alive` with the id; a second spawn while alive is rejected; killing
the dwarf in the sim then re-helloing returns `dead`; respawn after death
works and updates the record; spawn at pop_cap returns the crowded error;
players.json round-trips through save and load; EntityView carries `owner`
and decorated events name the owner. Client flow verified end to end with
headless Playwright: welcome overlay, spawn, owner marker and status line,
death overlay (against a fast-starvation test data dir), respawn.
