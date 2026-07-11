import { world } from "./world";
import type { PlayerMsg, SnapshotMsg, TickMsg } from "./types";

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
    const msg = JSON.parse(ev.data) as SnapshotMsg | TickMsg | PlayerMsg;
    if (msg.type === "snapshot") world.applySnapshot(msg);
    else if (msg.type === "tick") world.applyTick(msg);
    else if (msg.type === "player") world.applyPlayer(msg);
  };
  ws.onclose = () => setTimeout(connect, 1000);
}

export function sendTimescale(scale: number) {
  if (ws?.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "timescale", scale }));
  world.setTimescaleOptimistic(scale);
}

export function sendReset() {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "reset" }));
}

export function sendSpawn(name: string) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "spawn", player: playerToken(), name }));
}

export function sendTorch(x: number, y: number) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "torch", player: playerToken(), x, y }));
}
