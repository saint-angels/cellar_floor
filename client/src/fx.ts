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
