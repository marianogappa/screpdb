BEGIN;

-- Main replay tables
CREATE TABLE IF NOT EXISTS replays (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_path TEXT UNIQUE NOT NULL,
	file_checksum TEXT UNIQUE NOT NULL,
	file_name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	replay_date TEXT NOT NULL,
	title TEXT,
	host TEXT,
	map_name TEXT NOT NULL,
	map_width INTEGER NOT NULL,
	map_height INTEGER NOT NULL,
	duration_seconds INTEGER NOT NULL,
	frame_count INTEGER NOT NULL,
	engine_version TEXT NOT NULL,
	engine TEXT NOT NULL,
	game_speed TEXT NOT NULL,
	game_type TEXT NOT NULL,
	home_team_size TEXT NOT NULL,
	avail_slots_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS players (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	race TEXT NOT NULL CHECK (race IN ('Zerg', 'Terran', 'Protoss', 'UNKNOWN')),
	type TEXT NOT NULL CHECK (type IN ('Inactive', 'Computer', 'Human', 'Rescue Passive', '(Unused)', 'Computer Controlled', 'Open', 'Neutral', 'Closed', 'UNKNOWN')),
	color TEXT NOT NULL CHECK (color IN ('Red', 'Blue', 'Teal', 'Purple', 'Orange', 'Brown', 'White', 'Yellow', 'Green', 'Pale Yellow', 'Tan', 'Aqua', 'Pale Green', 'Blueish Grey', 'Pale Yellow2', 'Cyan', 'Pink', 'Olive', 'Lime', 'Navy', 'Dark Aqua', 'Magenta', 'Grey', 'Black', 'UNKNOWN')),
	team INTEGER NOT NULL,
	is_observer BOOLEAN NOT NULL,
	apm INTEGER NOT NULL,
	eapm INTEGER NOT NULL, -- effective apm is apm excluding actions deemed ineffective
	is_winner BOOLEAN NOT NULL,
	start_location_x INTEGER,
	start_location_y INTEGER,
	start_location_oclock INTEGER,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS commands (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	frame INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	action_type TEXT NOT NULL CHECK (action_type IN ('Keep Alive', 'Save Game', 'Load Game', 'Restart Game', 'Select', 'Select Add', 'Select Remove', 'Build', 'Vision', 'Alliance', 'Game Speed', 'Pause', 'Resume', 'Cheat', 'Hotkey', 'Right Click', 'Targeted Order', 'Cancel Build', 'Cancel Morph', 'Stop', 'Carrier Stop', 'Reaver Stop', 'Order Nothing', 'Return Cargo', 'Train', 'Cancel Train', 'Cloack', 'Decloack', 'Unit Morph', 'Unsiege', 'Siege', 'Train Fighter', 'Unload All', 'Unload', 'Merge Archon', 'Hold Position', 'Burrow', 'Unburrow', 'Cancel Nuke', 'Lift Off', 'Tech', 'Cancel Tech', 'Upgrade', 'Cancel Upgrade', 'Cancel Addon', 'Building Morph', 'Stim', 'Sync', 'Voice Enable', 'Voice Disable', 'Voice Squelch', 'Voice Unsquelch', '[Lobby] Start Game', '[Lobby] Download Percentage', '[Lobby] Change Game Slot', '[Lobby] New Net Player', '[Lobby] Joined Game', '[Lobby] Change Race', '[Lobby] Team Game Team', '[Lobby] UMS Team', '[Lobby] Melee Team', '[Lobby] Swap Players', '[Lobby] Saved Data', 'Briefing Start', 'Latency', 'Replay Speed', 'Leave Game', 'Minimap Ping', 'Merge Dark Archon', 'Make Game Public', 'Chat', 'Land', 'UNKNOWN')),
	x INTEGER,
	y INTEGER,

	-- Common fields (used by multiple command types)
	is_queued BOOLEAN,
	order_name TEXT CHECK (order_name IS NULL OR order_name IN ('Die', 'Stop', 'Guard', 'PlayerGuard', 'TurretGuard', 'BunkerGuard', 'Move', 'ReaverStop', 'Attack1', 'Attack2', 'AttackUnit', 'AttackFixedRange', 'AttackTile', 'Hover', 'AttackMove', 'InfestedCommandCenter', 'UnusedNothing', 'UnusedPowerup', 'TowerGuard', 'TowerAttack', 'VultureMine', 'StayInRange', 'TurretAttack', 'Nothing', 'Unused_24', 'DroneStartBuild', 'DroneBuild', 'CastInfestation', 'MoveToInfest', 'InfestingCommandCenter', 'PlaceBuilding', 'PlaceProtossBuilding', 'CreateProtossBuilding', 'ConstructingBuilding', 'Repair', 'MoveToRepair', 'PlaceAddon', 'BuildAddon', 'Train', 'RallyPointUnit', 'RallyPointTile', 'ZergBirth', 'ZergUnitMorph', 'ZergBuildingMorph', 'IncompleteBuilding', 'IncompleteMorphing', 'BuildNydusExit', 'EnterNydusCanal', 'IncompleteWarping', 'Follow', 'Carrier', 'ReaverCarrierMove', 'CarrierStop', 'CarrierAttack', 'CarrierMoveToAttack', 'CarrierIgnore2', 'CarrierFight', 'CarrierHoldPosition', 'Reaver', 'ReaverAttack', 'ReaverMoveToAttack', 'ReaverFight', 'ReaverHoldPosition', 'TrainFighter', 'InterceptorAttack', 'ScarabAttack', 'RechargeShieldsUnit', 'RechargeShieldsBattery', 'ShieldBattery', 'InterceptorReturn', 'DroneLand', 'BuildingLand', 'BuildingLiftOff', 'DroneLiftOff', 'LiftingOff', 'ResearchTech', 'Upgrade', 'Larva', 'SpawningLarva', 'Harvest1', 'Harvest2', 'MoveToGas', 'WaitForGas', 'HarvestGas', 'ReturnGas', 'MoveToMinerals', 'WaitForMinerals', 'MiningMinerals', 'Harvest3', 'Harvest4', 'ReturnMinerals', 'Interrupted', 'EnterTransport', 'PickupIdle', 'PickupTransport', 'PickupBunker', 'Pickup4', 'PowerupIdle', 'Sieging', 'Unsieging', 'WatchTarget', 'InitCreepGrowth', 'SpreadCreep', 'StoppingCreepGrowth', 'GuardianAspect', 'ArchonWarp', 'CompletingArchonSummon', 'HoldPosition', 'QueenHoldPosition', 'Cloak', 'Decloak', 'Unload', 'MoveUnload', 'FireYamatoGun', 'MoveToFireYamatoGun', 'CastLockdown', 'Burrowing', 'Burrowed', 'Unburrowing', 'CastDarkSwarm', 'CastParasite', 'CastSpawnBroodlings', 'CastEMPShockwave', 'NukeWait', 'NukeTrain', 'NukeLaunch', 'NukePaint', 'NukeUnit', 'CastNuclearStrike', 'NukeTrack', 'InitializeArbiter', 'CloakNearbyUnits', 'PlaceMine', 'RightClickAction', 'SuicideUnit', 'SuicideLocation', 'SuicideHoldPosition', 'CastRecall', 'Teleport', 'CastScannerSweep', 'Scanner', 'CastDefensiveMatrix', 'CastPsionicStorm', 'CastIrradiate', 'CastPlague', 'CastConsume', 'CastEnsnare', 'CastStasisField', 'CastHallucination', 'Hallucination2', 'ResetCollision', 'ResetHarvestCollision', 'Patrol', 'CTFCOPInit', 'CTFCOPStarted', 'CTFCOP2', 'ComputerAI', 'AtkMoveEP', 'HarassMove', 'AIPatrol', 'GuardPost', 'RescuePassive', 'Neutral', 'ComputerReturn', 'InitializePsiProvider', 'SelfDestructing', 'Critter', 'HiddenGun', 'OpenDoor', 'CloseDoor', 'HideTrap', 'RevealTrap', 'EnableDoodad', 'DisableDoodad', 'WarpIn', 'Medic', 'MedicHeal', 'HealMove', 'MedicHoldPosition', 'MedicHealToIdle', 'CastRestoration', 'CastDisruptionWeb', 'CastMindControl', 'DarkArchonMeld', 'CastFeedback', 'CastOpticalFlare', 'CastMaelstrom', 'JunkYardDog', 'Fatal', 'None', 'UNKNOWN')),

	-- Unit information (normalized fields)
	unit_type TEXT CHECK (unit_type IS NULL OR unit_type IN ('Marine', 'Ghost', 'Vulture', 'Goliath', 'Goliath Turret', 'Siege Tank (Tank Mode)', 'Siege Tank Turret (Tank Mode)', 'SCV', 'Wraith', 'Science Vessel', 'Gui Motang (Firebat)', 'Dropship', 'Battlecruiser', 'Spider Mine', 'Nuclear Missile', 'Terran Civilian', 'Sarah Kerrigan (Ghost)', 'Alan Schezar (Goliath)', 'Alan Schezar Turret', 'Jim Raynor (Vulture)', 'Jim Raynor (Marine)', 'Tom Kazansky (Wraith)', 'Magellan (Science Vessel)', 'Edmund Duke (Tank Mode)', 'Edmund Duke Turret (Tank Mode)', 'Edmund Duke (Siege Mode)', 'Edmund Duke Turret (Siege Mode)', 'Arcturus Mengsk (Battlecruiser)', 'Hyperion (Battlecruiser)', 'Norad II (Battlecruiser)', 'Terran Siege Tank (Siege Mode)', 'Siege Tank Turret (Siege Mode)', 'Firebat', 'Scanner Sweep', 'Medic', 'Larva', 'Egg', 'Zergling', 'Hydralisk', 'Ultralisk', 'Drone', 'Overlord', 'Mutalisk', 'Guardian', 'Queen', 'Defiler', 'Scourge', 'Torrasque (Ultralisk)', 'Matriarch (Queen)', 'Infested Terran', 'Infested Kerrigan (Infested Terran)', 'Unclean One (Defiler)', 'Hunter Killer (Hydralisk)', 'Devouring One (Zergling)', 'Kukulza (Mutalisk)', 'Kukulza (Guardian)', 'Yggdrasill (Overlord)', 'Valkyrie', 'Mutalisk Cocoon', 'Corsair', 'Dark Templar', 'Devourer', 'Dark Archon', 'Probe', 'Zealot', 'Dragoon', 'High Templar', 'Archon', 'Shuttle', 'Scout', 'Arbiter', 'Carrier', 'Interceptor', 'Protoss Dark Templar (Hero)', 'Zeratul (Dark Templar)', 'Tassadar/Zeratul (Archon)', 'Fenix (Zealot)', 'Fenix (Dragoon)', 'Tassadar (Templar)', 'Mojo (Scout)', 'Warbringer (Reaver)', 'Gantrithor (Carrier)', 'Reaver', 'Observer', 'Scarab', 'Danimoth (Arbiter)', 'Aldaris (Templar)', 'Artanis (Scout)', 'Rhynadon (Badlands Critter)', 'Bengalaas (Jungle Critter)', 'Cargo Ship (Unused)', 'Mercenary Gunship (Unused)', 'Scantid (Desert Critter)', 'Kakaru (Twilight Critter)', 'Ragnasaur (Ashworld Critter)', 'Ursadon (Ice World Critter)', 'Lurker Egg', 'Raszagal (Corsair)', 'Samir Duran (Ghost)', 'Alexei Stukov (Ghost)', 'Map Revealer', 'Gerard DuGalle (BattleCruiser)', 'Lurker', 'Infested Duran (Infested Terran)', 'Disruption Web', 'Command Center', 'ComSat', 'Nuclear Silo', 'Supply Depot', 'Refinery', 'Barracks', 'Academy', 'Factory', 'Starport', 'Control Tower', 'Science Facility', 'Covert Ops', 'Physics Lab', 'Machine Shop', 'Repair Bay (Unused)', 'Engineering Bay', 'Armory', 'Missile Turret', 'Bunker', 'Norad II (Crashed)', 'Ion Cannon', 'Uraj Crystal', 'Khalis Crystal', 'Infested CC', 'Hatchery', 'Lair', 'Hive', 'Nydus Canal', 'Hydralisk Den', 'Defiler Mound', 'Greater Spire', 'Queens Nest', 'Evolution Chamber', 'Ultralisk Cavern', 'Spire', 'Spawning Pool', 'Creep Colony', 'Spore Colony', 'Unused Zerg Building1', 'Sunken Colony', 'Zerg Overmind (With Shell)', 'Overmind', 'Extractor', 'Mature Chrysalis', 'Cerebrate', 'Cerebrate Daggoth', 'Unused Zerg Building2', 'Nexus', 'Robotics Facility', 'Pylon', 'Assimilator', 'Unused Protoss Building1', 'Observatory', 'Gateway', 'Unused Protoss Building2', 'Photon Cannon', 'Citadel of Adun', 'Cybernetics Core', 'Templar Archives', 'Forge', 'Stargate', 'Stasis Cell/Prison', 'Fleet Beacon', 'Arbiter Tribunal', 'Robotics Support Bay', 'Shield Battery', 'Khaydarin Crystal Formation', 'Protoss Temple', 'Xel''Naga Temple', 'Mineral Field (Type 1)', 'Mineral Field (Type 2)', 'Mineral Field (Type 3)', 'Cave (Unused)', 'Cave-in (Unused)', 'Cantina (Unused)', 'Mining Platform (Unused)', 'Independent Command Center (Unused)', 'Independent Starport (Unused)', 'Independent Jump Gate (Unused)', 'Ruins (Unused)', 'Khaydarin Crystal Formation (Unused)', 'Vespene Geyser', 'Warp Gate', 'Psi Disrupter', 'Zerg Marker', 'Terran Marker', 'Protoss Marker', 'Zerg Beacon', 'Terran Beacon', 'Protoss Beacon', 'Zerg Flag Beacon', 'Terran Flag Beacon', 'Protoss Flag Beacon', 'Power Generator', 'Overmind Cocoon', 'Dark Swarm', 'Floor Missile Trap', 'Floor Hatch (Unused)', 'Left Upper Level Door', 'Right Upper Level Door', 'Left Pit Door', 'Right Pit Door', 'Floor Gun Trap', 'Left Wall Missile Trap', 'Left Wall Flame Trap', 'Right Wall Missile Trap', 'Right Wall Flame Trap', 'Start Location', 'Flag', 'Young Chrysalis', 'Psi Emitter', 'Data Disc', 'Khaydarin Crystal', 'Mineral Cluster Type 1', 'Mineral Cluster Type 2', 'Protoss Vespene Gas Orb Type 1', 'Protoss Vespene Gas Orb Type 2', 'Zerg Vespene Gas Sac Type 1', 'Zerg Vespene Gas Sac Type 2', 'Terran Vespene Gas Tank Type 1', 'Terran Vespene Gas Tank Type 2', 'None', 'UNKNOWN')), -- Single unit type
	unit_types TEXT, -- JSON array of unit types for multiple units

	-- Tech command fields
	tech_name TEXT CHECK (tech_name IS NULL OR tech_name IN ('Stim Packs', 'Lockdown', 'EMP Shockwave', 'Spider Mines', 'Scanner Sweep', 'Tank Siege Mode', 'Defensive Matrix', 'Irradiate', 'Yamato Gun', 'Cloaking Field', 'Personnel Cloaking', 'Burrowing', 'Infestation', 'Spawn Broodlings', 'Dark Swarm', 'Plague', 'Consume', 'Ensnare', 'Parasite', 'Psionic Storm', 'Hallucination', 'Recall', 'Stasis Field', 'Archon Warp', 'Restoration', 'Disruption Web', 'Unused 26', 'Mind Control', 'Dark Archon Meld', 'Feedback', 'Optical Flare', 'Maelstrom', 'Lurker Aspect', 'Unused 33', 'Healing', 'UNKNOWN')),

	-- Upgrade command fields
	upgrade_name TEXT CHECK (upgrade_name IS NULL OR upgrade_name IN ('Terran Infantry Armor', 'Terran Vehicle Plating', 'Terran Ship Plating', 'Zerg Carapace', 'Zerg Flyer Carapace', 'Protoss Ground Armor', 'Protoss Air Armor', 'Terran Infantry Weapons', 'Terran Vehicle Weapons', 'Terran Ship Weapons', 'Zerg Melee Attacks', 'Zerg Missile Attacks', 'Zerg Flyer Attacks', 'Protoss Ground Weapons', 'Protoss Air Weapons', 'Protoss Plasma Shields', 'U-238 Shells (Marine Range)', 'Ion Thrusters (Vulture Speed)', 'Titan Reactor (Science Vessel Energy)', 'Ocular Implants (Ghost Sight)', 'Moebius Reactor (Ghost Energy)', 'Apollo Reactor (Wraith Energy)', 'Colossus Reactor (Battle Cruiser Energy)', 'Ventral Sacs (Overlord Transport)', 'Antennae (Overlord Sight)', 'Pneumatized Carapace (Overlord Speed)', 'Metabolic Boost (Zergling Speed)', 'Adrenal Glands (Zergling Attack)', 'Muscular Augments (Hydralisk Speed)', 'Grooved Spines (Hydralisk Range)', 'Gamete Meiosis (Queen Energy)', 'Defiler Energy', 'Singularity Charge (Dragoon Range)', 'Leg Enhancement (Zealot Speed)', 'Scarab Damage', 'Reaver Capacity', 'Gravitic Drive (Shuttle Speed)', 'Sensor Array (Observer Sight)', 'Gravitic Booster (Observer Speed)', 'Khaydarin Amulet (Templar Energy)', 'Apial Sensors (Scout Sight)', 'Gravitic Thrusters (Scout Speed)', 'Carrier Capacity', 'Khaydarin Core (Arbiter Energy)', 'Argus Jewel (Corsair Energy)', 'Argus Talisman (Dark Archon Energy)', 'Caduceus Reactor (Medic Energy)', 'Chitinous Plating (Ultralisk Armor)', 'Anabolic Synthesis (Ultralisk Speed)', 'Charon Boosters (Goliath Range)', 'UNKNOWN')),

	-- Hotkey command fields
	hotkey_type TEXT CHECK (hotkey_type IS NULL OR hotkey_type IN ('Assign', 'Select', 'Add', 'UNKNOWN')),
	hotkey_group INTEGER,

	-- Game Speed command fields
	game_speed TEXT CHECK (game_speed IS NULL OR game_speed IN ('Slowest', 'Slower', 'Slow', 'Normal', 'Fast', 'Faster', 'Fastest', 'UNKNOWN')),

	-- Vision command fields
	vision_player_ids TEXT, -- JSON array of player IDs

	-- Alliance command fields
	alliance_player_ids TEXT, -- JSON array of player IDs
	is_allied_victory BOOLEAN,

	-- General command fields (for unhandled commands)
	general_data TEXT, -- Hex string of raw data

	-- Chat and leave game fields
	chat_message TEXT,
	leave_reason TEXT CHECK (leave_reason IS NULL OR leave_reason IN ('Quit', 'Defeat', 'Victory', 'Finished', 'Draw', 'Dropped', 'UNKNOWN')),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Low-value command actions are stored separately to keep analytics scans lean.
CREATE TABLE IF NOT EXISTS commands_low_value (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	frame INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	action_type TEXT NOT NULL CHECK (action_type IN ('Keep Alive', 'Save Game', 'Load Game', 'Restart Game', 'Select', 'Select Add', 'Select Remove', 'Build', 'Vision', 'Alliance', 'Game Speed', 'Pause', 'Resume', 'Cheat', 'Hotkey', 'Right Click', 'Targeted Order', 'Cancel Build', 'Cancel Morph', 'Stop', 'Carrier Stop', 'Reaver Stop', 'Order Nothing', 'Return Cargo', 'Train', 'Cancel Train', 'Cloack', 'Decloack', 'Unit Morph', 'Unsiege', 'Siege', 'Train Fighter', 'Unload All', 'Unload', 'Merge Archon', 'Hold Position', 'Burrow', 'Unburrow', 'Cancel Nuke', 'Lift Off', 'Tech', 'Cancel Tech', 'Upgrade', 'Cancel Upgrade', 'Cancel Addon', 'Building Morph', 'Stim', 'Sync', 'Voice Enable', 'Voice Disable', 'Voice Squelch', 'Voice Unsquelch', '[Lobby] Start Game', '[Lobby] Download Percentage', '[Lobby] Change Game Slot', '[Lobby] New Net Player', '[Lobby] Joined Game', '[Lobby] Change Race', '[Lobby] Team Game Team', '[Lobby] UMS Team', '[Lobby] Melee Team', '[Lobby] Swap Players', '[Lobby] Saved Data', 'Briefing Start', 'Latency', 'Replay Speed', 'Leave Game', 'Minimap Ping', 'Merge Dark Archon', 'Make Game Public', 'Chat', 'Land', 'UNKNOWN')),
	x INTEGER,
	y INTEGER,

	-- Common fields (used by multiple command types)
	is_queued BOOLEAN,
	order_name TEXT CHECK (order_name IS NULL OR order_name IN ('Die', 'Stop', 'Guard', 'PlayerGuard', 'TurretGuard', 'BunkerGuard', 'Move', 'ReaverStop', 'Attack1', 'Attack2', 'AttackUnit', 'AttackFixedRange', 'AttackTile', 'Hover', 'AttackMove', 'InfestedCommandCenter', 'UnusedNothing', 'UnusedPowerup', 'TowerGuard', 'TowerAttack', 'VultureMine', 'StayInRange', 'TurretAttack', 'Nothing', 'Unused_24', 'DroneStartBuild', 'DroneBuild', 'CastInfestation', 'MoveToInfest', 'InfestingCommandCenter', 'PlaceBuilding', 'PlaceProtossBuilding', 'CreateProtossBuilding', 'ConstructingBuilding', 'Repair', 'MoveToRepair', 'PlaceAddon', 'BuildAddon', 'Train', 'RallyPointUnit', 'RallyPointTile', 'ZergBirth', 'ZergUnitMorph', 'ZergBuildingMorph', 'IncompleteBuilding', 'IncompleteMorphing', 'BuildNydusExit', 'EnterNydusCanal', 'IncompleteWarping', 'Follow', 'Carrier', 'ReaverCarrierMove', 'CarrierStop', 'CarrierAttack', 'CarrierMoveToAttack', 'CarrierIgnore2', 'CarrierFight', 'CarrierHoldPosition', 'Reaver', 'ReaverAttack', 'ReaverMoveToAttack', 'ReaverFight', 'ReaverHoldPosition', 'TrainFighter', 'InterceptorAttack', 'ScarabAttack', 'RechargeShieldsUnit', 'RechargeShieldsBattery', 'ShieldBattery', 'InterceptorReturn', 'DroneLand', 'BuildingLand', 'BuildingLiftOff', 'DroneLiftOff', 'LiftingOff', 'ResearchTech', 'Upgrade', 'Larva', 'SpawningLarva', 'Harvest1', 'Harvest2', 'MoveToGas', 'WaitForGas', 'HarvestGas', 'ReturnGas', 'MoveToMinerals', 'WaitForMinerals', 'MiningMinerals', 'Harvest3', 'Harvest4', 'ReturnMinerals', 'Interrupted', 'EnterTransport', 'PickupIdle', 'PickupTransport', 'PickupBunker', 'Pickup4', 'PowerupIdle', 'Sieging', 'Unsieging', 'WatchTarget', 'InitCreepGrowth', 'SpreadCreep', 'StoppingCreepGrowth', 'GuardianAspect', 'ArchonWarp', 'CompletingArchonSummon', 'HoldPosition', 'QueenHoldPosition', 'Cloak', 'Decloak', 'Unload', 'MoveUnload', 'FireYamatoGun', 'MoveToFireYamatoGun', 'CastLockdown', 'Burrowing', 'Burrowed', 'Unburrowing', 'CastDarkSwarm', 'CastParasite', 'CastSpawnBroodlings', 'CastEMPShockwave', 'NukeWait', 'NukeTrain', 'NukeLaunch', 'NukePaint', 'NukeUnit', 'CastNuclearStrike', 'NukeTrack', 'InitializeArbiter', 'CloakNearbyUnits', 'PlaceMine', 'RightClickAction', 'SuicideUnit', 'SuicideLocation', 'SuicideHoldPosition', 'CastRecall', 'Teleport', 'CastScannerSweep', 'Scanner', 'CastDefensiveMatrix', 'CastPsionicStorm', 'CastIrradiate', 'CastPlague', 'CastConsume', 'CastEnsnare', 'CastStasisField', 'CastHallucination', 'Hallucination2', 'ResetCollision', 'ResetHarvestCollision', 'Patrol', 'CTFCOPInit', 'CTFCOPStarted', 'CTFCOP2', 'ComputerAI', 'AtkMoveEP', 'HarassMove', 'AIPatrol', 'GuardPost', 'RescuePassive', 'Neutral', 'ComputerReturn', 'InitializePsiProvider', 'SelfDestructing', 'Critter', 'HiddenGun', 'OpenDoor', 'CloseDoor', 'HideTrap', 'RevealTrap', 'EnableDoodad', 'DisableDoodad', 'WarpIn', 'Medic', 'MedicHeal', 'HealMove', 'MedicHoldPosition', 'MedicHealToIdle', 'CastRestoration', 'CastDisruptionWeb', 'CastMindControl', 'DarkArchonMeld', 'CastFeedback', 'CastOpticalFlare', 'CastMaelstrom', 'JunkYardDog', 'Fatal', 'None', 'UNKNOWN')),

	-- Unit information (normalized fields)
	unit_type TEXT CHECK (unit_type IS NULL OR unit_type IN ('Marine', 'Ghost', 'Vulture', 'Goliath', 'Goliath Turret', 'Siege Tank (Tank Mode)', 'Siege Tank Turret (Tank Mode)', 'SCV', 'Wraith', 'Science Vessel', 'Gui Motang (Firebat)', 'Dropship', 'Battlecruiser', 'Spider Mine', 'Nuclear Missile', 'Terran Civilian', 'Sarah Kerrigan (Ghost)', 'Alan Schezar (Goliath)', 'Alan Schezar Turret', 'Jim Raynor (Vulture)', 'Jim Raynor (Marine)', 'Tom Kazansky (Wraith)', 'Magellan (Science Vessel)', 'Edmund Duke (Tank Mode)', 'Edmund Duke Turret (Tank Mode)', 'Edmund Duke (Siege Mode)', 'Edmund Duke Turret (Siege Mode)', 'Arcturus Mengsk (Battlecruiser)', 'Hyperion (Battlecruiser)', 'Norad II (Battlecruiser)', 'Terran Siege Tank (Siege Mode)', 'Siege Tank Turret (Siege Mode)', 'Firebat', 'Scanner Sweep', 'Medic', 'Larva', 'Egg', 'Zergling', 'Hydralisk', 'Ultralisk', 'Drone', 'Overlord', 'Mutalisk', 'Guardian', 'Queen', 'Defiler', 'Scourge', 'Torrasque (Ultralisk)', 'Matriarch (Queen)', 'Infested Terran', 'Infested Kerrigan (Infested Terran)', 'Unclean One (Defiler)', 'Hunter Killer (Hydralisk)', 'Devouring One (Zergling)', 'Kukulza (Mutalisk)', 'Kukulza (Guardian)', 'Yggdrasill (Overlord)', 'Valkyrie', 'Mutalisk Cocoon', 'Corsair', 'Dark Templar', 'Devourer', 'Dark Archon', 'Probe', 'Zealot', 'Dragoon', 'High Templar', 'Archon', 'Shuttle', 'Scout', 'Arbiter', 'Carrier', 'Interceptor', 'Protoss Dark Templar (Hero)', 'Zeratul (Dark Templar)', 'Tassadar/Zeratul (Archon)', 'Fenix (Zealot)', 'Fenix (Dragoon)', 'Tassadar (Templar)', 'Mojo (Scout)', 'Warbringer (Reaver)', 'Gantrithor (Carrier)', 'Reaver', 'Observer', 'Scarab', 'Danimoth (Arbiter)', 'Aldaris (Templar)', 'Artanis (Scout)', 'Rhynadon (Badlands Critter)', 'Bengalaas (Jungle Critter)', 'Cargo Ship (Unused)', 'Mercenary Gunship (Unused)', 'Scantid (Desert Critter)', 'Kakaru (Twilight Critter)', 'Ragnasaur (Ashworld Critter)', 'Ursadon (Ice World Critter)', 'Lurker Egg', 'Raszagal (Corsair)', 'Samir Duran (Ghost)', 'Alexei Stukov (Ghost)', 'Map Revealer', 'Gerard DuGalle (BattleCruiser)', 'Lurker', 'Infested Duran (Infested Terran)', 'Disruption Web', 'Command Center', 'ComSat', 'Nuclear Silo', 'Supply Depot', 'Refinery', 'Barracks', 'Academy', 'Factory', 'Starport', 'Control Tower', 'Science Facility', 'Covert Ops', 'Physics Lab', 'Machine Shop', 'Repair Bay (Unused)', 'Engineering Bay', 'Armory', 'Missile Turret', 'Bunker', 'Norad II (Crashed)', 'Ion Cannon', 'Uraj Crystal', 'Khalis Crystal', 'Infested CC', 'Hatchery', 'Lair', 'Hive', 'Nydus Canal', 'Hydralisk Den', 'Defiler Mound', 'Greater Spire', 'Queens Nest', 'Evolution Chamber', 'Ultralisk Cavern', 'Spire', 'Spawning Pool', 'Creep Colony', 'Spore Colony', 'Unused Zerg Building1', 'Sunken Colony', 'Zerg Overmind (With Shell)', 'Overmind', 'Extractor', 'Mature Chrysalis', 'Cerebrate', 'Cerebrate Daggoth', 'Unused Zerg Building2', 'Nexus', 'Robotics Facility', 'Pylon', 'Assimilator', 'Unused Protoss Building1', 'Observatory', 'Gateway', 'Unused Protoss Building2', 'Photon Cannon', 'Citadel of Adun', 'Cybernetics Core', 'Templar Archives', 'Forge', 'Stargate', 'Stasis Cell/Prison', 'Fleet Beacon', 'Arbiter Tribunal', 'Robotics Support Bay', 'Shield Battery', 'Khaydarin Crystal Formation', 'Protoss Temple', 'Xel''Naga Temple', 'Mineral Field (Type 1)', 'Mineral Field (Type 2)', 'Mineral Field (Type 3)', 'Cave (Unused)', 'Cave-in (Unused)', 'Cantina (Unused)', 'Mining Platform (Unused)', 'Independent Command Center (Unused)', 'Independent Starport (Unused)', 'Independent Jump Gate (Unused)', 'Ruins (Unused)', 'Khaydarin Crystal Formation (Unused)', 'Vespene Geyser', 'Warp Gate', 'Psi Disrupter', 'Zerg Marker', 'Terran Marker', 'Protoss Marker', 'Zerg Beacon', 'Terran Beacon', 'Protoss Beacon', 'Zerg Flag Beacon', 'Terran Flag Beacon', 'Protoss Flag Beacon', 'Power Generator', 'Overmind Cocoon', 'Dark Swarm', 'Floor Missile Trap', 'Floor Hatch (Unused)', 'Left Upper Level Door', 'Right Upper Level Door', 'Left Pit Door', 'Right Pit Door', 'Floor Gun Trap', 'Left Wall Missile Trap', 'Left Wall Flame Trap', 'Right Wall Missile Trap', 'Right Wall Flame Trap', 'Start Location', 'Flag', 'Young Chrysalis', 'Psi Emitter', 'Data Disc', 'Khaydarin Crystal', 'Mineral Cluster Type 1', 'Mineral Cluster Type 2', 'Protoss Vespene Gas Orb Type 1', 'Protoss Vespene Gas Orb Type 2', 'Zerg Vespene Gas Sac Type 1', 'Zerg Vespene Gas Sac Type 2', 'Terran Vespene Gas Tank Type 1', 'Terran Vespene Gas Tank Type 2', 'None', 'UNKNOWN')), -- Single unit type
	unit_types TEXT, -- JSON array of unit types for multiple units

	-- Tech command fields
	tech_name TEXT CHECK (tech_name IS NULL OR tech_name IN ('Stim Packs', 'Lockdown', 'EMP Shockwave', 'Spider Mines', 'Scanner Sweep', 'Tank Siege Mode', 'Defensive Matrix', 'Irradiate', 'Yamato Gun', 'Cloaking Field', 'Personnel Cloaking', 'Burrowing', 'Infestation', 'Spawn Broodlings', 'Dark Swarm', 'Plague', 'Consume', 'Ensnare', 'Parasite', 'Psionic Storm', 'Hallucination', 'Recall', 'Stasis Field', 'Archon Warp', 'Restoration', 'Disruption Web', 'Unused 26', 'Mind Control', 'Dark Archon Meld', 'Feedback', 'Optical Flare', 'Maelstrom', 'Lurker Aspect', 'Unused 33', 'Healing', 'UNKNOWN')),

	-- Upgrade command fields
	upgrade_name TEXT CHECK (upgrade_name IS NULL OR upgrade_name IN ('Terran Infantry Armor', 'Terran Vehicle Plating', 'Terran Ship Plating', 'Zerg Carapace', 'Zerg Flyer Carapace', 'Protoss Ground Armor', 'Protoss Air Armor', 'Terran Infantry Weapons', 'Terran Vehicle Weapons', 'Terran Ship Weapons', 'Zerg Melee Attacks', 'Zerg Missile Attacks', 'Zerg Flyer Attacks', 'Protoss Ground Weapons', 'Protoss Air Weapons', 'Protoss Plasma Shields', 'U-238 Shells (Marine Range)', 'Ion Thrusters (Vulture Speed)', 'Titan Reactor (Science Vessel Energy)', 'Ocular Implants (Ghost Sight)', 'Moebius Reactor (Ghost Energy)', 'Apollo Reactor (Wraith Energy)', 'Colossus Reactor (Battle Cruiser Energy)', 'Ventral Sacs (Overlord Transport)', 'Antennae (Overlord Sight)', 'Pneumatized Carapace (Overlord Speed)', 'Metabolic Boost (Zergling Speed)', 'Adrenal Glands (Zergling Attack)', 'Muscular Augments (Hydralisk Speed)', 'Grooved Spines (Hydralisk Range)', 'Gamete Meiosis (Queen Energy)', 'Defiler Energy', 'Singularity Charge (Dragoon Range)', 'Leg Enhancement (Zealot Speed)', 'Scarab Damage', 'Reaver Capacity', 'Gravitic Drive (Shuttle Speed)', 'Sensor Array (Observer Sight)', 'Gravitic Booster (Observer Speed)', 'Khaydarin Amulet (Templar Energy)', 'Apial Sensors (Scout Sight)', 'Gravitic Thrusters (Scout Speed)', 'Carrier Capacity', 'Khaydarin Core (Arbiter Energy)', 'Argus Jewel (Corsair Energy)', 'Argus Talisman (Dark Archon Energy)', 'Caduceus Reactor (Medic Energy)', 'Chitinous Plating (Ultralisk Armor)', 'Anabolic Synthesis (Ultralisk Speed)', 'Charon Boosters (Goliath Range)', 'UNKNOWN')),

	-- Hotkey command fields
	hotkey_type TEXT CHECK (hotkey_type IS NULL OR hotkey_type IN ('Assign', 'Select', 'Add', 'UNKNOWN')),
	hotkey_group INTEGER,

	-- Game Speed command fields
	game_speed TEXT CHECK (game_speed IS NULL OR game_speed IN ('Slowest', 'Slower', 'Slow', 'Normal', 'Fast', 'Faster', 'Fastest', 'UNKNOWN')),

	-- Vision command fields
	vision_player_ids TEXT, -- JSON array of player IDs

	-- Alliance command fields
	alliance_player_ids TEXT, -- JSON array of player IDs
	is_allied_victory BOOLEAN,

	-- General command fields (for unhandled commands)
	general_data TEXT, -- Hex string of raw data

	-- Chat and leave game fields
	chat_message TEXT,
	leave_reason TEXT CHECK (leave_reason IS NULL OR leave_reason IN ('Quit', 'Defeat', 'Victory', 'Finished', 'Draw', 'Dropped', 'UNKNOWN')),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS detected_patterns_replay (
	replay_id INTEGER NOT NULL,
	algorithm_version INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS detected_patterns_replay_team (
	replay_id INTEGER NOT NULL,
	team INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, team, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS detected_patterns_replay_player (
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, player_id, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_replays_file_path ON replays(file_path);
CREATE INDEX IF NOT EXISTS idx_replays_file_checksum ON replays(file_checksum);
CREATE INDEX IF NOT EXISTS idx_replays_replay_date ON replays(replay_date);
CREATE INDEX IF NOT EXISTS idx_players_replay_id ON players(replay_id);
CREATE INDEX IF NOT EXISTS idx_commands_player_id_action_type ON commands(player_id, action_type);
CREATE INDEX IF NOT EXISTS idx_commands_replay_id_player_id_action_type ON commands(replay_id, player_id, action_type);
CREATE INDEX IF NOT EXISTS idx_commands_replay_id_action_type_seconds ON commands(replay_id, action_type, seconds_from_game_start);
CREATE INDEX IF NOT EXISTS idx_commands_action_type_order_name ON commands(action_type, order_name);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_replay_id ON detected_patterns_replay(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_team_replay_id ON detected_patterns_replay_team(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_player_replay_id ON detected_patterns_replay_player(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_player_player_id ON detected_patterns_replay_player(player_id);

COMMIT;
