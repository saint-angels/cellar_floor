# Inspector Popup

2026-07-09

## Goal

Replace the side panel Inspector section with a popup that appears next to
the clicked entity on the map. The popup shows the same content the panel
shows today, for example:

    Rabbit #1133
    at 8,62
    fullness 9.0 / 10
    doing: idle
    home: 6,62
    fur: 1.0
    meat: 4.0

## Approach

DOM popup repositioned every animation frame (approach A of the ones
considered). A small absolutely positioned div is overlaid on the `#map`
container. Content re-renders on world tick as today; position updates
inside the existing render loop in `render.ts` using the same interpolated
entity coordinates as the creature dot, so the popup glides along with the
creature instead of snapping tile to tile.

Rejected alternatives: repositioning only on world tick (popup snaps while
the dot glides, janky at 1x) and drawing the popup on the canvas (pixelated
scaled text, manual layout).

## Behavior

- Clicking a creature selects it (existing pick logic unchanged) and opens
  the popup next to it.
- The popup is offset from the entity dot so it does not cover it, and is
  flipped or clamped inside the map area near edges so it never clips.
- Position accounts for the canvas being CSS scaled to fit the window
  (same rect math as `tileFromPixel`, inverted).
- Clicking empty ground deselects and closes the popup. No close button.
- Content updates live each tick. A dead entity shows "(dead)" as today.
  If the selected entity despawns, the popup closes.
- The Inspector section is removed from the side panel entirely.

## Changes

- `client/index.html`: remove the Inspector section from `#side`; add the
  popup element inside `#map` and its styles; make `#map` a positioning
  context (`position: relative`).
- `client/src/ui.ts`: `renderInspector` writes into the popup element and
  toggles its visibility based on selection.
- `client/src/render.ts`: in the frame loop, when an entity is selected,
  convert its interpolated canvas position to `#map` pixel coordinates and
  position the popup; hide it if the selected entity no longer exists.

No new dependencies. Server untouched.

## Testing

Manual: run the app, click a rabbit, verify the popup follows it smoothly,
updates stats live, flips near map edges, closes on empty-ground click,
shows "(dead)" on death, and disappears on despawn. Verify the side panel
no longer has an Inspector section. `cd client && npm run build` passes.
