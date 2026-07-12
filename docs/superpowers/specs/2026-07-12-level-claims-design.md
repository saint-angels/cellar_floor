# Level Bar and Claimed Upgrades Design

Date: 2026-07-12

## Purpose

Replace the forge-a-tier button with a Vampire Survivors style level
bar: cumulative mined gold fills it, each full bar draws ONE random
upgrade from a data pool, and the upgrade sits PENDING until a player
sees it and clicks Claim. Nothing ever applies behind the scenes
(user requirement); away-time levels queue up, so returning means
clicking through a stack of reveals.

Decisions: draws are uniform over non-maxed pool entries; confirmation
is mandatory for every upgrade; the tier/forge system from the previous
feature is superseded (its wire, intent, and bonus plumbing is reused).

## Data (upgrades.toml reshaped)

```toml
# level curve: reaching level N requires level_base * level_growth^(N-1)
# cumulative mined gold on top of the previous level
level_base = 2.0
level_growth = 1.6

# the draw pool; max = 0 means infinitely stackable
[[upgrade]]
name = "Sharper Picks"
kind = "damage"
amount = 1
max = 0

[[upgrade]]
name = "Lucky Veins"
kind = "luck"
amount = 1
max = 2

[[upgrade]]
name = "Chisel"
kind = "weapon"
amount = 1
max = 1
color = "#e8d44d"
radius = 10
period_ms = 1100

[[upgrade]]
name = "Hammer"
kind = "weapon"
amount = 2
max = 1
color = "#b87333"
radius = 18
period_ms = 2300
```

`data.Upgrade{Name, Kind string; Amount, Max int; Color string;
Radius int; PeriodMs int}`; `Config.Upgrades []Upgrade`,
`Config.LevelBase, LevelGrowth float64` from the same file. Kinds v1:
`damage` (adds Amount to every miner's damage), `luck` (adds Amount to
gold_min AND gold_max on drops), `weapon` (adds Amount damage AND an
extra orbiting tool drawn client-side with the entry's color, radius,
period). Validation: kinds from the fixed set; names unique and
non-empty; amount positive; max non-negative; weapon entries need
color and positive radius/period; level_base > 0, level_growth > 1.

## Sim

- World state (persisted): `Level int` (levels earned = draws made),
  `Pending []string` (drawn upgrade names, oldest first, unclaimed and
  INERT), `Claims map[string]int` (claimed counts by name). The old
  `UpgradeLevel` field is removed; old saves' value is dropped
  silently (the live world is young).
- Level thresholds: cumulative mined gold needed for level N is
  `sum(level_base * level_growth^(k-1) for k in 1..N)`, computed in
  floats and compared against GoldMined; a helper returns the target
  for the next level.
- A new tick step (after spreadStep): while `GoldMined >=
  nextLevelTarget(Level)`, increment Level and draw uniformly (world
  RNG) from pool entries whose `Claims[name] + pending occurrences <
  Max` (Max 0 = no cap); append the name to Pending and fire an event
  `"the colony reached level N: <name> awaits"`. If every entry is
  capped, still increment Level but enqueue nothing (event: "level N
  reached").
- Effects read CLAIMED counts only: `MineBonus()` = sum over claims of
  (damage and weapon kinds) Amount * count; gold drops use
  `gold_min + LuckBonus()` and `gold_max + LuckBonus()`.
- Determinism: draws happen in the tick loop via world RNG.

## Server

- The forge `upgrade` intent, `buyUpgrade`, and its tests are REPLACED
  by `{type:"claim", player}` -> `claimUpgrade(token)`: requires a
  living dwarf and a non-empty Pending; pops Pending[0], increments
  Claims[name], pending event `"<player> claimed <name>"`; errors
  "you need a living dwarf" / "nothing to claim".
- Wire: SnapshotMsg carries `upgrades` (pool), `level`,
  `goldMined`, `nextLevelGold` (target for the next level),
  `pending []string`, `claims map[string]int`. TickMsg carries
  `level`, `goldMined`, `nextLevelGold`, `pending`, `claims` (small;
  pending/claims only when changed is an optimization we skip, they
  are tiny). The previous upgradeLevel field is gone.
- Recap: unchanged mechanically, but the toast copy gains claims:
  the client appends `"N upgrades await!"` when pending is non-empty
  on reconnect (client-side, from the snapshot; no recap message
  change).

## Client

- Replace the forge button area with: a level bar (label `Lv 7`,
  fill = progress from the previous threshold to `nextLevelGold`) that
  updates every tick, and when `pending` is non-empty a claim card
  (reusing the toast styling) at the top of the map: `"Level 8
  reached: Chisel"` with a Claim button (enabled when alive), plus
  `"+2 more"` when the queue is deeper. Claiming pops the next one
  into view until the queue drains.
- Weapons render as extra orbiting tools on every mining dwarf: for
  each claimed weapon entry, an additional orbit using the entry's
  color/radius/period (same strike/debris/shake/number machinery keyed
  per cell; extra orbits also trigger strikes). Claimed damage shows
  up in the numbers automatically.
- The `#recap` toast text appends the pending-claims line.

## Compatibility and migration

Old saves load; the removed UpgradeLevel json field is ignored by Go.
Colonies that had forged tiers lose those bonuses (acceptable: the
live world is fresh and the curve refunds quickly). upgrades.toml's
new shape replaces the old tier list in the same commit.

## Out of scope

Weighted rarities, pity timers, per-player claims, speed or
torch-radius upgrade kinds, weapon leveling, choosing between options.

## Testing

- data: curve + pool parse; kind/weapon-field/curve validation.
- sim: threshold math (targets escalate by growth); crossing draws
  deterministically, appends pending, fires the event; capped entries
  excluded from draws; all-capped still levels; effects apply ONLY
  after claiming (pending damage does nothing); luck raises drop
  bounds; save round-trip keeps Level/Pending/Claims.
- server: claim intent branches, event, pop order (FIFO); wire fields.
- client: build; e2e: fill the first bar live (crust mining), claim
  card appears, claim applies (damage delta doubles when Sharper Picks
  drawn... assert via mining deltas whichever upgrade the seed draws),
  bar advances to Lv 2 target; away-stacking: advance far offline,
  reconnect shows multiple pending and the recap line; screenshots.
