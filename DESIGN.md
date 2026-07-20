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

- **One persistent world, a shared commons.** The server runs continuously,
  autosaves, and survives restarts; anything that matters lives in world.json.
  All players shape one cave together — influence is collective, not per-player
  (per-player constraints may come later).
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
- **The player influences, never commands — you garden the cave.** Player
  tools shape the environment, not the individual creature; creatures respond
  to the space you make. You don't pilot a dwarf, you place attractors and let
  the cave you've shaped pull it. An undirected dwarf is never ordered — it
  wanders the dug-out space until something it senses draws it in. Movement is
  emergent; the player tends a colony, not a unit.
- **Food is the single lever.** Hunger is the only thing that moves a dwarf
  with intent: it pursues food it senses — on foot, or by tunnelling through
  rock toward food buried beyond a wall — and mines only as the act of
  reaching it. Players plant food (spending gold) to steer; a dwarf with
  nothing to reach does not mine. Gold falls out of the rock broken on the way.
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
