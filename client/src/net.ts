import { world } from "./world";
import type { SnapshotMsg, TickMsg } from "./types";

let ws: WebSocket | null = null;

export function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data) as SnapshotMsg | TickMsg;
    if (msg.type === "snapshot") world.applySnapshot(msg);
    else if (msg.type === "tick") world.applyTick(msg);
  };
  ws.onclose = () => setTimeout(connect, 1000);
}

export function sendTimescale(scale: number) {
  ws?.readyState === WebSocket.OPEN &&
    ws.send(JSON.stringify({ type: "timescale", scale }));
}
