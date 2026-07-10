# Mining Visuals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An orbiting tool pixel on every actively mining dwarf that strikes the target cell once per revolution and spills terrain-colored debris.

**Architecture:** One protocol addition (`EntityView.mt` from the sim's `MineTarget`); everything else is a new client module `client/src/fx.ts` exposing a single `drawEffects(ctx, now, lerpMs)` called from the render loop. Orbit geometry does the hit timing (radius > half-tile sweeps through the adjacent target cell); a capped global particle array handles debris; an effects clock advances only while `timeScale > 0`.

**Tech Stack:** Go stdlib (protocol field), Canvas 2D in TypeScript. No new dependencies.

## Global Constraints

- Purely cosmetic; the sim is untouched (spec).
- Effects only for `action == "mining"` with `mt` present; nothing on other dwarves (spec).
- Tool constants (size, color, radius, period) in one block at the top of `fx.ts` (upgrade seam, spec).
- Paused world (`timeScale == 0`): no orbit motion, no new debris, particles frozen (spec).
- Particle cap 300; debris colors rock `#8a8a8a`, gold `#e8c84a`; tool `#cfd6dd` (spec).
- Verification on an isolated port and scratch data dir; the user's live server stays untouched (spec).
- Commit messages: one sentence, under 70 characters, no Claude attribution (user CLAUDE.md).

---

### Task 1: mt in EntityView

**Files:**
- Modify: `internal/server/protocol.go:8-29` (EntityView + ViewOf)
- Test: `internal/server/protocol_test.go` (append)

**Interfaces:**
- Consumes: `sim.Entity.MineTarget *sim.Point`.
- Produces: `EntityView.MT *sim.Point \`json:"mt,omitempty"\`` set by `ViewOf`.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/protocol_test.go`:

```go
func TestViewCarriesMineTarget(t *testing.T) {
	cfg := loadCfg(t)
	w := gen.Generate(7, cfg)
	d := w.Spawn("dwarf", sim.Point{X: 32, Y: 32})
	if v := ViewOf(d); v.MT != nil {
		t.Errorf("mt should be nil without a target, got %v", v.MT)
	}
	target := sim.Point{X: 40, Y: 32}
	d.MineTarget = &target
	v := ViewOf(d)
	if v.MT == nil || *v.MT != target {
		t.Errorf("mt = %v, want %v", v.MT, target)
	}
	b, err := json.Marshal(v)
	if err != nil || !strings.Contains(string(b), `"mt":{"x":40,"y":32}`) {
		t.Errorf("marshal: %s %v", b, err)
	}
}
```

Add `"strings"` to the imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestViewCarriesMineTarget -v`
Expected: FAIL to compile ("v.MT undefined")

- [ ] **Step 3: Implement**

In `EntityView` add after `Owner`:

```go
	MT     *sim.Point         `json:"mt,omitempty"`
```

In `ViewOf` add `MT: e.MineTarget,` to the returned literal.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v -run TestView && go test ./... -short && go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/protocol.go internal/server/protocol_test.go
git commit -m "Send mine target in entity views"
```

---

### Task 2: fx.ts orbit, debris, and render hook

**Files:**
- Create: `client/src/fx.ts`
- Modify: `client/src/types.ts` (mt on EntityView), `client/src/render.ts` (call drawEffects)

**Interfaces:**
- Consumes: `world.entities` (RenderEntity with `px/py/movedAt`), `world.terrain`, `world.timeScale`, `world.width`; `EntityView.mt?: {x,y}` (Task 1 wire).
- Produces: `drawEffects(ctx: CanvasRenderingContext2D, now: number, lerpMs: number): void`.

- [ ] **Step 1: Type**

In `client/src/types.ts`, add to `EntityView` after `owner`:

```ts
  mt?: { x: number; y: number };
```

- [ ] **Step 2: Create fx.ts**

```ts
import { world } from "./world";

const TILE = 12;

// tool tuning: the future upgrade system plugs in here
const ORBIT_RADIUS = 14;
const ORBIT_PERIOD_MS = 1600;
const TOOL_SIZE = 3;
const TOOL_COLOR = "#cfd6dd";
const DEBRIS_PER_HIT = 6;
const MAX_PARTICLES = 300;
const DEBRIS_COLORS: Record<number, string> = { 3: "#8a8a8a", 5: "#e8c84a" }; // rock, gold

interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number; ttl: number;
  color: string;
}

let particles: Particle[] = [];
let fxClock = 0;
let lastNow = 0;
const wasInside = new Map<number, boolean>();

export function drawEffects(ctx: CanvasRenderingContext2D, now: number, lerpMs: number) {
  const dt = lastNow ? Math.min(now - lastNow, 100) : 16;
  lastNow = now;
  const running = world.timeScale > 0;
  if (running) fxClock += dt;

  for (const e of world.entities.values()) {
    if (e.dead || e.action !== "mining" || !e.mt) {
      wasInside.delete(e.id);
      continue;
    }
    const t = Math.min(1, (now - e.movedAt) / lerpMs);
    const cx = (e.px + (e.x - e.px) * t) * TILE + TILE / 2;
    const cy = (e.py + (e.y - e.py) * t) * TILE + TILE / 2;
    const angle = (fxClock / ORBIT_PERIOD_MS) * Math.PI * 2 + e.id * 2.4;
    const tx = cx + Math.cos(angle) * ORBIT_RADIUS;
    const ty = cy + Math.sin(angle) * ORBIT_RADIUS;
    ctx.fillStyle = TOOL_COLOR;
    ctx.fillRect(tx - TOOL_SIZE / 2, ty - TOOL_SIZE / 2, TOOL_SIZE, TOOL_SIZE);

    const inside =
      tx >= e.mt.x * TILE && tx < (e.mt.x + 1) * TILE &&
      ty >= e.mt.y * TILE && ty < (e.mt.y + 1) * TILE;
    if (inside && !wasInside.get(e.id) && running) {
      const terrain = world.terrain[e.mt.y * world.width + e.mt.x];
      spawnDebris(tx, ty, cx, cy, DEBRIS_COLORS[terrain] ?? "#8a8a8a");
    }
    wasInside.set(e.id, inside);
  }

  if (running) {
    for (const p of particles) {
      p.x += (p.vx * dt) / 1000;
      p.y += (p.vy * dt) / 1000;
      p.life += dt;
    }
    particles = particles.filter((p) => p.life < p.ttl);
  }
  for (const p of particles) {
    ctx.globalAlpha = 1 - p.life / p.ttl;
    ctx.fillStyle = p.color;
    ctx.fillRect(p.x - 1, p.y - 1, 2, 2);
  }
  ctx.globalAlpha = 1;
}

// debris flies back away from the struck face, toward open ground
function spawnDebris(x: number, y: number, fromX: number, fromY: number, color: string) {
  const base = Math.atan2(fromY - y, fromX - x);
  for (let i = 0; i < DEBRIS_PER_HIT; i++) {
    if (particles.length >= MAX_PARTICLES) break;
    const a = base + (Math.random() - 0.5) * 1.8;
    const speed = 50 + Math.random() * 30;
    particles.push({
      x, y,
      vx: Math.cos(a) * speed,
      vy: Math.sin(a) * speed,
      life: 0,
      ttl: 400 + Math.random() * 300,
      color,
    });
  }
}
```

- [ ] **Step 3: Hook into the render loop**

In `client/src/render.ts` add the import:

```ts
import { drawEffects } from "./fx";
```

In `frame`, after the mining-bar loop and before `positionPopup`:

```ts
      drawEffects(ctx, now, lerpMs);
```

- [ ] **Step 4: Build**

Run: `cd client && npm run build && cd ..`
Expected: clean build

- [ ] **Step 5: End-to-end verification (verify skill recipes)**

Isolated server (scratch data dir with own `save_path`, `-addr :8083`, `-fresh`). Playwright:

1. Spawn a dwarf via the welcome overlay, `POST /api/advance?ticks=90000` so it reaches a face and mines; confirm via `/api/entities?species=dwarf` that `action == "mining"` and the response carries `mt`.
2. Sample ~20 frames over 2 seconds counting `#cfd6dd` tool pixels: present in most frames, and positions change between frames (orbit moves).
3. Across the same samples, debris pixels (`#8a8a8a`, alpha-faded variants allowed: accept channel values within 25) appear in at least one frame near the target cell.
4. Click pause; sample two frames 500 ms apart and assert the tool pixel position is identical (clock frozen); unpause.
5. Screenshot for the record. Stop the isolated server.

- [ ] **Step 6: Commit**

```bash
git add client/src/fx.ts client/src/types.ts client/src/render.ts
git commit -m "Add orbiting tool and debris effects for mining dwarves"
```
