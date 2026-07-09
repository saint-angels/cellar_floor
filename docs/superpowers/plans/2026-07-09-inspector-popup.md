# Inspector Popup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the side panel Inspector with a popup that follows the clicked entity on the map.

**Architecture:** A single absolutely positioned div overlaid on `#map`. `ui.ts` re-renders its text content on each world tick (as the side panel does today). `render.ts` repositions it every animation frame using the same interpolated entity coordinates as the creature dot, flipping near the right edge and clamping vertically so it never clips.

**Tech Stack:** TypeScript, Vite, plain DOM. No new dependencies.

Spec: `docs/superpowers/specs/2026-07-09-inspector-popup-design.md`

## Global Constraints

- No new dependencies; server untouched.
- Client has no test framework; verification is `cd client && npm run build` (runs `tsc`) plus the manual checks listed in each task.
- Commit messages: one sentence, under 70 characters, no Claude attribution.
- Manual checks need the app running: `go run ./cmd/cellarfloor` from the repo root, then open http://localhost:8080 (rebuild the client first), or use `cd client && npm run dev` against a running server.

---

### Task 1: Move inspector content into a popup element

Replaces the side panel Inspector section with a hidden popup div inside `#map` and points `renderInspector` at it. After this task the popup shows correct live text but sits at a fixed spot (top-left of the map); Task 2 makes it follow the entity. The app must build and run without errors after this task alone.

**Files:**
- Modify: `client/index.html`
- Modify: `client/src/ui.ts:102-123`

**Interfaces:**
- Produces: a `#popup` div inside `#map`, hidden via `display: none` when nothing is selected. Task 2 relies on the id `popup`, on `ui.ts` setting `display` to `none` when there is no selection, and on `ui.ts` never setting `display` to any visible value (Task 2's frame loop owns showing it).

- [ ] **Step 1: Update index.html**

In `client/index.html`, make three changes.

Change the `#map` rule (line 10) to add `position: relative`:

```css
#map { flex: 1; display: flex; align-items: center; justify-content: center; min-width: 0; position: relative; }
```

Replace the `#inspector` rule (line 16) with a `#popup` rule:

```css
#popup { position: absolute; display: none; background: rgba(20, 17, 15, 0.92); border: 1px solid #4a443c; padding: 6px 8px; white-space: pre; pointer-events: none; }
```

In the body, add the popup inside `#map` (line 24) and delete the Inspector section from `#side` (line 28):

```html
<div id="map"><canvas id="canvas"></canvas><div id="popup"></div></div>
```

Delete this line entirely:

```html
<div><h2>Inspector</h2><div id="inspector">click a creature</div></div>
```

- [ ] **Step 2: Point renderInspector at the popup**

In `client/src/ui.ts`, replace the whole `renderInspector` function (lines 102-123) with:

```ts
function renderInspector() {
  const box = document.getElementById("popup")!;
  const e = world.selectedId != null ? world.entities.get(world.selectedId) : null;
  if (!e) {
    box.style.display = "none";
    return;
  }
  const sp = world.species[e.s];
  const lines = [
    `${sp?.name ?? e.s} #${e.id}${e.dead ? " (dead)" : ""}`,
    `at ${e.x},${e.y}`,
  ];
  if (sp?.kind === "fauna" && !e.dead) {
    lines.push(`fullness ${e.full.toFixed(1)} / ${sp.stomachSize}`);
    lines.push(`doing: ${e.action || "idle"}`);
    if (e.home) lines.push(`home: ${e.home.x},${e.home.y}`);
  }
  if (e.res) {
    for (const [r, v] of Object.entries(e.res)) lines.push(`${r}: ${v.toFixed(1)}`);
  }
  box.textContent = lines.join("\n");
}
```

Two deliberate details: the null check is `world.selectedId != null` (the old truthiness check would break for entity id 0), and the function never sets `display` to a visible value; it only hides. Showing is Task 2's job, which avoids a one-frame flash at the wrong position.

- [ ] **Step 3: Verify the build**

Run: `cd client && npm run build`
Expected: `tsc` and `vite build` both succeed with no errors. There must be no remaining references to `"inspector"` in `client/src` or `client/index.html`; check with `grep -ri inspector client/src client/index.html` (expect no output).

- [ ] **Step 4: Manual check**

Run the app (see Global Constraints). Verify: the side panel has no Inspector section; clicking a creature draws the yellow selection box on it (popup is not visible yet, which is expected until Task 2); no console errors.

- [ ] **Step 5: Commit**

```bash
git add client/index.html client/src/ui.ts
git commit -m "Move inspector content into hidden map popup"
```

### Task 2: Position the popup in the render loop

Makes the popup appear next to the selected entity and glide with it, flipped near the right edge and clamped vertically.

**Files:**
- Modify: `client/src/render.ts:21-58`

**Interfaces:**
- Consumes: the `#popup` div from Task 1 (`ui.ts` fills its text and hides it on deselect/despawn; this task shows and positions it).
- Produces: nothing consumed by later tasks; this is the last task.

- [ ] **Step 1: Add popup positioning to the frame loop**

In `client/src/render.ts`, replace the whole `startRender` function (lines 21-58) with:

```ts
export function startRender(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext("2d")!;
  const map = document.getElementById("map")!;
  const popup = document.getElementById("popup")!;
  world.onChange(() => {
    if (!terrainCanvas || terrainCanvas.width !== world.width * TILE) renderTerrain();
    canvas.width = world.width * TILE;
    canvas.height = world.height * TILE;
  });

  function frame(now: number) {
    if (terrainCanvas) {
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(terrainCanvas, 0, 0);
      const lerpMs = world.tickIntervalMs / Math.max(world.timeScale, 1);
      for (const e of world.entities.values()) {
        const sp = world.species[e.s];
        if (!sp) continue;
        const t = Math.min(1, (now - e.movedAt) / lerpMs);
        const x = (e.px + (e.x - e.px) * t) * TILE;
        const y = (e.py + (e.y - e.py) * t) * TILE;
        ctx.fillStyle = e.dead ? "#443c38" : sp.color;
        if (sp.kind === "flora") {
          ctx.fillRect(x + 2, y + 2, TILE - 4, TILE - 4);
        } else {
          ctx.beginPath();
          ctx.arc(x + TILE / 2, y + TILE / 2, TILE / 2 - 1, 0, Math.PI * 2);
          ctx.fill();
        }
        if (e.id === world.selectedId) {
          ctx.strokeStyle = "#ffd75e";
          ctx.lineWidth = 2;
          ctx.strokeRect(x - 1, y - 1, TILE + 2, TILE + 2);
        }
      }
      positionPopup(now, lerpMs);
    }
    requestAnimationFrame(frame);
  }

  function positionPopup(now: number, lerpMs: number) {
    const sel = world.selectedId != null ? world.entities.get(world.selectedId) : null;
    if (!sel) {
      popup.style.display = "none";
      return;
    }
    const t = Math.min(1, (now - sel.movedAt) / lerpMs);
    const ex = (sel.px + (sel.x - sel.px) * t) * TILE;
    const ey = (sel.py + (sel.y - sel.py) * t) * TILE;
    const r = canvas.getBoundingClientRect();
    const m = map.getBoundingClientRect();
    const sx = r.width / canvas.width, sy = r.height / canvas.height;
    popup.style.display = "block";
    let left = r.left - m.left + (ex + TILE + 4) * sx;
    if (left + popup.offsetWidth > m.width) {
      left = r.left - m.left + (ex - 4) * sx - popup.offsetWidth;
    }
    let top = r.top - m.top + ey * sy;
    top = Math.max(0, Math.min(top, m.height - popup.offsetHeight));
    popup.style.left = `${left}px`;
    popup.style.top = `${top}px`;
  }

  requestAnimationFrame(frame);
}
```

What changed relative to the current code: the `map` and `popup` lookups at the top, the `positionPopup(now, lerpMs)` call after the entity loop, and the new `positionPopup` function. The entity drawing loop is unchanged.

How the math works: `ex, ey` are the same interpolated canvas-pixel coordinates used to draw the dot. `sx, sy` convert canvas pixels to screen pixels because CSS scales the canvas to fit (`max-width/max-height: 100%`), and `r.left - m.left` shifts into `#map`-relative coordinates since the canvas is centered inside `#map` and the popup is positioned relative to `#map`. The default anchor is 4 canvas pixels right of the dot; if the popup would overflow the map's right edge it flips to the left side of the dot, and the vertical position is clamped into the map. `positionPopup` also hides the popup when the selected entity no longer exists, which covers despawn between ticks.

- [ ] **Step 2: Verify the build**

Run: `cd client && npm run build`
Expected: `tsc` and `vite build` both succeed with no errors.

- [ ] **Step 3: Manual check**

Run the app (see Global Constraints) and verify against the spec:

1. Click a rabbit: popup appears beside it showing name/#id, position, fullness, doing, home, and resources, and glides along as the rabbit moves.
2. Stats update live as ticks pass (fullness and position change).
3. Click a creature near the right edge of the map: popup flips to the left of the dot and never clips outside the map area.
4. Click empty ground: popup disappears.
5. Speed up time (64x) and watch a selected rabbit die or despawn: popup shows "(dead)" while the body remains, and disappears when the entity is removed.
6. Click a bush or tree: popup shows its name, position, and resources (no fullness/doing lines).

- [ ] **Step 4: Commit**

```bash
git add client/src/render.ts
git commit -m "Anchor inspector popup to selected entity each frame"
```
