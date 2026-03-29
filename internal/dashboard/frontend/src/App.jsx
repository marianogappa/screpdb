import React, { useState, useEffect, useMemo, useRef } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import PieChart from './components/charts/PieChart';
import Gauge from './components/charts/Gauge';
import Table from './components/charts/Table';
import BarChart from './components/charts/BarChart';
import LineChart from './components/charts/LineChart';
import ScatterPlot from './components/charts/ScatterPlot';
import Histogram from './components/charts/Histogram';
import Heatmap from './components/charts/Heatmap';
import TimingScatterRows from './components/charts/TimingScatterRows';
import probeImg from './assets/units/probe.png';
import scvImg from './assets/units/scv.png';
import droneImg from './assets/units/drone.png';
import arbiterImg from './assets/units/arbiter.png';
import scoutImg from './assets/units/scout.png';
import reaverImg from './assets/units/reaver.png';
import overlordImg from './assets/units/overlord.png';
import scourgeImg from './assets/units/scourge.png';
import observerImg from './assets/units/observer.png';
import carrierImg from './assets/units/carrier.png';
import battlecruiserImg from './assets/units/battlecruiser.png';
import dropshipImg from './assets/units/dropship.png';
import scienceVesselImg from './assets/units/sciencevessel.png';
import wraithImg from './assets/units/wraith.png';
import marineImg from './assets/units/marine.png';
import zealotImg from './assets/units/zealot.png';
import dragoonImg from './assets/units/dragoon.png';
import siegetankImg from './assets/units/siegetank.png';
import zerglingImg from './assets/units/zergling.png';
import hydraliskImg from './assets/units/hydralisk.png';
import mutaliskImg from './assets/units/mutalisk.png';
import ultraliskImg from './assets/units/ultralisk.png';
import goliathImg from './assets/units/goliath.png';
import vultureImg from './assets/units/vulture.png';
import medicImg from './assets/units/medic.png';
import defilerImg from './assets/units/defiler.png';
import firebatImg from './assets/units/firebat.png';
import darktemplarImg from './assets/units/darktemplar.png';
import hightemplarImg from './assets/units/hightemplar.png';
import lurkerImg from './assets/units/lurker.png';
import academyBuildingImg from './assets/buildings/academy.webp';
import arbiterTribunalBuildingImg from './assets/buildings/arbitertribunal.webp';
import armoryBuildingImg from './assets/buildings/armory.webp';
import assimilatorBuildingImg from './assets/buildings/assimilator.webp';
import barracksBuildingImg from './assets/buildings/barracks.webp';
import bunkerBuildingImg from './assets/buildings/bunker.webp';
import citadelOfAdunBuildingImg from './assets/buildings/citadelofadun.webp';
import comsatBuildingImg from './assets/buildings/comsat.webp';
import commandCenterBuildingImg from './assets/buildings/commandcenter.webp';
import controlTowerBuildingImg from './assets/buildings/controltower.webp';
import covertOpsBuildingImg from './assets/buildings/covertops.webp';
import creepColonyBuildingImg from './assets/buildings/creepcolony.webp';
import cyberneticsCoreBuildingImg from './assets/buildings/cyberneticscore.webp';
import defilerMoundBuildingImg from './assets/buildings/defilermound.webp';
import engineeringBayBuildingImg from './assets/buildings/engineeringbay.webp';
import evolutionChamberBuildingImg from './assets/buildings/evolutionchamber.webp';
import extractorBuildingImg from './assets/buildings/extractor.webp';
import factoryBuildingImg from './assets/buildings/factory.webp';
import fleetBeaconBuildingImg from './assets/buildings/fleetbeacon.webp';
import forgeBuildingImg from './assets/buildings/forge.webp';
import gatewayBuildingImg from './assets/buildings/gateway.webp';
import greaterSpireBuildingImg from './assets/buildings/greaterspire.webp';
import hatcheryBuildingImg from './assets/buildings/hatchery.webp';
import hiveBuildingImg from './assets/buildings/hive.webp';
import hydraliskDenBuildingImg from './assets/buildings/hydraliskden.webp';
import infestedCcBuildingImg from './assets/buildings/infestedcc.webp';
import lairBuildingImg from './assets/buildings/lair.webp';
import machineShopBuildingImg from './assets/buildings/machineshop.webp';
import missileTurretBuildingImg from './assets/buildings/missileturret.webp';
import nexusBuildingImg from './assets/buildings/nexus.webp';
import nydusCanalBuildingImg from './assets/buildings/nyduscanal.webp';
import observatoryBuildingImg from './assets/buildings/observatory.webp';
import photonCannonBuildingImg from './assets/buildings/photoncannon.webp';
import physicsLabBuildingImg from './assets/buildings/physicslab.webp';
import pylonBuildingImg from './assets/buildings/pylon.webp';
import queensNestBuildingImg from './assets/buildings/queensnest.webp';
import refineryBuildingImg from './assets/buildings/refinery.webp';
import roboticsFacilityBuildingImg from './assets/buildings/roboticsfacility.webp';
import roboticsSupportBayBuildingImg from './assets/buildings/roboticssupportbay.webp';
import scienceFacilityBuildingImg from './assets/buildings/sciencefacility.webp';
import shieldBatteryBuildingImg from './assets/buildings/shieldbattery.webp';
import spawningPoolBuildingImg from './assets/buildings/spawningpool.webp';
import spireBuildingImg from './assets/buildings/spire.webp';
import sporeColonyBuildingImg from './assets/buildings/sporecolony.webp';
import stargateBuildingImg from './assets/buildings/stargate.webp';
import starportBuildingImg from './assets/buildings/starport.webp';
import sunkenColonyBuildingImg from './assets/buildings/sunkencolony.webp';
import supplyDepotBuildingImg from './assets/buildings/supplydepot.webp';
import templarArchivesBuildingImg from './assets/buildings/templararchives.webp';
import ultraliskCavernBuildingImg from './assets/buildings/ultraliskcavern.webp';
import './styles.css';

// Helper functions for localStorage
const getStoredVariableValues = (dashboardUrl) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : null;
  } catch (e) {
    console.error('Failed to load variable values from localStorage:', e);
    return null;
  }
};

const saveVariableValues = (dashboardUrl, values) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    localStorage.setItem(key, JSON.stringify(values));
  } catch (e) {
    console.error('Failed to save variable values to localStorage:', e);
  }
};

const AUTO_INGEST_SETTINGS_KEY = 'dashboard_auto_ingest_settings';

const getStoredAutoIngestSettings = () => {
  try {
    const stored = localStorage.getItem(AUTO_INGEST_SETTINGS_KEY);
    if (!stored) {
      return { enabled: false, intervalSeconds: 60 };
    }
    const parsed = JSON.parse(stored);
    const interval = Number.isFinite(parsed?.intervalSeconds) && parsed.intervalSeconds >= 60
      ? Math.floor(parsed.intervalSeconds)
      : 60;
    return {
      enabled: parsed?.enabled !== false,
      intervalSeconds: interval,
    };
  } catch (e) {
    console.error('Failed to load auto-ingest settings from localStorage:', e);
    return { enabled: false, intervalSeconds: 60 };
  }
};

const saveAutoIngestSettings = (settings) => {
  try {
    localStorage.setItem(AUTO_INGEST_SETTINGS_KEY, JSON.stringify(settings));
  } catch (e) {
    console.error('Failed to save auto-ingest settings to localStorage:', e);
  }
};

const formatDuration = (seconds) => {
  const total = Math.max(0, Math.floor(Number(seconds) || 0));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
};

const formatRelativeReplayDate = (value) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.floor((startOfToday.getTime() - startOfDate.getTime()) / 86400000);

  let dayLabel = '';
  if (diffDays === 0) dayLabel = 'Today';
  else if (diffDays === 1) dayLabel = 'Yesterday';
  else if (diffDays > 1) dayLabel = `${diffDays} days ago`;
  else dayLabel = date.toLocaleDateString();

  const hours = date.getHours();
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const hour12 = hours % 12 || 12;
  const ampm = hours >= 12 ? 'pm' : 'am';
  return `${dayLabel} at ${hour12}.${minutes}${ampm}`;
};

const getRaceIcon = (race) => {
  const value = String(race || '').toLowerCase();
  if (value === 'protoss') return probeImg;
  if (value === 'terran') return scvImg;
  if (value === 'zerg') return droneImg;
  return null;
};

const raceRank = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return 0;
  if (value === 'zerg') return 1;
  if (value === 'protoss') return 2;
  return 3;
};

const getGasMarkerIconForRace = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return refineryBuildingImg;
  if (value === 'zerg') return extractorBuildingImg;
  if (value === 'protoss') return assimilatorBuildingImg;
  return extractorBuildingImg;
};

const getExpansionMarkerIconForRace = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return commandCenterBuildingImg;
  if (value === 'zerg') return hatcheryBuildingImg;
  if (value === 'protoss') return nexusBuildingImg;
  return null;
};

const normalizeTimingDisplayLabel = (label) => {
  const text = String(label || '').trim();
  const match = text.match(/\(([^)]+)\)/);
  if (match && match[1]) return match[1].trim();
  return text;
};

const INLINE_UPGRADE_LABEL_MAP = {
  'Protoss Air Armor': 'Air Armor',
  'Protoss Air Weapons': 'Air ⚔️',
  'Protoss Ground Armor': 'Grnd Armor',
  'Protoss Ground Weapons': 'Grnd ⚔️',
  'Protoss Plasma Shields': 'Shields',
  'Terran Ship Weapons': 'Ship ⚔️',
  'Terran Vehicle Plating': 'Vehicle 🛡️',
  'Terran Vehicle Weapons': 'Vehicle ⚔️',
  'Zerg Carapace': '🛡️',
  'Zerg Flyer Attacks': '🦋 ⚔️',
  'Zerg Melee Attacks': 'Melee ⚔️',
  'Zerg Missile Attacks': 'Missile ⚔️',
};

const inlineTimingUpgradeLabel = (label, order) => {
  const base = String(label || '').trim();
  const abbreviated = INLINE_UPGRADE_LABEL_MAP[base];
  if (!abbreviated) return normalizeTimingDisplayLabel(base);
  const level = Math.max(1, Number(order) || 1);
  return `${abbreviated} +${level}`;
};

const HP_UPGRADE_NAMES = new Set([
  'Terran Infantry Armor',
  'Terran Vehicle Plating',
  'Terran Ship Plating',
  'Zerg Carapace',
  'Zerg Flyer Carapace',
  'Protoss Ground Armor',
  'Protoss Air Armor',
  'Terran Infantry Weapons',
  'Terran Vehicle Weapons',
  'Terran Ship Weapons',
  'Zerg Melee Attacks',
  'Zerg Missile Attacks',
  'Zerg Flyer Attacks',
  'Protoss Ground Weapons',
  'Protoss Air Weapons',
  'Protoss Plasma Shields',
]);

const UNIT_RANGE_UPGRADE_NAMES = new Set([
  'U-238 Shells (Marine Range)',
  'Ocular Implants (Ghost Sight)',
  'Antennae (Overlord Sight)',
  'Grooved Spines (Hydralisk Range)',
  'Singularity Charge (Dragoon Range)',
  'Sensor Array (Observer Sight)',
  'Charon Boosters (Goliath Range)',
  'Apial Sensors (Scout Sight)',
]);

const UNIT_SPEED_UPGRADE_NAMES = new Set([
  'Ion Thrusters (Vulture Speed)',
  'Pneumatized Carapace (Overlord Speed)',
  'Metabolic Boost (Zergling Speed)',
  'Muscular Augments (Hydralisk Speed)',
  'Leg Enhancement (Zealot Speed)',
  'Gravitic Drive (Shuttle Speed)',
  'Gravitic Booster (Observer Speed)',
  'Gravitic Thrusters (Scout Speed)',
  'Anabolic Synthesis (Ultralisk Speed)',
]);

const ENERGY_UPGRADE_NAMES = new Set([
  'Titan Reactor (Science Vessel Energy)',
  'Moebius Reactor (Ghost Energy)',
  'Apollo Reactor (Wraith Energy)',
  'Colossus Reactor (Battle Cruiser Energy)',
  'Gamete Meiosis (Queen Energy)',
  'Defiler Energy',
  'Khaydarin Core (Arbiter Energy)',
  'Argus Jewel (Corsair Energy)',
  'Khaydarin Amulet (Templar Energy)',
  'Argus Talisman (Dark Archon Energy)',
  'Caduceus Reactor (Medic Energy)',
]);

const CAPACITY_COOLDOWN_DAMAGE_UPGRADE_NAMES = new Set([
  'Scarab Damage',
  'Reaver Capacity',
  'Carrier Capacity',
  'Chitinous Plating (Ultralisk Armor)',
  'Adrenal Glands (Zergling Attack)',
  'Ventral Sacs (Overlord Transport)',
]);

const upgradeCategoryForName = (upgradeName) => {
  const value = String(upgradeName || '').trim();
  if (HP_UPGRADE_NAMES.has(value)) return 'hp_upgrades';
  if (UNIT_RANGE_UPGRADE_NAMES.has(value)) return 'unit_range';
  if (UNIT_SPEED_UPGRADE_NAMES.has(value)) return 'unit_speed';
  if (ENERGY_UPGRADE_NAMES.has(value)) return 'energy';
  if (CAPACITY_COOLDOWN_DAMAGE_UPGRADE_NAMES.has(value)) return 'capacity_cooldown_damage';
  return 'capacity_cooldown_damage';
};

const TIMING_CATEGORY_CONFIG = [
  { id: 'gas', label: 'Gas', title: 'Gas timings (1st-4th)', source: 'gas', markerMode: 'image', markerLabel: 'Gas structure' },
  { id: 'expansion', label: 'Expansion', title: 'Expansion timings (1st-4th)', source: 'expansion', markerMode: 'image', markerLabel: 'Expansion' },
  { id: 'hp_upgrades', label: 'HP Upgrades', title: 'HP upgrades timings', source: 'upgrades' },
  { id: 'unit_range', label: 'Unit Range', title: 'Unit range upgrades timings', source: 'upgrades' },
  { id: 'unit_speed', label: 'Unit Speed', title: 'Unit speed upgrades timings', source: 'upgrades' },
  { id: 'energy', label: 'Energy', title: 'Energy upgrades timings', source: 'upgrades' },
  { id: 'capacity_cooldown_damage', label: 'Capacity/Cooldown/Damage', title: 'Capacity, cooldown and damage upgrades timings', source: 'upgrades' },
  { id: 'tech', label: 'Tech', title: 'Tech research timings', source: 'tech' },
];

const TIMING_RACE_ORDER = ['terran', 'zerg', 'protoss'];

const prettyRaceName = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return 'Terran';
  if (value === 'zerg') return 'Zerg';
  if (value === 'protoss') return 'Protoss';
  return race || 'Unknown';
};

const UNIT_ICON_MAP = {
  probe: probeImg,
  scv: scvImg,
  drone: droneImg,
  arbiter: arbiterImg,
  protossarbiter: arbiterImg,
  scout: scoutImg,
  protossscout: scoutImg,
  reaver: reaverImg,
  protossreaver: reaverImg,
  overlord: overlordImg,
  zergoverlord: overlordImg,
  scourge: scourgeImg,
  zergscourge: scourgeImg,
  observer: observerImg,
  protossobserver: observerImg,
  carrier: carrierImg,
  battlecruiser: battlecruiserImg,
  terranbattlecruiser: battlecruiserImg,
  dropship: dropshipImg,
  terrandropship: dropshipImg,
  sciencevessel: scienceVesselImg,
  terransciencevessel: scienceVesselImg,
  wraith: wraithImg,
  terranwraith: wraithImg,
  marine: marineImg,
  siegetank: siegetankImg,
  siegetanktankmode: siegetankImg,
  siegetankturrettankmode: siegetankImg,
  terransiegetanksiegemode: siegetankImg,
  siegetankturretsiegemode: siegetankImg,
  zealot: zealotImg,
  dragoon: dragoonImg,
  zergling: zerglingImg,
  hydralisk: hydraliskImg,
  mutalisk: mutaliskImg,
  ultralisk: ultraliskImg,
  goliath: goliathImg,
  vulture: vultureImg,
  medic: medicImg,
  defiler: defilerImg,
  zergdefiler: defilerImg,
  firebat: firebatImg,
  darktemplar: darktemplarImg,
  hightemplar: hightemplarImg,
  lurker: lurkerImg,
  academy: academyBuildingImg,
  arbitertribunal: arbiterTribunalBuildingImg,
  armory: armoryBuildingImg,
  assimilator: assimilatorBuildingImg,
  barracks: barracksBuildingImg,
  bunker: bunkerBuildingImg,
  citadelofadun: citadelOfAdunBuildingImg,
  comsat: comsatBuildingImg,
  commandcenter: commandCenterBuildingImg,
  controltower: controlTowerBuildingImg,
  covertops: covertOpsBuildingImg,
  creepcolony: creepColonyBuildingImg,
  cyberneticscore: cyberneticsCoreBuildingImg,
  defilermound: defilerMoundBuildingImg,
  engineeringbay: engineeringBayBuildingImg,
  evolutionchamber: evolutionChamberBuildingImg,
  extractor: extractorBuildingImg,
  factory: factoryBuildingImg,
  fleetbeacon: fleetBeaconBuildingImg,
  forge: forgeBuildingImg,
  gateway: gatewayBuildingImg,
  greaterspire: greaterSpireBuildingImg,
  hatchery: hatcheryBuildingImg,
  hive: hiveBuildingImg,
  hydraliskden: hydraliskDenBuildingImg,
  infestedcc: infestedCcBuildingImg,
  lair: lairBuildingImg,
  machineshop: machineShopBuildingImg,
  missileturret: missileTurretBuildingImg,
  nexus: nexusBuildingImg,
  nyduscanal: nydusCanalBuildingImg,
  observatory: observatoryBuildingImg,
  photoncannon: photonCannonBuildingImg,
  physicslab: physicsLabBuildingImg,
  pylon: pylonBuildingImg,
  queensnest: queensNestBuildingImg,
  refinery: refineryBuildingImg,
  roboticsfacility: roboticsFacilityBuildingImg,
  roboticssupportbay: roboticsSupportBayBuildingImg,
  sciencefacility: scienceFacilityBuildingImg,
  shieldbattery: shieldBatteryBuildingImg,
  spawningpool: spawningPoolBuildingImg,
  spire: spireBuildingImg,
  sporecolony: sporeColonyBuildingImg,
  stargate: stargateBuildingImg,
  starport: starportBuildingImg,
  sunkencolony: sunkenColonyBuildingImg,
  supplydepot: supplyDepotBuildingImg,
  templararchives: templarArchivesBuildingImg,
  ultraliskcavern: ultraliskCavernBuildingImg,
};

const normalizeUnitName = (value) => String(value || '').toLowerCase().replace(/\s+/g, '').replace(/[^a-z0-9]/g, '');

const getUnitIcon = (unitType) => UNIT_ICON_MAP[normalizeUnitName(unitType)] || null;

const BUILDING_TYPE_KEYS = new Set([
  'academy', 'arbitertribunal', 'armory', 'assimilator', 'barracks', 'bunker', 'citadelofadun', 'comsat', 'commandcenter',
  'controltower', 'covertops', 'creepcolony', 'cyberneticscore', 'defilermound', 'engineeringbay', 'evolutionchamber',
  'extractor', 'factory', 'fleetbeacon', 'forge', 'gateway', 'greaterspire', 'hatchery', 'hive', 'hydraliskden', 'infestedcc',
  'lair', 'machineshop', 'missileturret', 'nexus', 'nyduscanal', 'observatory', 'photoncannon', 'physicslab', 'pylon',
  'queensnest', 'refinery', 'roboticsfacility', 'roboticssupportbay', 'sciencefacility', 'shieldbattery', 'spawningpool', 'spire',
  'sporecolony', 'stargate', 'starport', 'sunkencolony', 'supplydepot', 'templararchives', 'ultraliskcavern',
]);

const WORKER_UNIT_KEYS = new Set(['scv', 'drone', 'probe']);
const SPELLCASTER_UNIT_KEYS = new Set([
  'ghost', 'medic', 'sciencevessel', 'queen', 'defiler', 'hightemplar', 'darkarchon', 'arbiter',
]);

const UNIT_TIER_MAP = {
  scv: 1, drone: 1, probe: 1, marine: 1, firebat: 1, medic: 1, vulture: 1, goliath: 2, ghost: 2, wraith: 2, valkyrie: 2,
  siegetank: 2, siegetanktankmode: 2, siegetankturrettankmode: 2, terransiegetanksiegemode: 2, siegetankturretsiegemode: 2,
  sciencevessel: 2, dropship: 2, battlecruiser: 3,
  zergling: 1, hydralisk: 1, lurker: 2, mutalisk: 2, scourge: 2, queen: 2, defiler: 2, guardian: 3, devourer: 3, ultralisk: 3,
  zealot: 1, dragoon: 1, darktemplar: 2, hightemplar: 2, reaver: 2, shuttle: 2, observer: 2, corsair: 2, scout: 2, archon: 3, arbiter: 3, carrier: 3,
};

const BUILDING_TIER_MAP = {
  commandcenter: 1, supplydepot: 1, barracks: 1, refinery: 1, engineeringbay: 1, missileturret: 1, bunker: 1, academy: 1,
  factory: 2, armory: 2, starport: 2, comsat: 2, machineshop: 2, controltower: 2, sciencefacility: 2, physicslab: 3, covertops: 3,
  nexus: 1, pylon: 1, gateway: 1, assimilator: 1, forge: 1, photoncannon: 1, cyberneticscore: 1, shieldbattery: 1,
  roboticsfacility: 2, citadelofadun: 2, stargate: 2, observatory: 2, roboticssupportbay: 2, templararchives: 2, fleetbeacon: 3, arbitertribunal: 3,
  hatchery: 1, spawningpool: 1, extractor: 1, evolutionchamber: 1, creepcolony: 1, hydraliskden: 1, lair: 2, sporecolony: 2, sunkencolony: 2,
  nyduscanal: 2, queensnest: 2, hive: 3, spire: 2, greaterspire: 3, ultraliskcavern: 3, defilermound: 3, infestedcc: 3,
};
const DEFENSIVE_BUILDING_KEYS = new Set([
  'photoncannon',
  'sporecolony',
  'sunkencolony',
  'creepcolony',
  'missileturret',
]);

const formatPercent = (value) => `${((Number(value) || 0) * 100).toFixed(1)}%`;

const DEFAULT_SUMMARY_FILTERS = {
  search: '',
  player: '',
  location: '',
  nuke: false,
  drop: false,
  recall: false,
  becameRace: false,
  rush: false,
};

const SUMMARY_TOPIC_PATTERNS = {
  nuke: /\bnuke|nuclear\b/i,
  drop: /\bdrop|dropship|shuttle\b/i,
  recall: /\brecall\b/i,
  becameRace: /\b(became|becomes)\s+(terran|zerg)\b|\bbecame_(terran|zerg)\b/i,
  rush: /\brush|all[\s-]?in|cheese\b/i,
};

const LOCATION_HINTS = [
  { key: 'expa', matcher: /\bexpa|expansion|expand\b/i },
  { key: 'main', matcher: /\bmain\b/i },
  { key: 'natural', matcher: /\bnatural\b/i },
  { key: 'third', matcher: /\bthird\b/i },
  { key: 'fourth', matcher: /\bfourth\b/i },
  { key: 'center', matcher: /\bcenter|middle\b/i },
  { key: 'top', matcher: /\btop|north\b/i },
  { key: 'bottom', matcher: /\bbottom|south\b/i },
  { key: 'left', matcher: /\bleft|west\b/i },
  { key: 'right', matcher: /\bright|east\b/i },
];

const extractEventLocationTags = (description) => {
  const tags = new Set();
  const text = String(description || '').toLowerCase();
  LOCATION_HINTS.forEach((hint) => {
    if (hint.matcher.test(text)) tags.add(hint.key);
  });
  const strictClockMatches = text.matchAll(/\b([1-9]|1[0-2])\s*o'?clock\b/g);
  for (const match of strictClockMatches) {
    tags.add(match[1]);
  }
  const directionalClockMatches = text.matchAll(/\b(?:at|to|near|towards|from)\s+([1-9]|1[0-2])\b/g);
  for (const match of directionalClockMatches) {
    tags.add(match[1]);
  }
  return Array.from(tags);
};

const extractLocationOptions = (events) => {
  const found = new Set();
  (events || []).forEach((event) => {
    extractEventLocationTags(event?.description).forEach((tag) => found.add(tag));
  });
  return Array.from(found).sort((a, b) => {
    const numA = Number(a);
    const numB = Number(b);
    if (Number.isFinite(numA) && Number.isFinite(numB)) return numA - numB;
    if (Number.isFinite(numA)) return -1;
    if (Number.isFinite(numB)) return 1;
    return a.localeCompare(b);
  });
};

const isPatternTruthy = (value) => {
  const normalized = String(value || '').trim().toLowerCase();
  return normalized === 'yes' || normalized === 'true';
};

const prettyPatternName = (patternName) => {
  const trimmed = String(patternName || '').trim();
  if (!trimmed) return '';
  const splitUppercase = trimmed.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
  return splitUppercase
    .replace(/_/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase());
};

const patternIconForName = (patternName) => {
  const normalized = normalizeUnitName(patternName);
  if (normalized.includes('battlecruiser')) return battlecruiserImg;
  if (normalized.includes('carrier')) return carrierImg;
  return getUnitIcon(patternName);
};

const minuteFromValue = (value) => {
  const trimmed = String(value || '').trim();
  const clockMatch = trimmed.match(/^(\d+):(\d{2})$/);
  if (clockMatch) return Number(clockMatch[1]);
  const asNumber = Number(trimmed);
  if (Number.isFinite(asNumber)) return Math.floor(asNumber / 60);
  return null;
};

const formatPatternPillText = (rawName, rawValue, isTruthy) => {
  if (isTruthy) {
    if (rawName.toLowerCase() === 'never researched') return 'Never Researched';
    return `Did ${rawName}`;
  }
  const lowerName = rawName.toLowerCase();
  if (lowerName === 'became terran' || lowerName === 'became zerg') {
    const minute = minuteFromValue(rawValue);
    if (minute !== null) return `${rawName} at ${minute} mins`;
  }
  if (lowerName.includes('used hotkey groups')) {
    return `${rawName.replace(/\s+at$/i, '')} ${rawValue}`;
  }
  if (lowerName.includes('made drops') || lowerName.includes('made recalls')) {
    const minute = minuteFromValue(rawValue);
    if (minute !== null) return `${rawName} at min ${minute}`;
  }
  return `${rawName} at ${rawValue}`;
};

const renderPatternPill = (pattern, keyPrefix, team) => {
  const rawName = prettyPatternName(pattern?.pattern_name);
  if (!rawName) return null;
  const rawValue = String(pattern?.value || '').trim();
  if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
    return null;
  }
  const isTruthy = isPatternTruthy(pattern?.value);
  const normalizedPatternName = normalizeUnitName(pattern?.pattern_name);
  let icon = patternIconForName(pattern?.pattern_name);
  const text = formatPatternPillText(rawName, rawValue, isTruthy);
  let content = <span>{text}</span>;
  if (isTruthy) {
    if (normalizedPatternName === 'quickfactory') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          <span>Quick</span>
          {getUnitIcon('factory') ? <img src={getUnitIcon('factory')} alt="Factory" className="workflow-pattern-icon" /> : null}
        </span>
      );
    } else if (normalizedPatternName === 'gatethenforge') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          {getUnitIcon('gateway') ? <img src={getUnitIcon('gateway')} alt="Gateway" className="workflow-pattern-icon" /> : null}
          <span className="workflow-pattern-arrow">then</span>
          {getUnitIcon('forge') ? <img src={getUnitIcon('forge')} alt="Forge" className="workflow-pattern-icon" /> : null}
        </span>
      );
    } else if (normalizedPatternName === 'forgethengate') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          {getUnitIcon('forge') ? <img src={getUnitIcon('forge')} alt="Forge" className="workflow-pattern-icon" /> : null}
          <span className="workflow-pattern-arrow">then</span>
          {getUnitIcon('gateway') ? <img src={getUnitIcon('gateway')} alt="Gateway" className="workflow-pattern-icon" /> : null}
        </span>
      );
    } else if (normalizedPatternName === 'hatchbeforepool') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          {getUnitIcon('hatchery') ? <img src={getUnitIcon('hatchery')} alt="Hatchery" className="workflow-pattern-icon" /> : null}
          <span className="workflow-pattern-arrow">then</span>
          {getUnitIcon('spawningpool') ? <img src={getUnitIcon('spawningpool')} alt="Spawning Pool" className="workflow-pattern-icon" /> : null}
        </span>
      );
    } else if (normalizedPatternName === 'mech') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          {getUnitIcon('siegetank') ? <img src={getUnitIcon('siegetank')} alt="Tank" className="workflow-pattern-icon" /> : null}
          <span>Mech</span>
        </span>
      );
    } else if (normalizedPatternName === 'carriers' || normalizedPatternName === 'battlecruisers') {
      content = <span>x10+</span>;
    } else if (icon) {
      content = <span>Did</span>;
    }
  }
  const key = `${keyPrefix}-${team ? `team-${team}-` : ''}${pattern?.pattern_name}-${pattern?.value}`;
  return (
    <span key={key} className={`workflow-pattern-pill${isTruthy ? ' workflow-pattern-pill-strong' : ''}`}>
      {team !== undefined ? <span className="team-dot" style={{ backgroundColor: getTeamColor(team) }}></span> : null}
      {icon ? <img src={icon} alt={rawName} className="workflow-pattern-icon" /> : null}
      {content}
    </span>
  );
};

const formatSigned = (value) => {
  const n = Number(value) || 0;
  if (n > 0) return `+${n.toFixed(2)}`;
  return n.toFixed(2);
};

const FINGERPRINT_METRIC_HELP = {
  'Games using hotkeys (%)': 'Per game, this is 100% when at least one hotkey is used, then averaged across all games. Games with missing command streams may appear as 0%.',
  'Games with queued orders (%)': 'Share of games where at least one queued command exists. This depends on queued flags in command logs.',
  'Assigned hotkey / Used Hotkey actions': 'Per game, Assign hotkey actions divided by Select hotkey actions, expressed as a percentage and averaged across games.',
  'Hotkey commands as % of total': 'Per game, hotkey commands divided by all commands, expressed as a percentage and averaged across games.',
  'Replays with at least one Rally Point (%)': 'Percentage of this player\'s replays where they issued at least one Rally Point command.',
  'Rally Points per 10 minutes (rally replays)': 'Average number of Rally Point commands per 10 minutes, computed only over replays where at least one was used.',
  'Action type diversity': 'Average per-game breadth of action types plus targeted-order variety. It shows variety, not quality or efficiency.',
};

const metricHelpText = (metricName) => FINGERPRINT_METRIC_HELP[metricName] || 'Computed from this player\'s replay command data. Sparse or noisy command streams can skew values.';
const PLAYER_OUTLIER_HELP = [
  'Baselines are computed against human, non-observer players of the same primary race only.',
  'For Protoss players, non-Protoss techs/upgrades and non-Protoss cast orders are excluded to avoid mind-control leakage.',
  'Techs/Upgrades use "used at least once in a game" rates. Targeted Orders use share of total order instances (not replay incidence).',
  'An item appears if it passes either threshold: "Rare signature" (TF-IDF) or "Much more frequent than peers" (ratio vs baseline).',
].join(' ');

const OUTLIER_CATEGORY_SUBTITLE = {
  'Tech researched': 'Rate = games with this tech at least once / total same-race games.',
  'Upgrades researched': 'Rate = games with this upgrade at least once / total same-race games.',
  'Targeted orders': 'Rate = this order count / all targeted-order counts for that race (raw instances).',
};

const HelpTooltip = ({ text, label }) => (
  <span className="workflow-help-wrap" aria-label={label || 'Explanation'}>
    <span className="workflow-metric-help">ⓘ</span>
    <span className="workflow-help-bubble">{text}</span>
  </span>
);

const outlierQualifierClassName = (qualifier) => {
  const normalized = String(qualifier || '').toLowerCase();
  if (normalized.includes('rare signature')) return 'workflow-outlier-pill workflow-outlier-pill-rare';
  if (normalized.includes('much more frequent than peers')) return 'workflow-outlier-pill workflow-outlier-pill-frequent';
  return 'workflow-outlier-pill';
};

const prettyMetricValue = (metric) => {
  const value = Number(metric?.player_value) || 0;
  if (String(metric?.metric || '').toLowerCase().includes('%')) {
    if (Math.abs(value) <= 1) return formatPercent(value);
    return `${value.toFixed(1)}%`;
  }
  if (String(metric?.metric || '').toLowerCase().includes('seconds')) {
    return formatDuration(value);
  }
  return value.toFixed(2);
};

const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

const getTeamColor = (team) => {
  const n = Number(team) || 0;
  return TEAM_COLORS[Math.abs(n) % TEAM_COLORS.length];
};

const teamColorRgba = (team, alpha = 0.14) => {
  const hex = getTeamColor(team).replace('#', '');
  const expanded = hex.length === 3 ? hex.split('').map((c) => `${c}${c}`).join('') : hex;
  const r = parseInt(expanded.slice(0, 2), 16);
  const g = parseInt(expanded.slice(2, 4), 16);
  const b = parseInt(expanded.slice(4, 6), 16);
  return `rgba(${Number.isNaN(r) ? 96 : r}, ${Number.isNaN(g) ? 165 : g}, ${Number.isNaN(b) ? 250 : b}, ${alpha})`;
};

const WORKFLOW_GAMES_PAGE_SIZE = 30;

const toggleFilterValue = (values, value) => {
  const normalized = String(value || '').trim();
  if (!normalized) return values;
  if (values.includes(normalized)) {
    return values.filter((item) => item !== normalized);
  }
  return [...values, normalized];
};

const teamGroupsFromPlayers = (players) => {
  const groups = [];
  const byTeam = new Map();
  (players || []).forEach((player) => {
    const team = Number(player?.team || 0);
    if (!byTeam.has(team)) {
      byTeam.set(team, []);
      groups.push(byTeam.get(team));
    }
    byTeam.get(team).push(player);
  });
  return groups;
};

function App() {
  const storedAutoIngest = getStoredAutoIngestSettings();
  const [currentDashboardUrl, setCurrentDashboardUrl] = useState('default');
  const [dashboard, setDashboard] = useState(null);
  const [dashboards, setDashboards] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showDashboardManager, setShowDashboardManager] = useState(false);
  const [showEditDashboard, setShowEditDashboard] = useState(false);
  const [newWidgetPrompt, setNewWidgetPrompt] = useState('');
  const [creatingWidget, setCreatingWidget] = useState(false);
  const [variableValues, setVariableValues] = useState({});
  const [openaiEnabled, setOpenaiEnabled] = useState(false);
  const [editingWidget, setEditingWidget] = useState(null);
  const [replayCount, setReplayCount] = useState(null);
  const [showIngestPanel, setShowIngestPanel] = useState(false);
  const [ingestMessage, setIngestMessage] = useState('');
  const [ingestForm, setIngestForm] = useState({
    watch: false,
    stopAfterN: 50,
    clean: false,
    autoIngestEnabled: storedAutoIngest.enabled,
    autoIngestIntervalSeconds: storedAutoIngest.intervalSeconds,
  });
  const autoIngestInFlight = useRef(false);
  const [activeView, setActiveView] = useState('games');
  const workflowViewHistoryRef = useRef([]);
  const [workflowGames, setWorkflowGames] = useState([]);
  const [workflowGamesLoading, setWorkflowGamesLoading] = useState(false);
  const [workflowGamesPage, setWorkflowGamesPage] = useState(1);
  const [workflowGamesTotal, setWorkflowGamesTotal] = useState(0);
  const [workflowGamesFilterOptions, setWorkflowGamesFilterOptions] = useState({
    players: [],
    maps: [],
    durations: [],
    featuring: [],
  });
  const [workflowGamesFilters, setWorkflowGamesFilters] = useState({
    player: [],
    map: [],
    duration: [],
    featuring: [],
  });
  const [workflowGameDetailLoading, setWorkflowGameDetailLoading] = useState(false);
  const [workflowPlayerLoading, setWorkflowPlayerLoading] = useState(false);
  const [selectedReplayId, setSelectedReplayId] = useState(null);
  const [selectedPlayerKey, setSelectedPlayerKey] = useState('');
  const [workflowGame, setWorkflowGame] = useState(null);
  const [workflowGameTab, setWorkflowGameTab] = useState('summary');
  const [workflowPlayer, setWorkflowPlayer] = useState(null);
  const [workflowPlayerMetrics, setWorkflowPlayerMetrics] = useState(null);
  const [workflowPlayerMetricsLoading, setWorkflowPlayerMetricsLoading] = useState(false);
  const [workflowPlayerMetricsError, setWorkflowPlayerMetricsError] = useState('');
  const [workflowPlayerOutliers, setWorkflowPlayerOutliers] = useState(null);
  const [workflowPlayerOutliersLoading, setWorkflowPlayerOutliersLoading] = useState(false);
  const [workflowPlayerOutliersError, setWorkflowPlayerOutliersError] = useState('');
  const [workflowQuestion, setWorkflowQuestion] = useState('');
  const [workflowAnswer, setWorkflowAnswer] = useState(null);
  const [askingWorkflow, setAskingWorkflow] = useState(false);
  const [topPlayerColors, setTopPlayerColors] = useState({});
  const [workflowSummaryFilters, setWorkflowSummaryFilters] = useState(DEFAULT_SUMMARY_FILTERS);
  const [workflowProductionTab, setWorkflowProductionTab] = useState('units');
  const [workflowUnitFilterMode, setWorkflowUnitFilterMode] = useState('all');
  const [workflowUnitNameFilter, setWorkflowUnitNameFilter] = useState('');
  const [workflowBuildingFilterMode, setWorkflowBuildingFilterMode] = useState('all');
  const [workflowBuildingNameFilter, setWorkflowBuildingNameFilter] = useState('');
  const [workflowTimingCategory, setWorkflowTimingCategory] = useState('gas');
  const [workflowHpUpgradeFilters, setWorkflowHpUpgradeFilters] = useState({
    terran: '',
    zerg: '',
    protoss: '',
  });

  const loadDashboard = async (url, varValues = null, skipVarInit = false) => {
    try {
      setLoading(true);
      setError(null);

      // If no varValues provided, try to load from localStorage
      if (!varValues) {
        const stored = getStoredVariableValues(url);
        if (stored && Object.keys(stored).length > 0) {
          varValues = stored;
        }
      }

      const data = await api.getDashboard(url, varValues);
      setDashboard(data);
      setCurrentDashboardUrl(url);

      // Update variable values state
      if (varValues) {
        setVariableValues(varValues);
        // Save to localStorage
        saveVariableValues(url, varValues);
      } else if (data.variables && !skipVarInit) {
        // Initialize variable values with first option if not set
        const newVarValues = {};
        let needsReload = false;
        Object.keys(data.variables).forEach(varName => {
          if (data.variables[varName].possible_values?.length > 0) {
            newVarValues[varName] = data.variables[varName].possible_values[0];
            needsReload = true;
          }
        });
        if (needsReload && Object.keys(newVarValues).length > 0) {
          setVariableValues(newVarValues);
          // Save to localStorage
          saveVariableValues(url, newVarValues);
          // Reload with initialized values
          await loadDashboard(url, newVarValues, true);
          return;
        }
        setVariableValues(newVarValues);
        // Save to localStorage
        saveVariableValues(url, newVarValues);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const loadDashboards = async () => {
    try {
      const data = await api.listDashboards();
      setDashboards(data);
    } catch (err) {
      console.error('Failed to load dashboards:', err);
    }
  };

  const loadWorkflowGames = async ({ page = workflowGamesPage, filters = workflowGamesFilters } = {}) => {
    try {
      setWorkflowGamesLoading(true);
      const safePage = Math.max(1, Number(page) || 1);
      const offset = (safePage - 1) * WORKFLOW_GAMES_PAGE_SIZE;
      const data = await api.listWorkflowGames({
        limit: WORKFLOW_GAMES_PAGE_SIZE,
        offset,
        filters,
      });
      const items = data?.items || [];
      setWorkflowGames(items);
      setWorkflowGamesTotal(Number(data?.total) || 0);
      if (data?.filter_options) {
        setWorkflowGamesFilterOptions(data.filter_options);
      }
      if (!selectedReplayId && items.length > 0) {
        setSelectedReplayId(items[0].replay_id);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowGamesLoading(false);
    }
  };

  const loadTopPlayerColors = async () => {
    try {
      const data = await api.getWorkflowPlayerColors();
      setTopPlayerColors(data?.player_colors || {});
    } catch (err) {
      console.error('Failed to load top player colors:', err);
    }
  };

  const openWorkflowGame = async (replayId) => {
    try {
      setWorkflowGameDetailLoading(true);
      setError(null);
      const data = await api.getWorkflowGame(replayId);
      setWorkflowGame(data);
      setWorkflowGameTab('summary');
      setSelectedReplayId(replayId);
      setWorkflowAnswer(null);
      setWorkflowQuestion('');
      setWorkflowSummaryFilters(DEFAULT_SUMMARY_FILTERS);
      setWorkflowProductionTab('units');
      setWorkflowUnitFilterMode('all');
      setWorkflowUnitNameFilter('');
      setWorkflowBuildingFilterMode('all');
      setWorkflowBuildingNameFilter('');
      setWorkflowTimingCategory('gas');
      setWorkflowHpUpgradeFilters({ terran: '', zerg: '', protoss: '' });
      navigateWorkflowView('game');
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowGameDetailLoading(false);
    }
  };

  const openWorkflowPlayer = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    const loadPlayerMetrics = async () => {
      if (!normalizedPlayerKey) return;
      try {
        setWorkflowPlayerMetricsLoading(true);
        setWorkflowPlayerMetricsError('');
        const metricsData = await api.getWorkflowPlayerMetrics(normalizedPlayerKey);
        setWorkflowPlayerMetrics(metricsData);
      } catch (err) {
        setWorkflowPlayerMetricsError(err.message || 'Failed to load metrics');
        setWorkflowPlayerMetrics(null);
      } finally {
        setWorkflowPlayerMetricsLoading(false);
      }
    };
    const loadPlayerOutliers = async () => {
      if (!normalizedPlayerKey) return;
      try {
        setWorkflowPlayerOutliersLoading(true);
        setWorkflowPlayerOutliersError('');
        const outlierData = await api.getWorkflowPlayerOutliers(normalizedPlayerKey);
        setWorkflowPlayerOutliers(outlierData);
      } catch (err) {
        setWorkflowPlayerOutliersError(err.message || 'Failed to load outliers');
        setWorkflowPlayerOutliers(null);
      } finally {
        setWorkflowPlayerOutliersLoading(false);
      }
    };
    try {
      setWorkflowPlayerLoading(true);
      setError(null);
      const data = await api.getWorkflowPlayer(playerKey);
      setWorkflowPlayer(data);
      setWorkflowPlayerMetrics(null);
      setWorkflowPlayerMetricsError('');
      setWorkflowPlayerMetricsLoading(false);
      setWorkflowPlayerOutliers(null);
      setWorkflowPlayerOutliersError('');
      setWorkflowPlayerOutliersLoading(false);
      setSelectedPlayerKey(playerKey);
      setWorkflowAnswer(null);
      setWorkflowQuestion('');
      navigateWorkflowView('player');
      loadPlayerMetrics();
      loadPlayerOutliers();
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowPlayerLoading(false);
    }
  };

  useEffect(() => {
    // Load dashboard with stored variable values if available
    const stored = getStoredVariableValues('default');
    loadDashboard('default', stored || undefined);
    loadDashboards();
    loadTopPlayerColors();
    checkOpenAIStatus();
  }, []);

  useEffect(() => {
    loadWorkflowGames({ page: workflowGamesPage, filters: workflowGamesFilters });
  }, [workflowGamesPage, workflowGamesFilters]);

  useEffect(() => {
    saveAutoIngestSettings({
      enabled: ingestForm.autoIngestEnabled,
      intervalSeconds: ingestForm.autoIngestIntervalSeconds,
    });
  }, [ingestForm.autoIngestEnabled, ingestForm.autoIngestIntervalSeconds]);

  useEffect(() => {
    if (!ingestForm.autoIngestEnabled) {
      return undefined;
    }

    const intervalSeconds = Math.max(60, Number(ingestForm.autoIngestIntervalSeconds) || 60);
    let cancelled = false;

    const runAutoIngest = async () => {
      if (cancelled || autoIngestInFlight.current) return;
      autoIngestInFlight.current = true;
      try {
        await api.startIngest({
          watch: false,
          stop_after_n_reps: 1,
          clean: false,
        });
        await loadWorkflowGames();
      } catch (err) {
        console.error('Auto-ingest failed:', err);
      } finally {
        autoIngestInFlight.current = false;
      }
    };

    const timer = window.setInterval(runAutoIngest, intervalSeconds * 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [ingestForm.autoIngestEnabled, ingestForm.autoIngestIntervalSeconds]);

  const checkOpenAIStatus = async () => {
    try {
      const response = await fetch('/api/health');
      if (response.ok) {
        const data = await response.json();
        setOpenaiEnabled(data.openai_enabled || false);
        setReplayCount(typeof data.total_replays === 'number' ? data.total_replays : 0);
      }
    } catch (err) {
      console.error('Failed to check OpenAI status:', err);
    }
  };

  const handleCreateWidget = async (e) => {
    e.preventDefault();
    if (!newWidgetPrompt.trim() || creatingWidget) return;

    try {
      setCreatingWidget(true);
      setError(null);
      await api.createWidget(currentDashboardUrl, newWidgetPrompt);
      setNewWidgetPrompt('');
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreatingWidget(false);
    }
  };

  const handleCreateWidgetWithoutPrompt = async () => {
    if (creatingWidget) return;

    try {
      setCreatingWidget(true);
      setError(null);
      const widget = await api.createWidget(currentDashboardUrl, '');
      setCreatingWidget(false);
      // Config should already be parsed as an object from the backend
      const config = widget.config || { type: 'table' };
      // Open the edit widget fullscreen for the newly created widget
      setEditingWidget({
        id: widget.id,
        name: widget.name,
        description: widget.description ? { valid: true, string: widget.description } : null,
        query: widget.query || '',
        config: config,
        results: [],
      });
    } catch (err) {
      setError(err.message);
      setCreatingWidget(false);
    }
  };

  const handleUpdateDashboard = async (data) => {
    try {
      await api.updateDashboard(currentDashboardUrl, data);
      setShowEditDashboard(false);
      await loadDashboard(currentDashboardUrl);
      await loadDashboards();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeleteWidget = async (widgetId) => {
    if (!confirm('Are you sure you want to delete this widget?')) return;

    try {
      await api.deleteWidget(currentDashboardUrl, widgetId);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdateWidget = async (widgetId, data) => {
    if (data.prompt) {
      data = { prompt: data.prompt }
    }
    try {
      await api.updateWidget(currentDashboardUrl, widgetId, data);
      setEditingWidget(null);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleIngestSubmit = async (e) => {
    e.preventDefault();
    setIngestMessage('');
    try {
      await api.startIngest({
        watch: ingestForm.watch,
        stop_after_n_reps: ingestForm.stopAfterN || 0,
        clean: ingestForm.clean,
      });
      setIngestMessage('Ingestion started in the background.');
      await loadWorkflowGames();
      setShowIngestPanel(false);
    } catch (err) {
      setIngestMessage(err.message || 'Failed to start ingestion.');
    }
  };

  const handleSwitchDashboard = (url) => {
    setVariableValues({});
    loadDashboard(url);
  };

  const handleVariableChange = async (varName, value) => {
    const newVarValues = { ...variableValues, [varName]: value };
    setVariableValues(newVarValues);
    // Save to localStorage
    saveVariableValues(currentDashboardUrl, newVarValues);
    await loadDashboard(currentDashboardUrl, newVarValues);
  };

  const setWorkflowGameSingleFilter = (name, nextValue) => {
    setWorkflowGamesPage(1);
    setWorkflowGamesFilters((prev) => ({
      ...prev,
      [name]: nextValue ? [nextValue] : [],
    }));
  };

  const toggleWorkflowGameMultiFilter = (name, value) => {
    setWorkflowGamesPage(1);
    setWorkflowGamesFilters((prev) => ({
      ...prev,
      [name]: toggleFilterValue(prev[name] || [], value),
    }));
  };

  const clearWorkflowGamesFilters = () => {
    setWorkflowGamesPage(1);
    setWorkflowGamesFilters({
      player: [],
      map: [],
      duration: [],
      featuring: [],
    });
  };

  const navigateWorkflowView = (nextView) => {
    setActiveView((currentView) => {
      if (currentView === nextView) return currentView;
      workflowViewHistoryRef.current.push(currentView);
      if (workflowViewHistoryRef.current.length > 30) {
        workflowViewHistoryRef.current.shift();
      }
      return nextView;
    });
  };

  const goBackWorkflowView = () => {
    setActiveView((currentView) => {
      while (workflowViewHistoryRef.current.length > 0) {
        const previous = workflowViewHistoryRef.current.pop();
        if (previous && previous !== currentView) {
          return previous;
        }
      }
      return 'games';
    });
  };

  const handleWorkflowAsk = async (e) => {
    e.preventDefault();
    const question = workflowQuestion.trim();
    if (!question || askingWorkflow) return;
    try {
      setAskingWorkflow(true);
      setWorkflowAnswer(null);
      if (activeView === 'game' && workflowGame?.replay_id) {
        const response = await api.askWorkflowGame(workflowGame.replay_id, question);
        setWorkflowAnswer(response);
      } else if (activeView === 'player' && workflowPlayer?.player_key) {
        const response = await api.askWorkflowPlayer(workflowPlayer.player_key, question);
        setWorkflowAnswer(response);
      }
    } catch (err) {
      setWorkflowAnswer({
        title: 'AI Error',
        description: 'The question could not be answered.',
        config: { type: 'text' },
        text_answer: `Failed to ask AI: ${err.message}`,
        results: [],
        columns: [],
      });
    } finally {
      setAskingWorkflow(false);
    }
  };

  const playerAccentColor = (nameOrKey) => {
    const key = String(nameOrKey || '').trim().toLowerCase();
    return topPlayerColors[key] || '';
  };

  const renderPlayerLabel = (name) => {
    const color = playerAccentColor(name);
    if (!color) return <span>{name}</span>;
    return <span style={{ color, fontWeight: 600 }}>{name}</span>;
  };

  const renderPlayersMatchup = (label) => {
    const sides = String(label || '').split(' vs ');
    return sides.map((side, sideIndex) => (
      <span key={`${side}-${sideIndex}`}>
        {side.split(', ').map((name, idx) => (
          <span key={`${name}-${idx}`}>
            {renderPlayerLabel(name)}
            {idx < side.split(', ').length - 1 ? ', ' : ''}
          </span>
        ))}
        {sideIndex < sides.length - 1 ? ' vs ' : ''}
      </span>
    ));
  };

  const renderWorkflowGameListPlayers = (game) => {
    const players = Array.isArray(game?.players) ? game.players : [];
    if (players.length === 0) {
      return renderPlayersMatchup(game?.players_label || '');
    }
    const groups = teamGroupsFromPlayers(players);
    return groups.map((group, groupIdx) => {
      const hasTeam = group.length > 1;
      return (
        <span key={`team-${groupIdx}`}>
          {hasTeam ? '(' : ''}
          {group.map((player, idx) => (
            <span key={`${player.player_id}-${idx}`}>
              {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
              {renderPlayerLabel(player.name)}
              {idx < group.length - 1 ? ' & ' : ''}
            </span>
          ))}
          {hasTeam ? ')' : ''}
          {groupIdx < groups.length - 1 ? ' vs ' : ''}
        </span>
      );
    });
  };

  const renderWorkflowAiResult = () => {
    if (!workflowAnswer) return null;
    const config = workflowAnswer.config || { type: 'text' };
    const data = workflowAnswer.results || [];
    const columns = workflowAnswer.columns || [];
    const chartProps = { data, config };

    if (config.type === 'text') {
      return (
        <div className="workflow-answer">
          {workflowAnswer.title ? <div className="workflow-answer-title">{workflowAnswer.title}</div> : null}
          <div>{workflowAnswer.text_answer || workflowAnswer.description || 'No text answer returned.'}</div>
        </div>
      );
    }

    let content = null;
    switch (config.type) {
      case 'gauge':
        content = <Gauge {...chartProps} />;
        break;
      case 'table':
        content = <Table {...chartProps} columns={columns} />;
        break;
      case 'pie_chart':
        content = <PieChart {...chartProps} />;
        break;
      case 'bar_chart':
        content = <BarChart {...chartProps} />;
        break;
      case 'line_chart':
        content = <LineChart {...chartProps} />;
        break;
      case 'scatter_plot':
        content = <ScatterPlot {...chartProps} />;
        break;
      case 'histogram':
        content = <Histogram {...chartProps} />;
        break;
      case 'heatmap':
        content = <Heatmap {...chartProps} />;
        break;
      default:
        content = <div className="chart-empty">Unknown AI chart type: {String(config.type || '')}</div>;
        break;
    }

    return (
      <div className="workflow-answer-chart">
        {workflowAnswer.title ? <div className="workflow-answer-title">{workflowAnswer.title}</div> : null}
        {workflowAnswer.description ? <div className="workflow-answer-description">{workflowAnswer.description}</div> : null}
        <div className="workflow-answer-visual">{content}</div>
      </div>
    );
  };

  const sortedWidgets = dashboard?.widgets
    ? [...dashboard.widgets].sort((a, b) => {
      const orderA = a.widget_order?.valid ? a.widget_order.int64 : 0;
      const orderB = b.widget_order?.valid ? b.widget_order.int64 : 0;
      return orderA - orderB;
    })
    : [];

  const workflowLocationOptions = useMemo(
    () => extractLocationOptions(workflowGame?.game_events || []),
    [workflowGame?.game_events],
  );

  const summaryTextMatches = (text) => {
    const value = String(text || '').toLowerCase();
    if (workflowSummaryFilters.search && !value.includes(workflowSummaryFilters.search.toLowerCase())) {
      return false;
    }
    const activeTopics = Object.entries(SUMMARY_TOPIC_PATTERNS)
      .filter(([key]) => workflowSummaryFilters[key])
      .map(([, matcher]) => matcher);
    if (activeTopics.length > 0 && !activeTopics.some((matcher) => matcher.test(value))) {
      return false;
    }
    return true;
  };

  const filteredReplayPatterns = workflowGame?.replay_patterns || [];
  const filteredTeamPatterns = workflowGame?.team_patterns || [];
  const workflowPlayerOutlierGroups = useMemo(() => {
    const grouped = new Map();
    (workflowPlayerOutliers?.items || []).forEach((item) => {
      const key = String(item?.category || 'Other');
      if (!grouped.has(key)) grouped.set(key, []);
      grouped.get(key).push(item);
    });
    return Array.from(grouped.entries());
  }, [workflowPlayerOutliers]);

  const filteredGameEvents = (workflowGame?.game_events || []).filter((event) => {
    if (workflowSummaryFilters.player && !String(event.description || '').toLowerCase().includes(workflowSummaryFilters.player.toLowerCase())) {
      return false;
    }
    if (workflowSummaryFilters.location) {
      const eventTags = extractEventLocationTags(event.description || '');
      if (!eventTags.includes(workflowSummaryFilters.location)) return false;
    }
    return summaryTextMatches(`${event.type} ${event.description}`);
  });

  const workflowPlayers = workflowGame?.players || [];
  const workflowPlayerNameWidthCh = useMemo(() => {
    const longestNameLength = workflowPlayers.reduce((longest, player) => {
      const nameLength = String(player?.name || '').trim().length;
      return Math.max(longest, nameLength);
    }, 0);
    if (!longestNameLength) return 15;
    return Math.max(12, Math.min(24, longestNameLength + 3));
  }, [workflowPlayers]);
  const workflowPlayersById = useMemo(
    () => new Map(workflowPlayers.map((player) => [player.player_id, player])),
    [workflowPlayers],
  );
  const hasTeamInfo = useMemo(() => {
    const uniqueTeams = new Set(workflowPlayers.map((player) => player.team));
    return uniqueTeams.size > 1;
  }, [workflowPlayers]);
  const workflowTimingCategoryConfig = useMemo(
    () => TIMING_CATEGORY_CONFIG.find((cfg) => cfg.id === workflowTimingCategory) || TIMING_CATEGORY_CONFIG[0],
    [workflowTimingCategory],
  );
  const workflowTimingSeries = useMemo(() => {
    const timings = workflowGame?.timings || {};
    const sourceSeries = Array.isArray(timings?.[workflowTimingCategoryConfig.source])
      ? timings[workflowTimingCategoryConfig.source]
      : [];
    const sortedSeries = [...sourceSeries].sort((a, b) => {
      const raceDiff = raceRank(workflowPlayersById.get(a?.player_id)?.race) - raceRank(workflowPlayersById.get(b?.player_id)?.race);
      if (raceDiff !== 0) return raceDiff;
      const nameA = String(a?.name || '').toLowerCase();
      const nameB = String(b?.name || '').toLowerCase();
      if (nameA !== nameB) return nameA.localeCompare(nameB);
      return Number(a?.player_id || 0) - Number(b?.player_id || 0);
    });

    return sortedSeries.map((playerSeries) => {
      const playerRace = String(workflowPlayersById.get(playerSeries?.player_id)?.race || '').trim();
      const sourcePoints = Array.isArray(playerSeries?.points) ? playerSeries.points : [];
      const points = sourcePoints
        .map((point) => {
          const second = Number(point?.second);
          if (!Number.isFinite(second)) return null;
          const order = Number(point?.order) || 0;
          const rawLabel = String(point?.label || '').trim();
          const upgradeCategory = workflowTimingCategoryConfig.source === 'upgrades' ? upgradeCategoryForName(rawLabel) : '';
          if (workflowTimingCategoryConfig.source === 'upgrades' && upgradeCategory !== workflowTimingCategory) return null;
          let displayLabel = rawLabel;
          let categoryLabel = 'Timing';
          let markerImage = null;
          let markerLabel = '';
          let isRepeatable = false;
          let maxLevel = 1;

          if (workflowTimingCategoryConfig.source === 'upgrades') {
            displayLabel = inlineTimingUpgradeLabel(rawLabel, order);
            categoryLabel = workflowTimingCategoryConfig.label;
            isRepeatable = upgradeCategory === 'hp_upgrades';
            maxLevel = isRepeatable ? 3 : 1;
          } else if (workflowTimingCategoryConfig.source === 'tech') {
            displayLabel = normalizeTimingDisplayLabel(rawLabel);
            categoryLabel = 'Tech';
          } else if (workflowTimingCategory === 'gas') {
            displayLabel = `Gas #${order || 1}`;
            categoryLabel = 'Gas';
            markerImage = getGasMarkerIconForRace(playerRace);
            markerLabel = workflowTimingCategoryConfig.markerLabel || 'Gas';
          } else if (workflowTimingCategory === 'expansion') {
            displayLabel = `Expansion #${order || 1}`;
            categoryLabel = 'Expansion';
            markerImage = getExpansionMarkerIconForRace(playerRace);
            markerLabel = workflowTimingCategoryConfig.markerLabel || 'Expansion';
          }

          return {
            ...point,
            second,
            order,
            label: rawLabel,
            display_label: displayLabel,
            category: upgradeCategory || workflowTimingCategory,
            category_label: categoryLabel,
            race: playerRace,
            marker_image: markerImage,
            marker_label: markerLabel,
            is_repeatable: isRepeatable,
            max_level: maxLevel,
          };
        })
        .filter(Boolean);

      return {
        ...playerSeries,
        race: playerRace,
        race_icon: getRaceIcon(playerRace),
        points,
      };
    });
  }, [workflowGame?.timings, workflowTimingCategoryConfig, workflowTimingCategory, workflowPlayersById]);
  const workflowTimingUsesLabelColors = useMemo(
    () => ['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(workflowTimingCategory),
    [workflowTimingCategory],
  );
  const workflowTimingAxisMode = useMemo(
    () => (['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(workflowTimingCategory) ? 'compressed15' : 'linear'),
    [workflowTimingCategory],
  );
  const workflowTimingInlineLegend = useMemo(
    () => ['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(workflowTimingCategory),
    [workflowTimingCategory],
  );
  const workflowTimingAxisTrimMaxSecond = useMemo(() => {
    if (!['gas', 'expansion'].includes(workflowTimingCategory)) return undefined;
    const maxPointSecond = workflowTimingSeries.reduce((maxSecond, playerSeries) => {
      const playerMax = (playerSeries?.points || []).reduce((innerMax, point) => {
        const second = Number(point?.second);
        return Number.isFinite(second) ? Math.max(innerMax, second) : innerMax;
      }, 0);
      return Math.max(maxSecond, playerMax);
    }, 0);
    return maxPointSecond > 0 ? maxPointSecond : undefined;
  }, [workflowTimingCategory, workflowTimingSeries]);
  const workflowTimingNotice = useMemo(
    () => (workflowTimingCategory === 'expansion'
      ? '⚠️ These are base expansions, not just Nexus/Hatchery/CC buildings.'
      : ''),
    [workflowTimingCategory],
  );
  const workflowHpTimingByRace = useMemo(() => {
    if (workflowTimingCategory !== 'hp_upgrades') return [];
    return TIMING_RACE_ORDER.map((race) => {
      const raceSeries = workflowTimingSeries.filter((playerSeries) => String(playerSeries?.race || '').trim().toLowerCase() === race);
      const labelOptions = Array.from(new Set(
        raceSeries.flatMap((playerSeries) => (playerSeries?.points || []).map((point) => String(point?.label || '').trim()))
          .filter(Boolean),
      )).sort((a, b) => a.localeCompare(b));
      const selectedValue = String(workflowHpUpgradeFilters[race] || '').trim();
      const selected = labelOptions.includes(selectedValue) ? selectedValue : (labelOptions[0] || '');
      const filteredSeries = raceSeries.map((playerSeries) => ({
        ...playerSeries,
        points: (playerSeries?.points || [])
          .filter((point) => selected && String(point?.label || '').trim() === selected)
          .map((point) => ({
            ...point,
            display_label: `+${Math.max(1, Number(point?.order) || 1)}`,
          })),
      }));
      return {
        race,
        raceLabel: prettyRaceName(race),
        labelOptions,
        selected,
        series: filteredSeries,
      };
    }).filter((entry) => entry.series.some((playerSeries) => (playerSeries?.points || []).length > 0));
  }, [workflowTimingCategory, workflowTimingSeries, workflowHpUpgradeFilters]);

  const filterProductionEntries = (entries, view) => {
    const mode = view === 'units' ? workflowUnitFilterMode : workflowBuildingFilterMode;
    const nameNeedle = String(view === 'units' ? workflowUnitNameFilter : workflowBuildingNameFilter).trim().toLowerCase();
    return (entries || []).filter((entry) => {
      const unitType = String(entry?.unit_type || '');
      const key = normalizeUnitName(unitType);
      const isBuilding = BUILDING_TYPE_KEYS.has(key);
      if (view === 'units' && isBuilding) return false;
      if (view === 'buildings' && !isBuilding) return false;
      if (nameNeedle && !unitType.toLowerCase().includes(nameNeedle)) return false;
      if (mode === 'all') return true;
      if (view === 'units') {
        if (mode === 'workers') return WORKER_UNIT_KEYS.has(key);
        if (mode === 'non-workers') return !WORKER_UNIT_KEYS.has(key);
        if (mode === 'spellcasters') return SPELLCASTER_UNIT_KEYS.has(key);
        if (mode === 'tier-1') return UNIT_TIER_MAP[key] === 1;
        if (mode === 'tier-2') return UNIT_TIER_MAP[key] === 2;
        if (mode === 'tier-3') return UNIT_TIER_MAP[key] === 3;
      } else {
        if (mode === 'defenses') return DEFENSIVE_BUILDING_KEYS.has(key);
        if (mode === 'tier-1') return BUILDING_TIER_MAP[key] === 1;
        if (mode === 'tier-2') return BUILDING_TIER_MAP[key] === 2;
        if (mode === 'tier-3') return BUILDING_TIER_MAP[key] === 3;
      }
      return true;
    });
  };

  const workflowGamesTotalPages = Math.max(1, Math.ceil((Number(workflowGamesTotal) || 0) / WORKFLOW_GAMES_PAGE_SIZE));
  const workflowGamesFrom = workflowGames.length === 0 ? 0 : ((workflowGamesPage - 1) * WORKFLOW_GAMES_PAGE_SIZE) + 1;
  const workflowGamesTo = workflowGames.length === 0
    ? 0
    : Math.min((workflowGamesPage - 1) * WORKFLOW_GAMES_PAGE_SIZE + workflowGames.length, Number(workflowGamesTotal) || 0);

  if (loading && !dashboard && activeView === 'dashboards') {
    return (
      <div className="app">
        <div className="loading">Loading dashboard...</div>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="dashboard-container">
        <div className="workflow-nav">
          <button className={`btn-manage ${activeView === 'games' ? 'workflow-nav-active' : ''}`} onClick={() => navigateWorkflowView('games')}>Games</button>
          <button className={`btn-manage ${activeView === 'dashboards' ? 'workflow-nav-active' : ''}`} onClick={() => navigateWorkflowView('dashboards')}>Custom Dashboards</button>
          <button onClick={() => setShowIngestPanel((prev) => !prev)} className="btn-manage">{showIngestPanel ? 'Close Ingest' : 'Ingest'}</button>
        </div>

        {showIngestPanel && (
          <div className="ingest-panel">
            <div className="ingest-header">
              <div className="ingest-title">Ingest Replays</div>
              <div className="ingest-subtitle">Ingestion happens in the background.</div>
            </div>
            <form onSubmit={handleIngestSubmit} className="ingest-form">
              <div className="ingest-grid">
                <label className="ingest-field">
                  <span>Ingest last N replays</span>
                  <input
                    type="number"
                    min="1"
                    value={ingestForm.stopAfterN}
                    onChange={(e) => setIngestForm({ ...ingestForm, stopAfterN: parseInt(e.target.value || '0', 10) })}
                  />
                </label>
                <label className="ingest-field ingest-checkbox">
                  <span>Erase existing data</span>
                  <input
                    type="checkbox"
                    checked={ingestForm.clean}
                    onChange={(e) => setIngestForm({ ...ingestForm, clean: e.target.checked })}
                  />
                </label>
                <label className="ingest-field ingest-checkbox">
                  <span>Auto-ingest latest replay</span>
                  <input
                    type="checkbox"
                    checked={ingestForm.autoIngestEnabled}
                    onChange={(e) => setIngestForm({ ...ingestForm, autoIngestEnabled: e.target.checked })}
                  />
                </label>
                <label className="ingest-field">
                  <span>Auto-ingest interval (seconds)</span>
                  <input
                    type="number"
                    min="60"
                    value={ingestForm.autoIngestIntervalSeconds}
                    onChange={(e) => setIngestForm({
                      ...ingestForm,
                      autoIngestIntervalSeconds: parseInt(e.target.value || '60', 10),
                    })}
                    disabled={!ingestForm.autoIngestEnabled}
                  />
                </label>
              </div>
              <div className="ingest-actions">
                <button type="submit" className="btn-create-ai">
                  Start Ingestion
                </button>
                <button
                  type="button"
                  className="btn-create-manual"
                  onClick={() => setShowIngestPanel(false)}
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        )}

        {error && <div className="error-message">{error}</div>}

        {activeView === 'games' && (
          <div className="workflow-panel">
            <h2>Games</h2>
            <div className="workflow-summary-filter-row workflow-games-filter-row">
              <select
                className="workflow-summary-filter-select"
                value={workflowGamesFilters.player[0] || ''}
                onChange={(e) => setWorkflowGameSingleFilter('player', e.target.value)}
              >
                <option value="">Any player (5+ games)</option>
                {(workflowGamesFilterOptions.players || []).map((option) => (
                  <option key={`wf-player-${option.key}`} value={option.key}>
                    {option.label} ({option.games})
                  </option>
                ))}
              </select>
              <select
                className="workflow-summary-filter-select"
                value={workflowGamesFilters.map[0] || ''}
                onChange={(e) => setWorkflowGameSingleFilter('map', e.target.value)}
              >
                <option value="">Any map (top 15)</option>
                {(workflowGamesFilterOptions.maps || []).map((option) => (
                  <option key={`wf-map-${option.key}`} value={option.key}>
                    {option.label} ({option.games})
                  </option>
                ))}
              </select>
              <div className="workflow-pattern-pills workflow-games-filter-pills">
                {(workflowGamesFilterOptions.durations || []).map((option) => {
                  const active = (workflowGamesFilters.duration || []).includes(option.key);
                  return (
                    <button
                      key={`wf-duration-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleWorkflowGameMultiFilter('duration', option.key)}
                    >
                      {option.label} ({option.games})
                    </button>
                  );
                })}
              </div>
              <div className="workflow-pattern-pills workflow-games-filter-pills">
                {(workflowGamesFilterOptions.featuring || []).map((option) => {
                  const active = (workflowGamesFilters.featuring || []).includes(option.key);
                  return (
                    <button
                      key={`wf-feature-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleWorkflowGameMultiFilter('featuring', option.key)}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
              <button type="button" className="btn-create-manual" onClick={clearWorkflowGamesFilters}>Clear filters</button>
            </div>
            {workflowGamesLoading ? (
              <div className="loading">Loading games...</div>
            ) : (
              <>
                <table className="data-table workflow-table">
                  <thead>
                    <tr>
                      <th>Played</th>
                      <th>Players</th>
                      <th>Map</th>
                      <th>Duration</th>
                      <th>Featuring</th>
                    </tr>
                  </thead>
                  <tbody>
                    {workflowGames.map((game) => (
                      <tr key={game.replay_id} className={selectedReplayId === game.replay_id ? 'workflow-selected-row' : ''} onClick={() => openWorkflowGame(game.replay_id)}>
                        <td>{formatRelativeReplayDate(game.replay_date)}</td>
                        <td>{renderWorkflowGameListPlayers(game)}</td>
                        <td>{game.map_name}</td>
                        <td>{formatDuration(game.duration_seconds)}</td>
                        <td>
                          {(game.featuring || []).length === 0 ? (
                            <span className="workflow-empty-inline">-</span>
                          ) : (
                            <div className="workflow-pattern-pills">
                              {(game.featuring || []).map((pill) => (
                                <span key={`${game.replay_id}-${pill}`} className="workflow-pattern-pill workflow-feature-pill">
                                  <span>{pill}</span>
                                </span>
                              ))}
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div className="workflow-pagination-row">
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={workflowGamesPage <= 1 || workflowGamesLoading}
                    onClick={() => setWorkflowGamesPage((prev) => Math.max(1, prev - 1))}
                  >
                    Previous
                  </button>
                  <span>
                    Page {workflowGamesPage} / {workflowGamesTotalPages} - Showing {workflowGamesFrom}-{workflowGamesTo} of {workflowGamesTotal}
                  </span>
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={workflowGamesPage >= workflowGamesTotalPages || workflowGamesLoading}
                    onClick={() => setWorkflowGamesPage((prev) => Math.min(workflowGamesTotalPages, prev + 1))}
                  >
                    Next
                  </button>
                </div>
              </>
            )}
          </div>
        )}

        {activeView === 'game' && (
          <div className="workflow-panel">
            {workflowGameDetailLoading ? (
              <div className="loading">Loading game report...</div>
            ) : workflowGame ? (
              <>
                <div className="workflow-title-row">
                  <h2>{renderPlayersMatchup(workflowGame.players?.map((p) => p.name).join(' vs '))}</h2>
                  <button className="btn-switch" onClick={goBackWorkflowView}>Back</button>
                </div>
                <div className="workflow-meta">
                  <span>{formatRelativeReplayDate(workflowGame.replay_date)}</span>
                  <span>{workflowGame.map_name}</span>
                  <span>{formatDuration(workflowGame.duration_seconds)}</span>
                </div>
                <div className="workflow-nav">
                  <button className={`btn-switch ${workflowGameTab === 'summary' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('summary')}>Summary</button>
                  <button className={`btn-switch ${workflowGameTab === 'events' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('events')}>Game Events</button>
                  <button className={`btn-switch ${workflowGameTab === 'units' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('units')}>Units</button>
                  <button className={`btn-switch ${workflowGameTab === 'timings' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('timings')}>Timings</button>
                </div>

                {workflowGameTab === 'summary' && (
                  <>
                    <div className="workflow-player-rows" style={{ '--workflow-player-name-width': `${workflowPlayerNameWidthCh}ch` }}>
                      {(workflowGame.players || []).map((player) => (
                        <div key={player.player_id} className="workflow-player-row" style={{ borderLeft: `3px solid ${getTeamColor(player.team)}` }}>
                          <div className="workflow-player-line">
                            <strong
                              className="workflow-player-name"
                              style={playerAccentColor(player.player_key) ? { color: playerAccentColor(player.player_key) } : undefined}
                            >
                              {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                              {player.name}
                            </strong>
                            <div className="workflow-player-actions">
                              <span className="workflow-player-apm"><strong>APM</strong> {player.apm}</span>
                              <button className="workflow-link-btn" onClick={() => openWorkflowPlayer(player.player_key)}>View Player Summary</button>
                            </div>
                            {player.detected_patterns?.map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-${idx}`))}
                          </div>
                        </div>
                      ))}
                    </div>
                    {(filteredReplayPatterns.length > 0 || filteredTeamPatterns.length > 0) && (
                      <div className="workflow-card">
                        {filteredReplayPatterns.length > 0 && (
                          <div className="workflow-pattern-pills">
                            {filteredReplayPatterns.map((pattern, idx) => renderPatternPill(pattern, `replay-${idx}`))}
                          </div>
                        )}
                        {filteredTeamPatterns.length > 0 && (
                          <div className="workflow-pattern-pills">
                            {filteredTeamPatterns.map((pattern, idx) => renderPatternPill(pattern, `team-${idx}`, pattern.team))}
                          </div>
                        )}
                      </div>
                    )}
                  </>
                )}

                {workflowGameTab === 'events' && (
                  <div className="workflow-card">
                    <div className="workflow-summary-filter-row">
                      <input
                        type="text"
                        className="workflow-summary-filter-input"
                        placeholder="Filter events..."
                        value={workflowSummaryFilters.search}
                        onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, search: e.target.value }))}
                      />
                      <select
                        className="workflow-summary-filter-select"
                        value={workflowSummaryFilters.player}
                        onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, player: e.target.value }))}
                      >
                        <option value="">Any player</option>
                        {(workflowGame.players || []).map((player) => (
                          <option key={player.player_id} value={player.name}>{player.name}</option>
                        ))}
                      </select>
                      <select
                        className="workflow-summary-filter-select"
                        value={workflowSummaryFilters.location}
                        onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, location: e.target.value }))}
                      >
                        <option value="">Any location</option>
                        {workflowLocationOptions.map((loc) => (
                          <option key={loc} value={loc}>{loc}</option>
                        ))}
                      </select>
                      <label className="workflow-summary-filter-check">
                        <input
                          type="checkbox"
                          checked={workflowSummaryFilters.nuke}
                          onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, nuke: e.target.checked }))}
                        />
                        nuke
                      </label>
                      <label className="workflow-summary-filter-check">
                        <input
                          type="checkbox"
                          checked={workflowSummaryFilters.drop}
                          onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, drop: e.target.checked }))}
                        />
                        drop
                      </label>
                      <label className="workflow-summary-filter-check">
                        <input
                          type="checkbox"
                          checked={workflowSummaryFilters.recall}
                          onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, recall: e.target.checked }))}
                        />
                        recall
                      </label>
                      <label className="workflow-summary-filter-check">
                        <input
                          type="checkbox"
                          checked={workflowSummaryFilters.becameRace}
                          onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, becameRace: e.target.checked }))}
                        />
                        became race
                      </label>
                      <label className="workflow-summary-filter-check">
                        <input
                          type="checkbox"
                          checked={workflowSummaryFilters.rush}
                          onChange={(e) => setWorkflowSummaryFilters((prev) => ({ ...prev, rush: e.target.checked }))}
                        />
                        rush
                      </label>
                    </div>
                    <div className="workflow-card-title"><span>Game events</span></div>
                    {filteredGameEvents.length > 0 ? (
                      <div className="workflow-events">
                        {filteredGameEvents.map((event, idx) => (
                          <div key={`${event.second}-${idx}`} className="workflow-event-row">
                            <span>{formatDuration(event.second)}</span>
                            <span>{event.description}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="chart-empty">No summary items match current filters.</div>
                    )}
                  </div>
                )}

                {workflowGameTab === 'units' && (
                  <div className="workflow-card">
                    <div className="workflow-production-top-row">
                      <div className="workflow-production-tabs" role="tablist" aria-label="Production type tabs">
                        <button
                          className={`workflow-production-tab ${workflowProductionTab === 'units' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setWorkflowProductionTab('units')}
                          role="tab"
                          aria-selected={workflowProductionTab === 'units'}
                        >
                          Units
                        </button>
                        <button
                          className={`workflow-production-tab ${workflowProductionTab === 'buildings' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setWorkflowProductionTab('buildings')}
                          role="tab"
                          aria-selected={workflowProductionTab === 'buildings'}
                        >
                          Buildings
                        </button>
                      </div>
                      <div className="workflow-units-notice">
                        ⚠️ Replay command streams capture successful production intent, not guaranteed finished unit/building creation.
                        Entries cannot be deduplicated reliably, so expect unevenly inflated numbers. This makes build-order detection very hard.
                      </div>
                    </div>
                    <div className="workflow-summary-filter-row">
                      {workflowProductionTab === 'units' ? (
                        <>
                          <div className="workflow-radio-group">
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="all"
                                checked={workflowUnitFilterMode === 'all'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>All units</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="workers"
                                checked={workflowUnitFilterMode === 'workers'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>Workers only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="non-workers"
                                checked={workflowUnitFilterMode === 'non-workers'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>Non-workers only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="spellcasters"
                                checked={workflowUnitFilterMode === 'spellcasters'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>Spellcasters only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="tier-2"
                                checked={workflowUnitFilterMode === 'tier-2'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>Tier 2 only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="tier-3"
                                checked={workflowUnitFilterMode === 'tier-3'}
                                onChange={(e) => setWorkflowUnitFilterMode(e.target.value)}
                              />
                              <span>Tier 3 only</span>
                            </label>
                          </div>
                          <input
                            type="text"
                            className="workflow-summary-filter-input"
                            placeholder="Filter unit name..."
                            value={workflowUnitNameFilter}
                            onChange={(e) => setWorkflowUnitNameFilter(e.target.value)}
                          />
                        </>
                      ) : (
                        <>
                          <div className="workflow-radio-group">
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="all"
                                checked={workflowBuildingFilterMode === 'all'}
                                onChange={(e) => setWorkflowBuildingFilterMode(e.target.value)}
                              />
                              <span>All buildings</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="defenses"
                                checked={workflowBuildingFilterMode === 'defenses'}
                                onChange={(e) => setWorkflowBuildingFilterMode(e.target.value)}
                              />
                              <span>Defenses only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="tier-2"
                                checked={workflowBuildingFilterMode === 'tier-2'}
                                onChange={(e) => setWorkflowBuildingFilterMode(e.target.value)}
                              />
                              <span>Tier 2 only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="tier-3"
                                checked={workflowBuildingFilterMode === 'tier-3'}
                                onChange={(e) => setWorkflowBuildingFilterMode(e.target.value)}
                              />
                              <span>Tier 3 only</span>
                            </label>
                          </div>
                          <input
                            type="text"
                            className="workflow-summary-filter-input"
                            placeholder="Filter building name..."
                            value={workflowBuildingNameFilter}
                            onChange={(e) => setWorkflowBuildingNameFilter(e.target.value)}
                          />
                        </>
                      )}
                    </div>
                    <div className="table-container">
                      <table className="data-table workflow-table workflow-production-table">
                        <thead>
                          <tr>
                            <th>Slice</th>
                            {workflowPlayers.map((player) => (
                              <th
                                key={player.player_id}
                                style={hasTeamInfo ? { backgroundColor: teamColorRgba(player.team, 0.2) } : undefined}
                              >
                                {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                                {player.name}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {(workflowGame.units_by_slice || []).map((slice) => (
                            <tr key={slice.slice_start_second}>
                              <td>{slice.slice_label}</td>
                              {workflowPlayers.map((player) => {
                                const playerSlice = (slice.players || []).find((item) => item.player_id === player.player_id);
                                const filtered = filterProductionEntries(playerSlice?.units || [], workflowProductionTab);
                                return (
                                  <td
                                    key={`${slice.slice_start_second}-${player.player_id}`}
                                    style={hasTeamInfo ? { backgroundColor: teamColorRgba(player.team, 0.08) } : undefined}
                                  >
                                    {filtered.length === 0 ? (
                                      <span className="workflow-empty-inline">-</span>
                                    ) : (
                                      <div className="workflow-unit-chips">
                                        {filtered.map((unit) => (
                                          <span key={`${player.player_id}-${unit.unit_type}`} className="workflow-unit-chip">
                                            {getUnitIcon(unit.unit_type) ? <img src={getUnitIcon(unit.unit_type)} alt={unit.unit_type} className="workflow-unit-chip-icon" /> : null}
                                            <strong className="workflow-unit-chip-count">x{unit.count}</strong>
                                          </span>
                                        ))}
                                      </div>
                                    )}
                                  </td>
                                );
                              })}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}

                {workflowGameTab === 'timings' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-production-tabs workflow-timing-tabs" role="tablist" aria-label="Timing category tabs">
                      {TIMING_CATEGORY_CONFIG.map((cfg) => (
                        <button
                          key={cfg.id}
                          className={`workflow-production-tab ${workflowTimingCategory === cfg.id ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setWorkflowTimingCategory(cfg.id)}
                          role="tab"
                          aria-selected={workflowTimingCategory === cfg.id}
                        >
                          {cfg.label}
                        </button>
                      ))}
                    </div>
                    {workflowTimingCategory === 'hp_upgrades' ? (
                      <>
                        {workflowHpTimingByRace.map((raceChart) => (
                          <div key={`hp-${raceChart.race}`} className="workflow-card">
                            <div className="workflow-card-title"><span>{`${raceChart.raceLabel} HP upgrades timings`}</span></div>
                            <div className="workflow-radio-group">
                              {raceChart.labelOptions.map((labelName) => (
                                <label key={`${raceChart.race}-${labelName}`} className="workflow-radio-option">
                                  <input
                                    type="radio"
                                    name={`workflow-hp-filter-${raceChart.race}`}
                                    value={labelName}
                                    checked={raceChart.selected === labelName}
                                    onChange={(e) => setWorkflowHpUpgradeFilters((prev) => ({ ...prev, [raceChart.race]: e.target.value }))}
                                  />
                                  <span>{labelName}</span>
                                </label>
                              ))}
                            </div>
                            <TimingScatterRows
                              title=""
                              series={raceChart.series}
                              durationSeconds={workflowGame.duration_seconds}
                              colorByLabel={workflowTimingUsesLabelColors}
                              showLegend={false}
                              markerMode={workflowTimingCategoryConfig.markerMode || 'dot'}
                              axisMode={workflowTimingAxisMode}
                              maxSecondOverride={workflowTimingAxisTrimMaxSecond}
                              inlineLegend={true}
                              rowLabelMode="worker-icon"
                              rowGroupingMode="none"
                            />
                          </div>
                        ))}
                      </>
                    ) : (
                      <TimingScatterRows
                        title={workflowTimingCategoryConfig.title}
                        series={workflowTimingSeries}
                        durationSeconds={workflowGame.duration_seconds}
                        colorByLabel={workflowTimingUsesLabelColors}
                        showLegend={workflowTimingUsesLabelColors && !workflowTimingInlineLegend}
                        markerMode={workflowTimingCategoryConfig.markerMode || 'dot'}
                        axisMode={workflowTimingAxisMode}
                        maxSecondOverride={workflowTimingAxisTrimMaxSecond}
                        inlineLegend={workflowTimingInlineLegend}
                        noticeText={workflowTimingNotice}
                        rowLabelMode={workflowTimingInlineLegend ? 'worker-icon' : (['gas', 'expansion'].includes(workflowTimingCategory) ? 'name-only' : 'race-suffix')}
                        rowGroupingMode={workflowTimingInlineLegend ? 'race' : 'none'}
                      />
                    )}
                  </div>
                )}
              </>
            ) : (
              <div className="chart-empty">Select a game from the Games tab.</div>
            )}

            {workflowGame && workflowGameTab === 'summary' && (
              <>
                <form onSubmit={handleWorkflowAsk} className="workflow-ask-form">
                  <input
                    className="widget-creation-input"
                    value={workflowQuestion}
                    onChange={(e) => setWorkflowQuestion(e.target.value)}
                    placeholder={openaiEnabled ? 'Ask AI about this game...' : 'Enable AI to ask questions'}
                    disabled={!openaiEnabled || askingWorkflow}
                  />
                  <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || askingWorkflow || !workflowQuestion.trim()}>
                    {askingWorkflow ? 'Asking...' : 'Ask AI'}
                  </button>
                </form>
                {renderWorkflowAiResult()}
              </>
            )}
          </div>
        )}

        {activeView === 'player' && (
          <div className="workflow-panel">
            {workflowPlayerLoading ? (
              <div className="loading">Loading player report...</div>
            ) : workflowPlayer ? (
              <>
                <div className="workflow-title-row">
                  <div className="workflow-player-title-wrap">
                    <h2 style={playerAccentColor(workflowPlayer.player_key) ? { color: playerAccentColor(workflowPlayer.player_key) } : undefined}>{workflowPlayer.player_name}</h2>
                    {(Number(workflowPlayer.games_played) || 0) < 5 ? (
                      <span className="workflow-inline-warning">⚠️ Fewer than 5 replays: we cannot provide reliable player-level insights yet.</span>
                    ) : null}
                  </div>
                  <button className="btn-switch" onClick={goBackWorkflowView}>Back</button>
                </div>
                <div className="workflow-meta">
                  <span><strong>Games</strong> {workflowPlayer.games_played}</span>
                  <span><strong>Win rate</strong> {(workflowPlayer.win_rate * 100).toFixed(1)}%</span>
                  <span><strong>APM</strong> {workflowPlayer.average_apm?.toFixed(1)}</span>
                  <span><strong>EAPM</strong> {workflowPlayer.average_eapm?.toFixed(1)}</span>
                </div>
                <div className="workflow-cards">
                  <div className="workflow-card workflow-card-race-behaviours">
                    {workflowPlayerMetricsLoading ? <div className="chart-empty">Loading metrics...</div> : null}
                    {!workflowPlayerMetricsLoading && workflowPlayerMetricsError ? <div className="chart-empty">{workflowPlayerMetricsError}</div> : null}
                    {!workflowPlayerMetricsLoading && !workflowPlayerMetricsError && (workflowPlayerMetrics?.race_behaviour_sections || []).length === 0 ? (
                      <div className="chart-empty">No race behaviour sections available.</div>
                    ) : null}
                    {!workflowPlayerMetricsLoading && !workflowPlayerMetricsError && (workflowPlayerMetrics?.race_behaviour_sections || []).map((section) => (
                      <div key={section.race} className="workflow-race-behaviour-section">
                        <div className="workflow-card-subtitle">
                          {getRaceIcon(section.race) ? <img src={getRaceIcon(section.race)} alt={section.race} className="unit-icon-inline workflow-race-title-icon" /> : null}
                          <span>{section.race}</span>
                        </div>
                        <div className="workflow-subtle-note">
                          {`${section.game_count} games (${((Number(section.game_rate) || 0) * 100).toFixed(1)}%), ${section.wins} wins, ${((Number(section.win_rate) || 0) * 100).toFixed(1)}% win rate`}
                        </div>
                        {(section.common_behaviours || []).length === 0 ? <div className="chart-empty">No common behaviours at 20%+ for this race.</div> : null}
                        {(section.common_behaviours || []).map((item, idx) => (
                          <div key={`${section.race}-${item.name}`} className="workflow-pattern-row">
                            <span>{renderPatternPill({ pattern_name: item.name, value: 'true' }, `player-common-${section.race}-${idx}`)}</span>
                            <span>{`${((Number(item.game_rate) || 0) * 100).toFixed(1)}% (${item.replay_count}/${section.game_count})`}</span>
                          </div>
                        ))}
                      </div>
                    ))}
                  </div>
                  <div className="workflow-card workflow-card-fingerprints">
                    <div className="workflow-card-title"><span>Player fingerprints</span></div>
                    <div className="workflow-card-subtitle">Core metrics</div>
                    {workflowPlayerMetricsLoading ? <div className="chart-empty">Loading metrics...</div> : null}
                    {!workflowPlayerMetricsLoading && workflowPlayerMetricsError ? <div className="chart-empty">{workflowPlayerMetricsError}</div> : null}
                    {!workflowPlayerMetricsLoading && !workflowPlayerMetricsError && (workflowPlayerMetrics?.fingerprint_metrics || []).map((metric) => (
                      <div key={metric.metric} className="workflow-metric-compare-row workflow-metric-compare-row-simple">
                        <span className="workflow-metric-label-with-help">
                          <span>{metric.metric}</span>
                          <HelpTooltip text={metricHelpText(metric.metric)} label={`${metric.metric} explanation`} />
                        </span>
                        <span>{prettyMetricValue(metric)}</span>
                      </div>
                    ))}
                    <div className="workflow-card-subtitle">
                      <span>Distinctive outliers</span>
                      <HelpTooltip text={PLAYER_OUTLIER_HELP} label="Outlier calculation explanation" />
                    </div>
                    <div className="workflow-subtle-note">Same-race, human-only baselines. Protoss outliers exclude non-Protoss spell leakage. Targeted orders use raw-instance share; tech/upgrades use per-game incidence.</div>
                    {workflowPlayerOutliersLoading ? <div className="chart-empty">Loading outliers...</div> : null}
                    {!workflowPlayerOutliersLoading && workflowPlayerOutliersError ? <div className="chart-empty">{workflowPlayerOutliersError}</div> : null}
                    {!workflowPlayerOutliersLoading && !workflowPlayerOutliersError && workflowPlayerOutlierGroups.length === 0 ? (
                      <div className="chart-empty">No outliers crossed current thresholds.</div>
                    ) : null}
                    {!workflowPlayerOutliersLoading && !workflowPlayerOutliersError && workflowPlayerOutlierGroups.map(([category, items]) => (
                      <div key={category} className="workflow-outlier-group">
                        <div className="workflow-card-subtitle">
                          <span>{category}</span>
                          <HelpTooltip
                            text={OUTLIER_CATEGORY_SUBTITLE[category] || 'Compared against same-race human baseline.'}
                            label={`${category} baseline definition`}
                          />
                        </div>
                        {items.map((item) => (
                          <div key={`${item.category}-${item.race}-${item.name}`} className="workflow-pattern-row">
                            <span>{item.pretty_name}</span>
                            <span className="workflow-outlier-expl">
                              <span className="workflow-outlier-rate">{`${((Number(item.player_rate) || 0) * 100).toFixed(0)}%`}</span>
                              <span>you</span>
                              <span>vs</span>
                              <span className="workflow-outlier-rate-muted">{`${((Number(item.baseline_rate) || 0) * 100).toFixed(0)}%`}</span>
                              <span>baseline</span>
                              {(item.qualified_by || []).map((qualifier) => (
                                <span key={`${item.name}-${qualifier}`} className={outlierQualifierClassName(qualifier)}>{qualifier}</span>
                              ))}
                            </span>
                          </div>
                        ))}
                      </div>
                    ))}
                  </div>
                  <div className="workflow-card">
                    <div className="workflow-card-title"><span>Go to Recent Games</span></div>
                    {workflowPlayer.recent_games?.slice(0, 6).map((g) => (
                      <div key={g.replay_id}>
                        <button className="workflow-link-btn" onClick={() => openWorkflowGame(g.replay_id)}>{formatRelativeReplayDate(g.replay_date)} - {g.map_name}</button>
                      </div>
                    ))}
                  </div>
                </div>
              </>
            ) : (
              <div className="chart-empty">Select a player from a game report.</div>
            )}
            <form onSubmit={handleWorkflowAsk} className="workflow-ask-form">
              <input
                className="widget-creation-input"
                value={workflowQuestion}
                onChange={(e) => setWorkflowQuestion(e.target.value)}
                placeholder={openaiEnabled ? 'Ask AI about this player...' : 'Enable AI to ask questions'}
                disabled={!openaiEnabled || askingWorkflow}
              />
              <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || askingWorkflow || !workflowQuestion.trim()}>
                {askingWorkflow ? 'Asking...' : 'Ask AI'}
              </button>
            </form>
            {renderWorkflowAiResult()}
          </div>
        )}

        {activeView === 'dashboards' && (
          <>
            <div className="dashboard-header">
              <div className="dashboard-title">
                <div className="dashboard-title-left">
                  <h1>{dashboard?.name || 'Dashboard'}</h1>
                  <button
                    onClick={() => setShowEditDashboard(true)}
                    className="btn-edit-dashboard"
                    title="Edit dashboard"
                  >
                    ✎
                  </button>
                </div>
                <div className="dashboard-actions">
                  <select
                    value={currentDashboardUrl}
                    onChange={(e) => handleSwitchDashboard(e.target.value)}
                    className="dashboard-select"
                  >
                    {dashboards.map((d) => (
                      <option key={d.url} value={d.url}>
                        {d.name}
                      </option>
                    ))}
                  </select>
                  <button
                    onClick={() => setShowDashboardManager(true)}
                    className="btn-manage"
                  >
                    Manage Dashboards
                  </button>
                </div>
              </div>

              <div className="widget-creation-section">
                {openaiEnabled ? (
                  <form onSubmit={handleCreateWidget} className="widget-creation-form">
                    <div className="widget-creation-input-group">
                      <input
                        type="text"
                        value={newWidgetPrompt}
                        onChange={(e) => setNewWidgetPrompt(e.target.value)}
                        placeholder="Ask to add a new graph or chart..."
                        className="widget-creation-input"
                        disabled={creatingWidget}
                      />
                      <button
                        type="submit"
                        disabled={creatingWidget || !newWidgetPrompt.trim()}
                        className="btn-create-ai"
                      >
                        <span className="btn-icon">✨</span>
                        Create with AI
                      </button>
                      <div className="widget-creation-divider">or</div>
                      <button
                        type="button"
                        onClick={handleCreateWidgetWithoutPrompt}
                        disabled={creatingWidget}
                        className="btn-create-manual"
                      >
                        Create Manually
                      </button>
                    </div>
                  </form>
                ) : (
                  <div className="widget-creation-form">
                    <div className="widget-creation-input-group">
                      <button
                        type="button"
                        onClick={handleCreateWidgetWithoutPrompt}
                        disabled={creatingWidget}
                        className="btn-create-manual-primary"
                      >
                        Create Widget
                      </button>
                      <div className="widget-creation-info">
                        <span className="info-icon">ℹ️</span>
                        <span className="info-text">AI-powered creation requires --openai-api-key flag</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {dashboard?.variables && Object.keys(dashboard.variables).length > 0 && (
                <div className="variables-container" style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginTop: '1rem' }}>
                  {Object.entries(dashboard.variables).map(([varName, variable]) => (
                    <div key={varName} className="variable-select" style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                      <label htmlFor={`var-${varName}`} style={{ fontSize: '0.875rem', fontWeight: '500' }}>
                        {variable.display_name}
                      </label>
                      <select
                        id={`var-${varName}`}
                        value={variableValues[varName] || ''}
                        onChange={(e) => handleVariableChange(varName, e.target.value)}
                        style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #ccc', minWidth: '200px' }}
                      >
                        {variable.possible_values?.map((value, idx) => (
                          <option key={idx} value={value}>
                            {value}
                          </option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="widgets-grid">
              {sortedWidgets.map((widget) => (
                <Widget
                  key={widget.id}
                  widget={widget}
                  dashboardUrl={currentDashboardUrl}
                  variableValues={variableValues}
                  onDelete={handleDeleteWidget}
                  onUpdate={handleUpdateWidget}
                />
              ))}
            </div>
          </>
        )}
      </div>

      {creatingWidget && (
        <WidgetCreationSpinner />
      )}

      {showDashboardManager && (
        <DashboardManager
          dashboards={dashboards}
          currentUrl={currentDashboardUrl}
          onClose={() => setShowDashboardManager(false)}
          onRefresh={loadDashboards}
          onSwitch={handleSwitchDashboard}
        />
      )}

      {showEditDashboard && dashboard && (
        <EditDashboardModal
          dashboard={dashboard}
          onClose={() => setShowEditDashboard(false)}
          onSave={handleUpdateDashboard}
        />
      )}

      {editingWidget && (
        <EditWidgetFullscreen
          widget={editingWidget}
          dashboardUrl={currentDashboardUrl}
          onClose={() => {
            setEditingWidget(null);
            loadDashboard(currentDashboardUrl);
          }}
          onSave={(data) => handleUpdateWidget(editingWidget.id, data)}
        />
      )}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null
            ? `${replayCount.toLocaleString()} replays in database. You can trigger an ingestion using the button above.`
            : 'Loading replay count...'}
        </div>
        {ingestMessage && <div className="footer-right">{ingestMessage}</div>}
      </div>
    </div>
  );
}

export default App;
