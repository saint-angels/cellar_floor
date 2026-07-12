import { world } from "./world";
import type { PlayerMsg, RecapMsg, SnapshotMsg, TickMsg } from "./types";

let ws: WebSocket | null = null;

const TOKEN_KEY = "cellar-player-token";

function playerToken(): string {
  let t = localStorage.getItem(TOKEN_KEY);
  if (!t) {
    t = crypto.randomUUID();
    localStorage.setItem(TOKEN_KEY, t);
  }
  return t;
}

export function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => ws?.send(JSON.stringify({ type: "hello", player: playerToken() }));
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data) as SnapshotMsg | TickMsg | PlayerMsg | RecapMsg;
    if (msg.type === "snapshot") world.applySnapshot(msg);
    else if (msg.type === "tick") world.applyTick(msg);
    else if (msg.type === "player") world.applyPlayer(msg);
    else if (msg.type === "recap") world.applyRecap(msg);
  };
  ws.onclose = () => setTimeout(connect, 1000);
}

// world-level controls carry the admin token; public servers ignore
// these intents without it (set via localStorage.setItem("admin", ...))
function adminToken(): string {
  return localStorage.getItem("admin") ?? "";
}

export function sendTimescale(scale: number) {
  if (ws?.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "timescale", scale, admin: adminToken() }));
  world.setTimescaleOptimistic(scale);
}

export function sendReset() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "reset", admin: adminToken() }));
}

export function sendDebug(action: string, name = "", n = 0) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "debug", action, name, n, admin: adminToken() }));
}

export function sendSpawn(name: string) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "spawn", player: playerToken(), name }));
}

export function sendTorch(x: number, y: number) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "torch", player: playerToken(), x, y }));
}

export function sendClaim() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "claim", player: playerToken() }));
}
