import { world } from "./world";
import { drawEffects } from "./fx";

const TILE = 12;
const TERRAIN_COLORS = ["#3d5a36", "#6b5537", "#2b4a63", "#3a3a3a", "#26221e", "#c9a227"]; // grass dirt water rock floor gold

let terrainCanvas: HTMLCanvasElement | null = null;

function renderTerrain() {
  terrainCanvas = document.createElement("canvas");
  terrainCanvas.width = world.width * TILE;
  terrainCanvas.height = world.height * TILE;
  const g = terrainCanvas.getContext("2d")!;
  for (let y = 0; y < world.height; y++) {
    for (let x = 0; x < world.width; x++) {
      g.fillStyle = TERRAIN_COLORS[world.terrain[y * world.width + x]] ?? "#000";
      g.fillRect(x * TILE, y * TILE, TILE, TILE);
    }
  }
}

export function startRender(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext("2d")!;
  const map = document.getElementById("map")!;
  const popup = document.getElementById("popup")!;
  let paintedVersion = -1;
  world.onChange(() => {
    if (!terrainCanvas || terrainCanvas.width !== world.width * TILE || paintedVersion !== world.terrainVersion) {
      renderTerrain();
      paintedVersion = world.terrainVersion;
    }
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
      if (world.playerDwarfId != null) {
        const me = world.entities.get(world.playerDwarfId);
        if (me && !me.dead) {
          const mt = Math.min(1, (now - me.movedAt) / lerpMs);
          const mx = (me.px + (me.x - me.px) * mt) * TILE + TILE / 2;
          const my = (me.py + (me.y - me.py) * mt) * TILE + TILE / 2;
          ctx.strokeStyle = "rgba(255, 255, 255, 0.65)";
          ctx.lineWidth = 1.5;
          ctx.beginPath();
          ctx.arc(mx, my, TILE / 2 + 2.5, 0, Math.PI * 2);
          ctx.stroke();
        }
      }
      for (const [k, p] of Object.entries(world.mining)) {
        const i = Number(k);
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
