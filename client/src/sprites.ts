import { world } from "./world";

// One atlas from the CC0 0x72 Dungeon Tileset II. Rows: floor variants
// (16x16 at y 0), dwarf_m frames (16x28 at y 16), dwarf_f frames (16x28
// at y 44, both idle f0-f3 then run f0-f3), coin frames (6x7 at y 72,
// x 0..31), then the market chest (16x16) at (32, 72).
export const atlas = new Image();
export let atlasReady = false;
atlas.src = "/sprites.png";
atlas.onload = () => {
  atlasReady = true;
  world.terrainVersion++; // repaint the terrain layer with floor tiles
};

export const FLOOR_Y = 0;
export const DWARF_ROWS = [16, 44]; // m, f
export const DWARF_W = 16;
export const DWARF_H = 28;
export const CHEST_X = 32;
export const CHEST_Y = 72;
export const CHEST_W = 16;
export const CHEST_H = 16;

// stable per-cell floor variant so the pattern does not shimmer
export function floorVariant(x: number, y: number): number {
  return ((x * 7 + y * 13 + ((x * y) | 0)) % 8 + 8) % 8;
}
