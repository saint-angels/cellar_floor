export interface EntityType {
  id: string;
  name: string;
  kind: "flora" | "fauna" | "structure";
  color: string;
  stomachSize: number;
  hungerThreshold: number;
  socialSize: number;
  socialThreshold: number;
  fearRadius: number;
  popFloor: number;
  popCap: number;
  lightRadius: number;
  eats: string[] | null;
  shelters: string[] | null;
}

export interface EntityView {
  id: number;
  s: string;
  x: number;
  y: number;
  dead?: boolean;
  full: number;
  action?: string;
  home?: { x: number; y: number };
  res?: Record<string, number>;
  owner?: string;
  mt?: { x: number; y: number };
  soc?: number;
  g24?: number;
  seenId?: number;
  seenTick?: number;
}

export interface PlayerMsg {
  type: "player";
  state: "none" | "alive" | "dead";
  dwarfId?: number;
  name?: string;
  error?: string;
}

export interface SimEvent {
  tick: number;
  type: string;
  actor: number;
  actorType: string;
  target?: number;
  targetType?: string;
  msg: string;
}

export interface TerrainDiff {
  i: number;
  t: number;
}

export interface SnapshotMsg {
  type: "snapshot";
  tick: number;
  width: number;
  height: number;
  terrain: string; // base64-encoded byte per cell (Go marshals []uint8 as base64)
  types: Record<string, EntityType>;
  entities: EntityView[];
  timeScale: number;
  gold: number;
  mining?: Record<string, number> | null;
}

export interface TickMsg {
  type: "tick";
  tick: number;
  timeScale: number;
  changed: EntityView[];
  removed: number[];
  events: SimEvent[];
  pops: Record<string, number>;
  gold: number;
  mining?: Record<string, number> | null;
  terrain?: TerrainDiff[] | null;
}

export interface RenderEntity extends EntityView {
  px: number; // previous x/y for interpolation
  py: number;
  movedAt: number; // performance.now() of last move
}
