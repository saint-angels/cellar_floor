import { world } from "./world";
import { sendReset, sendSpawn, sendTimescale, sendTorch } from "./net";
import { consumePan } from "./camera";
import type { SimEvent } from "./types";

export function initUI(
  canvas: HTMLCanvasElement,
  tileFromPixel: (c: HTMLCanvasElement, x: number, y: number) => { x: number; y: number },
) {
  initTimescale();
  initEvents();
  initInspector(canvas, tileFromPixel);
  initOverlay();
  initTorch();
  world.onChange(renderPops);
  world.onChange(renderGold);
  world.onChange(renderInspector);
  world.onChange(renderOverlay);
  world.onChange(renderMyDwarf);
}

let spectating = false;
let torchArmed = false;

function setTorchArmed(on: boolean) {
  torchArmed = on;
  const btn = document.getElementById("torch-btn") as HTMLButtonElement;
  btn.classList.toggle("armed", on);
  btn.textContent = on ? "click the map" : "torch (1 gold)";
  document.getElementById("map")!.classList.toggle("placing", on);
}

function initTorch() {
  const btn = document.getElementById("torch-btn") as HTMLButtonElement;
  btn.onclick = () => setTorchArmed(!torchArmed);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") setTorchArmed(false);
  });
  world.onChange(() => {
    btn.disabled = !(world.playerState === "alive" && world.gold >= 1);
    if (btn.disabled && torchArmed) setTorchArmed(false);
    const err = document.getElementById("torch-error")!;
    err.textContent = world.playerState === "alive" ? world.playerError : "";
  });
}

function initOverlay() {
  const btn = document.getElementById("spawn-btn") as HTMLButtonElement;
  const input = document.getElementById("player-name") as HTMLInputElement;
  const watch = document.getElementById("watch-link")!;
  btn.onclick = () => sendSpawn(input.value);
  input.onkeydown = (e) => { if (e.key === "Enter") sendSpawn(input.value); };
  watch.addEventListener("click", (e) => {
    e.preventDefault();
    spectating = true;
    renderOverlay();
  });
}

function renderOverlay() {
  const overlay = document.getElementById("overlay")!;
  const title = document.getElementById("overlay-title")!;
  const text = document.getElementById("overlay-text")!;
  const input = document.getElementById("player-name") as HTMLInputElement;
  const btn = document.getElementById("spawn-btn")!;
  const errBox = document.getElementById("overlay-error")!;
  const st = world.playerState;
  const show = st === "dead" || (st === "none" && !spectating);
  overlay.style.display = show ? "flex" : "none";
  if (!show) return;
  if (st === "dead") {
    title.textContent = "Your dwarf has died";
    text.textContent = "The cellar is unforgiving. Send another?";
    btn.textContent = "Spawn a new dwarf";
  } else {
    title.textContent = "A dwarf awaits";
    text.textContent = "Name yourself and send a dwarf to dig for gold.";
    btn.textContent = "Spawn a dwarf";
  }
  if (!input.value && world.playerName) input.value = world.playerName;
  errBox.textContent = world.playerError;
}

function renderMyDwarf() {
  const box = document.getElementById("mydwarf")!;
  if (world.playerState === "alive" && world.playerDwarfId != null) {
    const e = world.entities.get(world.playerDwarfId);
    if (e) {
      const cap = world.types[e.s]?.stomachSize ?? 0;
      box.textContent = `#${e.id}, ${e.action || "idle"}, fullness ${e.full.toFixed(1)} / ${cap}`;
      return;
    }
  }
  box.textContent = world.playerState === "dead" ? "dead" : "none";
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
  const reset = document.createElement("button");
  reset.textContent = "reset";
  reset.className = "reset";
  let armedAt = 0;
  reset.onclick = () => {
    if (Date.now() - armedAt < 3000) {
      sendReset();
      armedAt = 0;
      reset.textContent = "reset";
      return;
    }
    armedAt = Date.now();
    reset.textContent = "really?";
    setTimeout(() => {
      if (armedAt !== 0 && Date.now() - armedAt >= 3000) {
        armedAt = 0;
        reset.textContent = "reset";
      }
    }, 3100);
  };
  box.appendChild(reset);
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
      label.style.color = world.types[sid]?.color ?? "#fff";
      label.dataset.sid = sid;
      c = document.createElement("canvas");
      c.width = 120;
      c.height = 24;
      sparks[sid] = c;
      row.append(label, c);
      box.appendChild(row);
    }
    const label = c.parentElement!.querySelector("span")!;
    label.textContent = `${world.types[sid]?.name ?? sid}: ${hist[hist.length - 1] ?? 0}`;
    const g = c.getContext("2d")!;
    g.clearRect(0, 0, c.width, c.height);
    const max = Math.max(...hist, world.types[sid]?.popCap ?? 1);
    g.strokeStyle = world.types[sid]?.color ?? "#fff";
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
    if (consumePan()) return;
    const t = tileFromPixel(canvas, ev.clientX, ev.clientY);
    if (torchArmed) {
      sendTorch(t.x, t.y);
      setTorchArmed(false);
      return;
    }
    let picked: number | null = null;
    let bestD = 3;
    for (const e of world.entities.values()) {
      const sp = world.types[e.s];
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
  const sp = world.types[e.s];
  const lines = [
    `${sp?.name ?? e.s} #${e.id}${e.owner ? ` (${e.owner})` : ""}${e.dead ? " (dead)" : ""}`,
    `at ${e.x},${e.y}`,
  ];
  if (sp?.kind === "fauna" && !e.dead) {
    lines.push(`fullness ${e.full.toFixed(1)} / ${sp.stomachSize}`);
    if (sp.socialSize > 0) {
      lines.push(`social ${(e.soc ?? 0).toFixed(1)} / ${sp.socialSize}`);
    }
    lines.push(`gold today: ${e.g24 ?? 0}`);
    lines.push(`doing: ${e.action || "idle"}`);
    if (e.home) lines.push(`home: ${e.home.x},${e.home.y}`);
  }
  if (e.res) {
    for (const [r, v] of Object.entries(e.res)) lines.push(`${r}: ${v.toFixed(1)}`);
  }
  box.textContent = lines.join("\n");
}
