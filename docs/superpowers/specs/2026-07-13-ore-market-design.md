# Ore Hauling and the Market Design

Date: 2026-07-13

## Purpose

Mining no longer pays gold at the rock face. Blocks yield ORE into the
miner's bag; the dwarf hauls a full bag back to a MARKET in the clearing
and sells it there for colony gold. The walk is the point: movement
becomes visible prosperity and movement speed becomes a progression
stat (new Swift Boots upgrade).

Decisions: bag size 3 (data-driven per type), Swift Boots speed upgrade
(+25% per claim, max 3), the market renders as the 0x72 golden chest.

## Data

- `EntityType` gains `CarryCapacity int` (toml `carry_capacity`, json
  `carryCapacity`, default 0, validated non-negative) and `Market bool`
  (toml/json `market`). Dwarf gets `carry_capacity = 3`.
- New structure in entities.toml:

```toml
[type.market]
name = "Market"
kind = "structure"
color = "#b8860b"
market = true
```

  No lifespan (permanent, like the campfire), no light.
- gen.toml gains `market = "market"`: worldgen places one entity of
  that type on the first passable tile scanning a clockwise ring
  outward from `center + (2, 0)`. Empty string = no market placed.
- upgrades.toml gains:

```toml
[[upgrade]]
name = "Swift Boots"
kind = "speed"
amount = 25
max = 3
```

  New kind `speed` (amount = percent added to move speed). Validation:
  kinds set gains "speed"; speed needs positive amount and no
  color/radius/period requirement.

## Sim

- `Entity.Ore int` (json `ore,omitempty`), persisted.
- The mine drop roll keeps its chances and luck bonus but the payout
  branches on the miner's type: `CarryCapacity > 0` adds the rolled
  amount to `e.Ore` (event type "ore", msg "<name> struck ore");
  `CarryCapacity == 0` keeps TODAY'S behavior exactly (instant
  Gold/GoldMined/GoldStrike) so simple fixtures and any future
  bagless miner still work.
- New `haulStep(e, fullOnly bool)` and aiStep ordering:

```
flee > food > social
haulStep(e, true)   // bag full: go sell
mineStep(e)
haulStep(e, false)  // bag has ore but nothing minable: sell the rest
shelter > wander
```

  haulStep finds the nearest living entity whose type has Market
  (path via pathToward, action "hauling ore"); when adjacent
  (Chebyshev <= 1) it deposits: `w.Gold += e.Ore`,
  `w.GoldMined += e.Ore` (the LEVEL BAR moves at the market, not the
  rock), a GoldStrike record for the thought/g24 systems, event
  `{Type: "sold", Amount: n, Msg: "<name> sold <n> ore"}`, action
  "selling", `e.Ore = 0`. No market reachable = keep mining (return
  false; ore just accumulates past the cap harmlessly).
- `sim.Event` gains `Amount int` (json `amount,omitempty`).
- Speed: `World.SpeedFactor() float64 = 1 + sum(amount*claims)/100`
  over claimed `speed` upgrades; a `moveSpeed(e)` helper replaces the
  four `e.MoveAcc += ...Speed` sites (ai.go x3, mine.go x1) so every
  walk benefits.
- Migration: SetConfig spawns one market next to the campfire (first
  passable ring tile) when the config defines a Market type but the
  world has no living one. Old saves get their market without a reset.

## Server / wire

- `EntityView.Ore int` (json `ore,omitempty`).
- Events pass through as-is; "sold" gets the same owner-name
  decoration as other actor events.

## Client

- Atlas gains the golden chest (`chest_full_open_anim_f0`, 16x16) at
  (32, 72); regenerate client/public/sprites.png with the same PIL
  script pattern. sprites.ts exports the chest coords.
- render.ts: structures whose type has `market` render the chest
  sprite (12x12, fallback to the flame pixel pre-atlas); dwarves with
  `ore > 0` get a 3px `#d9c14a` nugget drawn at their lower-left, so
  loaded dwarves read at a glance.
- Popup and Your Dwarf panel gain a `carrying N/3 ore` line for types
  with capacity.
- On a "sold" event, fx pops a gold `+N` float (color `#ffd75e`) at
  the seller's position (event Actor id).
- Offer card desc for speed: `+N% move speed`.
- entities.toml dwarf thoughts gain
  `{ when = "hauling", text = "off to market with my haul" }`;
  composeThought maps "hauling" to `(e.ore ?? 0) > 0`.

## Compatibility

Old saves: Ore defaults 0; market spawned on load; forge-era fields
unaffected. Live claims keep working; Swift Boots simply joins the
draw pool. Balance: drop rates unchanged, so income rate now equals
mining rate minus walk time; the crust ring sits 1-2 tiles from the
clearing so early trips are short, deep tunnels pay for Swift Boots.

## Out of scope

Bag-size upgrades, multiple markets, per-ore-type prices, corpse
looting, market construction by players.

## Testing

- data: carry_capacity/market parse and validation; speed kind valid,
  bad kinds still rejected; pool count 7 with Swift Boots shape.
- gen: market placed near center on passable ground; absent when
  gen.market is "".
- sim: ore accrues instead of gold (capacity type) and legacy direct
  gold still works (capacity 0); full bag hauls to market, deposits,
  Gold/GoldMined jump by the bag, sold event carries Amount; partial
  haul when no lit faces remain; SpeedFactor moves a dwarf measurably
  faster with claims; SetConfig spawns exactly one market for old
  saves and never duplicates; Ore survives save round-trip.
- server: EntityView carries ore; sold event decorated with the
  owner's name.
- e2e on :8083: fresh world, spawn, torch the crust, watch the full
  loop (ore pops at the face, dwarf walks to the chest, gold and the
  level bar jump on deposit); claim Swift Boots via debug and see
  trips speed up; screenshots of the chest, a loaded dwarf, and the
  deposit float.
