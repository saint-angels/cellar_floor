package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

// socialCfg: fast social dynamics so tests run in a handful of ticks.
// Drain 1/tick from size 10, refill 2/tick, radius 2, threshold 4.
func socialCfg() *data.Config {
	return &data.Config{
		Sim: data.SimConfig{TickRate: 2},
		Types: map[string]*data.EntityType{
			"shroom": {ID: "shroom", Name: "Shroom", Kind: "flora", Color: "#fff",
				Produces: []data.Produce{{Resource: "shroom", Amount: 6, Max: 6}}},
			"dwarfish": {ID: "dwarfish", Name: "Dwarfish", Kind: "fauna", Color: "#fff",
				Eats: []string{"shroom"}, BiteSize: 2, StomachSize: 10, HungerThreshold: 0,
				Metabolism: 0.0001, StarveTicks: 100000, Speed: 1, Lifespan: 1 << 30,
				MatureAge: 1 << 30, PopCap: 10, DecayTicks: 100, MineTicks: 100,
				SocialSize: 10, SocialThreshold: 4, SocialRadius: 2,
				SocialDrain: 1, SocialRefill: 2},
			"sunstone": {ID: "sunstone", Name: "Sunstone", Kind: "structure", Color: "#fff",
				LightRadius: 40, Lifespan: 0},
		},
	}
}

func newSocialWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld(20, 20, 1, socialCfg())
	w.Spawn("sunstone", Point{0, 0}) // fully lit: no fear of the dark in these tests
	return w
}

func TestSocialSeededAndDrains(t *testing.T) {
	w := newSocialWorld(t)
	e := w.Spawn("dwarfish", Point{5, 5})
	if e.Social != 5 {
		t.Fatalf("spawn seeds social = %v, want half of 10", e.Social)
	}
	w.Step()
	if e.Social >= 5 {
		t.Fatalf("social should drain alone: %v", e.Social)
	}
}

func TestCompanyRefillsBothAndRecordsSeen(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{6, 5})
	a.Social, b.Social = 3, 3
	w.Step()
	if a.Social <= 3 || b.Social <= 3 {
		t.Fatalf("both should refill in company: %v %v", a.Social, b.Social)
	}
	if a.SeenID != b.ID || b.SeenID != a.ID || a.SeenTick != w.Tick {
		t.Fatalf("seen not recorded: a saw %d@%d, b saw %d", a.SeenID, a.SeenTick, b.SeenID)
	}
}

func TestLonelySeeksNearestCompanion(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{2, 2})
	b := w.Spawn("dwarfish", Point{15, 15})
	a.Social, b.Social = 2, 10
	before := Dist(a.Pos, b.Pos)
	w.Step()
	if a.Action != "seeking company" {
		t.Fatalf("action = %q, want seeking company", a.Action)
	}
	if Dist(a.Pos, b.Pos) >= before {
		t.Fatal("lonely dwarf must move toward its companion")
	}
}

func TestSocializesUntilFullThenReturnsToWork(t *testing.T) {
	w := newSocialWorld(t)
	w.Terrain[3+7*20] = TerrainRock // a face at {3,7} so mining is available
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{6, 5})
	a.Social, b.Social = 2, 10
	w.Step()
	if a.Action != "socializing" {
		t.Fatalf("action = %q, want socializing (companion already in radius)", a.Action)
	}
	for i := 0; i < 10 && a.Social < 10; i++ {
		w.Step()
	}
	if a.Social < 10 {
		t.Fatalf("social never filled: %v", a.Social)
	}
	w.Step()
	if a.Action == "socializing" || a.Action == "seeking company" {
		t.Fatalf("full dwarf should return to work, action = %q", a.Action)
	}
}

func TestHungryLonelyEatsFirst(t *testing.T) {
	w := newSocialWorld(t)
	cfg := w.Cfg()
	cfg.Types["dwarfish"].HungerThreshold = 4
	shroom := w.Spawn("shroom", Point{6, 5})
	_ = shroom
	a := w.Spawn("dwarfish", Point{5, 5})
	b := w.Spawn("dwarfish", Point{15, 15})
	_ = b
	a.Social, a.Fullness = 2, 1
	w.Step()
	if a.Action == "seeking company" || a.Action == "socializing" {
		t.Fatalf("hunger must outrank loneliness, action = %q", a.Action)
	}
}

func TestLoneSurvivorSkipsSeeking(t *testing.T) {
	w := newSocialWorld(t)
	w.Terrain[3+7*20] = TerrainRock
	a := w.Spawn("dwarfish", Point{5, 5})
	a.Social = 1
	w.Step()
	if a.Action == "seeking company" {
		t.Fatal("a lone survivor has nobody to seek and should work instead")
	}
}

func TestSocialSurvivesSaveLoadWithMigration(t *testing.T) {
	w := newSocialWorld(t)
	a := w.Spawn("dwarfish", Point{5, 5})
	a.Social = 7.5
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(socialCfg())
	if w2.Entities[a.ID].Social != 7.5 {
		t.Fatalf("social lost in round trip: %v", w2.Entities[a.ID].Social)
	}
	// migration: an old save has Social 0 on a living social fauna
	w2.Entities[a.ID].Social = 0
	w2.SetConfig(socialCfg())
	if w2.Entities[a.ID].Social != 5 {
		t.Fatalf("migration should seed half-full: %v", w2.Entities[a.ID].Social)
	}
}
