import { world } from "./world";

const TILE = 12;
const TERRAIN_COLORS = ["#3d5a36", "#6b5537", "#2b4a63", "#5a5a5a"]; // grass dirt water rock

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
    }
    requestAnimationFrame(frame);
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
