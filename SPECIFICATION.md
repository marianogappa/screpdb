# screpdb specification

> **Generated file — do not edit by hand.** Run `go generate ./...` to rebuild it,
> then commit. CI fails if it's stale or if any value isn't test-backed.

## Why this exists

screpdb makes a lot of derived claims — "this is a **9 Pool**", "your Spawning
Pool was 6s late", "a Zealot takes 25.2s". They all rest on **golden values**
baked into the code.

This document lets you audit them:

- Every value is read straight from the constants the app runs on (the doc is generated *from* them).
- Every value is checked by a test (`go test ./...`).

So the doc can't drift from the code, and the code can't be silently wrong.

Each section is a short intro plus a fixed-column table — readable by humans,
parseable by machines. Keys are sorted, so diffs stay small.

## Contents

- [Unit & building names](#unit--building-names)
- [Order → unit attribution](#order--unit-attribution)
- [Build times](#build-times)
- [Tech research](#tech-research)
- [Upgrades](#upgrades)
- [Worker & flying units](#worker--flying-units)
- [Unit geometry](#unit-geometry)
- [Building geometry](#building-geometry)
- [Higher-level action types](#higherlevel-action-types)
- [Replay enums](#replay-enums)
- [Build orders & expert timings](#build-orders--expert-timings)
- [Build-order rule deadlines](#buildorder-rule-deadlines)
- [Absence-marker game-length thresholds](#absencemarker-gamelength-thresholds)
- [Detection scalars & versioning](#detection-scalars--versioning)
- [Early-game unit economics](#earlygame-unit-economics)
- [Worker gather rates](#worker-gather-rates)
- [Tech tree: producers](#tech-tree-producers)
- [Tech tree: prerequisites](#tech-tree-prerequisites)
- [Featuring strip order](#featuring-strip-order)
- [Game-event featuring chips](#gameevent-featuring-chips)

## Unit & building names

The canonical names screpdb uses for every unit and building, grouped by race. Every name shown in the UI is one of these strings.

| Race | Category | Names |
| --- | --- | --- |
| Terran | Units | Battlecruiser, Dropship, Firebat, Ghost, Goliath, Marine, Medic, SCV, Science Vessel, Siege Tank (Tank Mode), Siege Tank Turret (Tank Mode), Valkyrie, Vulture, Wraith |
| Terran | Buildings | Academy, Armory, Barracks, Bunker, ComSat, Command Center, Control Tower, Covert Ops, Engineering Bay, Factory, Machine Shop, Missile Turret, Nuclear Silo, Physics Lab, Refinery, Science Facility, Starport, Supply Depot |
| Zerg | Units | Defiler, Devourer, Drone, Guardian, Hydralisk, Infested Terran, Lurker, Mutalisk, Overlord, Queen, Scourge, Ultralisk, Zergling |
| Zerg | Buildings | Creep Colony, Defiler Mound, Evolution Chamber, Extractor, Greater Spire, Hatchery, Hive, Hydralisk Den, Infested CC, Lair, Nydus Canal, Queens Nest, Spawning Pool, Spire, Spore Colony, Sunken Colony, Ultralisk Cavern |
| Protoss | Units | Arbiter, Archon, Carrier, Corsair, Dark Archon, Dark Templar, Dragoon, High Templar, Observer, Probe, Reaver, Scout, Shuttle, Zealot |
| Protoss | Buildings | Arbiter Tribunal, Assimilator, Citadel of Adun, Cybernetics Core, Fleet Beacon, Forge, Gateway, Nexus, Observatory, Photon Cannon, Pylon, Robotics Facility, Robotics Support Bay, Shield Battery, Stargate, Templar Archives |

## Order → unit attribution

Orders/actions that belong to exactly one unit type (e.g. `CastPsionicStorm` can only be a High Templar). screpdb uses this to attribute a command to the unit that issued it. Generic orders (Move, Attack, Hold) belong to many units and are omitted. `OrderName` and `ActionType` are separate namespaces.

| Key | Namespace | Issued by | Race |
| --- | --- | --- | --- |
| ArchonWarp | OrderName | High Templar | Protoss |
| Carrier | OrderName | Carrier | Protoss |
| CarrierAttack | OrderName | Carrier | Protoss |
| CarrierFight | OrderName | Carrier | Protoss |
| CarrierHoldPosition | OrderName | Carrier | Protoss |
| CarrierIgnore2 | OrderName | Carrier | Protoss |
| CarrierMoveToAttack | OrderName | Carrier | Protoss |
| CarrierStop | ActionType | Carrier | Protoss |
| CarrierStop | OrderName | Carrier | Protoss |
| CastConsume | OrderName | Defiler | Zerg |
| CastDarkSwarm | OrderName | Defiler | Zerg |
| CastDefensiveMatrix | OrderName | Science Vessel | Terran |
| CastDisruptionWeb | OrderName | Corsair | Protoss |
| CastEMPShockwave | OrderName | Science Vessel | Terran |
| CastEnsnare | OrderName | Queen | Zerg |
| CastFeedback | OrderName | Dark Archon | Protoss |
| CastHallucination | OrderName | High Templar | Protoss |
| CastInfestation | OrderName | Queen | Zerg |
| CastIrradiate | OrderName | Science Vessel | Terran |
| CastLockdown | OrderName | Ghost | Terran |
| CastMaelstrom | OrderName | Dark Archon | Protoss |
| CastMindControl | OrderName | Dark Archon | Protoss |
| CastNuclearStrike | OrderName | Ghost | Terran |
| CastOpticalFlare | OrderName | Medic | Terran |
| CastParasite | OrderName | Queen | Zerg |
| CastPlague | OrderName | Defiler | Zerg |
| CastPsionicStorm | OrderName | High Templar | Protoss |
| CastRecall | OrderName | Arbiter | Protoss |
| CastRestoration | OrderName | Medic | Terran |
| CastSpawnBroodlings | OrderName | Queen | Zerg |
| CastStasisField | OrderName | Science Vessel | Terran |
| CloakNearbyUnits | OrderName | Arbiter | Protoss |
| CreateProtossBuilding | OrderName | Probe | Protoss |
| DarkArchonMeld | OrderName | Dark Archon | Protoss |
| DroneBuild | OrderName | Drone | Zerg |
| DroneLand | OrderName | Drone | Zerg |
| DroneLiftOff | OrderName | Drone | Zerg |
| DroneStartBuild | OrderName | Drone | Zerg |
| FireYamatoGun | OrderName | Battlecruiser | Terran |
| GuardianAspect | OrderName | Mutalisk | Zerg |
| Hallucination2 | OrderName | High Templar | Protoss |
| HealMove | OrderName | Medic | Terran |
| InfestingCommandCenter | OrderName | Queen | Zerg |
| InitializeArbiter | OrderName | Arbiter | Protoss |
| InterceptorAttack | OrderName | Carrier | Protoss |
| InterceptorReturn | OrderName | Carrier | Protoss |
| Medic | OrderName | Medic | Terran |
| MedicHeal | OrderName | Medic | Terran |
| MedicHealToIdle | OrderName | Medic | Terran |
| MedicHoldPosition | OrderName | Medic | Terran |
| MoveToFireYamatoGun | OrderName | Battlecruiser | Terran |
| MoveToInfest | OrderName | Queen | Zerg |
| MoveToRepair | OrderName | SCV | Terran |
| NukeLaunch | OrderName | Ghost | Terran |
| NukePaint | OrderName | Ghost | Terran |
| NukeTrack | OrderName | Ghost | Terran |
| NukeTrain | OrderName | Ghost | Terran |
| NukeUnit | OrderName | Ghost | Terran |
| NukeWait | OrderName | Ghost | Terran |
| PlaceMine | OrderName | Vulture | Terran |
| PlaceProtossBuilding | OrderName | Probe | Protoss |
| QueenHoldPosition | OrderName | Queen | Zerg |
| Reaver | OrderName | Reaver | Protoss |
| ReaverAttack | OrderName | Reaver | Protoss |
| ReaverFight | OrderName | Reaver | Protoss |
| ReaverHoldPosition | OrderName | Reaver | Protoss |
| ReaverMoveToAttack | OrderName | Reaver | Protoss |
| ReaverStop | ActionType | Reaver | Protoss |
| ReaverStop | OrderName | Reaver | Protoss |
| Repair | OrderName | SCV | Terran |
| ScarabAttack | OrderName | Reaver | Protoss |
| Siege | ActionType | Siege Tank (Tank Mode) | Terran |
| Sieging | OrderName | Siege Tank (Tank Mode) | Terran |
| Unsiege | ActionType | Terran Siege Tank (Siege Mode) | Terran |
| Unsieging | OrderName | Terran Siege Tank (Siege Mode) | Terran |
| VultureMine | OrderName | Vulture | Terran |

## Build times

Build time in seconds at "Fastest" speed (every competitive replay uses it). Single source of truth for all timing logic — detection, expert timings, and the economy table all read these values. Zerglings are timed per pair (one egg makes two).

| Unit / Building | Build time (s) |
| --- | --- |
| Academy | 50 |
| Assimilator | 25 |
| Barracks | 50 |
| Bunker | 19 |
| Command Center | 75 |
| Creep Colony | 12 |
| Cybernetics Core | 38 |
| Drone | 12.6 |
| Engineering Bay | 38 |
| Evolution Chamber | 25 |
| Extractor | 25 |
| Factory | 50 |
| Forge | 25 |
| Gateway | 38 |
| Hatchery | 75 |
| Machine Shop | 25 |
| Marine | 15 |
| Missile Turret | 18.9 |
| Mutalisk | 25 |
| Nexus | 75 |
| Overlord | 25 |
| Photon Cannon | 31.5 |
| Probe | 12.6 |
| Pylon | 19 |
| Refinery | 25 |
| SCV | 12.6 |
| Spawning Pool | 50 |
| Spire | 75 |
| Starport | 44 |
| Sunken Colony | 12 |
| Supply Depot | 25 |
| Zealot | 25.2 |
| Zergling | 18 |

## Tech research

One-shot researches that unlock an ability or morph (Stim Packs, Lurker Aspect, Psionic Storm, …): where each is researched, its cost, and its duration at "Fastest" speed. screpdb uses the duration to time when the ability becomes available.

| Tech | Race | Researched at | Minerals | Gas | Duration (s) |
| --- | --- | --- | --- | --- | --- |
| Burrowing | Zerg | Hatchery | 100 | 100 | 80 |
| Cloaking Field | Terran | Control Tower | 150 | 150 | 63 |
| Consume | Zerg | Defiler Mound | 100 | 100 | 63 |
| Disruption Web | Protoss | Fleet Beacon | 200 | 200 | 50 |
| EMP Shockwave | Terran | Science Facility | 200 | 200 | 75.6 |
| Ensnare | Zerg | Queens Nest | 100 | 100 | 50 |
| Hallucination | Protoss | Templar Archives | 150 | 150 | 50.4 |
| Irradiate | Terran | Science Facility | 200 | 200 | 50.4 |
| Lockdown | Terran | Covert Ops | 200 | 200 | 63 |
| Lurker Aspect | Zerg | Hydralisk Den | 200 | 200 | 75.6 |
| Maelstrom | Protoss | Templar Archives | 100 | 100 | 63 |
| Mind Control | Protoss | Templar Archives | 200 | 200 | 75.6 |
| Optical Flare | Terran | Academy | 100 | 100 | 75.6 |
| Personnel Cloaking | Terran | Covert Ops | 100 | 100 | 50 |
| Plague | Zerg | Defiler Mound | 200 | 200 | 63 |
| Psionic Storm | Protoss | Templar Archives | 200 | 200 | 75.6 |
| Recall | Protoss | Arbiter Tribunal | 150 | 150 | 76 |
| Restoration | Terran | Academy | 100 | 100 | 50.4 |
| Spawn Broodlings | Zerg | Queens Nest | 100 | 100 | 50 |
| Spider Mines | Terran | Machine Shop | 100 | 100 | 50.4 |
| Stasis Field | Protoss | Arbiter Tribunal | 150 | 150 | 63 |
| Stim Packs | Terran | Academy | 100 | 100 | 50.4 |
| Tank Siege Mode | Terran | Machine Shop | 150 | 150 | 50.4 |
| Yamato Gun | Terran | Physics Lab | 100 | 100 | 75.6 |

## Upgrades

Passive upgrades: where each is researched, its max level (1 one-shot, 3 tiered) and each level's cost. Level cells read `minerals / gas / seconds`; unused levels show an em dash. Used to time when an upgrade completes.

| Upgrade | Race | Researched at | Max level | L1 (m/g/s) | L2 (m/g/s) | L3 (m/g/s) |
| --- | --- | --- | --- | --- | --- | --- |
| Adrenal Glands (Zergling Attack) | Zerg | Spawning Pool | 1 | 200 / 200 / 63 | — | — |
| Anabolic Synthesis (Ultralisk Speed) | Zerg | Ultralisk Cavern | 1 | 200 / 200 / 83.79 | — | — |
| Antennae (Overlord Sight) | Zerg | Lair | 1 | 150 / 150 / 83.79 | — | — |
| Apial Sensors (Scout Sight) | Protoss | Fleet Beacon | 1 | 100 / 100 / 105 | — | — |
| Apollo Reactor (Wraith Energy) | Terran | Control Tower | 1 | 200 / 200 / 105 | — | — |
| Argus Jewel (Corsair Energy) | Protoss | Fleet Beacon | 1 | 100 / 100 / 105 | — | — |
| Argus Talisman (Dark Archon Energy) | Protoss | Templar Archives | 1 | 150 / 150 / 105 | — | — |
| Caduceus Reactor (Medic Energy) | Terran | Academy | 1 | 150 / 150 / 105 | — | — |
| Carrier Capacity | Protoss | Fleet Beacon | 1 | 100 / 100 / 63 | — | — |
| Charon Boosters (Goliath Range) | Terran | Machine Shop | 1 | 100 / 100 / 84 | — | — |
| Chitinous Plating (Ultralisk Armor) | Zerg | Ultralisk Cavern | 1 | 150 / 150 / 83.79 | — | — |
| Colossus Reactor (Battle Cruiser Energy) | Terran | Physics Lab | 1 | 150 / 150 / 105 | — | — |
| Defiler Energy | Zerg | Defiler Mound | 1 | 150 / 150 / 104.58 | — | — |
| Gamete Meiosis (Queen Energy) | Zerg | Queens Nest | 1 | 150 / 150 / 104.58 | — | — |
| Gravitic Booster (Observer Speed) | Protoss | Observatory | 1 | 150 / 150 / 84 | — | — |
| Gravitic Drive (Shuttle Speed) | Protoss | Robotics Support Bay | 1 | 200 / 200 / 105 | — | — |
| Gravitic Thrusters (Scout Speed) | Protoss | Fleet Beacon | 1 | 200 / 200 / 105 | — | — |
| Grooved Spines (Hydralisk Range) | Zerg | Hydralisk Den | 1 | 150 / 150 / 63 | — | — |
| Ion Thrusters (Vulture Speed) | Terran | Machine Shop | 1 | 100 / 100 / 63 | — | — |
| Khaydarin Amulet (Templar Energy) | Protoss | Templar Archives | 1 | 150 / 150 / 105 | — | — |
| Khaydarin Core (Arbiter Energy) | Protoss | Arbiter Tribunal | 1 | 150 / 150 / 105 | — | — |
| Leg Enhancement (Zealot Speed) | Protoss | Citadel of Adun | 1 | 150 / 150 / 84 | — | — |
| Metabolic Boost (Zergling Speed) | Zerg | Spawning Pool | 1 | 100 / 100 / 63 | — | — |
| Moebius Reactor (Ghost Energy) | Terran | Covert Ops | 1 | 150 / 150 / 105 | — | — |
| Muscular Augments (Hydralisk Speed) | Zerg | Hydralisk Den | 1 | 150 / 150 / 63 | — | — |
| Ocular Implants (Ghost Sight) | Terran | Covert Ops | 1 | 100 / 100 / 105 | — | — |
| Pneumatized Carapace (Overlord Speed) | Zerg | Lair | 1 | 150 / 150 / 83.79 | — | — |
| Protoss Air Armor | Protoss | Cybernetics Core | 3 | 150 / 150 / 167.58 | 225 / 225 / 180.18 | 300 / 300 / 192.78 |
| Protoss Air Weapons | Protoss | Cybernetics Core | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Protoss Ground Armor | Protoss | Forge | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Protoss Ground Weapons | Protoss | Forge | 3 | 100 / 100 / 167.58 | 150 / 150 / 180.18 | 200 / 200 / 192.78 |
| Protoss Plasma Shields | Protoss | Forge | 3 | 200 / 200 / 167.58 | 300 / 300 / 180.18 | 400 / 400 / 192.78 |
| Reaver Capacity | Protoss | Robotics Support Bay | 1 | 200 / 200 / 105 | — | — |
| Scarab Damage | Protoss | Robotics Support Bay | 1 | 200 / 200 / 105 | — | — |
| Sensor Array (Observer Sight) | Protoss | Observatory | 1 | 150 / 150 / 84 | — | — |
| Singularity Charge (Dragoon Range) | Protoss | Cybernetics Core | 1 | 150 / 150 / 105 | — | — |
| Terran Infantry Armor | Terran | Engineering Bay | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Terran Infantry Weapons | Terran | Engineering Bay | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Terran Ship Plating | Terran | Armory | 3 | 150 / 150 / 167.58 | 225 / 225 / 180.18 | 300 / 300 / 192.78 |
| Terran Ship Weapons | Terran | Armory | 3 | 100 / 100 / 167.58 | 150 / 150 / 180.18 | 200 / 200 / 192.78 |
| Terran Vehicle Plating | Terran | Armory | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Terran Vehicle Weapons | Terran | Armory | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Titan Reactor (Science Vessel Energy) | Terran | Science Facility | 1 | 150 / 150 / 105 | — | — |
| U-238 Shells (Marine Range) | Terran | Academy | 1 | 150 / 150 / 63 | — | — |
| Ventral Sacs (Overlord Transport) | Zerg | Lair | 1 | 200 / 200 / 100.8 | — | — |
| Zerg Carapace | Zerg | Evolution Chamber | 3 | 150 / 150 / 167.58 | 225 / 225 / 180.18 | 300 / 300 / 192.78 |
| Zerg Flyer Attacks | Zerg | Spire | 3 | 100 / 100 / 167.58 | 175 / 175 / 180.18 | 250 / 250 / 192.78 |
| Zerg Flyer Carapace | Zerg | Spire | 3 | 150 / 150 / 167.58 | 225 / 225 / 180.18 | 300 / 300 / 192.78 |
| Zerg Melee Attacks | Zerg | Evolution Chamber | 3 | 100 / 100 / 167.58 | 150 / 150 / 180.18 | 200 / 200 / 192.78 |
| Zerg Missile Attacks | Zerg | Evolution Chamber | 3 | 100 / 100 / 167.58 | 150 / 150 / 180.18 | 200 / 200 / 192.78 |

## Worker & flying units

Two unit sets the detectors special-case, both excluded from drop-composition estimates: workers, and flying units (can't be carried in a transport).

| Category | Units |
| --- | --- |
| Workers | Drone, Probe, SCV |
| Flying (non-transportable) | Arbiter, Battlecruiser, Carrier, Corsair, Devourer, Dropship, Guardian, Mutalisk, Mutalisk Cocoon, Observer, Overlord, Queen, Science Vessel, Scourge, Scout, Shuttle, Valkyrie, Wraith |

## Unit geometry

Pixel dimensions screpdb uses to draw unit overlays on the map.

| Unit | Width (px) | Height (px) |
| --- | --- | --- |
| Arbiter | 44 | 44 |
| Archon | 32 | 32 |
| Battlecruiser | 75 | 59 |
| Carrier | 64 | 64 |
| Corsair | 36 | 32 |
| Dark Archon | 32 | 32 |
| Dark Templar | 24 | 26 |
| Defiler | 27 | 25 |
| Devourer | 44 | 44 |
| Dragoon | 32 | 32 |
| Drone | 23 | 23 |
| Dropship | 49 | 37 |
| Firebat | 23 | 22 |
| Ghost | 15 | 22 |
| Goliath | 32 | 32 |
| Guardian | 44 | 44 |
| High Templar | 24 | 24 |
| Hydralisk | 21 | 23 |
| Infested Terran | 17 | 20 |
| Lurker | 32 | 32 |
| Marine | 17 | 20 |
| Medic | 17 | 20 |
| Mutalisk | 44 | 44 |
| Observer | 32 | 32 |
| Overlord | 50 | 50 |
| Probe | 23 | 23 |
| Queen | 48 | 48 |
| Reaver | 32 | 32 |
| SCV | 23 | 23 |
| Science Vessel | 65 | 50 |
| Scourge | 24 | 24 |
| Scout | 36 | 32 |
| Shuttle | 40 | 32 |
| Siege Tank (Tank Mode) | 32 | 32 |
| Siege Tank Turret (Tank Mode) | 32 | 32 |
| Ultralisk | 38 | 32 |
| Valkyrie | 49 | 37 |
| Vulture | 32 | 32 |
| Wraith | 38 | 30 |
| Zealot | 23 | 19 |
| Zergling | 16 | 16 |

## Building geometry

Pixel dimensions for building overlays. `Box` is the placement footprint, `Real` is the visible sprite, and the gaps are the insets from box to sprite on each side.

| Building | Box W | Box H | Real W | Real H | Gap T | Gap L | Gap R | Gap B |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Academy | 96 | 64 | 85 | 57 | 0 | 8 | 3 | 7 |
| Arbiter Tribunal | 96 | 64 | 89 | 57 | 4 | 4 | 3 | 3 |
| Armory | 96 | 64 | 96 | 55 | 0 | 0 | 0 | 9 |
| Assimilator | 128 | 64 | 97 | 57 | 0 | 16 | 15 | 7 |
| Barracks | 128 | 96 | 105 | 73 | 8 | 16 | 7 | 15 |
| Bunker | 64 | 64 | 33 | 49 | 0 | 16 | 15 | 15 |
| Citadel of Adun | 96 | 64 | 65 | 49 | 8 | 24 | 7 | 7 |
| ComSat | 64 | 64 | 69 | 42 | 16 | -5 | 0 | 6 |
| Command Center | 128 | 96 | 117 | 83 | 7 | 6 | 5 | 6 |
| Control Tower | 64 | 64 | 76 | 47 | 8 | -15 | 3 | 9 |
| Covert Ops | 64 | 64 | 76 | 47 | 8 | -15 | 3 | 9 |
| Creep Colony | 64 | 64 | 48 | 48 | 8 | 8 | 8 | 8 |
| Cybernetics Core | 96 | 64 | 81 | 49 | 8 | 8 | 7 | 7 |
| Defiler Mound | 128 | 64 | 97 | 37 | 0 | 16 | 15 | 27 |
| Engineering Bay | 128 | 96 | 97 | 61 | 16 | 16 | 15 | 19 |
| Evolution Chamber | 96 | 64 | 77 | 53 | 0 | 4 | 15 | 11 |
| Extractor | 128 | 64 | 128 | 64 | 0 | 0 | 0 | 0 |
| Factory | 128 | 96 | 113 | 81 | 8 | 8 | 7 | 7 |
| Fleet Beacon | 96 | 64 | 88 | 57 | 0 | 8 | 0 | 7 |
| Forge | 96 | 64 | 73 | 45 | 8 | 12 | 11 | 11 |
| Gateway | 128 | 96 | 97 | 73 | 16 | 16 | 15 | 7 |
| Greater Spire | 64 | 64 | 57 | 57 | 0 | 4 | 3 | 7 |
| Hatchery | 128 | 96 | 99 | 65 | 16 | 15 | 14 | 15 |
| Hive | 128 | 96 | 99 | 65 | 16 | 15 | 14 | 15 |
| Hydralisk Den | 96 | 64 | 81 | 57 | 0 | 8 | 7 | 7 |
| Infested CC | 128 | 96 | 117 | 83 | 7 | 6 | 5 | 6 |
| Lair | 128 | 96 | 99 | 65 | 16 | 15 | 14 | 15 |
| Machine Shop | 64 | 64 | 71 | 49 | 8 | -7 | 0 | 7 |
| Missile Turret | 64 | 64 | 33 | 49 | 0 | 16 | 15 | 15 |
| Nexus | 128 | 96 | 113 | 79 | 9 | 8 | 7 | 8 |
| Nuclear Silo | 64 | 64 | 69 | 42 | 16 | -5 | 0 | 6 |
| Nydus Canal | 64 | 64 | 64 | 64 | 0 | 0 | 0 | 0 |
| Observatory | 96 | 64 | 89 | 45 | 16 | 4 | 3 | 3 |
| Photon Cannon | 64 | 64 | 41 | 33 | 16 | 12 | 11 | 15 |
| Physics Lab | 64 | 64 | 76 | 47 | 8 | -15 | 3 | 9 |
| Pylon | 64 | 64 | 33 | 33 | 20 | 16 | 15 | 11 |
| Queens Nest | 96 | 64 | 71 | 57 | 4 | 10 | 15 | 3 |
| Refinery | 128 | 64 | 113 | 64 | 0 | 8 | 7 | 0 |
| Robotics Facility | 96 | 64 | 77 | 37 | 16 | 12 | 7 | 11 |
| Robotics Support Bay | 96 | 64 | 65 | 53 | 0 | 16 | 15 | 11 |
| Science Facility | 128 | 96 | 97 | 77 | 10 | 16 | 15 | 9 |
| Shield Battery | 96 | 64 | 65 | 33 | 16 | 16 | 15 | 15 |
| Spawning Pool | 96 | 64 | 77 | 47 | 4 | 12 | 7 | 13 |
| Spire | 64 | 64 | 57 | 57 | 0 | 4 | 3 | 7 |
| Spore Colony | 64 | 64 | 48 | 48 | 8 | 8 | 8 | 8 |
| Stargate | 128 | 96 | 97 | 73 | 8 | 16 | 15 | 15 |
| Starport | 128 | 96 | 97 | 79 | 8 | 16 | 15 | 9 |
| Sunken Colony | 64 | 64 | 48 | 48 | 8 | 8 | 8 | 8 |
| Supply Depot | 96 | 64 | 77 | 49 | 10 | 10 | 9 | 5 |
| Templar Archives | 96 | 64 | 65 | 49 | 8 | 16 | 15 | 7 |
| Ultralisk Cavern | 96 | 64 | 73 | 64 | 0 | 8 | 15 | 0 |

## Higher-level action types

screpdb collapses many raw commands into a few higher-level action types (train, build, morph). These are the exact string values it stores and matches on.

| Action type | Value |
| --- | --- |
| Train | Train |
| Build | Build |
| Unit Morph | Unit Morph |

## Replay enums

Fixed value sets screpdb reads from a parsed replay. Races are screpdb's; speeds and colors come from the screp parser (`github.com/icza/screp/rep/repcore`); matchups are derived from race initials.

| Enum | Values | Notes |
| --- | --- | --- |
| Races | Terran, Protoss, Zerg | Random resolves to the actual race at game start. |
| Game speeds | Slowest, Slower, Slow, Normal, Fast, Faster, Fastest | Competitive replays are always played on Fastest. |
| Player colors | Red, Blue, Teal, Purple, Orange, Brown, White, Yellow, Green, Pale Yellow, Tan, Aqua, Pale Green, Blueish Grey, Pale Yellow2, Cyan, Pink, Olive, Lime, Navy, Dark Aqua, Magenta, Grey, Black | The player's slot color as parsed from the replay. |
| 1v1 matchups | PvP, PvT, PvZ, TvT, TvZ, ZvZ | Race initials sorted alphabetically; team/FFA games use the same scheme (e.g. PPvZZ). |
| Start location (clock) | 1–12 | The o'clock position of the player's start location on the map. |

## Build orders & expert timings

The openings screpdb recognizes and each milestone's "progamer ideal" timing. The Build Orders tab marks a milestone on-time if it lands within the tolerance window around its target. Targets are seconds from game start ("Fastest" speed); tolerance is the accepted early/late deviation.

| Build order | Race | Milestone | Target (s) | Tolerance (s) |
| --- | --- | --- | --- | --- |
| 1 Gate (no expa) | Protoss | Pylon | 48 | ±6 |
| 1 Gate (no expa) | Protoss | Gateway | 88 | ±15 |
| 1 Gate Core | Protoss | Pylon | 48 | ±4 |
| 1 Gate Core | Protoss | Gateway | 86 | ±6 |
| 1 Gate Core | Protoss | Assimilator | 116 | ±10 |
| 1 Gate Core | Protoss | Cybernetics Core | 138 | ±10 |
| 1 Gate Reaver | Protoss | Robotics Facility | 252 | −60 / +70 |
| 1 Gate Reaver | Protoss | First Reaver | 408 | −90 / +120 |
| 1-1-1 | Terran | Supply Depot | 57 | −10 / +24 |
| 1-1-1 | Terran | Barracks | 85 | −28 / +20 |
| 1-1-1 | Terran | Refinery | 98 | −15 / +70 |
| 1-1-1 | Terran | Factory | 160 | −15 / +70 |
| 1-1-1 | Terran | Starport | 226 | −40 / +80 |
| 1-1-1 | Terran | First Wraith | 271 | −40 / +90 |
| 1-1-1 into Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 1-1-1 into Mech | Terran | Barracks | 85 | −26 / +18 |
| 1-1-1 into Mech | Terran | Refinery | 99 | −12 / +70 |
| 1-1-1 into Mech | Terran | Factory | 153 | −12 / +80 |
| 1-1-1 into Mech | Terran | Starport | 254 | −50 / +70 |
| 1-1-1 into Mech | Terran | First Siege Tank | 312 | −80 / +120 |
| 1-Base Bio | Terran | Supply Depot | 56 | −10 / +24 |
| 1-Base Bio | Terran | Barracks | 84 | −28 / +20 |
| 1-Base Bio | Terran | Refinery | 185 | −80 / +50 |
| 1-Base Bio | Terran | Academy | 230 | −45 / +90 |
| 10 Hatch | Zerg | Hatchery | 80 | ±5 |
| 10 Hatch | Zerg | Spawning Pool | 110 | −3 / +10 |
| 10 Pool | Zerg | Spawning Pool | 92 | ±5 |
| 10 Pool | Zerg | First Zerglings | 142 | ±4 |
| 11 Hatch | Zerg | Hatchery | 94 | ±5 |
| 11 Hatch | Zerg | Spawning Pool | 116 | −3 / +10 |
| 11 Pool | Zerg | Spawning Pool | 98 | ±5 |
| 11 Pool | Zerg | First Zerglings | 148 | ±4 |
| 12 Hatch | Zerg | Hatchery | 98 | ±5 |
| 12 Hatch | Zerg | Spawning Pool | 116 | −3 / +10 |
| 12 Pool | Zerg | Spawning Pool | 104 | ±5 |
| 12 Pool | Zerg | First Zerglings | 154 | ±4 |
| 13 Hatch | Zerg | Hatchery | 104 | ±5 |
| 13 Hatch | Zerg | Spawning Pool | 122 | −3 / +10 |
| 2 Fact before Expa | Terran | 1st Factory | 147 | −25 / +40 |
| 2 Fact before Expa | Terran | 2nd Factory | 177 | −40 / +60 |
| 2 Gate | Protoss | Pylon | 48 | ±4 |
| 2 Gate | Protoss | 1st Gateway | 70 | ±6 |
| 2 Gate | Protoss | 2nd Gateway | 86 | ±10 |
| 2 Gate | Protoss | First Zealot | 108 | ±3 |
| 2 Hatch Hydra | Zerg | Hydralisk Den | 214 | −25 / +90 |
| 2 Hatch Hydra | Zerg | First Hydralisks | 250 | −40 / +120 |
| 2 Hatch Muta | Zerg | Spire | 249 | −35 / +70 |
| 2 Hatch Muta | Zerg | First Mutalisks | 327 | −40 / +90 |
| 2 Port Wraith | Terran | 1st Starport | 205 | −25 / +70 |
| 2 Port Wraith | Terran | 2nd Starport | 212 | −25 / +70 |
| 2 Port Wraith | Terran | First Wraith | 253 | −40 / +90 |
| 2-Base Bio | Terran | Supply Depot | 56 | −10 / +24 |
| 2-Base Bio | Terran | Barracks | 84 | −28 / +20 |
| 2-Base Bio | Terran | Refinery | 185 | −80 / +50 |
| 2-Base Bio | Terran | Academy | 230 | −45 / +90 |
| 2-Base Bio | Terran | Command Center | 300 | −80 / +60 |
| 2-Fac Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 2-Fac Mech | Terran | Barracks | 85 | −26 / +18 |
| 2-Fac Mech | Terran | Refinery | 100 | −12 / +70 |
| 2-Fac Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 2-Fac Mech | Terran | 2nd Factory | 275 | −90 / +130 |
| 2-Fac Mech | Terran | First Siege Tank | 290 | −60 / +120 |
| 2-Fac Tankless Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 2-Fac Tankless Mech | Terran | Barracks | 85 | −26 / +18 |
| 2-Fac Tankless Mech | Terran | Refinery | 100 | −12 / +70 |
| 2-Fac Tankless Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 2-Fac Tankless Mech | Terran | First Vulture | 205 | −20 / +50 |
| 2-Fac Tankless Mech | Terran | 2nd Factory | 243 | −90 / +130 |
| 3 Hatch Lurker | Zerg | Hydralisk Den | 270 | −50 / +60 |
| 3 Hatch Lurker | Zerg | First Lurkers | 417 | −80 / +120 |
| 3-Fac Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 3-Fac Mech | Terran | Barracks | 85 | −26 / +18 |
| 3-Fac Mech | Terran | Refinery | 100 | −12 / +70 |
| 3-Fac Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 3-Fac Mech | Terran | First Siege Tank | 290 | −60 / +120 |
| 3-Fac Mech | Terran | 3rd Factory | 458 | −90 / +130 |
| 3-Fac Tankless Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 3-Fac Tankless Mech | Terran | Barracks | 85 | −26 / +18 |
| 3-Fac Tankless Mech | Terran | Refinery | 100 | −12 / +70 |
| 3-Fac Tankless Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 3-Fac Tankless Mech | Terran | First Vulture | 205 | −20 / +50 |
| 3-Fac Tankless Mech | Terran | 3rd Factory | 349 | −90 / +130 |
| 4 Hatch | Zerg | Hatchery | 40 | ±6 |
| 4 Hatch | Zerg | Spawning Pool | 70 | −6 / +12 |
| 4 Pool | Zerg | Spawning Pool | 33 | ±4 |
| 4 Pool | Zerg | First Zerglings | 83 | ±3 |
| 4-Fac Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 4-Fac Mech | Terran | Barracks | 85 | −26 / +18 |
| 4-Fac Mech | Terran | Refinery | 100 | −12 / +70 |
| 4-Fac Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 4-Fac Mech | Terran | First Siege Tank | 290 | −60 / +120 |
| 4-Fac Mech | Terran | 4th Factory | 480 | −90 / +130 |
| 4-Fac Tankless Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 4-Fac Tankless Mech | Terran | Barracks | 85 | −26 / +18 |
| 4-Fac Tankless Mech | Terran | Refinery | 100 | −12 / +70 |
| 4-Fac Tankless Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 4-Fac Tankless Mech | Terran | First Vulture | 205 | −20 / +50 |
| 4-Fac Tankless Mech | Terran | 4th Factory | 422 | −90 / +130 |
| 5 Hatch | Zerg | Hatchery | 50 | ±6 |
| 5 Hatch | Zerg | Spawning Pool | 80 | −6 / +12 |
| 5 Pool | Zerg | Spawning Pool | 45 | ±5 |
| 5 Pool | Zerg | First Zerglings | 95 | ±4 |
| 5-Fac Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 5-Fac Mech | Terran | Barracks | 85 | −26 / +18 |
| 5-Fac Mech | Terran | Refinery | 100 | −12 / +70 |
| 5-Fac Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 5-Fac Mech | Terran | First Siege Tank | 290 | −60 / +120 |
| 5-Fac Mech | Terran | 5th Factory | 501 | −90 / +130 |
| 5-Fac Tankless Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 5-Fac Tankless Mech | Terran | Barracks | 85 | −26 / +18 |
| 5-Fac Tankless Mech | Terran | Refinery | 100 | −12 / +70 |
| 5-Fac Tankless Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 5-Fac Tankless Mech | Terran | First Vulture | 205 | −20 / +50 |
| 5-Fac Tankless Mech | Terran | 5th Factory | 471 | −90 / +130 |
| 6 Hatch | Zerg | Hatchery | 58 | ±6 |
| 6 Hatch | Zerg | Spawning Pool | 88 | −6 / +12 |
| 6 Pool | Zerg | Spawning Pool | 52 | ±5 |
| 6 Pool | Zerg | First Zerglings | 102 | ±4 |
| 6+ Fac Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 6+ Fac Mech | Terran | Barracks | 85 | −26 / +18 |
| 6+ Fac Mech | Terran | Refinery | 100 | −12 / +70 |
| 6+ Fac Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 6+ Fac Mech | Terran | First Siege Tank | 290 | −60 / +120 |
| 6+ Fac Mech | Terran | 6th Factory | 528 | −90 / +130 |
| 6+ Fac Tankless Mech | Terran | Supply Depot | 55 | −10 / +24 |
| 6+ Fac Tankless Mech | Terran | Barracks | 85 | −26 / +18 |
| 6+ Fac Tankless Mech | Terran | Refinery | 100 | −12 / +70 |
| 6+ Fac Tankless Mech | Terran | 1st Factory | 152 | −12 / +80 |
| 6+ Fac Tankless Mech | Terran | First Vulture | 205 | −20 / +50 |
| 6+ Fac Tankless Mech | Terran | 6th Factory | 560 | −90 / +130 |
| 7 Hatch | Zerg | Hatchery | 66 | ±6 |
| 7 Hatch | Zerg | Spawning Pool | 96 | −6 / +12 |
| 7 Pool | Zerg | Spawning Pool | 60 | ±5 |
| 7 Pool | Zerg | First Zerglings | 110 | ±4 |
| 8 Hatch | Zerg | Hatchery | 70 | ±6 |
| 8 Hatch | Zerg | Spawning Pool | 100 | −6 / +12 |
| 8 Pool | Zerg | Spawning Pool | 67 | ±5 |
| 8 Pool | Zerg | First Zerglings | 117 | ±4 |
| 9 Hatch | Zerg | Hatchery | 73 | ±4 |
| 9 Hatch | Zerg | Spawning Pool | 103 | −6 / +10 |
| 9 Overpool | Zerg | Spawning Pool | 80 | ±5 |
| 9 Overpool | Zerg | First Zerglings | 130 | ±4 |
| 9 Pool | Zerg | Spawning Pool | 73 | ±4 |
| 9 Pool | Zerg | First Zerglings | 123 | ±3 |
| 9 Pool into Hatchery | Zerg | Spawning Pool | 73 | ±4 |
| 9 Pool into Hatchery | Zerg | Hatchery | 118 | ±5 |
| 9 Pool into Hatchery | Zerg | First Zerglings | 123 | ±3 |
| BBS | Terran | 1st Barracks | 60 | ±8 |
| BBS | Terran | 2nd Barracks | 80 | ±8 |
| BBS | Terran | Supply Depot | 100 | ±10 |
| Bunker Rush | Terran | Barracks | 60 | ±10 |
| Bunker Rush | Terran | Bunker | 130 | ±20 |
| CC First | Terran | Supply Depot | 62 | ±8 |
| CC First | Terran | Command Center | 145 | ±20 |
| CC First | Terran | Barracks | 165 | ±20 |
| Factory Expand | Terran | Factory | 150 | −20 / +60 |
| Factory Expand | Terran | Command Center | 229 | −30 / +80 |
| Forge Cannon (no expa) | Protoss | Forge | 90 | ±20 |
| Forge Cannon (no expa) | Protoss | Photon Cannon | 130 | ±30 |
| Forge Cannon Gate before expa | Protoss | Forge | 96 | −30 / +60 |
| Forge Cannon Gate before expa | Protoss | Photon Cannon | 126 | −30 / +60 |
| Forge Cannon Gate before expa | Protoss | Gateway | 144 | −30 / +80 |
| Forge Expand | Protoss | Pylon | 48 | ±4 |
| Forge Expand | Protoss | Forge | 86 | ±8 |
| Forge Expand | Protoss | Photon Cannon | 130 | ±20 |
| Forge Expand | Protoss | Nexus | 152 | ±15 |
| Forge Gate Cannon before expa | Protoss | Forge | 96 | −30 / +60 |
| Forge Gate Cannon before expa | Protoss | Gateway | 130 | −30 / +70 |
| Forge Gate Cannon before expa | Protoss | Photon Cannon | 160 | −30 / +80 |
| Gate Expand | Protoss | Pylon | 48 | ±4 |
| Gate Expand | Protoss | Gateway | 88 | ±10 |
| Gate Expand | Protoss | Nexus | 165 | ±15 |
| Gate Forge Cannon before expa | Protoss | Gateway | 70 | −20 / +40 |
| Gate Forge Cannon before expa | Protoss | Forge | 120 | −30 / +60 |
| Gate Forge Cannon before expa | Protoss | Photon Cannon | 155 | −30 / +80 |
| Goliath | Terran | Supply Depot | 55 | −10 / +24 |
| Goliath | Terran | Barracks | 86 | −28 / +18 |
| Goliath | Terran | Refinery | 102 | −12 / +70 |
| Goliath | Terran | Factory | 161 | −15 / +120 |
| Goliath | Terran | Armory | 242 | −40 / +100 |
| Goliath | Terran | First Goliath | 339 | −75 / +70 |
| Nexus First | Protoss | Pylon | 48 | ±4 |
| Nexus First | Protoss | Nexus | 145 | ±20 |
| Nexus First | Protoss | Gateway | 175 | ±20 |

## Build-order rule deadlines

Each opener's detector commits its decision once the replay passes this second — the last moment the rule could still flip. "End of replay" means it's decided only when the game ends.

| Build order | Race | Rule deadline (s) |
| --- | --- | --- |
| 1 Gate (no expa) | Protoss | 320 |
| 1 Gate Core | Protoss | 180 |
| 1 Gate Reaver | Protoss | 600 |
| 1-1-1 | Terran | 600 |
| 1-1-1 into Mech | Terran | 600 |
| 1-Base Bio | Terran | 600 |
| 10 Hatch | Zerg | 180 |
| 10 Pool | Zerg | 180 |
| 11 Hatch | Zerg | 180 |
| 11 Pool | Zerg | 180 |
| 12 Hatch | Zerg | 180 |
| 12 Pool | Zerg | 180 |
| 13 Hatch | Zerg | 180 |
| 2 Fact before Expa | Terran | 360 |
| 2 Gate | Protoss | 180 |
| 2 Hatch Hydra | Zerg | 600 |
| 2 Hatch Muta | Zerg | 600 |
| 2 Port Wraith | Terran | 600 |
| 2-Base Bio | Terran | 600 |
| 2-Fac Mech | Terran | 600 |
| 2-Fac Tankless Mech | Terran | 600 |
| 3 Hatch Lurker | Zerg | 600 |
| 3-Fac Mech | Terran | 600 |
| 3-Fac Tankless Mech | Terran | 600 |
| 4 Hatch | Zerg | 180 |
| 4 Pool | Zerg | 60 |
| 4-Fac Mech | Terran | 600 |
| 4-Fac Tankless Mech | Terran | 600 |
| 5 Hatch | Zerg | 180 |
| 5 Pool | Zerg | 180 |
| 5-Fac Mech | Terran | 600 |
| 5-Fac Tankless Mech | Terran | 600 |
| 6 Hatch | Zerg | 180 |
| 6 Pool | Zerg | 180 |
| 6+ Fac Mech | Terran | 600 |
| 6+ Fac Tankless Mech | Terran | 600 |
| 7 Hatch | Zerg | 180 |
| 7 Pool | Zerg | 180 |
| 8 Hatch | Zerg | 180 |
| 8 Pool | Zerg | 180 |
| 9 Hatch | Zerg | 150 |
| 9 Overpool | Zerg | 180 |
| 9 Pool | Zerg | 180 |
| 9 Pool into Hatchery | Zerg | 180 |
| BBS | Terran | 120 |
| Bunker Rush | Terran | end of replay |
| CC First | Terran | 200 |
| Factory Expand | Terran | 360 |
| Forge Cannon (no expa) | Protoss | 320 |
| Forge Cannon Gate before expa | Protoss | 320 |
| Forge Expand | Protoss | 260 |
| Forge Gate Cannon before expa | Protoss | 320 |
| Gate Expand | Protoss | 220 |
| Gate Forge Cannon before expa | Protoss | 320 |
| Gateway (Other) | Protoss | 320 |
| Goliath | Terran | 600 |
| Nexus First | Protoss | 200 |
| Pool/Hatch (Other) | Zerg | 240 |
| Terran (Other) | Terran | 600 |

## Absence-marker game-length thresholds

"Never X" markers (e.g. *Never upgraded*) only fire on games long enough for the absence to mean something. The minimum length is matchup-specific — the 5th-percentile first-occurrence time across a large progamer 1v1 corpus. Outer race is the player's, inner is the opponent's.

| Marker | Own race | Opp race | Min game length (s) |
| --- | --- | --- | --- |
| Never researched | Protoss | Protoss | 332 |
| Never researched | Protoss | Terran | 496 |
| Never researched | Protoss | Zerg | 396 |
| Never researched | Terran | Protoss | 238 |
| Never researched | Terran | Terran | 245 |
| Never researched | Terran | Zerg | 252 |
| Never researched | Zerg | Protoss | 339 |
| Never researched | Zerg | Terran | 243 |
| Never upgraded | Protoss | Protoss | 169 |
| Never upgraded | Protoss | Terran | 163 |
| Never upgraded | Protoss | Zerg | 237 |
| Never upgraded | Terran | Protoss | 293 |
| Never upgraded | Terran | Terran | 264 |
| Never upgraded | Terran | Zerg | 233 |
| Never upgraded | Zerg | Protoss | 128 |
| Never upgraded | Zerg | Terran | 175 |
| Never upgraded | Zerg | Zerg | 125 |

## Detection scalars & versioning

Standalone constants the detectors depend on — dedup windows, muta/turret burst thresholds, cliff-drop corner boxes, the viewport window, and the algorithm version (bump it to force re-detection).

| Constant | Value | Meaning |
| --- | --- | --- |
| Algorithm version | 49 | Detection algorithm revision; incremented to trigger re-detection. |
| Build dedup gap (s) | 3 | Repeat Build orders of the same building at the same tile, closer than this, are one event (double-tap / misclick); different-tile placements are kept. |
| Build dedup max second (s) | 240 | Past this second, dedup stops and every Build is observed as-is (a tile can be legitimately rebuilt on later). |
| Mutalisk burst window (s) | 30 | Window within which the Mutalisk morphs must cluster. |
| Mutalisk burst min count | 3 | Minimum Mutalisks in the window to count as a burst. |
| Turret burst window (s) | 60 | Window within which the Missile Turrets must cluster. |
| Turret burst min count | 3 | Minimum Missile Turrets in the window to count as a burst. |
| Cliff-drop corner width (px) | 150 | Width of the corner box a drop must land in to count as a cliff drop. |
| Cliff-drop corner height (px) | 150 | Height of that corner box. |
| Viewport window start (s) | 420 | Second from which viewport-multitasking is measured. |
| Viewport width (px) | 704 | Width of the screen viewport in map pixels. |
| Viewport height (px) | 512 | Height of the screen viewport in map pixels. |

## Early-game unit economics

What producing each early-game unit/building costs and does to supply. Supply Δ is the cap increase from supply structures (Pylon/Depot/Overlord = +8); supply cost is what a unit consumes. Build times match the Build times section.

| Subject | Minerals | Gas | Build time (s) | Supply Δ | Supply cost |
| --- | --- | --- | --- | --- | --- |
| Academy | 150 | 0 | 50 | 0 | 0 |
| Assimilator | 100 | 0 | 25 | 0 | 0 |
| Barracks | 150 | 0 | 50 | 0 | 0 |
| Bunker | 100 | 0 | 19 | 0 | 0 |
| Command Center | 400 | 0 | 75 | 0 | 0 |
| Creep Colony | 75 | 0 | 12 | 0 | 0 |
| Cybernetics Core | 200 | 0 | 38 | 0 | 0 |
| Drone | 50 | 0 | 12.6 | 0 | 1 |
| Engineering Bay | 125 | 0 | 38 | 0 | 0 |
| Evolution Chamber | 75 | 0 | 25 | 0 | 0 |
| Extractor | 50 | 0 | 25 | 0 | 0 |
| Factory | 200 | 0 | 50 | 0 | 0 |
| Forge | 150 | 0 | 25 | 0 | 0 |
| Gateway | 150 | 0 | 38 | 0 | 0 |
| Hatchery | 300 | 0 | 75 | 0 | 0 |
| Machine Shop | 50 | 0 | 25 | 0 | 0 |
| Marine | 50 | 0 | 15 | 0 | 1 |
| Nexus | 400 | 0 | 75 | 0 | 0 |
| Overlord | 100 | 0 | 25 | 8 | 0 |
| Photon Cannon | 150 | 0 | 31.5 | 0 | 0 |
| Probe | 50 | 0 | 12.6 | 0 | 1 |
| Pylon | 100 | 0 | 19 | 8 | 0 |
| Refinery | 100 | 0 | 25 | 0 | 0 |
| SCV | 50 | 0 | 12.6 | 0 | 1 |
| Spawning Pool | 200 | 0 | 50 | 0 | 0 |
| Starport | 150 | 0 | 44 | 0 | 0 |
| Sunken Colony | 50 | 0 | 12 | 0 | 0 |
| Supply Depot | 100 | 0 | 25 | 8 | 0 |
| Zealot | 100 | 0 | 25.2 | 0 | 2 |
| Zergling | 50 | 0 | 18 | 0 | 2 |

## Worker gather rates

Mineral income per worker per minute at a near base, used in early-game economy math.

| Worker | Minerals / min |
| --- | --- |
| Drone | 67.1 |
| Probe | 68.1 |
| SCV | 65 |

## Tech tree: producers

Which building produces each early-game unit. The engine won't run a Train order without its producer, so a surviving Train is strong evidence the producer existed — the spam filter relies on this.

| Unit | Produced by |
| --- | --- |
| Drone | Hatchery |
| Marine | Barracks |
| Overlord | Hatchery |
| Probe | Nexus |
| SCV | Command Center |
| Zealot | Gateway |
| Zergling | Spawning Pool |

## Tech tree: prerequisites

Buildings that must already exist before another can be placed (beyond the producer) — e.g. a Photon Cannon needs a Pylon and a Forge.

| Building | Requires |
| --- | --- |
| Academy | Barracks |
| Assimilator | Nexus |
| Bunker | Barracks |
| Citadel of Adun | Pylon, Cybernetics Core |
| Cybernetics Core | Pylon, Gateway |
| Factory | Barracks |
| Fleet Beacon | Pylon, Stargate |
| Forge | Pylon |
| Gateway | Pylon |
| Machine Shop | Factory |
| Observatory | Pylon, Robotics Facility |
| Photon Cannon | Pylon, Forge |
| Robotics Facility | Pylon, Cybernetics Core |
| Robotics Support Bay | Pylon, Robotics Facility |
| Stargate | Pylon, Cybernetics Core |
| Starport | Factory |
| Sunken Colony | Creep Colony, Spawning Pool |
| Templar Archives | Pylon, Citadel of Adun |

## Featuring strip order

The fixed left-to-right order of chips in the games-list "Featuring" strip — a mix of marker keys and game-event keys. Every key resolves to a marker or a game-event feature (next section), enforced by a test.

| # | Feature key |
| --- | --- |
| 1 | cannon_rush |
| 2 | bunker_rush |
| 3 | zergling_rush |
| 4 | proxy_gate |
| 5 | proxy_rax |
| 6 | proxy_factory |
| 7 | proxy_starport |
| 8 | manner_pylon |
| 9 | drop |
| 10 | mind_control |
| 11 | threw_nukes |
| 12 | made_recalls |
| 13 | made_maelstrom |
| 14 | offensive_nydus |
| 15 | bo_4_pool |
| 16 | bo_9_pool |
| 17 | bo_9_overpool |
| 18 | bo_12_pool |
| 19 | bo_9_pool_hatch |
| 20 | bo_9_hatch |
| 21 | bo_10_hatch |
| 22 | bo_11_hatch |
| 23 | bo_12_hatch |
| 24 | bo_13_hatch |
| 25 | three_hatch_muta |
| 26 | bo_z_2hatch_muta |
| 27 | bo_z_3hatch_lurker |
| 28 | bo_z_2hatch_hydra |
| 29 | bo_2_gate |
| 30 | bo_1_gate_core |
| 31 | bo_nexus_first |
| 32 | bo_gate_expand |
| 33 | bo_forge_expa |
| 34 | bo_p_1gate_reaver |
| 35 | bo_p_gate_forge_cannon |
| 36 | bo_p_forge_cannon_gate |
| 37 | bo_p_forge_gate_cannon |
| 38 | bo_bbs |
| 39 | bo_cc_first |
| 40 | bo_t_goliath |
| 41 | bo_t_bio_1base |
| 42 | bo_t_bio_2base |
| 43 | bo_t_111_mech |
| 44 | bo_t_mech_2fac |
| 45 | bo_t_mech_3fac |
| 46 | bo_t_mech_4fac |
| 47 | bo_t_mech_5fac |
| 48 | bo_t_mech_6fac |
| 49 | bo_t_tankless_2fac |
| 50 | bo_t_tankless_3fac |
| 51 | bo_t_tankless_4fac |
| 52 | bo_t_tankless_5fac |
| 53 | bo_t_tankless_6fac |
| 54 | bo_t_111 |
| 55 | bo_t_factory_expand |
| 56 | bo_t_2port_wraith |
| 57 | bo_t_2fact_expa |
| 58 | double_stargate |
| 59 | crazy_zerg |
| 60 | guardians |
| 61 | carriers |
| 62 | battlecruisers |
| 63 | ten_plus_scouts |
| 64 | cliff_drop |

## Game-event featuring chips

Featuring chips for narrative game events that aren't markers (cannon rush, drop, mind control, …): each with a label and one or more unit icons.

| Key | Label | Icons |
| --- | --- | --- |
| bunker_rush | Bunker rush | bunker |
| cannon_rush | Cannon rush | photoncannon |
| drop | Drop | shuttle |
| mind_control | Mind control | darkarchon |
| proxy_factory | Proxy factory | factory |
| proxy_gate | Proxy gateway | gateway |
| proxy_rax | Proxy barracks | barracks |
| proxy_starport | Proxy starport | starport |
| zergling_rush | Zergling rush | zergling |
