import { world } from "./world";
import { sendTimescale } from "./net";
import type { SimEvent } from "./types";

export function initUI(
  canvas: HTMLCanvasElement,
  tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {
  initTimescale();
  initEvents();
  initInspector(canvas, tileFromPixel);
  world.onChange(renderPops);
  world.onChange(renderGold);
  world.onChange(renderInspector);
}

function renderGold() {
  const el = document.getElementById("gold")!;
  el.textContent = String(world.gold);
}

function initTimescale() {
  const box = document.getElementById("timescale")!;
  for (const s of [0, 1, 8, 64]) {
    const b = document.createElement("button");
    b.textContent = s === 0 ? "pause" : `${s}x`;
    b.dataset.scale = String(s);
    b.onclick = () => sendTimescale(s);
    box.appendChild(b);
  }
  world.onChange(() => {
    for (const b of box.querySelectorAll("button")) {
      b.classList.toggle("active", Number(b.dataset.scale) === world.timeScale);
    }
  });
}

function initEvents() {
  const box = document.getElementById("events")!;
  world.onEvents((evs: SimEvent[]) => {
    for (const ev of evs) {
      const d = document.createElement("div");
      d.textContent = `[${ev.tick}] ${ev.msg}`;
      box.prepend(d);
    }
    while (box.children.length > 100) box.lastChild!.remove();
  });
}

const sparks: Record<string, HTMLCanvasElement> = {};

function renderPops() {
  const box = document.getElementById("pops")!;
  for (const [sid, hist] of Object.entries(world.popHistory)) {
    let c = sparks[sid];
    if (!c) {
      const row = document.createElement("div");
      row.className = "pop-row";
      const label = document.createElement("span");
      label.style.color = world.species[sid]?.color ?? "#fff";
      label.dataset.sid = sid;
      c = document.createElement("canvas");
      c.width = 120;
      c.height = 24;
      sparks[sid] = c;
      row.append(label, c);
      box.appendChild(row);
    }
    const label = c.parentElement!.querySelector("span")!;
    label.textContent = `${world.species[sid]?.name ?? sid}: ${hist[hist.length - 1] ?? 0}`;
    const g = c.getContext("2d")!;
    g.clearRect(0, 0, c.width, c.height);
    const max = Math.max(...hist, world.species[sid]?.popCap ?? 1);
    g.strokeStyle = world.species[sid]?.color ?? "#fff";
    g.beginPath();
    hist.forEach((v, i) => {
      const x = (i / 119) * c.width;
      const y = c.height - (v / max) * (c.height - 2) - 1;
      i === 0 ? g.moveTo(x, y) : g.lineTo(x, y);
    });
    g.stroke();
  }
}

function initInspector(
  canvas: HTMLCanvasElement,
  tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {
  canvas.addEventListener("click", (ev) => {
    const t = tileFromPixel(canvas, ev.clientX, ev.clientY);
    let picked: number | null = null;
    let bestD = 3;
    for (const e of world.entities.values()) {
      const sp = world.species[e.s];
      const d = Math.max(Math.abs(e.x - t.x), Math.abs(e.y - t.y));
      // prefer fauna over flora on the same tile
      const score = d + (sp?.kind === "flora" ? 0.5 : 0);
      if (score < bestD) {
        bestD = score;
        picked = e.id;
      }
    }
    world.selectedId = picked;
    renderInspector();
  });
}

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
