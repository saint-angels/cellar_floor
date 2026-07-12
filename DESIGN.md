# Design Constraints

The rules that shape Cellar Floor. Features should fit these; changing one
is a deliberate decision, not a side effect.

## Pace

- **Work takes awhile; needs are lively.** Mining and the economy run on
  hours to real days: a rock cell takes about a day to mine, and the payoff
  of the game is checking back later. Bodily needs (hunger, company) cycle
  in minutes so the colony visibly moves while you watch.
- **The canonical world runs at 1x wall-clock.** The 8x/64x buttons are dev
  and observation tools, not gameplay.

## World

- **One persistent world.** The server runs continuously, autosaves, and
  survives restarts; anything that matters lives in world.json.
- **Deterministic per seed.** The same seed produces the same world and the
  same history. Sim code never depends on map iteration order or wall-clock
  time.
- **The engine knows no entity types.** All creatures, resources, and balance
  live in data/*.toml; engine code implements generic behaviors (eating,
  mining) that data switches on. New content should be data first, code
  only for genuinely new behavior.

## Inhabitants

- **Creatures are autonomous.** Nobody takes orders; behavior emerges from
  needs and simple priorities (danger, food, work, rest).
- **The player influences, never commands.** Player tools shape the
  environment (torches) and the creatures respond to it. The opening
  states this: every rock face starts beyond the campfire's light, so
  the first torch is the first decision, and an undirected dwarf stays
  in the clearing, afraid of the dark.
- **The events feed is the narrator.** Anything notable that happens should
  emit a small human-readable event ("Dwarf struck gold").

## Presentation

- **One canvas, tiny tiles, dark palette.** The whole map is a single
  12px-tile canvas in a dark cellar palette; muted terrain, warmer accents
  for living things and gold. No DOM per entity.
- **The client is a viewer.** It renders state and sends only small intents
  (timescale, torch placement); all truth lives on the server.

## Code

- **Small stack, few dependencies.** Go stdlib plus gorilla/websocket on the
  server, TypeScript with zero runtime dependencies on the client. New
  dependencies need a strong reason.
- **Inspectable while running.** The debug API (/api/state, /api/entities)
  exists so live behavior can be verified without scraping pixels; keep it
  current as state grows.
