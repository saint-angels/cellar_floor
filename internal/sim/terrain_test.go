package sim

import "testing"

func TestTerrainTypesAndMutation(t *testing.T) {
	if !Passable(TerrainFloor) || Passable(TerrainWater) || Passable(TerrainRock) {
		t.Error("passability wrong")
	}
	if !Mineable(TerrainRock) || Mineable(TerrainDirt) || Mineable(TerrainFloor) {
		t.Error("mineability wrong")
	}
	if TerrainName(TerrainFloor) != "floor" || TerrainName(TerrainRock) != "rock" {
		t.Error("terrain names wrong")
	}

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
