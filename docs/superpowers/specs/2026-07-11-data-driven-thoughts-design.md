# Data-Driven Thoughts Design

Date: 2026-07-11

## Purpose

Thought bubble copy and conditions currently live hardcoded in
`client/src/render.ts` (`composeThought`). Move them into the data files
so wording, ordering, and presence are content edits, per the data-first
constraint. Also stop showing bubbles permanently: a thought pops up for
10 seconds about once a minute per dwarf.

## Data shape

Thoughts become a per-type list in `data/entities.toml`; any fauna type
may carry one, and a type without a list shows no bubble:

```toml
thoughts = [
  { when = "starving",      text = "starving..." },
  { when = "hungry",        text = "hungry" },
  { when = "lonely",        text = "feeling lonely" },
  { when = "struck_gold",   text = "struck {gold} gold today!" },
  { when = "seen_recently", text = "seen {name} recently!" },
  { when = "always",        text = "content" },
]
```

List order is priority order; the first matching condition wins. The
current hardcoded strings move verbatim into the dwarf block, so nothing
changes visually.

## Condition vocabulary

Fixed in code, validated at load. Evaluated client-side against streamed
fields exactly as today:

- `starving`: fullness <= 0
- `hungry`: fullness < hunger threshold
- `lonely`: social_size > 0 and soc < social threshold
- `struck_gold`: gold in last 24h > 0
- `seen_recently`: seenTick within the trailing 24h of ticks
- `always`: matches unconditionally

Unknown `when` keys or empty `text` fail `data.Validate` with the type id
and the offending key in the message, so a typo cannot silently kill
bubbles.

## Templates

`{gold}` substitutes the 24h gold count; `{name}` substitutes the seen
entity's owner name, falling back to "a dwarf". Substitution is a plain
string replace on the client. Unknown placeholders pass through
untouched (content author sees their typo on screen).

## Plumbing

`data.EntityType` gains `Thoughts []Thought` with
`type Thought struct { When string toml/json "when"; Text string toml/json "text" }`.
The list rides the existing types map to the client; zero wire changes.
`composeThought(e)` becomes a loop over `sp.thoughts`: evaluate the named
condition, substitute placeholders, return the text; null when no rule
matches or the list is absent.

## Display cadence

A bubble is visible for `THOUGHT_SHOW_MS = 10000` out of every
`THOUGHT_PERIOD_MS = 60000` of wall-clock time, per entity, phase-offset
by entity id (`(now + e.id * 7919) % PERIOD < SHOW`) so dwarves pop at
different moments. Constants live in render.ts next to the bubble
drawing; they are presentation pacing, not game balance, so they stay in
code. Wall clock means a paused world can still pop thoughts; the click
popup remains the always-available detail view.

## Out of scope

Expression-language conditions, per-thought durations or cadence in
data, server-side thought evaluation, transient event-triggered pops.

## Testing

- data: thoughts list parses from entities.toml; unknown `when` rejected
  with type id in the error; empty `text` rejected; a type with no list
  validates fine.
- client: build clean; a quick headless check that a bubble appears
  within a 60s window and is absent at a phase known to be outside the
  10s show window is impractical to time-travel, so instead assert via a
  short Playwright run that the bubble band appears within ~65 seconds
  of watching a lonely dwarf OR stub the clock via page.evaluate reading
  the same modular arithmetic; a screenshot with a visible bubble plus
  one with none (same dwarf, different phase) is the acceptance
  evidence.
