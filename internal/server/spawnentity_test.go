package server

import "testing"

func TestSpawnEntityHappyPath(t *testing.T) {
	s := newPlayerServer(t)
	p := findFreeDirt(t, s)
	before := s.world.CountAlive("campfire")
	ev := s.spawnEntity("campfire", p.X, p.Y)
	if ev == nil {
		t.Fatal("spawnEntity returned nil on a valid tile")
	}
	if ev.Type != "placed" {
		t.Fatalf("event type = %q, want placed", ev.Type)
	}
	if got := s.world.CountAlive("campfire"); got != before+1 {
		t.Fatalf("campfire count = %d, want %d", got, before+1)
	}
	found := false
	for _, e := range s.world.Entities {
		if e.Type == "campfire" && e.Pos == p {
			found = true
		}
	}
	if !found {
		t.Fatal("campfire not spawned on the target tile")
	}
}

func TestSpawnEntityValidation(t *testing.T) {
	s := newPlayerServer(t)
	free := findFreeDirt(t, s)

	if ev := s.spawnEntity("dragon", free.X, free.Y); ev != nil {
		t.Fatal("unknown type must be rejected")
	}
	if ev := s.spawnEntity("campfire", -1, 0); ev != nil {
		t.Fatal("out of bounds must be rejected")
	}
	rock := findRock(t, s)
	if ev := s.spawnEntity("campfire", rock.X, rock.Y); ev != nil {
		t.Fatal("impassable cell must be rejected")
	}
	structPos := findStructure(t, s)
	if ev := s.spawnEntity("campfire", structPos.X, structPos.Y); ev != nil {
		t.Fatal("a second structure on an occupied cell must be rejected")
	}
}

// Fauna and flora may share a tile with a structure, unlike a second
// structure, so placing them on the gen campfire's cell is allowed.
func TestSpawnEntityNonStructureStacksOnStructure(t *testing.T) {
	s := newPlayerServer(t)
	structPos := findStructure(t, s)
	if ev := s.spawnEntity("mushroom", structPos.X, structPos.Y); ev == nil {
		t.Fatal("a mushroom on a structure cell should be allowed")
	}
}

// Food (flora) can be buried in rock so dwarves dig to it; a live animal
// cannot be dropped inside a wall.
func TestSpawnFoodBuriesInRockButFaunaCannot(t *testing.T) {
	s := newPlayerServer(t)
	rock := findRock(t, s)
	if ev := s.spawnEntity("mushroom", rock.X, rock.Y); ev == nil {
		t.Fatal("flora should bury into mineable rock")
	}
	if ev := s.spawnEntity("rabbit", rock.X, rock.Y); ev != nil {
		t.Fatal("fauna must not spawn inside rock")
	}
}
