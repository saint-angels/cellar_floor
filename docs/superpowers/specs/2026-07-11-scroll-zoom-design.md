# Scroll-to-Zoom Design

Date: 2026-07-11

## Purpose

Mouse-wheel zoom on the map canvas, 1x to 8x, centered on the cursor,
with drag-to-pan while zoomed. Pure viewer feature; no sim or protocol
changes.

## Approach

A CSS transform on the canvas element: `translate(tx, ty) scale(z)`,
origin top-left, managed by a new `client/src/camera.ts`. No canvas
context transforms and no render-loop changes: `tileFromPixel` and
`positionPopup` already measure via `getBoundingClientRect`, which is
transform-aware, so clicks, torch placement, and the inspector popup
keep working untouched. `image-rendering: pixelated` keeps the chunky
look; bubbles, bars, and effects scale with the world.

## Behavior

- Wheel zooms multiplicatively (`z *= exp(-deltaY * 0.0015)`), clamped
  to [1, 8]. The world point under the cursor stays fixed:
  `t' = (p + t) - (p / z) * z'` per axis, where p is the cursor offset
  inside the scaled canvas rect.
- Pan by dragging anywhere in `#map`; translate is clamped to
  `[base * (1 - z), 0]` per axis so the map always covers its original
  box. At z = 1 the clamp forces (0,0): today's exact layout.
- A drag is a drag once cumulative movement exceeds 5 px; the click that
  ends a real drag is swallowed (`consumePan()` guard at the top of the
  inspector's click handler) so panning never selects an entity or
  places a torch.
- Cursor: `grab` on the canvas while zoomed, `grabbing` mid-drag; the
  torch-placement crosshair still wins while arming.
- `#map` gains `overflow: hidden` so the scaled canvas clips instead of
  spilling over the sidebar.

## Files

- Create `client/src/camera.ts`: `initCamera(canvas)` (state, wheel,
  mousedown/move/up listeners, transform application, clamping) and
  `consumePan(): boolean`.
- Modify `client/src/main.ts` (call initCamera), `client/src/ui.ts`
  (consumePan guard first in the canvas click handler),
  `client/index.html` (overflow, cursor rules).

## Out of scope

Touch/pinch gestures, zoom UI buttons, remembering zoom across reloads,
minimap.

## Testing

Client build clean. Headless Playwright on an isolated server: wheel
over a corner, assert the canvas bounding rect grew (zoomed) and that
`tileFromPixel`-driven selection still picks the dwarf under the cursor
at zoom (click a known dwarf tile, popup appears); drag, assert the rect
moved and that the drag-ending click did NOT select anything; wheel back
out, assert rect returns to the fitted size. Screenshot zoomed in.
