package sim

import (
	"encoding/json"
	"testing"

	"cellarfloor/internal/data"
)

func levelCfg() *data.Config {
	cfg := mineCfg() // fast mining world helpers
	cfg.LevelBase = 2
	cfg.LevelGrowth = 2
	cfg.Upgrades = []data.Upgrade{
		{Name: "Sharper", Kind: "damage", Amount: 1, Max: 0},
		{Name: "Lucky", Kind: "luck", Amount: 1, Max: 1},
	}
	return cfg
}

func TestLevelTargetsEscalate(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	if got := w.NextLevelGold(); got != 2 {
		t.Fatalf("level 1 target = %d, want 2", got)
	}
	w.Level = 1
	if got := w.NextLevelGold(); got != 6 { // 2 + 4
		t.Fatalf("level 2 target = %d, want 6", got)
	}
	w.Level = 2
	if got := w.NextLevelGold(); got != 14 { // 2 + 4 + 8
		t.Fatalf("level 3 target = %d, want 14", got)
	}
}

func TestPrevLevelGold(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	if got := w.PrevLevelGold(); got != 0 {
		t.Fatalf("level 0 prev = %d, want 0", got)
	}
	w.Level = 1
	if got := w.PrevLevelGold(); got != 2 {
		t.Fatalf("level 1 prev = %d, want 2", got)
	}
	w.Level = 2
	if got := w.PrevLevelGold(); got != 6 { // 2 + 4
		t.Fatalf("level 2 prev = %d, want 6", got)
	}
}

func TestCrossingRollsAnOffer(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.GoldMined = 7 // covers targets 2 and 6
	evs := w.Step()
	if w.Level != 2 {
		t.Fatalf("level = %d, want 2", w.Level)
	}
	if w.PendingLevels != 2 {
		t.Fatalf("pendingLevels = %d, want 2", w.PendingLevels)
	}
	// the pool has two entries, so the offer holds both, distinct
	if len(w.Offer) != 2 || w.Offer[0] == w.Offer[1] {
		t.Fatalf("offer = %v, want two distinct choices", w.Offer)
	}
	named := 0
	for _, ev := range evs {
		if ev.Type == "level" {
			named++
		}
	}
	if named != 2 {
		t.Fatalf("level events = %d, want 2", named)
	}
	// determinism
	w2 := NewWorld(5, 5, 1, levelCfg())
	w2.GoldMined = 7
	w2.Step()
	for i := range w.Offer {
		if w.Offer[i] != w2.Offer[i] {
			t.Fatal("offers not deterministic")
		}
	}
}

func TestClaimOfferConsumesALevelAndRollsNext(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.GoldMined = 7
	w.Step() // level 2, pendingLevels 2, offer rolled
	if w.ClaimOffer("Nonsense") {
		t.Fatal("claiming an unoffered name must fail")
	}
	pick := w.Offer[0]
	if !w.ClaimOffer(pick) {
		t.Fatalf("claiming %q failed", pick)
	}
	if w.Claims[pick] != 1 || w.PendingLevels != 1 {
		t.Fatalf("claims %v pendingLevels %d after claim", w.Claims, w.PendingLevels)
	}
	if len(w.Offer) == 0 {
		t.Fatal("a stacked level must roll the next offer")
	}
	if !w.ClaimOffer(w.Offer[0]) {
		t.Fatal("second claim failed")
	}
	if w.PendingLevels != 0 || len(w.Offer) != 0 {
		t.Fatalf("queue must drain: pendingLevels %d offer %v", w.PendingLevels, w.Offer)
	}
}

func TestLegacyPendingMigratesToLevels(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Pending = []string{"Sharper", "Lucky"}
	w.SetConfig(levelCfg())
	if len(w.Pending) != 0 || w.PendingLevels != 2 || len(w.Offer) == 0 {
		t.Fatalf("migration: pending %v pendingLevels %d offer %v", w.Pending, w.PendingLevels, w.Offer)
	}
}

func TestPendingIsInertUntilClaimed(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.PendingLevels = 2
	w.Offer = []string{"Sharper", "Lucky"}
	if w.MineBonus() != 0 {
		t.Fatal("offered upgrades must not add damage")
	}
	w.Claims = map[string]int{"Sharper": 3}
	if w.MineBonus() != 3 {
		t.Fatalf("MineBonus = %d, want 3", w.MineBonus())
	}
}

func TestCappedEntriesLeaveTheDrawPool(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Claims = map[string]int{"Lucky": 1} // Lucky at max
	w.GoldMined = 1000
	for i := 0; i < 5; i++ {
		w.Step()
	}
	for _, name := range w.Offer {
		if name == "Lucky" {
			t.Fatal("capped entry offered")
		}
	}
	if w.Level == 0 || w.PendingLevels == 0 {
		t.Fatalf("levels should still accrue: level %d pendingLevels %d", w.Level, w.PendingLevels)
	}
}

func TestLuckRaisesDropBounds(t *testing.T) {
	w := newMineWorld(t) // chance 1, min=max=2
	w.Cfg().Upgrades = []data.Upgrade{{Name: "Lucky", Kind: "luck", Amount: 1, Max: 2}}
	w.Claims = map[string]int{"Lucky": 2}
	e := w.Spawn("miner", Point{2, 2})
	_ = e
	for i := 0; i < 30; i++ {
		w.Step()
	}
	if w.Gold != 4 { // 2 + luck 2, min == max keeps it exact
		t.Fatalf("gold = %d, want 4 with +2 luck", w.Gold)
	}
}

func TestLevelStateSurvivesSaveLoad(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Level = 3
	w.PendingLevels = 1
	w.Offer = []string{"Sharper"}
	w.Claims = map[string]int{"Lucky": 1}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var w2 World
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatal(err)
	}
	w2.SetConfig(levelCfg())
	if w2.Level != 3 || w2.PendingLevels != 1 || len(w2.Offer) != 1 || w2.Claims["Lucky"] != 1 {
		t.Fatalf("state lost: %d %d %v %v", w2.Level, w2.PendingLevels, w2.Offer, w2.Claims)
	}
}
