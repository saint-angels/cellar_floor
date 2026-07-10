# Mining Visuals Design

Date: 2026-07-10

## Purpose

Make mining visible, Vampire Survivors style: a small tool pixel orbits each
actively mining dwarf and strikes the rock cell it works; every hit spills
debris pixels. Purely cosmetic; the sim is untouched. This is the first step
toward tool upgrades as the main progression avenue (later) and gold
milestone rewards (later).

## Renderer decision

Canvas 2D stays. Everything wanted here (and vector-like effects in general:
arcs, trails, glow) is comfortably within Canvas 2D limits at this entity
count. Effects live in a new `client/src/fx.ts` module with a tiny surface
(one `drawEffects` call from the render loop), so a future swap to a WebGL
renderer (PixiJS for heavy 2D, Three.js only for a real 3D pivot) is
contained to the renderer files. No runtime dependency is added now.

## Protocol

`EntityView` gains `mt` (`json:"mt,omitempty"`), copied from the sim's
`Entity.MineTarget` in `ViewOf`. Only set while the entity has a target.
This is the only server change.

## The orbit

- Shown only for dwarves whose `action` is `"mining"` and that carry `mt`.
  Walking, idle, and eating dwarves show nothing.
- A 3px steel pixel (`#cfd6dd`) orbits the dwarf's interpolated position at
  radius 14 canvas px, one revolution per ~1.6 s, phase offset by entity id
  so neighbors don't swing in sync.
- The radius exceeds a half-tile on purpose: once per revolution the tool
  sweeps through the adjacent target cell. That geometric crossing is the
  hit; no timers.

## The debris

- On the frame the tool enters the target cell's rect (per-dwarf was-inside
  flag), 6 debris pixels spawn at the contact point.
- 2x2 px, colored by the struck terrain: gray chips (`#8a8a8a`) off rock,
  warm sparks (`#e8c84a`) off gold. Velocity 50-80 px/s biased back away
  from the face, lifetime 400-700 ms, alpha fades to zero.
- One global particle array capped at 300; updated with per-frame dt inside
  the existing requestAnimationFrame loop.

## Time behavior

Effects advance on a clock that only runs while `timeScale > 0`: a paused
world is a still diorama (no orbit motion, no new debris, particles frozen).
At higher speeds the clock accelerates sublinearly, capped so swings stay
readable: 1x swings at normal pace, 8x at 2x, 64x at 3x. The orbit
visualizes that work is happening and hints at how fast, without trying to
render one swing per sim tick (at 64x that would be an unreadable blur).

## Upgrade seam (not built now)

Tool size, color, orbit radius, and period sit in one constants block at the
top of `fx.ts`. The future tool-upgrade system changes those per dwarf;
nothing else is speculative.

## Testing

Go: `ViewOf` includes `mt` for an entity with a mine target and omits it
otherwise. Client: build, then headless Playwright against an isolated
server (scratch data dir and port; the user's live server stays untouched):
fast-forward with `/api/advance` until a dwarf mines, then sample frames and
assert the steel tool pixel appears near the target cell and debris-colored
pixels appear and fade across frames; assert a paused world stops the
motion. Screenshot for the record.
