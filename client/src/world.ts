import type { EntityView, PlayerMsg, RenderEntity, SimEvent, SnapshotMsg, TickMsg, Species } from "./types";

type Cb = () => void;

export class WorldState {
  width = 0;
  height = 0;
  terrain: number[] = [];
  species: Record<string, Species> = {};
  entities = new Map<number, RenderEntity>();
  tick = 0;
  timeScale = 1;
  tickIntervalMs = 500;
  gold = 0;
  mining: Record<string, number> = {};
  terrainVersion = 0;
  playerState: "unknown" | "none" | "alive" | "dead" = "unknown";
  playerDwarfId: number | null = null;
  playerName = "";
  playerError = "";
  popHistory: Record<string, number[]> = {};
  selectedId: number | null = null;

  private eventCbs: ((evs: SimEvent[]) => void)[] = [];
  private changeCbs: Cb[] = [];

  onEvents(cb: (evs: SimEvent[]) => void) { this.eventCbs.push(cb); }
  onChange(cb: Cb) { this.changeCbs.push(cb); }
  private fireChange() { for (const cb of this.changeCbs) cb(); }

  applySnapshot(m: SnapshotMsg) {
    this.width = m.width;
    this.height = m.height;
    // Go sends []uint8 terrain as a base64 string
    this.terrain = Array.from(Uint8Array.from(atob(m.terrain), (c) => c.charCodeAt(0)));
    this.terrainVersion++;
    this.gold = m.gold ?? 0;
    this.mining = m.mining ?? {};
    this.species = m.species;
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    this.entities.clear();
    for (const e of (m.entities ?? [])) this.upsert(e);
    this.checkOwnDwarf();
    this.fireChange();
  }

  // a reset snapshot or a tick can both take the player's dwarf away; ids
  // restart on reset, so the id may now belong to an unrelated entity
  private checkOwnDwarf() {
    if (this.playerState === "alive" && this.playerDwarfId != null) {
      const mine = this.entities.get(this.playerDwarfId);
      if (!mine || mine.dead || mine.s !== "dwarf") {
        this.playerState = "dead";
        this.playerDwarfId = null;
      }
    }
  }

  applyTick(m: TickMsg) {
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    for (const e of (m.changed ?? [])) this.upsert(e);
    for (const id of (m.removed ?? [])) this.entities.delete(id);
    this.gold = m.gold ?? this.gold;
    this.mining = m.mining ?? {};
    const diffs = m.terrain ?? [];
    if (diffs.length) {
      for (const d of diffs) this.terrain[d.i] = d.t;
      this.terrainVersion++;
    }
    this.checkOwnDwarf();
    for (const [sid, n] of Object.entries(m.pops ?? {})) {
      (this.popHistory[sid] ??= []).push(n);
      if (this.popHistory[sid].length > 120) this.popHistory[sid].shift();
    }
    const evs = m.events ?? [];
    if (evs.length) for (const cb of this.eventCbs) cb(evs);
    this.fireChange();
  }

  // a paused server stops broadcasting ticks, so the client applies the
  // scale it requested immediately; the next tick confirms it
  setTimescaleOptimistic(scale: number) {
    this.timeScale = scale;
    this.fireChange();
  }

  applyPlayer(m: PlayerMsg) {
    this.playerState = m.state;
    this.playerDwarfId = m.dwarfId ?? null;
    if (m.name) this.playerName = m.name;
    this.playerError = m.error ?? "";
    this.fireChange();
  }

  private upsert(e: EntityView) {
    const prev = this.entities.get(e.id);
    const re = e as RenderEntity;
    if (prev && (prev.x !== e.x || prev.y !== e.y)) {
      re.px = prev.x; re.py = prev.y; re.movedAt = performance.now();
    } else {
      re.px = prev?.px ?? e.x; re.py = prev?.py ?? e.y; re.movedAt = prev?.movedAt ?? 0;
    }
    this.entities.set(e.id, re);
  }
}

export const world = new WorldState();
