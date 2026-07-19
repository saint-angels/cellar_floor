import { world } from "./world";
import { drawEffects, drawShakes } from "./fx";
import { atlas, atlasReady, CHEST_H, CHEST_W, CHEST_X, CHEST_Y, DWARF_H, DWARF_ROWS, DWARF_W, FLOOR_Y, floorVariant } from "./sprites";

const TILE = 12;

let terrainCanvas: HTMLCanvasElement | null = null;
let veilCanvas: HTMLCanvasElement | null = null;
const facing = new Map<number, number>(); // last horizontal direction per dwarf

function renderTerrain() {
  terrainCanvas = document.createElement("canvas");
  terrainCanvas.width = world.width * TILE;
  terrainCanvas.height = world.height * TILE;
  const g = terrainCanvas.getContext("2d")!;
  g.imageSmoothingEnabled = false;
  for (let y = 0; y < world.height; y++) {
    for (let x = 0; x < world.width; x++) {
      const tt = world.terrainTypes[world.terrain[y * world.width + x]];
      g.fillStyle = tt?.color ?? "#000";
      g.fillRect(x * TILE, y * TILE, TILE, TILE);
      if (tt?.id === "floor" && atlasReady) {
        g.drawImage(atlas, floorVariant(x, y) * 16, FLOOR_Y, 16, 16, x * TILE, y * TILE, TILE, TILE);
      }
    }
  }
}

function renderVeil() {
  veilCanvas = document.createElement("canvas");
  veilCanvas.width = world.width * TILE;
  veilCanvas.height = world.height * TILE;
  const g = veilCanvas.getContext("2d")!;
  g.clearRect(0, 0, veilCanvas.width, veilCanvas.height);
  g.fillStyle = "rgba(0, 0, 0, 0.75)";
  for (let y = 0; y < world.height; y++) {
    for (let x = 0; x < world.width; x++) {
      if (!world.lit[y * world.width + x]) g.fillRect(x * TILE, y * TILE, TILE, TILE);
    }
  }
}

// bubble pacing: each entity shows its thought for SHOW ms out of every
// PERIOD ms of wall clock, phase-offset by id so dwarves pop at
// different moments
const THOUGHT_PERIOD_MS = 60000;
const THOUGHT_SHOW_MS = 3000;

function thoughtVisible(id: number, now: number): boolean {
  return (now + id * 7919) % THOUGHT_PERIOD_MS < THOUGHT_SHOW_MS;
}

// thought rules live in the type's data (entities.toml thoughts list);
// the first rule whose named condition matches wins
export function composeThought(e: import("./types").RenderEntity): string | null {
  const sp = world.types[e.s];
  if (!sp || sp.kind !== "fauna" || e.dead || !sp.thoughts) return null;
  const dayTicks = 86400 * (1000 / world.tickIntervalMs);
  for (const rule of sp.thoughts) {
    let match = false;
    switch (rule.when) {
      case "starving": match = e.full <= 0; break;
      case "hungry": match = e.full < sp.hungerThreshold; break;
      case "lonely": match = sp.socialSize > 0 && (e.soc ?? 0) < sp.socialThreshold; break;
      case "struck_gold": match = (e.g24 ?? 0) > 0; break;
      case "hauling": match = (e.ore ?? 0) > 0; break;
      case "seen_recently": match = !!e.seenTick && world.tick - e.seenTick <= dayTicks; break;
      case "always": match = true; break;
    }
    if (!match) continue;
    const seen = e.seenId != null ? world.entities.get(e.seenId) : undefined;
    return rule.text
      .replace("{gold}", String(e.g24 ?? 0))
      .replace("{name}", seen?.owner ?? "a dwarf");
  }
  return null;
}

export function startRender(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext("2d")!;
  const map = document.getElementById("map")!;
  const popup = document.getElementById("popup")!;
  let paintedVersion = -1;
  let paintedLight = -1;
  world.onChange(() => {
    if (!terrainCanvas || terrainCanvas.width !== world.width * TILE || paintedVersion !== world.terrainVersion) {
      renderTerrain();
      paintedVersion = world.terrainVersion;
    }
    if (!veilCanvas || veilCanvas.width !== world.width * TILE || paintedLight !== world.lightVersion) {
      renderVeil();
      paintedLight = world.lightVersion;
    }
    canvas.width = world.width * TILE;
    canvas.height = world.height * TILE;
  });

  function frame(now: number) {
    if (terrainCanvas) {
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(terrainCanvas, 0, 0);
      if (veilCanvas) ctx.drawImage(veilCanvas, 0, 0);
      drawShakes(ctx, now);
      const lerpMs = world.tickIntervalMs / Math.max(world.timeScale, 1);
      // draw order: live fauna (dwarves, rabbits) render above the flora,
      // corpses, and structures they can stand on. Live fauna never share a
      // tile with each other (occupancy forbids it), so the layering is exact.
      const zOf = (e: import("./types").RenderEntity) => {
        if (e.dead) return 0;
        const s = world.types[e.s];
        if (!s || s.kind === "flora") return 1;
        if (s.kind === "structure") return 2;
        return 3;
      };
      const drawList = [...world.entities.values()].sort((a, b) => zOf(a) - zOf(b));
      for (const e of drawList) {
        const sp = world.types[e.s];
        if (!sp) continue;
        const t = Math.min(1, (now - e.movedAt) / lerpMs);
        const x = (e.px + (e.x - e.px) * t) * TILE;
        const y = (e.py + (e.y - e.py) * t) * TILE;
        ctx.fillStyle = e.dead ? "#443c38" : sp.color;
        if (sp.kind === "flora") {
          ctx.fillRect(x + 2, y + 2, TILE - 4, TILE - 4);
        } else if (sp.kind === "structure") {
          if (sp.market && atlasReady && !e.dead) {
            ctx.drawImage(atlas, CHEST_X, CHEST_Y, CHEST_W, CHEST_H, x, y, TILE, TILE);
          } else {
            ctx.fillRect(x + 4, y + 4, 4, 4); // flame pixel; dead stubs use the shared dead color
          }
        } else if (e.s === "dwarf" && !e.dead && atlasReady) {
          const dx = e.x - e.px;
          if (dx !== 0) facing.set(e.id, dx > 0 ? 1 : -1);
          const dir = facing.get(e.id) ?? 1;
          const moving = (e.px !== e.x || e.py !== e.y) && now - e.movedAt < lerpMs * 1.5;
          const frame = (Math.floor(now / 140) + e.id) % 4 + (moving ? 4 : 0);
          const w = DWARF_W * (TILE / 16);
          const h = DWARF_H * (TILE / 16);
          ctx.save();
          ctx.translate(x + TILE / 2, y + TILE - h);
          ctx.scale(dir, 1);
          ctx.drawImage(atlas, frame * 16, DWARF_ROWS[e.id % 2], DWARF_W, DWARF_H, -w / 2, 0, w, h);
          ctx.restore();
          if ((e.ore ?? 0) > 0) {
            ctx.fillStyle = "#d9c14a"; // an ore nugget marks a loaded hauler
            ctx.fillRect(x, y + TILE - 3, 3, 3);
          }
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
      // highlight what YOUR dwarf is working on: the mined face as a box,
      // or the food/companion it seeks as a ring
      if (world.playerDwarfId != null) {
        const me = world.entities.get(world.playerDwarfId);
        if (me && !me.dead) {
          ctx.strokeStyle = "rgba(255, 255, 255, 0.65)";
          ctx.lineWidth = 0.75;
          if (me.mt && (me.action === "mining" || me.action === "heading to mine")) {
            ctx.save();
            ctx.translate(me.mt.x * TILE + TILE / 2, me.mt.y * TILE + TILE / 2);
            ctx.rotate(((now % 3000) / 3000) * Math.PI * 2);
            ctx.strokeRect(-(TILE - 2) / 2, -(TILE - 2) / 2, TILE - 2, TILE - 2);
            ctx.restore();
          } else if (me.tid) {
            const tgt = world.entities.get(me.tid);
            if (tgt && !tgt.dead) {
              const tt = Math.min(1, (now - tgt.movedAt) / lerpMs);
              const tx2 = (tgt.px + (tgt.x - tgt.px) * tt) * TILE + TILE / 2;
              const ty2 = (tgt.py + (tgt.y - tgt.py) * tt) * TILE + TILE / 2;
              ctx.beginPath();
              ctx.arc(tx2, ty2, TILE / 2 + 2.5, 0, Math.PI * 2);
              ctx.stroke();
            }
          }
        }
      }
      ctx.font = "9px ui-monospace, monospace";
      ctx.textAlign = "center";
      for (const e of world.entities.values()) {
        if (!thoughtVisible(e.id, now)) continue;
        const thought = composeThought(e);
        if (!thought) continue;
        const t = Math.min(1, (now - e.movedAt) / lerpMs);
        const bx = (e.px + (e.x - e.px) * t) * TILE + TILE / 2;
        const by = (e.py + (e.y - e.py) * t) * TILE - 4;
        const w2 = ctx.measureText(thought).width / 2 + 4;
        ctx.fillStyle = "rgba(20, 17, 15, 0.85)";
        ctx.fillRect(bx - w2, by - 10, w2 * 2, 12);
        ctx.fillStyle = "#cfc9bf";
        ctx.fillText(thought, bx, by - 1);
      }
      ctx.textAlign = "start";
      for (const [k, dmg] of Object.entries(world.mining)) {
        const i = Number(k);
        const hp = world.terrainTypes[world.terrain[i]]?.hitPoints ?? 0;
        const p = hp > 0 ? dmg / hp : 0;
        const bx = (i % world.width) * TILE;
        const by = Math.floor(i / world.width) * TILE;
        ctx.fillStyle = "#1a1815";
        ctx.fillRect(bx + 1, by + 2, TILE - 2, 3);
        ctx.fillStyle = "#ffb347";
        ctx.fillRect(bx + 1, by + 2, (TILE - 2) * Math.min(p, 1), 3);
      }
      drawEffects(ctx, now, lerpMs);
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

export function tileFromPixel(canvas: HTMLCanvasElement, cx: number, cy: number) {
  const r = canvas.getBoundingClientRect();
  const sx = canvas.width / r.width, sy = canvas.height / r.height;
  return {
    x: Math.floor(((cx - r.left) * sx) / TILE),
    y: Math.floor(((cy - r.top) * sy) / TILE),
  };
}
