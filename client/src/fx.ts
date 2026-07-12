import { world } from "./world";

const TILE = 12;

// tool tuning: the future upgrade system plugs in here
const ORBIT_RADIUS = 14;
const ORBIT_PERIOD_MS = 1600;
const TOOL_SIZE = 3;
const TOOL_COLOR = "#cfd6dd";
const DEBRIS_PER_HIT = 6;
const MAX_PARTICLES = 300;
const DEBRIS_COLOR = "#8a8a8a";

const SHAKE_MS = 200;
const SHAKE_AMP = 1; // px, a very tiny rattle

const FLOAT_RISE_MS = 400;
const FLOAT_FADE_MS = 600;
const FLOAT_RISE_PX = 8;
const MAX_FLOATS = 40;
const FLOAT_COLOR = "#e8e2d8";

interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number; ttl: number;
  color: string;
}

interface FloatText {
  x: number; y: number;
  text: string;
  age: number;
}

let particles: Particle[] = [];
let floats: FloatText[] = [];
// per cell index: damage already shown, plus the terrain hit points captured
// at set time (terrain flips to floor on completion, losing the hp otherwise)
const shownDamage = new Map<number, { shown: number; hp: number }>();
let fxClock = 0;
let lastNow = 0;
const toolCell = new Map<number, number>();
// snapshots replace the world wholesale; local fx tracking would pop phantom
// remainder numbers at stale cells, so drop it when a new snapshot lands
let seenSnapshot = -1;
const shakes = new Map<number, number>(); // cell index -> strike time

const easeInQuad = (t: number) => t * t;

function spawnFloat(cellX: number, cellY: number, text: string) {
  if (floats.length >= MAX_FLOATS) floats.shift();
  floats.push({ x: cellX * TILE + TILE / 2, y: cellY * TILE - 2, text, age: 0 });
}

export function initFx() {
  world.onEvents((evs) => {
    for (const ev of evs) {
      if (ev.type !== "gold") continue;
      const e = world.entities.get(ev.actor);
      if (!e) continue;
      const cx = e.x * TILE + TILE / 2, cy = e.y * TILE + TILE / 2;
      for (let i = 0; i < 12; i++) {
        if (particles.length >= MAX_PARTICLES) break;
        const a = Math.random() * Math.PI * 2;
        const speed = 40 + Math.random() * 50;
        particles.push({ x: cx, y: cy, vx: Math.cos(a) * speed, vy: Math.sin(a) * speed,
          life: 0, ttl: 500 + Math.random() * 400, color: "#e8c84a" });
      }
    }
  });
}

export function drawEffects(ctx: CanvasRenderingContext2D, now: number, lerpMs: number) {
  if (world.snapshotVersion !== seenSnapshot) {
    seenSnapshot = world.snapshotVersion;
    shownDamage.clear();
    floats = [];
    toolCell.clear();
    shakes.clear();
  }
  const dt = lastNow ? Math.min(now - lastNow, 100) : 16;
  lastNow = now;
  const running = world.timeScale > 0;
  // dwarves visibly hustle at higher speeds: power law anchored so
  // 1x -> 1.0, 8x -> ~3.2, 64x -> 10.0 (blur at max speed is fine)
  if (running) fxClock += dt * Math.pow(Math.max(world.timeScale, 1), Math.log(10) / Math.log(64));

  for (const e of world.entities.values()) {
    if (e.dead || (world.types[e.s]?.lightRadius ?? 0) <= 0) continue;
    const gx = e.x * TILE + TILE / 2, gy = e.y * TILE + TILE / 2;
    const flicker = 0.16 + 0.05 * Math.sin(fxClock / 130 + e.id * 1.7);
    const grad = ctx.createRadialGradient(gx, gy, 2, gx, gy, TILE * 1.4);
    grad.addColorStop(0, `rgba(255, 190, 90, ${flicker})`);
    grad.addColorStop(1, "rgba(255, 190, 90, 0)");
    ctx.fillStyle = grad;
    ctx.fillRect(gx - TILE * 1.4, gy - TILE * 1.4, TILE * 2.8, TILE * 2.8);
  }

  for (const e of world.entities.values()) {
    if (e.dead || e.action !== "mining" || !e.mt) {
      toolCell.delete(e.id);
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

    const cx2 = Math.floor(tx / TILE);
    const cy2 = Math.floor(ty / TILE);
    const inWorld = cx2 >= 0 && cy2 >= 0 && cx2 < world.width && cy2 < world.height;
    const cell = inWorld ? cy2 * world.width + cx2 : -1;
    const prev = toolCell.get(e.id) ?? -1;
    const mineable = cell >= 0 && (world.terrainTypes[world.terrain[cell]]?.mineable ?? false);
    if (mineable && cell !== prev && running) {
      spawnDebris(tx, ty, cx, cy, DEBRIS_COLOR);
      shakes.set(cell, now);
      // only track cells the sim is actually damaging; a swept face with no
      // mining entry (unlit, or damage not yet on the wire) would otherwise
      // be baselined at 0 and the completion sweep would pop its full hp
      const dealt = world.mining[cell];
      if (dealt != null) {
        const rec = shownDamage.get(cell);
        if (rec == null) {
          // baseline silently on first sight (fresh page load mid-mine)
          const hp = world.terrainTypes[world.terrain[cell]]?.hitPoints ?? 0;
          shownDamage.set(cell, { shown: dealt, hp });
        } else if (dealt > rec.shown) {
          spawnFloat(cx2, cy2, String(dealt - rec.shown));
          rec.shown = dealt;
        }
      }
    }
    toolCell.set(e.id, cell);

    for (const u of world.upgrades) {
      if (u.kind !== "weapon" || !(world.claims[u.name] > 0)) continue;
      const wAngle = (fxClock / u.periodMs) * Math.PI * 2 + e.id * 2.4 + u.radius;
      const wx = cx + Math.cos(wAngle) * u.radius;
      const wy = cy + Math.sin(wAngle) * u.radius;
      ctx.fillStyle = u.color;
      ctx.fillRect(wx - TOOL_SIZE / 2, wy - TOOL_SIZE / 2, TOOL_SIZE, TOOL_SIZE);
      const wcx = Math.floor(wx / TILE);
      const wcy = Math.floor(wy / TILE);
      const inW = wcx >= 0 && wcy >= 0 && wcx < world.width && wcy < world.height;
      const wcell = inW ? wcy * world.width + wcx : -1;
      const wprev = toolCell.get(e.id * 131 + u.radius) ?? -1;
      const wmine = wcell >= 0 && (world.terrainTypes[world.terrain[wcell]]?.mineable ?? false);
      if (wmine && wcell !== wprev && running) {
        spawnDebris(wx, wy, cx, cy, DEBRIS_COLOR);
        shakes.set(wcell, now);
      }
      toolCell.set(e.id * 131 + u.radius, wcell);
    }
  }

  // completion sweep, once per frame: a tracked cell that left the mining map
  // finished mining; pop the remainder using the hp captured before the flip
  for (const [cell, rec] of shownDamage) {
    if (world.mining[cell] != null) continue;
    if (rec.hp > rec.shown) {
      spawnFloat(cell % world.width, Math.floor(cell / world.width), String(rec.hp - rec.shown));
    }
    shownDamage.delete(cell);
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

  // floats age only while running, matching the particle pause behavior
  if (running) for (const f of floats) f.age += dt;
  floats = floats.filter((f) => f.age < FLOAT_RISE_MS + FLOAT_FADE_MS);
  ctx.font = "9px ui-monospace, monospace";
  ctx.textAlign = "center";
  ctx.fillStyle = FLOAT_COLOR;
  for (const f of floats) {
    let alpha: number;
    let y = f.y;
    if (f.age < FLOAT_RISE_MS) {
      const t = easeInQuad(f.age / FLOAT_RISE_MS);
      alpha = t;
      y = f.y - FLOAT_RISE_PX * t;
    } else {
      alpha = 1 - (f.age - FLOAT_RISE_MS) / FLOAT_FADE_MS;
      y = f.y - FLOAT_RISE_PX;
    }
    ctx.globalAlpha = Math.max(0, Math.min(1, alpha));
    ctx.fillText(f.text, f.x, y);
  }
  ctx.globalAlpha = 1;
  ctx.textAlign = "start";
}

// drawShakes redraws recently struck tiles with a tiny decaying jitter.
// Called from the render loop between the veil and the entities so bars
// and creatures still draw on top.
export function drawShakes(ctx: CanvasRenderingContext2D, now: number) {
  for (const [cell, t0] of shakes) {
    const age = now - t0;
    if (age > SHAKE_MS) {
      shakes.delete(cell);
      continue;
    }
    const color = world.terrainTypes[world.terrain[cell]]?.color;
    if (!color) {
      shakes.delete(cell);
      continue;
    }
    const decay = 1 - age / SHAKE_MS;
    const dx = Math.round(Math.sin(age / 14 + cell) * SHAKE_AMP * decay * 1.99);
    const dy = Math.round(Math.cos(age / 11 + cell) * SHAKE_AMP * decay * 1.99);
    const x = (cell % world.width) * TILE;
    const y = Math.floor(cell / world.width) * TILE;
    ctx.fillStyle = color;
    ctx.fillRect(x + dx, y + dy, TILE, TILE);
  }
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
