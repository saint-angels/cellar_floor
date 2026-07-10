# World Reset Button Design

Date: 2026-07-10

## Purpose

A reset button next to the speed buttons that regenerates the world at
runtime, without restarting the server.

## Server

- New client intent `{type:"reset"}`, handled in the WebSocket reader like
  `timescale` and `spawn`.
- Under the world lock the server generates a brand-new world with seed
  `time.Now().UnixNano()` (seed choice is server-layer input, like any
  player action; the sim stays deterministic per seed), swaps `s.world`,
  then saves world and players and broadcasts a full snapshot so every open
  tab flips to the new map at once.
- Player records are kept. Every dwarf is gone, so each player's state
  resolves to `dead` through existing logic: the death screen appears with
  the name prefilled, one click to rejoin. No new player code.
- Gold, mining progress, and tick reset with the world (they live on it).
- The current timescale is preserved.

## Client

- A `reset` button appended to the `#timescale` row, styled like the speed
  buttons but in a muted warning red.
- Misclick protection without `confirm()` dialogs (they block automation):
  the first click arms the button and its label becomes `really?` for 3
  seconds; a second click within the window sends the reset, otherwise the
  label reverts.
- The broadcast snapshot replaces the world wholesale. The own-dwarf death
  check currently runs only in `applyTick`; it moves to a shared method
  called from both `applyTick` and `applySnapshot`, so a reset immediately
  flips the local player to `dead` and shows the death overlay.

## Out of scope (YAGNI)

- Choosing a seed or size in the UI.
- Restricting who may reset (single-cellar dev tool; revisit if it ever
  hurts).
- Confirmation server-side.

## Testing

Server: unit test on the reset method (world swapped to tick 0, no dwarves,
gold zero, player state reports `dead` with name intact); WebSocket test
(spawn, send reset, expect a fresh snapshot frame, hello reports `dead`).
Client: Playwright end to end: spawn, click reset once (label becomes
`really?`), click again, death overlay appears with prefilled name, respawn
works in the new world.
