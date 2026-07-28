Support AOE damage, so that "pickaxe" flying around a drawrf would hit mult blocks at the same time.


What if... we're calling them dwarfes but actually they aren't. <thinking emoji>
Just like usage of "Angels" in Evangelion is inverted.


# Garden the cave (control direction)

Committed pillar (see DESIGN.md): you shape the space, not the individual.
Indirect / attractor-style control — pheromone-trail lineage, From Dust /
Populous "tend the field." Direction below, not all built yet.

The attractor model:
- Default is WANDER, not idle. An undirected dwarf random-walks the excavated
  space. Walls bound the chaos: it can only roam where it (or someone) has dug,
  so early game they mill in the clearing, late game they roam the caverns —
  the cave you carve is the board.
- Sense radius lives on the FOOD item, not the dwarf. Each food is a beacon;
  a dwarf inside any beacon's radius pursues it. Show the radius as a thin
  ring so reach is plannable (tower-defense range indicator).
- Radius is the master knob and does double duty: it is both the CATCH RANGE
  (how far a beacon pulls a wanderer) and the DIG-COMMITMENT DEPTH (how far
  through rock a dwarf will tunnel to reach a beacon buried beyond a wall).
  So an expensive big-radius beacon buried deep = "commit to a long dig,"
  with no separate dig command. Price reach in gold.
- Greedy eat: a dwarf eats all food it reaches, even while full, then wanders
  on. Food never accumulates (kills the stockpile/larder problem); each item
  is a consumable command token. Chains are consumed as dwarves walk them.
- Control = laying food chains across open ground to lead a wanderer to a wall,
  then a beacon beyond the wall to make it dig through.

Decided — ownership: COLLECTIVE for now. Food is environmental; any dwarf
answers any beacon. The cave is a commons everyone terraforms together; think
"score by contribution (gold you caused)" over "your dwarf." Per-player
constraints may come later.

Decided — neglect: keep starvation, keep it a bit hardcore. Leaving the clock
at starve_hours = 48 (~2 days unfed). Fullness stays as a survival timer that
eating (even overeating) resets. Note the coupling: because a digging dwarf
runs at Fullness 0 the whole dig, the starve clock also bounds how deep a
single dig reaches before it must eat — relevant when tuning radius-as-depth.

Suggestions in this direction (backlog, not the current focus):
- Two food tiers: cheap small-radius CRUMBS for precise steering; expensive
  big-radius BEACONS to catch wanderers and commit deep digs.
- Repellents / anti-attractors: a thing dwarves avoid (fear radius), so you
  can route by shaping where they DON'T go, not only where they do.
- Durable vs consumable attractors: crumbs are consumed (ongoing gold sink,
  good for an idle game); a pricier durable beacon is set-and-forget. Offer both.
- Scent GRADIENT instead of a hard radius: dwarves follow increasing scent, so
  chains bend around obstacles without exact spacing. Softer, more forgiving;
  trades some legibility for robustness.
- Think in FLOWS: a wide recruiter beacon + a dig target = a stream of dwarves
  carving a highway. Design tuning around herds/throughput, not single units.
- The cave breathes: unused tunnels slowly mold/reclaim over time (mold already
  spreads into dark passable space), so the board isn't purely additive.
- Identity shift: if you tend a colony rather than pilot a dwarf, deemphasize
  per-player dwarf identity toward "the colony you shaped" (ties to the note
  above — maybe they aren't dwarves).

# Beyond gardens: gameplay research (2026-07-28)

Engine inventory finding: most wanted gameplay is ALREADY CODED and dormant
— zeroed out in data, not missing from the engine. Ranked by leverage:

1. DWARF REPRODUCTION — population as the real progression number.
   reproduceAndGuard is fully built (repro_threshold/chance/cost, mature,
   pop_cap, "born" events); every live fauna just sets repro_chance = 0.
   Flip it on for dwarves: well-fed dwarves breed -> gardens literally grow
   the colony -> more diggers -> more reach. Answers "I don't care about a
   bazillion coins": you care about MOUTHS. Starvation becomes real loss.
   Data-only. Pairs with gardens (they're the fuel). Decide: born dwarves
   are unowned colony-folk (fits the commons; players adopt/name later?).
2. BUYABLE LIGHT — the first permanent gold sink. Torches (burnout,
   light_radius) exist only in tests; live players cannot buy light at all
   (buyfood is flora-only, validation forbids costed non-flora). Small
   code: allow costed structures. Then: cheap burning torch (recurring
   sink) vs pricey eternal lantern. Feeds everything: lit faces to dig,
   mold-frontier war, garden protection, gold -> visible permanent stuff.
3. BURIED CACHES — discovery layer, data-only TODAY. Worldgen scatter can
   place flora on rock terrain (flora may sit buried in mineable rock);
   client draws beacon rings for all living food, so a buried cache shows
   a ring through the fog = a mystery marker that is NOT money. Strange
   mushrooms deep in the rock, big-radius, feast-sized.
4. A PREDATOR IN THE DEEP — stakes. Flee/hunt is diet-driven
   (Eats vs Produces); dwarves are unkillable only because they produce
   nothing. Give dwarf produces=[meat], add a wolf that eats meat, dwarf
   fear_radius > 0 -> dwarves flee, predators hunt. Only after population
   exists to threaten (needs 1). Data-only. Frustration risk: keep them
   deep/rare.
5. PER-PLAYER CONTRIBUTION — collective texture. No per-player score
   exists; per-dwarf GoldStrikes already tracks who caused what. Surface
   "gold you caused / blocks your dwarf broke" in recap + a small board.
6. SHELTER/HOME RHYTHM — shelterStep is fully coded, zero live users
   (no type sets shelters, home_range = 0). Dwarves with
   shelters=["mushroom"] would HOME at a garden: sortie-and-return rhythm
   for free. Cheap data experiment alongside gardens.

Smaller finds: structure lifespan/burnout live-unused (campfire eternal);
darkStep never fires for dwarves (they carry light); social system is
single-species (dwarf-only) but already makes them cluster at thresholds;
water is a permanent unmineable barrier (good geography, no interactions);
market is pure decor kept by migration; `desires`/`Aversion` fields are
dead code — delete or wire as food-preference variety bonus someday.

Suggested order: repro -> buyable light -> caches -> predator -> scoring.

# Garden plot (researched 2026-07-28, not yet built)

The engine already IS a garden engine — v1 is a pure data entry (flora with
cost > 0, regrow_days > 0), zero engine code. Verified by sim probe:

- A regrowing flora SURVIVES greedy eating: biteWorthy leaves the sub-bite
  stub, spent() never kills a regrower. Dwarf ate 42 meals in 800 ticks;
  plant lived.
- Dwarves GRAZE-CAMP: inside the radius they orbit and eat each unit as it
  regrows. A garden is an ANCHOR (home base / never-starve zone), not a
  pantry — effective feed rate = regrow rate, amount only matters after
  absences. Chains still pull dwarves out (nearest beacon wins, sticky digs
  hold); the garden recaptures them on return: base camp -> sortie rhythm.
- The shop sidebar auto-lists any flora with a cost: UI is free.
- Mold overgrows UNLIT garden tiles (spread ignores flora): a dark garden
  gets buried; the beacon still senses through, so lit-face dwarves dig it
  free. "Gardens need light" and "the cave takes back dark gardens" emerge
  with zero code.
- buyfood allows planting in mineable rock: a garden seeded INSIDE stone
  makes dwarves excavate a room around it ("seed a room").
- Rabbits eat the mushroom resource: if the garden produces "mushroom",
  rabbit raids on gardens come free whenever rabbits return.

Knobs (all TOML): sense_radius = anchor size (recommend small, 4-5, or one
garden pacifies a region); regrow_days = carrying capacity; amount/max =
feast size after an absence; cost = milestone price (~300-500 at 2/s chip
income — but couples to the unresolved mushroom-price/steering question).

Open decisions: resource identity (produce "mushroom" for future rabbit
interference vs a private "greens" — lean "mushroom"); level-gating the
shop entry (needs small code: unlock_level on the type + shop filter +
server check) — the first "level = new toy" unlock; no spawn cap exists,
cost is the only limiter (fine for the commons, revisit).

Risk to watch: graze-camping "retires" garden-adjacent dwarves; several
gardens could pacify the whole colony. The counterweight is steering cost
and chain design — same open question as mushroom pricing.

# The drive loop (why keep a tab open)

Diagnosis from the genre lessons (idle games, Noita, B&W, Vampire Survivors):
verbs without rewards. Three gaps, in leverage order:

1. Reward stream — income must be frequent, visible, causally legible.
   SHIPPED: chip gold (damage == gold, every damaged face floats its pay),
   golden-strike crits as the luck-scaled jackpot layer (see DESIGN.md Pace).
2. Goal ladder — always a visible "next purchase I can almost afford."
   SHIPPED (first rung): gold_vein blocks glint through the fog, chip 10x.
   Later rungs: richer/rarer glints deeper out, so the ladder is spatial:
   see glint -> chain mushrooms -> dig -> coins -> afford a longer chain ->
   deeper glint. Cave itself becomes the shop window.
3. Pushable systems — one verb (place food) needs multiple RESPONDERS, not
   more verbs: let rabbits eat bait chains (contested placement: fences,
   decoys, luring prey toward hungry wolves). Beacon+greedy rules are already
   species-agnostic; this is a data change.

The idle-game endgame is automation: hand-placed consumable mushrooms are the
"clicks"; a purchasable REGROWING garden plot is the "grandma" — the thing you
save for that keeps dwarves fed and digging while you're away. Also resolves
the hardcore 48h starve clock vs absence: a well-built garden is what earns a
safe return ("look how far they dug"), not a softer clock.
