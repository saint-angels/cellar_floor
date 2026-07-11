# Human-Readable Durations Design

Date: 2026-07-11

## Purpose

Raw tick counts and per-tick rates in the data files are unreadable
(`mine_ticks = 172800`, `metabolism = 0.00006`). Replace them with numeric
fields whose names carry the unit, in the style of the existing
`autosave_minutes = 5`. No duration strings like "1d"; plain numbers,
floats allowed.

## Approach

Convert at load; the engine is untouched. `data.EntityType` keeps its
internal tick and rate fields (`MineTicks`, `StarveTicks`, `DecayTicks`,
`Lifespan`, `MatureAge`, `Metabolism`, `Speed`, and `Produce.Regrow`) but
those fields lose their TOML tags (`toml:"-"`). New unit-named fields
carry the TOML tags and `json:"-"`. `data.Load` converts new fields into
the internal ones immediately after decoding, using `tick_rate` from
sim.toml (already loaded first). Save format, wire format, sim engine,
and client are untouched.

## Field mapping

Fixed units per field; one unit for all entity types:

| Old TOML field       | New TOML field        | Meaning                          | Dwarf/torch values |
|----------------------|-----------------------|----------------------------------|--------------------|
| `mine_ticks`         | `mine_hours`          | time to mine one rock cell       | dwarf 24           |
| `starve_ticks`       | `starve_hours`        | time on an empty stomach to die  | dwarf 48           |
| `decay_ticks`        | `decay_hours`         | corpse/stub lingers before removal | dwarf 24, torch 0.5 |
| `lifespan`           | `lifespan_days`       | max age; 0 = immortal            | dwarf 58, torch 1, campfire 0 |
| `mature_age`         | `mature_days`         | age before reproduction          | dwarf 6            |
| `metabolism`         | `stomach_drain_hours` | full stomach to empty            | dwarf 24           |
| `speed`              | `cells_per_second`    | walking speed                    | dwarf 1.0          |
| `produces[].regrow`  | `produces[].regrow_days` | empty to full regrowth        | mushroom 1.75      |

`autosave_minutes`, `tick_rate`, and the gold knobs stay as they are.

## Conversion formulas (in Load, after decode)

With `tr = cfg.Sim.TickRate`:

- Tick counts round to the nearest whole tick:
  `MineTicks = round(mine_hours * 3600 * tr)`, same for starve/decay;
  `Lifespan = round(lifespan_days * 86400 * tr)`, same for mature.
- Rates are derived directly, no intermediate tick rounding:
  `Metabolism = StomachSize / (stomach_drain_hours * 3600 * tr)` when the
  drain is positive, else 0.
  `Regrow = Max / (regrow_days * 86400 * tr)` when regrow_days is
  positive, else 0.
  `Speed = cells_per_second / tr`.

Zero keeps its existing "off" meaning everywhere: `lifespan_days 0` =
immortal, `regrow_days 0` = never regrows, `mine_hours 0` = not a miner.

## Value changes accepted

The live dwarf/mushroom values are rounded to clean times; all shifts are
cosmetic at this pace:

- `starve_hours = 48` (was 48.61 h): ~1% sooner starvation.
- `stomach_drain_hours = 24` (was 23.15 h): ~4% slower hunger.
- `lifespan_days = 58` (was 57.87 d) and `mature_days = 6` (was 5.79 d):
  irrelevant while dwarf repro_chance is 0.
- mushroom `regrow_days = 1.75` (was 1.736 d).

Mining, decay, torch lifespan/decay, and speed convert exactly.

## Legacy fixture

`internal/sim/testdata/legacy/entities.toml` (rabbit/wolf) converts to
the new fields with full float precision so the historical tick values
reproduce exactly after rounding (e.g. rabbit `starve_ticks 600` is 300
seconds, so `starve_hours = 0.08333333333333333`; every field computed,
not eyeballed).
A data test asserts the loaded legacy config yields the exact historical
tick values (rabbit starve 600, wolf starve 1400, rabbit lifespan 8000,
decay 400, speeds 0.5/0.6, metabolisms 0.02/0.012). Rates may differ from
the historical floats only at float64 epsilon scale, which the long-run
regression's range assertions absorb; the 50k-tick test still passing is
part of this feature's acceptance.

## Validation

Existing checks keep working because they run on the converted internal
fields. Error messages that name fields use the new TOML names (e.g.
fauna requires positive `stomach_drain_hours`, `cells_per_second`,
`starve_hours`, `decay_hours`, `lifespan_days`, plus the existing
non-time requirements). `mine_hours` and `light_radius` non-negative as
before. New-field sanity: negative values for any unit field are a
validation error, reported with the new name.

## Docs

`.claude/skills/verify/SKILL.md` mentions `mine_ticks`-era pacing facts;
update its wording if it names the old fields. DESIGN.md needs no change
(it speaks in real time already).

## Out of scope

Duration strings ("1d12h"), engine-native time units, renaming
`tick_rate` or `autosave_minutes`, changing any behavior beyond the two
accepted roundings above.

## Testing

- data: new fields parse; conversion formulas produce the expected ticks
  and rates for the live files (mine 172800, starve 345600, decay 172800
  and 3600, lifespan 10022400... asserted from the real data dir); legacy
  fixture round-trips to exact historical tick values; negative unit
  values rejected with new-name messages.
- Full suite green including the 50k-tick long-run regression.
- Client build green (client is untouched; gate only).
