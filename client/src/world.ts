import type { EntityView, RenderEntity, SimEvent, SnapshotMsg, TickMsg, Species } from "./types";

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
  popHistory: Record<string, number[]> = {};
  selectedId: number | null = null;

  private eventCbs: ((evs: SimEvent[]) => void)[] = [];
  private popCbs: ((pops: Record<string, number>) => void)[] = [];
  private changeCbs: Cb[] = [];

  onEvents(cb: (evs: SimEvent[]) => void) { this.eventCbs.push(cb); }
  onPops(cb: (pops: Record<string, number>) => void) { this.popCbs.push(cb); }
  onChange(cb: Cb) { this.changeCbs.push(cb); }
  private fireChange() { for (const cb of this.changeCbs) cb(); }

  applySnapshot(m: SnapshotMsg) {
    this.width = m.width;
    this.height = m.height;
    this.terrain = m.terrain;
    this.species = m.species;
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    this.entities.clear();
    for (const e of m.entities) this.upsert(e);
    this.fireChange();
  }

  applyTick(m: TickMsg) {
    this.tick = m.tick;
    this.timeScale = m.timeScale;
    for (const e of m.changed) this.upsert(e);
    for (const id of m.removed) this.entities.delete(id);
    for (const [sid, n] of Object.entries(m.pops)) {
      (this.popHistory[sid] ??= []).push(n);
      if (this.popHistory[sid].length > 120) this.popHistory[sid].shift();
    }
    if (m.events.length) for (const cb of this.eventCbs) cb(m.events);
    for (const cb of this.popCbs) cb(m.pops);
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
