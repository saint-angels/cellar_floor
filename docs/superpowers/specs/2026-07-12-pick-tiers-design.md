# Pick Tiers and Welcome-Back Recap Design

Date: 2026-07-12

## Purpose

Close the idle loop: mine, collect, IMPROVE. A colony-wide track of pick
upgrades turns gold into visibly bigger damage numbers and faster
mining, with the first purchase affordable inside the opening minute.
A welcome-back recap makes returning feel like opening a present
instead of squinting at a slightly different map.

Approved shape: colony-wide tiers (not per dwarf), additive integer
damage bonuses, escalating costs, bought from shared gold like torches;
recap built from world counters.

## Upgrades data

New `data/upgrades.toml`, ordered purchase track (buy strictly in
order):

```toml
# colony pick tiers, bought in order with shared gold
[[upgrade]]
name = "Copper Picks"
cost = 3
damage = 1

[[upgrade]]
name = "Iron Picks"
cost = 8
damage = 1

[[upgrade]]
name = "Steel Picks"
cost = 20
damage = 2

[[upgrade]]
name = "Mithril Picks"
cost = 50
damage = 3

[[upgrade]]
name = "Adamant Picks"
cost = 120
damage = 5
```

`data.Upgrade{Name string; Cost, Damage int}`, `Config.Upgrades
[]Upgrade` loaded from the file (missing file is an error like the
other data files; an empty list is valid and disables the feature).
Validation: names non-empty and unique, cost positive, damage positive.

Effective mining damage for any miner = its `mine_damage` + the sum of
purchased tiers' damage. With base 1 the curve runs 1, 2, 3, 5, 8, 13.
Tier growth later is a data edit.

## Sim

- `World.UpgradeLevel int` (json `upgradeLevel`): count of purchased
  tiers, persisted, zeroed by world reset (fresh world starts over).
- `(w *World) MineBonus() int`: sum of `Damage` over the first
  UpgradeLevel entries of `cfg.Upgrades`, clamped to the table length.
- `mineStep` deals `s.MineDamage + w.MineBonus()` per tick per cell
  (the AOE loop's single damage constant changes, nothing else).
- Recap counters on World, all persisted ints incremented where the
  events happen: `BlocksMined` (every completed cell), `GoldMined`
  (cumulative gold dropped, never decremented by spending),
  `MoldGrown` (every spread conversion and sprout).

## Server

- ws intent `{type:"upgrade", player}`: requires a living dwarf, a next
  tier to exist, and gold >= its cost. On success: gold -= cost,
  UpgradeLevel++, pending event `"<name> forged <tier name>"`. Errors
  reply on the connection like torch errors ("nothing left to forge",
  "not enough gold").
- Recap: `Player` gains `SeenTick int64`, `SeenBlocks, SeenGold,
  SeenMold int` (persisted in players.json). On every `hello` for a
  known player, the server sends a new `{type:"recap", ticks, blocks,
  gold, mold}` message (deltas since the stored snapshot, ticks =
  elapsed sim ticks) and then updates the snapshot to now. New players
  get no recap.
- Wire: SnapshotMsg gains `upgrades []data.Upgrade` (the table) and
  `upgradeLevel int`; TickMsg gains `upgradeLevel int` (cheap, like
  gold).

## Client

- Forge button next to the torch button: shows the next tier as
  `"Copper Picks (3g)"`, enabled when the player is alive and gold
  covers the cost; `"picks maxed"` disabled state when the track is
  done. Sends the upgrade intent; errors surface in the existing
  torch-error line. The purchase is visible instantly in the world:
  bigger floating numbers everywhere (no client rendering changes
  needed).
- Recap toast: on a recap message where `ticks >= 120` (a minute of
  absence) and any counter is nonzero, show a dismissible toast at the
  top of the map: `"While you were away (2h 13m): 14 blocks mined, 9
  gold mined, 3 tunnels molded over"`. Duration formatted from ticks
  at 2 ticks/s (minutes under an hour, hours+minutes above). Click to
  dismiss, auto-fade after 12 s. Zero-count parts are omitted.

## Opening micro-loop (why the numbers work)

Spawn purse is 5: torch (1g) directs the dwarf, crust blocks pop "3"s
(mold hp 6 at damage 1 takes 3 s... at damage 1 each strike delta ~3),
and Copper Picks (3g) is affordable immediately; after buying, the same
crust pops twice as fast with double deltas. Mine, collect, improve
inside 20 seconds. Crust gold (10% of ~56 blocks) plus early rock work
funds Iron within the first session; the day-scale rock economy then
stretches the same loop to the log-off/return rhythm.

## Out of scope

Per-dwarf tools or inventories, prestige/reset bonuses, recap event
lists (names of who struck gold), non-mining upgrades, refunds.

## Testing

- data: table parses in order; empty file valid; duplicate name, zero
  cost, zero damage rejected.
- sim: MineBonus sums purchased tiers and clamps; mineStep applies the
  bonus (a 10 hp cell dies in 5 ticks at level 1 with base 1... with
  bonus 1 = damage 2); counters increment on completion, gold drop,
  spread, and sprout; save round-trip keeps UpgradeLevel and counters.
- server: upgrade intent validation branches and event; recap deltas
  computed against the snapshot and snapshot advanced; recap absent
  for unknown tokens; wire carries upgrades table and level.
- e2e on an isolated port: buy Copper via the button, watch the same
  face's float deltas double (screenshot before/after); disconnect,
  advance a few thousand ticks, reconnect, recap toast appears with
  plausible numbers (screenshot); push publishes everything.
