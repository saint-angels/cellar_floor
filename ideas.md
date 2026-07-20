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

Open decision — ownership: food is environmental, so any dwarf answers any
beacon. Leaning collective: the cave is a commons everyone terraforms; score
by contribution (gold you caused), not by "your dwarf." Resolve before it ships.

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
