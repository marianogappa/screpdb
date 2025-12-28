package dashboard

const (
	schemaObservations = `
	- Replays have up to 8 players (and up to 4 observers) and a sequential list of commands/actions (like Chess). Command timing is tracked in "frames" since game start and also with a timestamp.
	- The commands table has action-type-specific fields, so for a given row many fields are null.

	- JOIN patterns:
		- players.replay_id = replays.id
		- commands.replay_id = replays.id
		- commands.player_id = players.id

	- Common WHERE clauses:
		- players.type = 'Human' (i.e. skip 'Computer' players)
		- players.is_observer = false (i.e. Observer players are not part of the game)

	action_types:
		- Build
		- Land
		- RightClick
		- TargetedOrder
		- Train
		- BuildInterceptorOrScarab
		- MinimapPing
		- CancelTrain
		- UnitMorph
		- Tech
		- Upgrade
		- GameSpeed
		- Hotkey
		- Chat
		- Vision
		- Alliance
		- LeaveGame
		- Stop
		- CarrierStop
		- ReaverStop
		- ReturnCargo
		- UnloadAll
		- HoldPosition
		- Burrow
		- Unburrow
		- Siege
		- Unsiege
		- Cloack
		- Decloack
		- Cheat

	unit_types:

		- Supply Depot
		- Forge
		- Hydralisk Den
		- Siege Tank (Tank Mode)
		- Barracks
		- Reaver
		- Engineering Bay
		- ComSat
		- Valkyrie
		- Corsair
		- Creep Colony
		- Extractor
		- Covert Ops
		- Gateway
		- Ultralisk
		- Academy
		- Nuclear Missile
		- Defiler Mound
		- Guardian
		- Spore Colony
		- Templar Archives
		- Arbiter
		- Hive
		- Firebat
		- Zealot
		- Arbiter Tribunal
		- Cybernetics Core
		- Wraith
		- Overlord
		- Evolution Chamber
		- Stargate
		- Physics Lab
		- Spawning Pool
		- Science Facility
		- Fleet Beacon
		- Goliath
		- Probe
		- Missile Turret
		- Sunken Colony
		- Robotics Support Bay
		- Vulture
		- Nuclear Silo
		- Medic
		- Observatory
		- Queen
		- High Templar
		- Starport
		- Ghost
		- Spire
		- Armory
		- Factory
		- Nexus
		- Marine
		- Bunker
		- Battlecruiser
		- Shield Battery
		- Robotics Facility
		- Mutalisk
		- Carrier
		- Hydralisk
		- Shuttle
		- Scourge
		- Observer
		- Greater Spire
		- Devourer
		- Scout
		- Drone
		- Machine Shop
		- Lair
		- Refinery
		- Dark Templar
		- SCV
		- Nydus Canal
		- Queens Nest
		- Dropship
		- Hatchery
		- Ultralisk Cavern
		- Assimilator
		- Science Vessel
		- Dragoon
		- Photon Cannon
		- Lurker
		- Defiler
		- Pylon
		- Control Tower
		- Zergling
		- Citadel of Adun
		- Command Center
	`

	starcraftKnowledge = `
"StarCraft: Remastered" is a real-time strategy game where players choose a race (Terran, Protoss or Zerg), build economies, form armies, sometimes ally other players, and battle for map control until one side destroys all opponent buildings to win.

Game Mechanics

- Economy: workers mine → resources → spend on units/buildings
- Tech tree: unlocks progressively with buildings
- Army control: composition, micro, positioning
- Fog of war: vision-limited
- Win condition: destroy all opponent buildings
- Maximum supply count: 200. Workers cost 1. Heavier units cost 2, 4, etc.

Units: combat, workers, spellcasters, transports, detectors
Resources: minerals, vespene gas, supply (food cap)
Buildings: tech tree enablers, production, economy, defense
Worker units: Drone (Zerg), Probe (Protoss), SCV (Terran)
Main building: Nexus (Protoss), Command Center (Terran), Hatchery/Lair/Hive (Zerg). Resources are gathered to these buildings.

Replay Essentials

- Timeline of actions (build orders, expansions, engagements)
- APM (actions per minute), supply, resources, spending efficiency
- Army composition over time
- Map size is in tiles (128x128, 96x256) but (x, y) is in pixels. 1 tile = 32 pixels.
- Map control, scouting, expansions

Macro vs Micro

- These commands are macro: Build, Train, BuildInterceptorOrScarab, UnitMorph, Tech, Upgrade
- These commands are micro: RightClick, TargetedOrder, Hotkey, UnloadAll, HoldPosition, Burrow, Unburrow, Siege, Unsiege, Cloack, Decloack

Report Metrics

- If you're asked to report on players, stick to "players.type = 'Human'" (skip Computer)
- 'players.is_winner' is not too accurate. Players may leave game after winning, replays may be incomplete.
- Resource collection & spending efficiency
- Player performance stats (APM, using hotkeys, macro vs micro balance, time to first building, time to first combat unit, time to expansion)
- Better players: have higher APMs, use hotkeys, > micro actions, if they have more workers they make more units/buildings.
- Build order timings (e.g. “2 Hatch Muta,” “1 Gate Expand”)

Meta terms

- "rush": when a player attacks another (e.g. RightClick, TargetedOrder w/ order_name Attack*) within a few minutes of game start
- "timing push": deliberate attack launched at a specific moment when a build order hits a temporary power spike (e.g. first tanks with siege, zealots become fast, mutalisks get +1 attack).
- "tech switch": Rapidly shifting production to a different unit tech path to exploit an opponent’s weak counters (e.g. mutalisks → lurkers, marine+medic → tank+goliath).
- "natural": The first "expansion" (main building) to gather more resources, which is in close proximity to the main starting location.
- "expa/expansion": Another expansion which is not necessarily the "natural".
- Main building starting locations are usually conveyed in o'clock positions (like 3, 6, 9, 12).

	`
)
