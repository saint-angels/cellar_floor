# Dwarf Thoughts and Social Need Design

Date: 2026-07-11

## Purpose

Every dwarf shows a one-line thought bubble over its head, visible to all
players: how it feels about hunger, company, and recent gold. Behind the
loneliness thought sits a real social need that drains alone, refills in
company, and pulls lonely dwarves off the rock face to find each other.

Decisions from brainstorming:

- **Dominant thought only**: one line per dwarf, most pressing state wins.
  Full detail lives in the click popup.
- **Meter + proximity refill**: social is a hunger-like meter, not a
  consumable resource. Being near another dwarf refills both sides;
  nobody is depleted. (Literal produces/eats reuse was rejected: the
  eat-from-living-fauna path kills the producer.)
- **Priority below food, above mining**: lonely dwarves stay safe and fed
  first, but abandon the mine face to socialize, giving the colony a
  gather-and-disperse rhythm.
- **Sim computes state, client composes text**: the server streams three
  small fields; all copy lives in the client renderer.

## Social need (sim)

Fauna with `social_size > 0` get a second meter, `Entity.Social`,
processed in the metabolism pass. New data fields on the entity type,
following the unit-named convention:

```toml
social_size = 10          # meter capacity
social_drain_days = 2     # full to empty when alone
social_refill_hours = 1   # empty to full when in company
social_radius = 3         # company counts within this Chebyshev distance
social_threshold = 4      # below this, the dwarf is lonely
```

Each tick, if another living same-type fauna is within `social_radius`,
the meter refills at the refill rate and the sim records the companion:
`Entity.SeenID` (entity id) and `Entity.SeenTick`. Otherwise the meter
drains. Both companions refill; company is mutual. Types with
`social_size = 0` skip all of it (mushrooms, structures, legacy fauna).
Internal per-tick rates derive at load exactly like stomach drain:
`socialDrain = social_size / (social_drain_days * 86400 * tick_rate)`,
`socialRefill = social_size / (social_refill_hours * 3600 * tick_rate)`.

## Seeking company (AI)

New step in `aiStep` between food and mining. Rules:

- `Social < social_threshold` and no companion in radius: move toward the
  nearest living same-type fauna, action `"seeking company"`. If none
  exists in the world, skip (a lone survivor works instead of pacing).
- A companion is within radius and `Social < social_size`: stay put,
  action `"socializing"`, until the meter is full. Staying-until-full is
  the hysteresis that stops threshold oscillation.
- Meter full or no need: fall through to mining as today.

Expected rhythm: miners drift apart for ~2 days, get lonely, converge,
refill in ~1 hour, disperse back to their faces.

## Gold in the last 24h (sim)

Each gold strike appends `{tick, amount}` to `Entity.GoldStrikes`,
pruned to the trailing 24h (86400 * tick_rate ticks) whenever appended
or read. At one cell per real day this holds a handful of entries. The
pruned sum is the dwarf's "gold today".

## Wire and persistence

- `EntityView` gains `soc` (float, social level), `g24` (int, gold in
  last 24h), `seenId`/`seenTick` (omitempty), populated in `ViewOf`.
- The types map already serializes json-tagged fields; `socialSize` and
  `socialThreshold` ride along for the client's thresholds.
- `Entity` persists `Social`, `GoldStrikes`, `SeenID`, `SeenTick` in
  world.json. Loading an older save (or one from before this feature):
  a living entity whose type has `social_size > 0` but whose `Social`
  is 0 initializes to `social_size / 2`, mirroring how Spawn seeds
  Fullness. Spawn also seeds `Social = social_size / 2`.

## Thought bubble (client)

One line above each living dwarf (any fauna, driven by streamed data,
no type names in the renderer). Dominant state, first match wins:

1. `starving...` when fullness is 0
2. `hungry` when fullness < hunger threshold
3. `feeling lonely` when soc < social threshold
4. `struck N gold today!` when g24 > 0
5. `seen [name] recently!` when seenTick is within 24h; name resolves
   live from the seen entity's owner (`Misha`), else `a dwarf`
6. `content`

Rendering: ~9px monospace text centered above the tile in a dark bubble
(same palette as the inspector popup: background rgba(20,17,15,0.85),
text #cfc9bf), drawn after entities so bubbles sit on top. Bubbles of
adjacent socializing dwarves overlap; v1 accepts that. Dead dwarves show
no bubble. The click popup gains two lines: `social X / Y` and
`gold today: N`.

## Out of scope

Bubble collision avoidance, per-player thought privacy, social between
different types, thought history, sounds, social affecting mining speed
or health.

## Testing

- Sim: drain when alone; refill and mutual refill in radius; SeenID and
  SeenTick recorded; lonely dwarf walks toward the nearest dwarf;
  socializes until full then returns to mining (hysteresis); hungry and
  lonely dwarf eats first; lone survivor skips seeking; gold window
  prunes beyond 24h; save round-trip keeps social state; old-save
  migration seeds half-full social.
- Server: ViewOf carries soc, g24, seenId/seenTick.
- Client: build clean; headless e2e on an isolated port: advance until a
  dwarf is lonely and assert the bubble text renders, then bring dwarves
  together with further advances and assert the thought changes.
