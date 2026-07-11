package sim

import "testing"

func TestTerrainTableMethods(t *testing.T) {
	w := flatWorld(t, 4, 4, 1) // its cfg carries the canonical five plus an appended soft type
	if !w.Passable(TerrainFloor) || w.Passable(TerrainWater) || w.Passable(TerrainRock) {
		t.Error("passability wrong")
	}
	if !w.Mineable(TerrainRock) || w.Mineable(TerrainDirt) || w.Mineable(TerrainFloor) {
		t.Error("mineability wrong")
	}
	soft := Terrain(5) // appended in the test cfg
	if !w.Mineable(soft) || w.Passable(soft) {
		t.Error("appended terrain not honored")
	}
	if w.TerrainName(soft) != "softish" || w.TerrainName(TerrainRock) != "rock" {
		t.Error("terrain names wrong")
	}
	if w.Passable(Terrain(99)) || w.Mineable(Terrain(99)) || w.TerrainName(Terrain(99)) != "unknown" {
		t.Error("out of range must be inert")
	}
}

func TestSetTerrainDirtyTracking(t *testing.T) {
	w := flatWorld(t, 4, 4, 1)
	w.SetTerrain(Point{2, 1}, TerrainFloor)
	w.SetTerrain(Point{2, 1}, TerrainFloor) // no-op, already floor
	w.SetTerrain(Point{3, 3}, TerrainRock)
	d := w.TerrainDirtyAndReset()
	if len(d) != 2 || d[0] != 1*4+2 || d[1] != 3*4+3 {
		t.Errorf("dirty = %v, want [6 15]", d)
	}
	if w.At(Point{2, 1}) != TerrainFloor {
		t.Error("SetTerrain did not apply")
	}
	if len(w.TerrainDirtyAndReset()) != 0 {
		t.Error("dirty set not reset")
	}
}
