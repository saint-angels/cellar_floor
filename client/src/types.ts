export interface Species {
  id: string;
  name: string;
  kind: "flora" | "fauna";
  color: string;
  stomachSize: number;
  fearRadius: number;
  popFloor: number;
  popCap: number;
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
}

export interface SimEvent {
  tick: number;
  type: string;
  actor: number;
  actorSpecies: string;
  target?: number;
  targetSpecies?: string;
  msg: string;
}

export interface SnapshotMsg {
  type: "snapshot";
  tick: number;
  width: number;
  height: number;
  terrain: number[];
  species: Record<string, Species>;
  entities: EntityView[];
  timeScale: number;
}

export interface TickMsg {
  type: "tick";
  tick: number;
  timeScale: number;
  changed: EntityView[];
  removed: number[];
  events: SimEvent[];
  pops: Record<string, number>;
}

export interface RenderEntity extends EntityView {
  px: number; // previous x/y for interpolation
  py: number;
  movedAt: number; // performance.now() of last move
}
