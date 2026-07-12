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

func TestCrossingDrawsIntoPending(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.GoldMined = 7 // covers targets 2 and 6
	evs := w.Step()
	if w.Level != 2 {
		t.Fatalf("level = %d, want 2", w.Level)
	}
	if len(w.Pending) != 2 {
		t.Fatalf("pending = %v, want two draws", w.Pending)
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
	for i := range w.Pending {
		if w.Pending[i] != w2.Pending[i] {
			t.Fatal("draws not deterministic")
		}
	}
}

func TestPendingIsInertUntilClaimed(t *testing.T) {
	w := NewWorld(5, 5, 1, levelCfg())
	w.Pending = []string{"Sharper", "Sharper"}
	if w.MineBonus() != 0 {
		t.Fatal("pending upgrades must not add damage")
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
	for _, name := range w.Pending {
		if name == "Lucky" {
			t.Fatal("capped entry drawn")
		}
	}
	if w.Level == 0 || len(w.Pending) == 0 {
		t.Fatalf("levels should still accrue: level %d pending %d", w.Level, len(w.Pending))
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
	w.Pending = []string{"Sharper"}
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
	if w2.Level != 3 || len(w2.Pending) != 1 || w2.Claims["Lucky"] != 1 {
		t.Fatalf("state lost: %d %v %v", w2.Level, w2.Pending, w2.Claims)
	}
}
