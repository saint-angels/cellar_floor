import { world } from "./world";
import { sendClaim, sendDebug, sendReset, sendSpawn, sendSpawnEntity, sendTimescale } from "./net";
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
  initLevel();
  initRecap();
  initDebug();
  world.onChange(renderPops);
  world.onChange(renderGold);
  world.onChange(renderInspector);
  world.onChange(renderOverlay);
  world.onChange(renderMyDwarf);
}

let spectating = false;
let placingEntity: string | null = null;

function updatePlacingCursor() {
  document.getElementById("map")!.classList.toggle("placing", placingEntity !== null);
}

function armEntityButtons(id: string | null) {
  for (const b of document.querySelectorAll<HTMLElement>("#debug-entities button")) {
    b.classList.toggle("armed", b.dataset.id === id);
  }
}

// debug: arm placement of an entity type; click the map to drop one. Stays
// armed for repeated placement until Escape or toggled off.
function setPlacingEntity(id: string | null) {
  placingEntity = id;
  armEntityButtons(id);
  if (id) {
    document.getElementById("debugmenu")!.style.display = "none";
  }
  updatePlacingCursor();
}

// a one-line effect description per upgrade kind for the offer buttons
function upgradeDesc(name: string): string {
  const u = world.upgrades.find((x) => x.name === name);
  if (!u) return "";
  switch (u.kind) {
    case "damage": return `+${u.amount} mining damage for every dwarf`;
    case "luck": return `+${u.amount} gold from every drop`;
    case "weapon": return `an orbiting weapon, +${u.amount} damage`;
    case "beam": return `a lance shot at the target, +${u.amount} damage`;
    case "missile": return `a homing missile, +${u.amount} damage on the target`;
    case "speed": return `+${u.amount}% move speed`;
    default: return "";
  }
}

function initLevel() {
  const label = document.getElementById("levellabel")!;
  const fill = document.getElementById("levelfill")!;
  const card = document.getElementById("claimcard")!;
  const text = document.getElementById("claimtext")!;
  const box = document.getElementById("offerbox")!;
  const more = document.getElementById("claimmore")!;
  let shownOffer = "";
  world.onChange(() => {
    const span = world.nextLevelGold - world.prevLevelGold;
    const into = Math.max(0, Math.min(span, world.goldMined - world.prevLevelGold));
    label.textContent = span > 0 ? `Lv ${world.level} ${into}/${span}` : `Lv ${world.level}`;
    fill.style.width = `${span > 0 ? (into / span) * 100 : 0}%`;
    if (world.offer.length === 0 || world.pendingLevels === 0) {
      card.style.display = "none";
      shownOffer = "";
      return;
    }
    text.textContent = `Level ${world.level - world.pendingLevels + 1} reached: choose an upgrade`;
    more.textContent = world.pendingLevels > 1 ? `+${world.pendingLevels - 1} more levels waiting` : "";
    // rebuild the buttons only when the offer itself changes, so a click
    // never lands on a row that was just re-created under the finger
    const key = world.offer.join("|");
    if (key !== shownOffer) {
      shownOffer = key;
      box.textContent = "";
      for (const name of world.offer) {
        const b = document.createElement("button");
        const title = document.createElement("span");
        title.textContent = name;
        const kind = document.createElement("span");
        kind.className = "kind";
        kind.textContent = upgradeDesc(name);
        b.append(title, kind);
        b.onclick = () => sendClaim(name);
        box.appendChild(b);
      }
    }
    for (const b of Array.from(box.querySelectorAll("button"))) {
      b.disabled = world.playerState !== "alive";
    }
    card.style.display = "block";
  });
}

function initRecap() {
  const box = document.getElementById("recap")!;
  let shown: import("./types").RecapMsg | null = null;
  box.onclick = () => {
    world.recap = null;
    shown = null;
    box.style.display = "none";
  };
  world.onChange(() => {
    const r = world.recap;
    if (!r) {
      shown = null;
      box.style.display = "none";
      return;
    }
    if (r === shown) return; // already rendered; ticks must not reset the fade
    shown = r;
    const secs = r.ticks / 2;
    const dur = secs >= 3600
      ? `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
      : `${Math.max(1, Math.floor(secs / 60))}m`;
    const parts = [];
    if (r.blocks) parts.push(`${r.blocks} blocks mined`);
    if (r.gold) parts.push(`${r.gold} gold mined`);
    if (r.mold) parts.push(`${r.mold} tunnels molded over`);
    const claimsLine = world.pendingLevels
      ? ` ${world.pendingLevels} level${world.pendingLevels > 1 ? "s" : ""} await your choice!`
      : "";
    box.textContent = `While you were away (${dur}): ${parts.join(", ")}.${claimsLine}`;
    box.style.display = "block";
    setTimeout(() => {
      if (world.recap === r) {
        world.recap = null;
        shown = null;
        box.style.display = "none";
      }
    }, 12000);
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
      const sp = world.types[e.s];
      const lines = [
        e.action || "idle",
        `fullness ${e.full.toFixed(1)} / ${sp?.stomachSize ?? 0}`,
      ];
      if ((sp?.socialSize ?? 0) > 0) {
        lines.push(`social ${(e.soc ?? 0).toFixed(1)} / ${sp!.socialSize}`);
      }
      if ((sp?.carryCapacity ?? 0) > 0) {
        lines.push(`carrying ${e.ore ?? 0}/${sp!.carryCapacity} ore`);
      }
      box.textContent = lines.join("\n");
      return;
    }
  }
  box.textContent = world.playerState === "dead" ? "dead" : "none";
}

function renderGold() {
  const el = document.getElementById("goldnum")!;
  el.textContent = String(world.gold);
}

// the debug menu (Tab) bundles the admin-gated world controls: speed,
// reset, gold grants, level completion, and claim counts
function initDebug() {
  const menu = document.getElementById("debugmenu")!;
  const toggle = () => {
    menu.style.display = menu.style.display === "block" ? "none" : "block";
  };
  window.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      setPlacingEntity(null); // cancel an armed entity placement
      return;
    }
    if (e.key !== "Tab") return;
    const t = e.target as HTMLElement;
    if (t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement) return;
    e.preventDefault();
    toggle();
  });
  // no Tab key on touch screens: a three-finger tap opens the menu
  window.addEventListener("touchstart", (e) => {
    if (e.touches.length === 3) toggle();
  });
  (document.getElementById("debug-gold10") as HTMLButtonElement).onclick = () => sendDebug("gold", "", 10);
  (document.getElementById("debug-gold100") as HTMLButtonElement).onclick = () => sendDebug("gold", "", 100);
  (document.getElementById("debug-level") as HTMLButtonElement).onclick = () => sendDebug("level");

  // one row per pool entry, built once the pool arrives; counts update live
  const box = document.getElementById("debug-claims")!;
  const counts = new Map<string, HTMLElement>();
  world.onChange(() => {
    if (counts.size !== world.upgrades.length) {
      box.textContent = "";
      counts.clear();
      for (const u of world.upgrades) {
        const row = document.createElement("div");
        row.className = "claim-row";
        const name = document.createElement("span");
        name.className = "name";
        name.textContent = u.name;
        const count = document.createElement("span");
        count.className = "count";
        const minus = document.createElement("button");
        minus.textContent = "-";
        minus.onclick = () => sendDebug("claims", u.name, -1);
        const plus = document.createElement("button");
        plus.textContent = "+";
        plus.onclick = () => sendDebug("claims", u.name, 1);
        row.append(name, count, minus, plus);
        box.appendChild(row);
        counts.set(u.name, count);
      }
    }
    for (const [name, el] of counts) el.textContent = String(world.claims[name] ?? 0);
  });

  // one button per supported entity type; arming one hides the menu and lets
  // you click the map to drop it. Built once the type table arrives.
  const ebox = document.getElementById("debug-entities")!;
  world.onChange(() => {
    const ids = Object.keys(world.types).sort((a, b) =>
      world.types[a].name.localeCompare(world.types[b].name),
    );
    if (ebox.childElementCount === ids.length) return;
    ebox.textContent = "";
    for (const id of ids) {
      const sp = world.types[id];
      const b = document.createElement("button");
      b.dataset.id = id;
      const sw = document.createElement("span");
      sw.className = "swatch";
      sw.style.background = sp.color;
      const label = document.createElement("span");
      label.textContent = sp.name;
      b.append(sw, label);
      b.onclick = () => setPlacingEntity(placingEntity === id ? null : id);
      ebox.appendChild(b);
    }
  });
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
    if (placingEntity) {
      sendSpawnEntity(placingEntity, t.x, t.y);
      return; // stay armed so several can be dropped; Escape to stop
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
    `${sp?.name ?? e.s}${e.owner ? ` (${e.owner})` : ""}${e.dead ? " (dead)" : ""}`,
    `at ${e.x},${e.y}`,
  ];
  if (sp?.kind === "fauna" && !e.dead) {
    lines.push(`fullness ${e.full.toFixed(1)} / ${sp.stomachSize}`);
    if (sp.socialSize > 0) {
      lines.push(`social ${(e.soc ?? 0).toFixed(1)} / ${sp.socialSize}`);
    }
    if ((sp.carryCapacity ?? 0) > 0) {
      lines.push(`carrying ${e.ore ?? 0}/${sp.carryCapacity} ore`);
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
