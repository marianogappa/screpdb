const unitIconURL = (mapKey) => `/api/custom/game-assets/unit?name=${encodeURIComponent(mapKey)}`;

const buildingIconURL = (mapKey) => `/api/custom/game-assets/building?name=${encodeURIComponent(mapKey)}`;

export const normalizeUnitName = (value) =>
  String(value || '')
    .toLowerCase()
    .replace(/\s+/g, '')
    .replace(/[^a-z0-9]/g, '');

const stripRacePrefix = (normalized) => {
  const prefixes = ['terran', 'protoss', 'zerg'];
  for (const prefix of prefixes) {
    if (normalized.startsWith(prefix) && normalized.length > prefix.length) {
      return normalized.slice(prefix.length);
    }
  }
  return normalized;
};

const stripModeSuffix = (withoutRace) =>
  withoutRace.replace(/siegemode$/, '').replace(/tankmode$/, '').replace(/turret$/, '');

const resolveIconMapKey = (unitType) => {
  const normalized = normalizeUnitName(unitType);
  if (!normalized) return '';
  const tryKeys = (key) => {
    if (!key) return '';
    if (gameAssetIconURLKeys.has(key)) return key;
    return '';
  };
  const a = tryKeys(normalized);
  if (a) return a;
  const b = tryKeys(stripRacePrefix(normalized));
  if (b) return b;
  const c = tryKeys(stripModeSuffix(stripRacePrefix(normalized)));
  return c || '';
};

const gameAssetIconURLKeys = new Set([
  'probe',
  'scv',
  'drone',
  'arbiter',
  'protossarbiter',
  'corsair',
  'protosscorsair',
  'scout',
  'protossscout',
  'reaver',
  'protossreaver',
  'overlord',
  'zergoverlord',
  'scourge',
  'zergscourge',
  'observer',
  'protossobserver',
  'carrier',
  'battlecruiser',
  'terranbattlecruiser',
  'dropship',
  'terrandropship',
  'sciencevessel',
  'terransciencevessel',
  'wraith',
  'terranwraith',
  'marine',
  'siegetank',
  'siegetanktankmode',
  'siegetankturrettankmode',
  'terransiegetanksiegemode',
  'siegetankturretsiegemode',
  'zealot',
  'dragoon',
  'zergling',
  'hydralisk',
  'mutalisk',
  'ultralisk',
  'goliath',
  'vulture',
  'medic',
  'defiler',
  'zergdefiler',
  'firebat',
  'darktemplar',
  'hightemplar',
  'lurker',
  'archon',
  'ghost',
  'valkyrie',
  'devourer',
  'darkarchon',
  'guardian',
  'infestedterran',
  'queen',
  'shuttle',
  'academy',
  'arbitertribunal',
  'armory',
  'assimilator',
  'barracks',
  'bunker',
  'citadelofadun',
  'comsat',
  'controltower',
  'covertops',
  'creepcolony',
  'cyberneticscore',
  'defilermound',
  'engineeringbay',
  'evolutionchamber',
  'extractor',
  'factory',
  'fleetbeacon',
  'forge',
  'gateway',
  'greaterspire',
  'hatchery',
  'hive',
  'hydraliskden',
  'infestedcc',
  'lair',
  'machineshop',
  'missileturret',
  'nexus',
  'nyduscanal',
  'observatory',
  'photoncannon',
  'physicslab',
  'pylon',
  'queensnest',
  'refinery',
  'roboticsfacility',
  'roboticssupportbay',
  'sciencefacility',
  'shieldbattery',
  'spawningpool',
  'spire',
  'sporecolony',
  'stargate',
  'starport',
  'sunkencolony',
  'supplydepot',
  'templararchives',
  'ultraliskcavern',
]);

const buildingKeys = new Set([
  'academy',
  'arbitertribunal',
  'armory',
  'assimilator',
  'barracks',
  'bunker',
  'citadelofadun',
  'comsat',
  'controltower',
  'covertops',
  'creepcolony',
  'cyberneticscore',
  'defilermound',
  'engineeringbay',
  'evolutionchamber',
  'extractor',
  'factory',
  'fleetbeacon',
  'forge',
  'gateway',
  'greaterspire',
  'hatchery',
  'hive',
  'hydraliskden',
  'infestedcc',
  'lair',
  'machineshop',
  'missileturret',
  'nexus',
  'nyduscanal',
  'observatory',
  'photoncannon',
  'physicslab',
  'pylon',
  'queensnest',
  'refinery',
  'roboticsfacility',
  'roboticssupportbay',
  'sciencefacility',
  'shieldbattery',
  'spawningpool',
  'spire',
  'sporecolony',
  'stargate',
  'starport',
  'sunkencolony',
  'supplydepot',
  'templararchives',
  'ultraliskcavern',
]);

export const getUnitIcon = (unitType) => {
  const key = resolveIconMapKey(unitType);
  if (!key) return null;
  if (buildingKeys.has(key)) {
    return buildingIconURL(key);
  }
  return unitIconURL(key);
};
