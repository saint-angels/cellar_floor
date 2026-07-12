# Ore Hauling and Market Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Mined blocks yield ore into the dwarf's bag; a market chest in the clearing converts hauled ore to colony gold; movement speed becomes an upgrade.

**Architecture:** Data-driven fields (`carry_capacity`, `market`, speed kind) flow through the existing toml -> sim -> wire -> canvas pipeline. The sim gains one AI step (haul) and one derived multiplier (SpeedFactor); payment moves from the mine roll to the deposit.

**Tech Stack:** Go server/sim, TypeScript canvas client, TOML data.

## Global Constraints

- Spec: docs/superpowers/specs/2026-07-13-ore-market-design.md — exact values live there (bag 3, Swift Boots kind "speed" amount 25 max 3, chest atlas coords (32,72), nugget color #d9c14a, float color #ffd75e, market color #b8860b).
- Commit messages: one sentence, under 70 chars, no Claude attribution, no em or en dashes anywhere.
- `CarryCapacity == 0` keeps the EXACT current instant-gold behavior; all existing fixtures must pass unmodified unless a test asserts the old dwarf economy specifically.
- The level bar (GoldMined) moves ONLY at deposit time for capacity types.
- gofmt clean; full `go vet ./... && go test -count=1 ./...` green per task from Task 2 on (Task 1 may leave sim tests red only if noted in the report; the repo must build).
- Client gate: `cd client && npx tsc --noEmit && npm run build`.
- Do not push; the controller pushes after final verification.

---

### Task 1: Data and worldgen (fields, toml, market placement)

**Files:** internal/data/data.go, internal/data/data_test.go, data/entities.toml, data/gen.toml, data/upgrades.toml, internal/gen/*.go (+ test)

**Interfaces produced:** `EntityType.CarryCapacity int` (toml carry_capacity, json carryCapacity), `EntityType.Market bool` (toml/json market), `GenConfig.Market string` (toml market), valid upgrade kind `speed` (positive amount; color/radius/period not required).

- [ ] data.go: add the three fields; validation: carry_capacity >= 0; kinds set gains "speed".
- [ ] entities.toml: dwarf `carry_capacity = 3`; add the `[type.market]` block from the spec. upgrades.toml: append Swift Boots. gen.toml: `market = "market"`.
- [ ] gen: after placing the center structure, if cfg.Gen.Market != "" spawn one entity of that type on the first passable tile scanning ring radius 1.. outward from center+(2,0), deterministic order (sort by cell index within each ring).
- [ ] Tests: parse/validation for the new fields (including rejecting negative carry_capacity and keeping unknown-kind rejection); pool has 7 entries, index 6 is Swift Boots exact shape; gen places exactly one market on passable ground near center, and none when Market is "".
- [ ] Gate: `go vet ./... && go test -count=1 ./internal/data/ ./internal/gen/` green; `go build ./...` green. Commit: "Add carry capacity market type and speed upgrades to the data"

### Task 2: Sim ore, hauling, deposit, speed factor, migration + wire

**Files:** internal/sim/world.go, internal/sim/mine.go, internal/sim/ai.go, internal/sim/tick.go (events only if needed), internal/sim/events.go, internal/server/protocol.go, new internal/sim/haul_test.go

**Interfaces consumed:** Task 1 fields. **Produced:** `Entity.Ore int` (json ore,omitempty), `sim.Event.Amount int` (json amount,omitempty), `World.SpeedFactor() float64`, EntityView `ore` on the wire.

- [ ] Entity.Ore; EntityView.Ore (json `ore,omitempty`), set in ViewOf.
- [ ] mine.go: branch the drop payout on `s.CarryCapacity > 0` per the spec (ore event type "ore", msg "<Name> struck ore"; else exact legacy behavior).
- [ ] ai.go: haulStep(e, fullOnly) + aiStep ordering per the spec (haul-full before mineStep, haul-partial after a false mineStep); action strings "hauling ore"/"selling"; deposit exactly as specced (Gold, GoldMined, GoldStrike+GoldLast24h, Event{Type:"sold", Amount, Actor, ActorType, Msg}); markDirty on ore changes.
- [ ] SpeedFactor over claimed speed upgrades; moveSpeed helper replaces the four `MoveAcc +=` sites.
- [ ] SetConfig migration: spawn one market by the campfire when the config has a Market type and the world lacks a living one (ring scan, passable), never duplicating.
- [ ] Tests (haul_test.go): the spec's sim list. Reuse mineCfg patterns; give the test miner carry_capacity where needed.
- [ ] Gate: full `go vet ./... && go test -count=1 ./...` green (the ~75s sim soak included). Commit: "Haul mined ore to the market for gold and claimable speed"

### Task 3: Client (chest, nugget, popup, sold float, speed desc, thought)

**Files:** client/public/sprites.png (regenerated), client/src/sprites.ts, render.ts, fx.ts, ui.ts, types.ts, data/entities.toml (thoughts only)

- [ ] Regenerate the atlas adding `chest_full_open_anim_f0.png` from `~/Downloads/0x72_DungeonTilesetII_v1.7/frames/` at (32, 72); keep every existing sprite position identical (extend the existing PIL script pattern; atlas stays 128x80). sprites.ts exports CHEST_X/CHEST_Y.
- [ ] types.ts: EntityView `ore?: number`; SimEvent `amount?: number`; EntityType `market?: boolean`.
- [ ] render.ts: market-typed structures draw the chest 12x12 when atlasReady (flame-pixel fallback); living dwarves with `ore > 0` get a 3px `#d9c14a` nugget at their lower-left.
- [ ] Popup + Your Dwarf: `carrying N/3 ore` line for capacity types (types carry carryCapacity via json).
- [ ] fx.ts: on events with type "sold", spawnFloat `+N` at the actor's tile in `#ffd75e` (world.onEvents, look the actor up in world.entities; skip if gone).
- [ ] ui.ts upgradeDesc: speed -> `+N% move speed`.
- [ ] entities.toml dwarf thoughts: add `{ when = "hauling", text = "off to market with my haul" }` BEFORE the always/idle rule; render.ts composeThought maps "hauling" to `(e.ore ?? 0) > 0`.
- [ ] Gate: client build clean plus `go test -count=1 ./internal/data/` (entities.toml still validates). Commit: "Show the market chest ore hauling and sold gold in the client"

### Task 4: End-to-end verification and docs (controller-supervised)

- [ ] e2e per the spec's list on :8083 (fresh world, REFRESHED scratch data dir).
- [ ] Update .claude/skills/verify/SKILL.md: the economy paragraphs (torch loop, "Ground truth ... gold drops") now describe ore -> haul -> market deposit, the `ore` field, the sold event, the speed kind, and the chest rendering.
- [ ] Full gate; the controller reviews everything and pushes.

## Self-Review Notes

- The capacity==0 legacy branch keeps every existing mine/level/luck test meaningful; only tests asserting dwarf-direct gold need touching (they should switch to a bagless "miner" type or assert via deposits).
- Luck applies at the ROLL (ore amount), not at deposit, so TestLuckRaisesDropBounds semantics survive with a capacity-0 miner.
- levelStep reads GoldMined and therefore fires at deposit; the e2e must assert the bar jumps at the chest, not at the face.
- pickMineTarget/haul interplay: hauling clears no MineTarget; a full dwarf simply walks to the market and back, its face may be re-picked naturally.
