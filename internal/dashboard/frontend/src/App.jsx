import React, { useState, useEffect, useMemo, useRef } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import GlobalReplayFilterModal from './components/GlobalReplayFilterModal';
import IngestModal from './components/IngestModal';
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
import FirstUnitEfficiencyTimelineRows from './components/charts/FirstUnitEfficiencyTimelineRows';
import probeImg from './assets/units/probe.png';
import scvImg from './assets/units/scv.png';
import droneImg from './assets/units/drone.png';
import arbiterImg from './assets/units/arbiter.png';
import corsairImg from './assets/units/corsair.png';
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

const formatDaysAgoCompact = (value) => {
  const days = Math.max(0, Number(value) || 0);
  if (days === 0) return 'Today';
  if (days === 1) return '1d ago';
  return `${days}d ago`;
};

const buildHistogramSummaryFromPlayers = (players) => {
  const safePlayers = Array.isArray(players)
    ? players
      .map((player) => ({
        ...player,
        player_key: String(player?.player_key || '').trim().toLowerCase(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.average_apm || 0),
        games_played: Number(player?.games_played || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0)
    : [];

  if (safePlayers.length === 0) {
    return {
      points: [],
      bins: [],
      mean: 0,
      stddev: 0,
      playersIncluded: 0,
      maxGames: 5,
    };
  }

  const values = safePlayers.map((player) => player.average_apm).sort((a, b) => a - b);
  const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
  const variance = values.reduce((sum, value) => {
    const diff = value - mean;
    return sum + (diff * diff);
  }, 0) / values.length;
  const stddev = Math.sqrt(variance);

  let binCount = Math.round(Math.sqrt(values.length));
  if (binCount < 8) binCount = 8;
  if (binCount > 24) binCount = 24;

  const minValue = values[0];
  const maxValue = values[values.length - 1];
  let bins = [];
  if (maxValue <= minValue) {
    bins = [{ x0: minValue, x1: minValue + 1, count: values.length }];
  } else {
    let width = (maxValue - minValue) / binCount;
    if (width <= 0) width = 1;
    bins = Array.from({ length: binCount }, (_, idx) => {
      const x0 = minValue + (idx * width);
      const x1 = idx === binCount - 1 ? maxValue : minValue + ((idx + 1) * width);
      return { x0, x1, count: 0 };
    });
    values.forEach((value) => {
      let idx = Math.floor((value - minValue) / width);
      if (idx < 0) idx = 0;
      if (idx >= binCount) idx = binCount - 1;
      bins[idx].count += 1;
    });
  }

  const maxGames = safePlayers.reduce((maxValue, player) => Math.max(maxValue, player.games_played), 5);
  return {
    points: safePlayers,
    bins,
    mean,
    stddev,
    playersIncluded: safePlayers.length,
    maxGames,
  };
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

const DEFAULT_HP_UPGRADE_BY_RACE = {
  terran: 'Terran Vehicle Weapons',
  protoss: 'Protoss Ground Weapons',
  zerg: 'Zerg Carapace',
};

const racePrefixForUpgrade = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (!value) return '';
  return `${value.charAt(0).toUpperCase()}${value.slice(1)} `;
};

const setHasUpgradeLoose = (upgradeSet, upgradeName) => {
  const value = String(upgradeName || '').trim();
  if (!value) return false;
  if (upgradeSet.has(value)) return true;
  for (const known of upgradeSet) {
    if (value.startsWith(`${known} `) || value.startsWith(`${known}+`) || value.startsWith(`${known} +`)) {
      return true;
    }
  }
  return false;
};

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
  if (setHasUpgradeLoose(HP_UPGRADE_NAMES, value)) return 'hp_upgrades';
  if (setHasUpgradeLoose(UNIT_RANGE_UPGRADE_NAMES, value)) return 'unit_range';
  if (setHasUpgradeLoose(UNIT_SPEED_UPGRADE_NAMES, value)) return 'unit_speed';
  if (setHasUpgradeLoose(ENERGY_UPGRADE_NAMES, value)) return 'energy';
  if (setHasUpgradeLoose(CAPACITY_COOLDOWN_DAMAGE_UPGRADE_NAMES, value)) return 'capacity_cooldown_damage';
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
const FIRST_UNIT_EFFICIENCY_GROUP_CONFIG = [
  { race: 'protoss', buildingName: 'Forge', unitNames: ['Photon Cannon'] },
  { race: 'protoss', buildingName: 'Gateway', unitNames: ['Zealot'] },
  { race: 'protoss', buildingName: 'Stargate', unitNames: ['Corsair', 'Scout'] },
  { race: 'protoss', buildingName: 'Fleet Beacon', unitNames: ['Carrier'] },
  { race: 'protoss', buildingName: 'Arbiter Tribunal', unitNames: ['Arbiter'] },
  { race: 'terran', buildingName: 'Barracks', unitNames: ['Marine'] },
  { race: 'terran', buildingName: 'Factory', unitNames: ['Vulture', 'Siege Tank'] },
  { race: 'terran', buildingName: 'Physics Lab', unitNames: ['Battlecruiser'] },
  { race: 'zerg', buildingName: 'Spawning Pool', unitNames: ['Zergling'] },
  { race: 'zerg', buildingName: 'Hydralisk Den', unitNames: ['Hydralisk'] },
  { race: 'zerg', buildingName: 'Spire', unitNames: ['Mutalisk', 'Scourge'] },
  { race: 'zerg', buildingName: 'Ultralisk Cavern', unitNames: ['Ultralisk'] },
  { race: 'zerg', buildingName: 'Defiler Mound', unitNames: ['Defiler'] },
];

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
  corsair: corsairImg,
  protosscorsair: corsairImg,
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
  if (lowerName.includes('threw nukes')) {
    const minute = minuteFromValue(rawValue);
    if (minute !== null) return `${rawName} at ${minute} mins`;
  }
  return `${rawName} at ${rawValue}`;
};

const shouldHidePatternFromSummaryPills = (pattern) => {
  const normalizedPatternName = normalizeUnitName(pattern?.pattern_name);
  return normalizedPatternName === 'viewportmultitasking';
};

const filterSummaryPillPatterns = (patterns) => (
  (patterns || []).filter((pattern) => !shouldHidePatternFromSummaryPills(pattern))
);

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

const PLAYER_OUTLIER_HELP = [
  'Baselines are computed against human, non-observer players of the same primary race only.',
  'For Protoss players, non-Protoss techs/upgrades and non-Protoss cast orders are excluded to avoid mind-control leakage.',
  'Orders use share of total order instances. Build, train, morph, tech, and upgrade items use the share of same-race games where the item appears at least once.',
  'An item appears if it passes either threshold: "Rare signature" (TF-IDF) or "Much more frequent than peers" (ratio vs baseline).',
].join(' ');

const PLAYER_INSIGHT_TYPES = {
  apm: 'apm',
  firstUnitDelay: 'first-unit-delay',
  unitProductionCadence: 'unit-production-cadence',
  viewportSwitchRate: 'viewport-switch-rate',
};

const VIEWPORT_SWITCH_RATE_CONFIG = {
  title: 'Viewport Switch Rate',
  playerField: 'average_viewport_switch_rate',
  gameField: 'viewport_switch_rate',
  axisLabel: 'Average switches per minute',
  overlayValueLabel: 'switches/min',
  valueFormatter: (value) => `${Number(value || 0).toFixed(2)} switches/min`,
  summaryFormatter: (value) => `${Number(value || 0).toFixed(2)}`,
  interpretation: 'Higher means the player more often jumps outside the prior viewport-sized area during the mid-game window.',
};

const LOW_USAGE_THRESHOLD = 0.1;

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

const insightScoreColor = (percentile) => {
  const clamped = Math.max(0, Math.min(100, Number(percentile) || 0));
  const hue = (clamped / 100) * 120;
  return `hsl(${hue}, 78%, 52%)`;
};

const insightScoreLabel = (percentile) => {
  const score = Number(percentile) || 0;
  if (score >= 90) return 'Elite';
  if (score >= 75) return 'Strong';
  if (score >= 55) return 'Solid';
  if (score >= 35) return 'Mixed';
  return 'Needs work';
};

const insightSummaryLabel = (percentile) => {
  const score = Math.max(0, Math.min(100, Number(percentile) || 0));
  if (score >= 99) return 'Best in sample';
  if (score >= 80) return `Top ${Math.max(1, Math.round(100 - score))}%`;
  return `Better than ${Math.round(score)}%`;
};

const playerInsightDestinationTab = (insightType) => {
  switch (String(insightType || '').trim()) {
    case PLAYER_INSIGHT_TYPES.apm:
      return 'apm-histogram';
    case PLAYER_INSIGHT_TYPES.firstUnitDelay:
      return 'first-unit-delay';
    case PLAYER_INSIGHT_TYPES.unitProductionCadence:
      return 'unit-production-cadence';
    case PLAYER_INSIGHT_TYPES.viewportSwitchRate:
      return 'viewport-multitasking';
    default:
      return 'summary';
  }
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
const WORKFLOW_PLAYERS_PAGE_SIZE = 30;

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

const mergeIngestLogEntries = (entries, event) => {
  if (!event || !event.message) {
    return entries;
  }

  if (event.append && entries.length > 0 && entries[entries.length - 1].append) {
    const next = [...entries];
    const last = next[next.length - 1];
    next[next.length - 1] = {
      ...last,
      level: event.level || last.level,
      message: `${last.message}${event.message}`,
      append: true,
    };
    return next;
  }

  return [...entries, {
    level: event.level || 'info',
    message: event.message,
    append: Boolean(event.append),
  }];
};

const hydrateIngestLogEntries = (events = []) => (
  (events || []).reduce((entries, event) => mergeIngestLogEntries(entries, event), [])
);

const sleep = (ms) => new Promise((resolve) => window.setTimeout(resolve, ms));

function App() {
  const storedAutoIngest = getStoredAutoIngestSettings();
  const [currentDashboardUrl, setCurrentDashboardUrl] = useState('default');
  const [dashboard, setDashboard] = useState(null);
  const [dashboards, setDashboards] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showDashboardManager, setShowDashboardManager] = useState(false);
  const [showEditDashboard, setShowEditDashboard] = useState(false);
  const [showGlobalReplayFilter, setShowGlobalReplayFilter] = useState(false);
  const [newWidgetPrompt, setNewWidgetPrompt] = useState('');
  const [creatingWidget, setCreatingWidget] = useState(false);
  const [variableValues, setVariableValues] = useState({});
  const [openaiEnabled, setOpenaiEnabled] = useState(false);
  const [editingWidget, setEditingWidget] = useState(null);
  const [replayCount, setReplayCount] = useState(null);
  const [globalReplayFilterConfig, setGlobalReplayFilterConfig] = useState(null);
  const [globalReplayFilterOptions, setGlobalReplayFilterOptions] = useState({
    top_maps: [],
    other_maps: [],
    top_players: [],
    other_players: [],
  });
  const [globalReplayFilterSaving, setGlobalReplayFilterSaving] = useState(false);
  const [globalReplayFilterError, setGlobalReplayFilterError] = useState('');
  const [showIngestPanel, setShowIngestPanel] = useState(false);
  const [ingestMessage, setIngestMessage] = useState('');
  const [ingestStatus, setIngestStatus] = useState('idle');
  const [ingestLogs, setIngestLogs] = useState([]);
  const [ingestInputDir, setIngestInputDir] = useState('');
  const [savedIngestInputDir, setSavedIngestInputDir] = useState('');
  const [ingestSettingsLoading, setIngestSettingsLoading] = useState(false);
  const [ingestSettingsSaving, setIngestSettingsSaving] = useState(false);
  const [ingestSocketState, setIngestSocketState] = useState('closed');
  const [autoIngestNotice, setAutoIngestNotice] = useState('');
  const [ingestForm, setIngestForm] = useState({
    watch: false,
    stopAfterN: 50,
    clean: false,
    storeRightClicks: false,
    skipHotkeys: false,
    autoIngestEnabled: storedAutoIngest.enabled,
    autoIngestIntervalSeconds: storedAutoIngest.intervalSeconds,
  });
  const autoIngestInFlight = useRef(false);
  const ingestSocketRef = useRef(null);
  const autoIngestNoticeTimerRef = useRef(null);
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
  const [workflowPlayerRecentGames, setWorkflowPlayerRecentGames] = useState([]);
  const [workflowPlayerRecentGamesLoading, setWorkflowPlayerRecentGamesLoading] = useState(false);
  const [workflowPlayerRecentGamesError, setWorkflowPlayerRecentGamesError] = useState('');
  const [workflowPlayerChatSummary, setWorkflowPlayerChatSummary] = useState(null);
  const [workflowPlayerChatSummaryLoading, setWorkflowPlayerChatSummaryLoading] = useState(false);
  const [workflowPlayerChatSummaryError, setWorkflowPlayerChatSummaryError] = useState('');
  const [workflowPlayerMetrics, setWorkflowPlayerMetrics] = useState(null);
  const [workflowPlayerMetricsLoading, setWorkflowPlayerMetricsLoading] = useState(false);
  const [workflowPlayerMetricsError, setWorkflowPlayerMetricsError] = useState('');
  const [workflowPlayerOutliers, setWorkflowPlayerOutliers] = useState(null);
  const [workflowPlayerOutliersLoading, setWorkflowPlayerOutliersLoading] = useState(false);
  const [workflowPlayerOutliersError, setWorkflowPlayerOutliersError] = useState('');
  const [workflowPlayers, setWorkflowPlayers] = useState([]);
  const [workflowPlayersLoading, setWorkflowPlayersLoading] = useState(false);
  const [workflowPlayersPage, setWorkflowPlayersPage] = useState(1);
  const [workflowPlayersTotal, setWorkflowPlayersTotal] = useState(0);
  const [workflowPlayersSortBy, setWorkflowPlayersSortBy] = useState('games');
  const [workflowPlayersSortDir, setWorkflowPlayersSortDir] = useState('desc');
  const [workflowPlayersTab, setWorkflowPlayersTab] = useState('summary');
  const [workflowPlayersFilterOptions, setWorkflowPlayersFilterOptions] = useState({
    races: [],
    last_played: [],
  });
  const [workflowPlayersFilters, setWorkflowPlayersFilters] = useState({
    name: '',
    onlyFivePlus: false,
    lastPlayed: [],
  });
  const [workflowPlayersApmHistogram, setWorkflowPlayersApmHistogram] = useState(null);
  const [workflowPlayersApmHistogramLoading, setWorkflowPlayersApmHistogramLoading] = useState(false);
  const [workflowPlayersApmHistogramError, setWorkflowPlayersApmHistogramError] = useState('');
  const [workflowPlayersApmMinGames, setWorkflowPlayersApmMinGames] = useState(5);
  const [workflowPlayersDelayHistogram, setWorkflowPlayersDelayHistogram] = useState(null);
  const [workflowPlayersDelayHistogramLoading, setWorkflowPlayersDelayHistogramLoading] = useState(false);
  const [workflowPlayersDelayHistogramError, setWorkflowPlayersDelayHistogramError] = useState('');
  const [workflowPlayersDelayMinSamples, setWorkflowPlayersDelayMinSamples] = useState(5);
  const [workflowPlayersDelaySelectedCases, setWorkflowPlayersDelaySelectedCases] = useState(['all']);
  const [workflowPlayersCadenceHistogram, setWorkflowPlayersCadenceHistogram] = useState(null);
  const [workflowPlayersCadenceHistogramLoading, setWorkflowPlayersCadenceHistogramLoading] = useState(false);
  const [workflowPlayersCadenceHistogramError, setWorkflowPlayersCadenceHistogramError] = useState('');
  const [workflowPlayersCadenceMinGames, setWorkflowPlayersCadenceMinGames] = useState(4);
  const [workflowPlayersViewportHistogram, setWorkflowPlayersViewportHistogram] = useState(null);
  const [workflowPlayersViewportHistogramLoading, setWorkflowPlayersViewportHistogramLoading] = useState(false);
  const [workflowPlayersViewportHistogramError, setWorkflowPlayersViewportHistogramError] = useState('');
  const [workflowPlayersViewportMinGames, setWorkflowPlayersViewportMinGames] = useState(4);
  const [workflowPlayerApmInsight, setWorkflowPlayerApmInsight] = useState(null);
  const [workflowPlayerApmInsightLoading, setWorkflowPlayerApmInsightLoading] = useState(false);
  const [workflowPlayerApmInsightError, setWorkflowPlayerApmInsightError] = useState('');
  const [workflowPlayerDelayInsight, setWorkflowPlayerDelayInsight] = useState(null);
  const [workflowPlayerDelayInsightLoading, setWorkflowPlayerDelayInsightLoading] = useState(false);
  const [workflowPlayerDelayInsightError, setWorkflowPlayerDelayInsightError] = useState('');
  const [workflowPlayerCadenceInsight, setWorkflowPlayerCadenceInsight] = useState(null);
  const [workflowPlayerCadenceInsightLoading, setWorkflowPlayerCadenceInsightLoading] = useState(false);
  const [workflowPlayerCadenceInsightError, setWorkflowPlayerCadenceInsightError] = useState('');
  const [workflowPlayerViewportInsight, setWorkflowPlayerViewportInsight] = useState(null);
  const [workflowPlayerViewportInsightLoading, setWorkflowPlayerViewportInsightLoading] = useState(false);
  const [workflowPlayerViewportInsightError, setWorkflowPlayerViewportInsightError] = useState('');
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
    terran: DEFAULT_HP_UPGRADE_BY_RACE.terran,
    zerg: DEFAULT_HP_UPGRADE_BY_RACE.zerg,
    protoss: DEFAULT_HP_UPGRADE_BY_RACE.protoss,
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

  const loadGlobalReplayFilterConfig = async () => {
    const data = await api.getGlobalReplayFilter();
    setGlobalReplayFilterConfig(data);
    return data;
  };

  const loadGlobalReplayFilterOptions = async () => {
    const data = await api.getGlobalReplayFilterOptions();
    setGlobalReplayFilterOptions({
      top_maps: data?.top_maps || [],
      other_maps: data?.other_maps || [],
      top_players: data?.top_players || [],
      other_players: data?.other_players || [],
    });
    return data;
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

  const loadWorkflowPlayers = async ({
    page = workflowPlayersPage,
    filters = workflowPlayersFilters,
    sortBy = workflowPlayersSortBy,
    sortDir = workflowPlayersSortDir,
  } = {}) => {
    try {
      setWorkflowPlayersLoading(true);
      const safePage = Math.max(1, Number(page) || 1);
      const offset = (safePage - 1) * WORKFLOW_PLAYERS_PAGE_SIZE;
      const data = await api.listWorkflowPlayers({
        limit: WORKFLOW_PLAYERS_PAGE_SIZE,
        offset,
        sortBy,
        sortDir,
        filters,
      });
      setWorkflowPlayers(data?.items || []);
      setWorkflowPlayersTotal(Number(data?.total) || 0);
      if (data?.filter_options) {
        setWorkflowPlayersFilterOptions(data.filter_options);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowPlayersLoading(false);
    }
  };

  const loadWorkflowPlayersApmHistogram = async () => {
    try {
      setWorkflowPlayersApmHistogramLoading(true);
      setWorkflowPlayersApmHistogramError('');
      const data = await api.getWorkflowPlayersApmHistogram();
      setWorkflowPlayersApmHistogram(data);
    } catch (err) {
      setWorkflowPlayersApmHistogramError(err.message || 'Failed to load players histogram');
      setWorkflowPlayersApmHistogram(null);
    } finally {
      setWorkflowPlayersApmHistogramLoading(false);
    }
  };

  const loadWorkflowPlayersDelayHistogram = async () => {
    try {
      setWorkflowPlayersDelayHistogramLoading(true);
      setWorkflowPlayersDelayHistogramError('');
      const data = await api.getWorkflowPlayersFirstUnitDelay();
      setWorkflowPlayersDelayHistogram(data);
      setWorkflowPlayersDelaySelectedCases(['all']);
    } catch (err) {
      setWorkflowPlayersDelayHistogramError(err.message || 'Failed to load players delay');
      setWorkflowPlayersDelayHistogram(null);
      setWorkflowPlayersDelaySelectedCases(['all']);
    } finally {
      setWorkflowPlayersDelayHistogramLoading(false);
    }
  };

  const loadWorkflowPlayersCadenceHistogram = async () => {
    try {
      setWorkflowPlayersCadenceHistogramLoading(true);
      setWorkflowPlayersCadenceHistogramError('');
      const data = await api.getWorkflowPlayersUnitProductionCadence({ filter: 'strict', minGames: 4, limit: 0 });
      setWorkflowPlayersCadenceHistogram(data);
    } catch (err) {
      setWorkflowPlayersCadenceHistogramError(err.message || 'Failed to load players unit production cadence');
      setWorkflowPlayersCadenceHistogram(null);
    } finally {
      setWorkflowPlayersCadenceHistogramLoading(false);
    }
  };

  const loadWorkflowPlayersViewportHistogram = async () => {
    try {
      setWorkflowPlayersViewportHistogramLoading(true);
      setWorkflowPlayersViewportHistogramError('');
      const data = await api.getWorkflowPlayersViewportMultitasking();
      setWorkflowPlayersViewportHistogram(data);
    } catch (err) {
      setWorkflowPlayersViewportHistogramError(err.message || 'Failed to load players viewport multitasking');
      setWorkflowPlayersViewportHistogram(null);
    } finally {
      setWorkflowPlayersViewportHistogramLoading(false);
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
      setWorkflowHpUpgradeFilters({
        terran: DEFAULT_HP_UPGRADE_BY_RACE.terran,
        zerg: DEFAULT_HP_UPGRADE_BY_RACE.zerg,
        protoss: DEFAULT_HP_UPGRADE_BY_RACE.protoss,
      });
      navigateWorkflowView('game');
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowGameDetailLoading(false);
    }
  };

  const loadWorkflowPlayerRecentGames = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerRecentGamesLoading(true);
      setWorkflowPlayerRecentGamesError('');
      const data = await api.getWorkflowPlayerRecentGames(normalizedPlayerKey);
      setWorkflowPlayerRecentGames(data?.recent_games || []);
    } catch (err) {
      setWorkflowPlayerRecentGamesError(err.message || 'Failed to load recent games');
      setWorkflowPlayerRecentGames([]);
    } finally {
      setWorkflowPlayerRecentGamesLoading(false);
    }
  };

  const loadWorkflowPlayerChatSummary = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerChatSummaryLoading(true);
      setWorkflowPlayerChatSummaryError('');
      const data = await api.getWorkflowPlayerChatSummary(normalizedPlayerKey);
      setWorkflowPlayerChatSummary(data?.chat_summary || null);
    } catch (err) {
      setWorkflowPlayerChatSummaryError(err.message || 'Failed to load chat summary');
      setWorkflowPlayerChatSummary(null);
    } finally {
      setWorkflowPlayerChatSummaryLoading(false);
    }
  };

  const loadWorkflowPlayerMetrics = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
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

  const loadWorkflowPlayerApmInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerApmInsightLoading(true);
      setWorkflowPlayerApmInsightError('');
      const insightData = await api.getWorkflowPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.apm);
      setWorkflowPlayerApmInsight(insightData);
    } catch (err) {
      setWorkflowPlayerApmInsightError(err.message || 'Failed to load APM insight');
      setWorkflowPlayerApmInsight(null);
    } finally {
      setWorkflowPlayerApmInsightLoading(false);
    }
  };

  const loadWorkflowPlayerOutliers = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
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

  const loadWorkflowPlayerDelayInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerDelayInsightLoading(true);
      setWorkflowPlayerDelayInsightError('');
      const delayData = await api.getWorkflowPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.firstUnitDelay);
      setWorkflowPlayerDelayInsight(delayData);
    } catch (err) {
      setWorkflowPlayerDelayInsightError(err.message || 'Failed to load delay insight');
      setWorkflowPlayerDelayInsight(null);
    } finally {
      setWorkflowPlayerDelayInsightLoading(false);
    }
  };

  const loadWorkflowPlayerCadenceInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerCadenceInsightLoading(true);
      setWorkflowPlayerCadenceInsightError('');
      const cadenceData = await api.getWorkflowPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.unitProductionCadence);
      setWorkflowPlayerCadenceInsight(cadenceData);
    } catch (err) {
      setWorkflowPlayerCadenceInsightError(err.message || 'Failed to load cadence insight');
      setWorkflowPlayerCadenceInsight(null);
    } finally {
      setWorkflowPlayerCadenceInsightLoading(false);
    }
  };

  const loadWorkflowPlayerViewportInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setWorkflowPlayerViewportInsightLoading(true);
      setWorkflowPlayerViewportInsightError('');
      const viewportData = await api.getWorkflowPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.viewportSwitchRate);
      setWorkflowPlayerViewportInsight(viewportData);
    } catch (err) {
      setWorkflowPlayerViewportInsightError(err.message || 'Failed to load viewport insight');
      setWorkflowPlayerViewportInsight(null);
    } finally {
      setWorkflowPlayerViewportInsightLoading(false);
    }
  };

  const openWorkflowPlayer = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    try {
      setWorkflowPlayerLoading(true);
      setError(null);
      const data = await api.getWorkflowPlayer(playerKey);
      setWorkflowPlayer(data);
      setWorkflowPlayerRecentGames([]);
      setWorkflowPlayerRecentGamesError('');
      setWorkflowPlayerRecentGamesLoading(false);
      setWorkflowPlayerChatSummary(null);
      setWorkflowPlayerChatSummaryError('');
      setWorkflowPlayerChatSummaryLoading(false);
      setWorkflowPlayerMetrics(null);
      setWorkflowPlayerMetricsError('');
      setWorkflowPlayerMetricsLoading(false);
      setWorkflowPlayerOutliers(null);
      setWorkflowPlayerOutliersError('');
      setWorkflowPlayerOutliersLoading(false);
      setWorkflowPlayerApmInsight(null);
      setWorkflowPlayerApmInsightError('');
      setWorkflowPlayerApmInsightLoading(false);
      setWorkflowPlayerDelayInsight(null);
      setWorkflowPlayerDelayInsightError('');
      setWorkflowPlayerDelayInsightLoading(false);
      setWorkflowPlayerCadenceInsight(null);
      setWorkflowPlayerCadenceInsightError('');
      setWorkflowPlayerCadenceInsightLoading(false);
      setWorkflowPlayerViewportInsight(null);
      setWorkflowPlayerViewportInsightError('');
      setWorkflowPlayerViewportInsightLoading(false);
      setSelectedPlayerKey(normalizedPlayerKey);
      setWorkflowAnswer(null);
      setWorkflowQuestion('');
      navigateWorkflowView('player');
      loadWorkflowPlayerRecentGames(normalizedPlayerKey);
      loadWorkflowPlayerChatSummary(normalizedPlayerKey);
      loadWorkflowPlayerMetrics(normalizedPlayerKey);
      loadWorkflowPlayerOutliers(normalizedPlayerKey);
      loadWorkflowPlayerApmInsight(normalizedPlayerKey);
      loadWorkflowPlayerDelayInsight(normalizedPlayerKey);
      loadWorkflowPlayerCadenceInsight(normalizedPlayerKey);
      loadWorkflowPlayerViewportInsight(normalizedPlayerKey);
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowPlayerLoading(false);
    }
  };

  const loadIngestSettings = async () => {
    try {
      setIngestSettingsLoading(true);
      const data = await api.getIngestSettings();
      const nextInputDir = String(data?.input_dir || '');
      setIngestInputDir(nextInputDir);
      setSavedIngestInputDir(nextInputDir);
      return nextInputDir;
    } catch (err) {
      setIngestMessage(err.message || 'Failed to load ingest settings.');
      return '';
    } finally {
      setIngestSettingsLoading(false);
    }
  };

  const persistIngestInputDir = async (inputDir = ingestInputDir) => {
    const trimmedInputDir = String(inputDir || '').trim();
    if (!trimmedInputDir) {
      throw new Error('Replay folder is required');
    }

    setIngestSettingsSaving(true);
    try {
      const data = await api.updateIngestSettings({ input_dir: trimmedInputDir });
      const nextInputDir = String(data?.input_dir || trimmedInputDir);
      setIngestInputDir(nextInputDir);
      setSavedIngestInputDir(nextInputDir);
      return nextInputDir;
    } finally {
      setIngestSettingsSaving(false);
    }
  };

  const showAutoIngestNotice = (message) => {
    if (autoIngestNoticeTimerRef.current) {
      window.clearTimeout(autoIngestNoticeTimerRef.current);
    }
    setAutoIngestNotice(message);
    autoIngestNoticeTimerRef.current = window.setTimeout(() => {
      setAutoIngestNotice('');
      autoIngestNoticeTimerRef.current = null;
    }, 3500);
  };

  const pollForReplayCountIncrease = async (baselineCount, intervalSeconds) => {
    const maxWaitMs = Math.max(5000, Math.floor(intervalSeconds * 1000 * 0.5));
    const stepMs = 3000;
    const attempts = Math.max(1, Math.floor(maxWaitMs / stepMs));

    for (let attempt = 0; attempt < attempts; attempt += 1) {
      await sleep(stepMs);
      try {
        const health = await api.getHealth();
        const totalReplays = Number(health?.total_replays || 0);
        if (totalReplays >= baselineCount + 1) {
          setReplayCount(totalReplays);
          setOpenaiEnabled(Boolean(health?.openai_enabled));
          return true;
        }
      } catch (err) {
        console.error('Failed to poll replay count after auto-ingest:', err);
      }
    }

    return false;
  };

  useEffect(() => {
    // Load dashboard with stored variable values if available
    const stored = getStoredVariableValues('default');
    loadDashboard('default', stored || undefined);
    loadDashboards();
    loadGlobalReplayFilterConfig().catch((err) => {
      console.error('Failed to load global replay filter config:', err);
    });
    loadGlobalReplayFilterOptions().catch((err) => {
      console.error('Failed to load global replay filter options:', err);
    });
    loadTopPlayerColors();
    checkOpenAIStatus();
  }, []);

  useEffect(() => {
    loadWorkflowGames({ page: workflowGamesPage, filters: workflowGamesFilters });
  }, [workflowGamesPage, workflowGamesFilters]);

  useEffect(() => {
    loadWorkflowPlayers({
      page: workflowPlayersPage,
      filters: workflowPlayersFilters,
      sortBy: workflowPlayersSortBy,
      sortDir: workflowPlayersSortDir,
    });
  }, [workflowPlayersPage, workflowPlayersFilters, workflowPlayersSortBy, workflowPlayersSortDir]);

  useEffect(() => {
    if (activeView !== 'players' || workflowPlayersTab !== 'apm-histogram') return;
    if (!workflowPlayersApmHistogram && !workflowPlayersApmHistogramLoading && !workflowPlayersApmHistogramError) {
      loadWorkflowPlayersApmHistogram();
    }
  }, [
    activeView,
    workflowPlayersTab,
    workflowPlayersApmHistogram,
    workflowPlayersApmHistogramLoading,
    workflowPlayersApmHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || workflowPlayersTab !== 'first-unit-delay') return;
    if (!workflowPlayersDelayHistogram && !workflowPlayersDelayHistogramLoading && !workflowPlayersDelayHistogramError) {
      loadWorkflowPlayersDelayHistogram();
    }
  }, [
    activeView,
    workflowPlayersTab,
    workflowPlayersDelayHistogram,
    workflowPlayersDelayHistogramLoading,
    workflowPlayersDelayHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || workflowPlayersTab !== 'unit-production-cadence') return;
    if (!workflowPlayersCadenceHistogram && !workflowPlayersCadenceHistogramLoading && !workflowPlayersCadenceHistogramError) {
      loadWorkflowPlayersCadenceHistogram();
    }
  }, [
    activeView,
    workflowPlayersTab,
    workflowPlayersCadenceHistogram,
    workflowPlayersCadenceHistogramLoading,
    workflowPlayersCadenceHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || workflowPlayersTab !== 'viewport-multitasking') return;
    if (!workflowPlayersViewportHistogram && !workflowPlayersViewportHistogramLoading && !workflowPlayersViewportHistogramError) {
      loadWorkflowPlayersViewportHistogram();
    }
  }, [
    activeView,
    workflowPlayersTab,
    workflowPlayersViewportHistogram,
    workflowPlayersViewportHistogramLoading,
    workflowPlayersViewportHistogramError,
  ]);

  useEffect(() => {
    saveAutoIngestSettings({
      enabled: ingestForm.autoIngestEnabled,
      intervalSeconds: ingestForm.autoIngestIntervalSeconds,
    });
  }, [ingestForm.autoIngestEnabled, ingestForm.autoIngestIntervalSeconds]);

  useEffect(() => {
    if (!showIngestPanel) {
      setIngestSocketState('closed');
      return undefined;
    }

    setIngestMessage('');
    void loadIngestSettings();
    setIngestSocketState('connecting');

    const socket = api.createIngestLogsSocket();
    ingestSocketRef.current = socket;

    socket.onopen = () => {
      setIngestSocketState('open');
    };

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        if (message.type === 'snapshot') {
          setIngestStatus(message.status || 'idle');
          setIngestLogs(hydrateIngestLogEntries(message.logs || []));
          if (message.error) {
            setIngestMessage(message.error);
          }
          return;
        }

        if (message.type === 'log' && message.log) {
          setIngestLogs((current) => mergeIngestLogEntries(current, message.log));
          return;
        }

        if (message.type === 'status') {
          setIngestStatus(message.status || 'idle');
          if (message.error) {
            setIngestMessage(message.error);
          } else if (message.status === 'running') {
            setIngestMessage('');
          } else if (message.status === 'completed') {
            setIngestMessage('Ingestion completed.');
            void refreshDataAfterGlobalReplayFilterSave();
          }
        }
      } catch (err) {
        console.error('Failed to parse ingest stream message:', err);
      }
    };

    socket.onerror = () => {
      setIngestSocketState('error');
    };

    socket.onclose = () => {
      if (ingestSocketRef.current === socket) {
        ingestSocketRef.current = null;
      }
      setIngestSocketState('closed');
    };

    return () => {
      if (ingestSocketRef.current === socket) {
        ingestSocketRef.current = null;
      }
      socket.close();
    };
  }, [showIngestPanel]);

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
        const health = await api.getHealth();
        const baselineCount = Number(health?.total_replays || 0);
        const ingestResponse = await api.startIngest({
          watch: false,
          stop_after_n_reps: 1,
          clean: false,
          store_right_clicks: false,
          skip_hotkeys: false,
        });
        if (!ingestResponse?.started) {
          return;
        }

        const didIncrease = await pollForReplayCountIncrease(baselineCount, intervalSeconds);
        if (didIncrease) {
          await refreshDataAfterGlobalReplayFilterSave();
          showAutoIngestNotice('auto-ingested new replays');
        }
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

  useEffect(() => () => {
    if (autoIngestNoticeTimerRef.current) {
      window.clearTimeout(autoIngestNoticeTimerRef.current);
    }
  }, []);

  const checkOpenAIStatus = async () => {
    try {
      const data = await api.getHealth();
      setOpenaiEnabled(Boolean(data?.openai_enabled));
      setReplayCount(typeof data?.total_replays === 'number' ? data.total_replays : 0);
      return data;
    } catch (err) {
      console.error('Failed to check OpenAI status:', err);
      return null;
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
      let nextInputDir = String(ingestInputDir || '').trim();
      if (!nextInputDir) {
        throw new Error('Replay folder is required');
      }
      if (nextInputDir !== String(savedIngestInputDir || '').trim()) {
        nextInputDir = await persistIngestInputDir(nextInputDir);
      }

      const response = await api.startIngest({
        input_dir: nextInputDir,
        watch: ingestForm.watch,
        stop_after_n_reps: ingestForm.stopAfterN || 0,
        clean: ingestForm.clean,
        store_right_clicks: ingestForm.storeRightClicks,
        skip_hotkeys: ingestForm.skipHotkeys,
      });

      if (response?.started) {
        setIngestStatus('running');
        setIngestLogs([]);
        setIngestMessage('');
        return;
      }

      if (response?.in_progress) {
        setIngestStatus('running');
        setIngestMessage('Ingestion is already in progress.');
        return;
      }
    } catch (err) {
      setIngestMessage(err.message || 'Failed to start ingestion.');
    }
  };

  const handleSaveIngestInputDir = async () => {
    setIngestMessage('');
    try {
      await persistIngestInputDir(ingestInputDir);
      setIngestMessage('Replay folder saved.');
    } catch (err) {
      setIngestMessage(err.message || 'Failed to save replay folder.');
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

  const refreshDataAfterGlobalReplayFilterSave = async () => {
    await Promise.all([
      loadWorkflowGames({ page: workflowGamesPage, filters: workflowGamesFilters }),
      loadWorkflowPlayers({
        page: workflowPlayersPage,
        filters: workflowPlayersFilters,
        sortBy: workflowPlayersSortBy,
        sortDir: workflowPlayersSortDir,
      }),
      loadDashboard(currentDashboardUrl, variableValues, true),
      loadTopPlayerColors(),
      checkOpenAIStatus(),
      loadGlobalReplayFilterOptions(),
    ]);

    if (activeView === 'game' && selectedReplayId) {
      try {
        await openWorkflowGame(selectedReplayId);
      } catch (err) {
        console.error('Failed to reload workflow game after global filter save:', err);
      }
    }
    if (activeView === 'player' && selectedPlayerKey) {
      try {
        await openWorkflowPlayer(selectedPlayerKey);
      } catch (err) {
        console.error('Failed to reload workflow player after global filter save:', err);
      }
    }
    if (workflowPlayersApmHistogram) {
      loadWorkflowPlayersApmHistogram();
    }
    if (workflowPlayersDelayHistogram) {
      loadWorkflowPlayersDelayHistogram();
    }
    if (workflowPlayersCadenceHistogram) {
      loadWorkflowPlayersCadenceHistogram();
    }
  };

  const handleSaveGlobalReplayFilter = async (nextConfig) => {
    try {
      setGlobalReplayFilterSaving(true);
      setGlobalReplayFilterError('');
      const saved = await api.updateGlobalReplayFilter(nextConfig);
      setGlobalReplayFilterConfig(saved);
      await refreshDataAfterGlobalReplayFilterSave();
      setShowGlobalReplayFilter(false);
    } catch (err) {
      setGlobalReplayFilterError(err.message || 'Failed to save main config');
    } finally {
      setGlobalReplayFilterSaving(false);
    }
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

  const setWorkflowPlayersSingleFilter = (name, nextValue) => {
    setWorkflowPlayersPage(1);
    setWorkflowPlayersFilters((prev) => ({
      ...prev,
      [name]: nextValue,
    }));
  };

  const toggleWorkflowPlayersMultiFilter = (name, value) => {
    setWorkflowPlayersPage(1);
    setWorkflowPlayersFilters((prev) => ({
      ...prev,
      [name]: toggleFilterValue(prev[name] || [], value),
    }));
  };

  const clearWorkflowPlayersFilters = () => {
    setWorkflowPlayersPage(1);
    setWorkflowPlayersFilters({
      name: '',
      onlyFivePlus: false,
      lastPlayed: [],
    });
    setWorkflowPlayersSortBy('games');
    setWorkflowPlayersSortDir('desc');
  };

  const setWorkflowPlayersSort = (sortBy) => {
    setWorkflowPlayersPage(1);
    setWorkflowPlayersSortBy((prevSortBy) => {
      if (prevSortBy === sortBy) {
        setWorkflowPlayersSortDir((prevDir) => (prevDir === 'asc' ? 'desc' : 'asc'));
        return prevSortBy;
      }
      setWorkflowPlayersSortDir(sortBy === 'games' || sortBy === 'last_played' ? 'desc' : 'asc');
      return sortBy;
    });
  };

  const toggleWorkflowPlayersDelayCase = (caseKey) => {
    const normalized = String(caseKey || '').trim();
    if (!normalized) return;
    setWorkflowPlayersDelaySelectedCases((prev) => {
      const current = Array.isArray(prev) ? prev : ['all'];
      if (normalized === 'all') return ['all'];
      const withoutAll = current.filter((value) => value && value !== 'all');
      const already = withoutAll.includes(normalized);
      if (already) {
        const next = withoutAll.filter((value) => value !== normalized);
        return next.length === 0 ? ['all'] : next;
      }
      return [...withoutAll, normalized];
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

  const openWorkflowPlayersSubview = (tab) => {
    const nextTab = String(tab || 'summary');
    setWorkflowPlayersTab(nextTab);
    navigateWorkflowView('players');
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
  const workflowPlayerOutlierItems = workflowPlayerOutliers?.items || [];

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

  const workflowGamePlayers = workflowGame?.players || [];
  const workflowPlayerInsights = [
    workflowPlayerViewportInsight,
    workflowPlayerApmInsight,
    workflowPlayerDelayInsight,
    workflowPlayerCadenceInsight,
  ].filter(Boolean);
  const workflowPlayerInsightLoading = workflowPlayerApmInsightLoading || workflowPlayerDelayInsightLoading || workflowPlayerCadenceInsightLoading || workflowPlayerViewportInsightLoading;
  const workflowPlayerInsightErrors = [
    workflowPlayerApmInsightError,
    workflowPlayerDelayInsightError,
    workflowPlayerCadenceInsightError,
    workflowPlayerViewportInsightError,
  ].filter(Boolean);
  const workflowPlayerUsagePills = useMemo(() => {
    const pills = [];
    if ((Number(workflowPlayer?.hotkey_usage_rate) || 0) < LOW_USAGE_THRESHOLD) {
      pills.push({
        key: 'no-hotkeys',
        label: 'Doesn\'t use hotkeys',
        title: `Detected in ${(Number(workflowPlayer?.hotkey_usage_rate) * 100).toFixed(1)}% of this player's games.`,
        className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey',
      });
    }
    if ((Number(workflowPlayer?.queued_game_rate) || 0) < LOW_USAGE_THRESHOLD) {
      pills.push({
        key: 'no-queued-orders',
        label: 'Doesn\'t use queued orders',
        title: `Detected in ${(Number(workflowPlayer?.queued_game_rate) * 100).toFixed(1)}% of this player's games.`,
        className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-queued',
      });
    }
    return pills;
  }, [workflowPlayer]);
  const workflowPlayerNameWidthCh = useMemo(() => {
    const longestNameLength = workflowGamePlayers.reduce((longest, player) => {
      const nameLength = String(player?.name || '').trim().length;
      return Math.max(longest, nameLength);
    }, 0);
    if (!longestNameLength) return 15;
    return Math.max(12, Math.min(24, longestNameLength + 3));
  }, [workflowGamePlayers]);
  const workflowPlayersById = useMemo(
    () => new Map(workflowGamePlayers.map((player) => [player.player_id, player])),
    [workflowGamePlayers],
  );
  const hasTeamInfo = useMemo(() => {
    const uniqueTeams = new Set(workflowGamePlayers.map((player) => player.team));
    return uniqueTeams.size > 1;
  }, [workflowGamePlayers]);
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
      const mappedPoints = sourcePoints
        .map((point) => {
          const second = Number(point?.second);
          if (!Number.isFinite(second)) return null;
          const order = Number(point?.order) || 0;
          const rawLabel = String(point?.label || '').trim();
          const upgradeCategory = workflowTimingCategoryConfig.source === 'upgrades' ? upgradeCategoryForName(rawLabel) : '';
          if (workflowTimingCategoryConfig.source === 'upgrades' && upgradeCategory !== workflowTimingCategory) return null;
          return {
            ...point,
            second,
            order,
            label: rawLabel,
            upgrade_category: upgradeCategory,
          };
        })
        .filter(Boolean);

      // Post-process noisy repeated commands:
      // - HP upgrades are repeatable up to 3 levels, so keep latest 3 per label.
      // - Other upgrades and tech are effectively one-off, so keep latest 1 per label.
      const pointsAfterPostProcess = (() => {
        const sourceType = workflowTimingCategoryConfig.source;
        if (sourceType !== 'upgrades' && sourceType !== 'tech') return mappedPoints;
        const byLabel = new Map();
        mappedPoints.forEach((point) => {
          const key = String(point?.label || '').trim();
          if (!key) return;
          if (!byLabel.has(key)) byLabel.set(key, []);
          byLabel.get(key).push(point);
        });
        const collapsed = [];
        byLabel.forEach((items) => {
          const sortedBySecond = [...items].sort((a, b) => {
            if (a.second === b.second) return a.order - b.order;
            return a.second - b.second;
          });
          const keepCount = sourceType === 'upgrades' && workflowTimingCategory === 'hp_upgrades' ? 3 : 1;
          const kept = sortedBySecond.slice(-keepCount);
          kept.forEach((item, idx) => {
            collapsed.push({
              ...item,
              order: idx + 1,
            });
          });
        });
        return collapsed.sort((a, b) => {
          if (a.second === b.second) return String(a.label || '').localeCompare(String(b.label || ''));
          return a.second - b.second;
        });
      })();

      const points = pointsAfterPostProcess.map((point) => {
        const order = Number(point?.order) || 0;
        const rawLabel = String(point?.label || '').trim();
        const upgradeCategory = String(point?.upgrade_category || '').trim();
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
      });

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
      const racePrefix = racePrefixForUpgrade(race);
      const labelOptions = Array.from(new Set(
        raceSeries.flatMap((playerSeries) => (playerSeries?.points || []).map((point) => String(point?.label || '').trim()))
          .filter((label) => {
            if (!label) return false;
            if (!racePrefix) return true;
            return label.startsWith(racePrefix);
          }),
      )).sort((a, b) => a.localeCompare(b));
      const selectedValue = String(workflowHpUpgradeFilters[race] || '').trim();
      const defaultForRace = String(DEFAULT_HP_UPGRADE_BY_RACE[race] || '').trim();
      const selected = labelOptions.includes(selectedValue)
        ? selectedValue
        : (labelOptions.includes(defaultForRace) ? defaultForRace : (labelOptions[0] || ''));
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
  const workflowFirstUnitEfficiencyGroups = useMemo(() => {
    const sourcePlayers = Array.isArray(workflowGame?.first_unit_efficiency) ? workflowGame.first_unit_efficiency : [];
    const normalizedPlayers = sourcePlayers.map((playerEntry) => ({
      ...playerEntry,
      race: String(playerEntry?.race || '').trim().toLowerCase(),
      entries: Array.isArray(playerEntry?.entries) ? playerEntry.entries : [],
    }));
    return FIRST_UNIT_EFFICIENCY_GROUP_CONFIG.map((cfg) => {
      const unitKeySet = new Set(cfg.unitNames.map((name) => normalizeUnitName(name)));
      const rows = normalizedPlayers
        .filter((playerEntry) => playerEntry.race === cfg.race)
        .map((playerEntry) => {
          const matched = playerEntry.entries.find((entry) => (
            normalizeUnitName(entry?.building_name) === normalizeUnitName(cfg.buildingName)
            && unitKeySet.has(normalizeUnitName(entry?.unit_name))
          ));
          if (!matched) return null;
          return {
            player_id: playerEntry.player_id,
            player_name: playerEntry.name,
            player_key: playerEntry.player_key,
            race: playerEntry.race,
            ...matched,
            building_icon: getUnitIcon(matched?.building_name || cfg.buildingName),
            unit_icon: getUnitIcon(matched?.unit_name),
          };
        })
        .filter(Boolean)
        .sort((a, b) => String(a?.player_name || '').localeCompare(String(b?.player_name || '')));
      if (rows.length === 0) return null;
      return {
        id: `${cfg.race}-${normalizeUnitName(cfg.buildingName)}`,
        race: cfg.race,
        building_name: cfg.buildingName,
        building_icon: getUnitIcon(cfg.buildingName),
        unit_names: cfg.unitNames,
        unit_icons: cfg.unitNames
          .map((unitName) => getUnitIcon(unitName))
          .filter(Boolean),
        rows,
      };
    }).filter(Boolean);
  }, [workflowGame?.first_unit_efficiency]);

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
  const workflowPlayersTotalPages = Math.max(1, Math.ceil((Number(workflowPlayersTotal) || 0) / WORKFLOW_PLAYERS_PAGE_SIZE));
  const workflowPlayersFrom = workflowPlayers.length === 0 ? 0 : ((workflowPlayersPage - 1) * WORKFLOW_PLAYERS_PAGE_SIZE) + 1;
  const workflowPlayersTo = workflowPlayers.length === 0
    ? 0
    : Math.min((workflowPlayersPage - 1) * WORKFLOW_PLAYERS_PAGE_SIZE + workflowPlayers.length, Number(workflowPlayersTotal) || 0);
  const playersApmHistogramPoints = useMemo(() => (
    (workflowPlayersApmHistogram?.players || [])
      .map((player) => ({
        value: Number(player?.average_apm),
        label: String(player?.player_name || '').trim(),
        player_key: String(player?.player_key || '').trim(),
        games_played: Number(player?.games_played || 0),
      }))
      .filter((player) => Number.isFinite(player.value) && player.label)
  ), [workflowPlayersApmHistogram]);
  const workflowPlayersApmProcessed = useMemo(() => {
    const minGames = Math.max(5, Number(workflowPlayersApmMinGames) || 5);
    const filtered = playersApmHistogramPoints
      .filter((player) => Number(player.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.games_played,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersApmHistogramPoints, workflowPlayersApmMinGames]);
  const workflowPlayersDelayCaseOptions = useMemo(() => (
    (workflowPlayersDelayHistogram?.case_options || [])
      .map((entry) => ({
        case_key: String(entry?.case_key || '').trim(),
        building_name: String(entry?.building_name || '').trim(),
        unit_name: String(entry?.unit_name || '').trim(),
        sample_count: Number(entry?.sample_count || 0),
      }))
      .filter((entry) => entry.case_key && entry.building_name && entry.unit_name)
  ), [workflowPlayersDelayHistogram]);
  const playersDelayHistogramPoints = useMemo(() => {
    const selected = new Set((workflowPlayersDelaySelectedCases || []).filter((value) => value && value !== 'all'));
    const useAll = selected.size === 0 || (workflowPlayersDelaySelectedCases || []).includes('all');
    return (workflowPlayersDelayHistogram?.players || [])
      .map((player) => {
        const caseAverages = Array.isArray(player?.case_averages) ? player.case_averages : [];
        const matched = caseAverages.filter((entry) => {
          const caseKey = String(entry?.case_key || '').trim();
          if (!caseKey) return false;
          if (useAll) return true;
          return selected.has(caseKey);
        });
        if (matched.length === 0) return null;
        const sampleCount = matched.reduce((sum, entry) => sum + (Number(entry?.sample_count || 0)), 0);
        if (sampleCount <= 0) return null;
        const weightedSum = matched.reduce((sum, entry) => (
          sum + (Number(entry?.average_delay_seconds || 0) * Number(entry?.sample_count || 0))
        ), 0);
        const avgDelay = weightedSum / sampleCount;
        return {
          value: avgDelay,
          label: String(player?.player_name || '').trim(),
          player_key: String(player?.player_key || '').trim(),
          sample_count: sampleCount,
        };
      })
      .filter((player) => player && Number.isFinite(player.value) && player.label);
  }, [workflowPlayersDelayHistogram, workflowPlayersDelaySelectedCases]);
  const workflowPlayersDelayProcessed = useMemo(() => {
    const minSamples = Math.max(5, Number(workflowPlayersDelayMinSamples) || 5);
    const filtered = playersDelayHistogramPoints
      .filter((player) => Number(player.sample_count || 0) >= minSamples)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.sample_count,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersDelayHistogramPoints, workflowPlayersDelayMinSamples]);
  const playersCadenceHistogramPoints = useMemo(() => (
    (workflowPlayersCadenceHistogram?.players || [])
      .map((player) => ({
        value: Number(player?.average_cadence_score),
        label: String(player?.player_name || '').trim(),
        player_key: String(player?.player_key || '').trim(),
        games_played: Number(player?.games_used || 0),
        average_rate_per_min: Number(player?.average_rate_per_min || 0),
        average_cv_gap: Number(player?.average_cv_gap || 0),
        average_burstiness: Number(player?.average_burstiness || 0),
        average_idle20_ratio: Number(player?.average_idle20_ratio || 0),
      }))
      .filter((player) => Number.isFinite(player.value) && player.label)
  ), [workflowPlayersCadenceHistogram]);
  const workflowPlayersCadenceProcessed = useMemo(() => {
    const minGames = Math.max(4, Number(workflowPlayersCadenceMinGames) || 4);
    const filtered = playersCadenceHistogramPoints
      .filter((player) => Number(player.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.games_played,
        average_rate_per_min: player.average_rate_per_min,
        average_cv_gap: player.average_cv_gap,
        average_burstiness: player.average_burstiness,
        average_idle20_ratio: player.average_idle20_ratio,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersCadenceHistogramPoints, workflowPlayersCadenceMinGames]);
  const workflowPlayersViewportProcessed = useMemo(() => {
    const minGames = Math.max(4, Number(workflowPlayersViewportMinGames) || 4);
    const filtered = (workflowPlayersViewportHistogram?.players || [])
      .filter((player) => Number(player?.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_CONFIG.playerField] || 0),
        games_played: Number(player?.games_played || 0),
        average_viewport_switch_rate: Number(player?.average_viewport_switch_rate || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0);
    return buildHistogramSummaryFromPlayers(filtered);
  }, [workflowPlayersViewportHistogram, workflowPlayersViewportMinGames]);
  const workflowGameCadenceProcessed = useMemo(() => {
    const rows = (workflowGame?.unit_production_cadence || [])
      .filter((player) => Boolean(player?.eligible))
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.cadence_score || 0),
        games_played: Number(player?.units_produced || 0),
        average_rate_per_min: Number(player?.rate_per_minute || 0),
        average_cv_gap: Number(player?.cv_gap || 0),
        average_burstiness: Number(player?.burstiness || 0),
        average_idle20_ratio: Number(player?.idle20_ratio || 0),
        window_seconds: Number(player?.window_seconds || 0),
        gap_count: Number(player?.gap_count || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm > 0);
    return buildHistogramSummaryFromPlayers(rows);
  }, [workflowGame]);
  const workflowGameViewportProcessed = useMemo(() => {
    const rows = (workflowGame?.viewport_multitasking || [])
      .filter((player) => Boolean(player?.eligible))
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_CONFIG.gameField] || 0),
        games_played: 1,
        viewport_switch_rate: Number(player?.viewport_switch_rate || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0);
    return buildHistogramSummaryFromPlayers(rows);
  }, [workflowGame]);
  const workflowPlayersSortIndicator = (sortBy) => {
    if (workflowPlayersSortBy !== sortBy) return '';
    return workflowPlayersSortDir === 'asc' ? '↑' : '↓';
  };

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
          <button className={`btn-manage ${activeView === 'players' ? 'workflow-nav-active' : ''}`} onClick={() => navigateWorkflowView('players')}>Players</button>
          <button onClick={() => {
            setGlobalReplayFilterError('');
            loadGlobalReplayFilterConfig().catch((err) => {
              console.error('Failed to refresh global replay filter config:', err);
            });
            loadGlobalReplayFilterOptions().catch((err) => {
              console.error('Failed to refresh global replay filter options:', err);
            });
            setShowGlobalReplayFilter(true);
          }} className="btn-manage">Settings</button>
          <button onClick={() => setShowIngestPanel(true)} className="btn-manage">Ingest</button>
          <button className={`btn-manage ${activeView === 'dashboards' ? 'workflow-nav-active' : ''}`} onClick={() => navigateWorkflowView('dashboards')}>Custom Dashboards</button>
        </div>

        {error && <div className="error-message">{error}</div>}

        {activeView === 'games' && (
          <div className="workflow-panel">
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

        {activeView === 'players' && (
          <div className="workflow-panel">
            <div className="workflow-nav">
              <button className={`btn-switch ${workflowPlayersTab === 'summary' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowPlayersTab('summary')}>Summary</button>
              <button className={`btn-switch ${workflowPlayersTab === 'apm-histogram' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowPlayersTab('apm-histogram')}>APM Histogram</button>
              <button className={`btn-switch ${workflowPlayersTab === 'first-unit-delay' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowPlayersTab('first-unit-delay')}>First Unit Delay</button>
              <button className={`btn-switch ${workflowPlayersTab === 'unit-production-cadence' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowPlayersTab('unit-production-cadence')}>Unit Production Cadence</button>
              <button className={`btn-switch ${workflowPlayersTab === 'viewport-multitasking' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowPlayersTab('viewport-multitasking')}>Viewport Multitasking</button>
            </div>

            {workflowPlayersTab === 'summary' ? (
              <>
                <div className="workflow-summary-filter-row workflow-games-filter-row">
                  <input
                    type="text"
                    className="workflow-summary-filter-input"
                    placeholder="Filter by player name..."
                    value={workflowPlayersFilters.name}
                    onChange={(e) => setWorkflowPlayersSingleFilter('name', e.target.value)}
                  />
                  <label className="workflow-summary-filter-check">
                    <input
                      type="checkbox"
                      checked={Boolean(workflowPlayersFilters.onlyFivePlus)}
                      onChange={(e) => setWorkflowPlayersSingleFilter('onlyFivePlus', e.target.checked)}
                    />
                    <span>Only 5+ games</span>
                  </label>
                  <div className="workflow-pattern-pills workflow-games-filter-pills">
                    {(workflowPlayersFilterOptions.last_played || []).map((option) => {
                      const active = (workflowPlayersFilters.lastPlayed || []).includes(option.key);
                      return (
                        <button
                          key={`wf-player-last-${option.key}`}
                          type="button"
                          className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                          onClick={() => toggleWorkflowPlayersMultiFilter('lastPlayed', option.key)}
                        >
                          {option.label} ({option.count})
                        </button>
                      );
                    })}
                  </div>
                  <button type="button" className="btn-create-manual" onClick={clearWorkflowPlayersFilters}>Clear filters</button>
                </div>
                {workflowPlayersLoading ? (
                  <div className="loading">Loading players...</div>
                ) : (
                  <>
                    <table className="data-table workflow-table">
                      <thead>
                        <tr>
                          <th className="workflow-sortable" onClick={() => setWorkflowPlayersSort('name')}>Name {workflowPlayersSortIndicator('name')}</th>
                          <th className="workflow-sortable" onClick={() => setWorkflowPlayersSort('race')}>Race {workflowPlayersSortIndicator('race')}</th>
                          <th className="workflow-sortable" onClick={() => setWorkflowPlayersSort('games')}>Games {workflowPlayersSortIndicator('games')}</th>
                          <th className="workflow-sortable" onClick={() => setWorkflowPlayersSort('apm')}>Avg APM {workflowPlayersSortIndicator('apm')}</th>
                          <th className="workflow-sortable" onClick={() => setWorkflowPlayersSort('last_played')}>Last played {workflowPlayersSortIndicator('last_played')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {workflowPlayers.map((player) => (
                          <tr key={player.player_key} className={selectedPlayerKey === player.player_key ? 'workflow-selected-row' : ''} onClick={() => openWorkflowPlayer(player.player_key)}>
                            <td style={playerAccentColor(player.player_key) ? { color: playerAccentColor(player.player_key), fontWeight: 600 } : undefined}>{player.player_name}</td>
                            <td>{player.race}</td>
                            <td>{player.games_played}</td>
                            <td>{Number(player.average_apm || 0).toFixed(1)}</td>
                            <td>{formatDaysAgoCompact(player.last_played_days_ago)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    <div className="workflow-pagination-row">
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={workflowPlayersPage <= 1 || workflowPlayersLoading}
                        onClick={() => setWorkflowPlayersPage((prev) => Math.max(1, prev - 1))}
                      >
                        Previous
                      </button>
                      <span>
                        Page {workflowPlayersPage} / {workflowPlayersTotalPages} - Showing {workflowPlayersFrom}-{workflowPlayersTo} of {workflowPlayersTotal}
                      </span>
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={workflowPlayersPage >= workflowPlayersTotalPages || workflowPlayersLoading}
                        onClick={() => setWorkflowPlayersPage((prev) => Math.min(workflowPlayersTotalPages, prev + 1))}
                      >
                        Next
                      </button>
                    </div>
                  </>
                )}
              </>
            ) : workflowPlayersTab === 'apm-histogram' ? (
              <div className="workflow-card workflow-card-fingerprints">
                <div className="workflow-card-title"><span>APM distribution</span></div>
                <div className="workflow-card-subtitle">
                  <span>How it is calculated</span>
                  <HelpTooltip text="Each player contributes one point: their average APM across non-observer games. Only players with 5+ games are included." label="APM histogram methodology" />
                </div>
                <div className="workflow-subtle-note">
                  Single-bell view over the eligible player population. Dots and labels can move vertically to reduce overlap, but each dot keeps the exact horizontal APM location.
                </div>
                {workflowPlayersApmHistogramLoading ? <div className="chart-empty">Loading APM histogram...</div> : null}
                {!workflowPlayersApmHistogramLoading && workflowPlayersApmHistogramError ? <div className="chart-empty">{workflowPlayersApmHistogramError}</div> : null}
                {!workflowPlayersApmHistogramLoading && !workflowPlayersApmHistogramError && workflowPlayersApmProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough player data to render this histogram yet.</div>
                ) : null}
                {!workflowPlayersApmHistogramLoading && !workflowPlayersApmHistogramError && workflowPlayersApmProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(5, Number(workflowPlayersApmMinGames) || 5)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="5"
                        max={String(Math.max(5, Number(workflowPlayersApmProcessed.maxGames) || 5))}
                        step="1"
                        value={String(Math.max(5, Number(workflowPlayersApmMinGames) || 5))}
                        onChange={(e) => setWorkflowPlayersApmMinGames(Math.max(5, Number(e.target.value) || 5))}
                      />
                    </div>
                    <div className="workflow-subtle-note">
                      This slider only filters already-loaded points client-side; it does not re-query the backend.
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: workflowPlayersApmProcessed.bins,
                        x_axis_label: 'Average APM',
                        y_axis_label: 'Density',
                        mean: workflowPlayersApmProcessed.mean,
                        stddev: workflowPlayersApmProcessed.stddev,
                        chart_height: 620,
                        overlay_points: workflowPlayersApmProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                        })),
                        on_overlay_point_click: openWorkflowPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(workflowPlayersApmProcessed.playersIncluded) || 0} players (>=${Math.max(5, Number(workflowPlayersApmMinGames) || 5)} games). Mean ${Number(workflowPlayersApmProcessed.mean || 0).toFixed(1)} APM, stddev ${Number(workflowPlayersApmProcessed.stddev || 0).toFixed(1)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : workflowPlayersTab === 'first-unit-delay' ? (
              <div className="workflow-card workflow-card-fingerprints">
                <div className="workflow-card-title"><span>First-unit delay distribution</span></div>
                <div className="workflow-card-subtitle">
                  <span>How it is calculated</span>
                  <HelpTooltip text="Each player contributes one point: their average (building ready -> first matching unit) delay over all valid observations. Observations are generated from the same race-specific mappings used by game-level First Unit Efficiency, but only for command events up to 7:00 game time. Gaps must be in the 0-20 second range." label="First-unit delay methodology" />
                </div>
                <div className="workflow-subtle-note">
                  Smaller values mean players tend to start the first matching unit sooner after a production building is expected to be ready.
                </div>
                <div className="workflow-subtle-note">
                  This is an imperfect proxy for execution quality. It can be distorted by worker travel, scouting pressure, map geometry, intentional tech pivots, and command-timestamp limitations.
                </div>
                <div className="workflow-subtle-note">
                  Cutoff rules: only build/train/morph commands at or before 7:00 are included, and a matching unit must be created within 20s of building ready time.
                </div>
                {!workflowPlayersDelayHistogramLoading && !workflowPlayersDelayHistogramError ? (
                  <>
                    <div className="workflow-card-subtitle"><span>Included building to unit cases</span></div>
                    <div className="workflow-pattern-pills workflow-games-filter-pills">
                      <button
                        type="button"
                        className={`workflow-filter-pill ${(workflowPlayersDelaySelectedCases || []).includes('all') ? 'workflow-filter-pill-active' : ''}`}
                        onClick={() => toggleWorkflowPlayersDelayCase('all')}
                      >
                        All
                      </button>
                      {workflowPlayersDelayCaseOptions.map((option) => {
                        const active = (workflowPlayersDelaySelectedCases || []).includes(option.case_key);
                        return (
                          <button
                            key={`wf-delay-case-${option.case_key}`}
                            type="button"
                            className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                            onClick={() => toggleWorkflowPlayersDelayCase(option.case_key)}
                          >
                            {`${option.building_name} -> ${option.unit_name} (${Number(option.sample_count || 0)})`}
                          </button>
                        );
                      })}
                    </div>
                  </>
                ) : null}
                {workflowPlayersDelayHistogramLoading ? <div className="chart-empty">Loading first-unit delay...</div> : null}
                {!workflowPlayersDelayHistogramLoading && workflowPlayersDelayHistogramError ? <div className="chart-empty">{workflowPlayersDelayHistogramError}</div> : null}
                {!workflowPlayersDelayHistogramLoading && !workflowPlayersDelayHistogramError && workflowPlayersDelayProcessed.points.length === 0 ? (
                  <div className="chart-empty">
                    Not enough player delay samples to render this distribution yet.
                    {!(workflowPlayersDelaySelectedCases || []).includes('all') ? (
                      <>
                        {' '}
                        <button
                          type="button"
                          className="workflow-link-btn"
                          onClick={() => setWorkflowPlayersDelaySelectedCases(['all'])}
                        >
                          Clear case filters
                        </button>
                      </>
                    ) : null}
                  </div>
                ) : null}
                {!workflowPlayersDelayHistogramLoading && !workflowPlayersDelayHistogramError && workflowPlayersDelayProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min samples (post-process): {Math.max(5, Number(workflowPlayersDelayMinSamples) || 5)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="5"
                        max={String(Math.max(5, Number(workflowPlayersDelayProcessed.maxGames) || 5))}
                        step="1"
                        value={String(Math.max(5, Number(workflowPlayersDelayMinSamples) || 5))}
                        onChange={(e) => setWorkflowPlayersDelayMinSamples(Math.max(5, Number(e.target.value) || 5))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: workflowPlayersDelayProcessed.bins,
                        x_axis_label: 'Average delay (seconds)',
                        y_axis_label: 'Density',
                        overlay_value_label: 's delay',
                        overlay_count_label: 'samples',
                        mean: workflowPlayersDelayProcessed.mean,
                        stddev: workflowPlayersDelayProcessed.stddev,
                        chart_height: 620,
                        overlay_points: workflowPlayersDelayProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                        })),
                        on_overlay_point_click: openWorkflowPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(workflowPlayersDelayProcessed.playersIncluded) || 0} players (>=${Math.max(5, Number(workflowPlayersDelayMinSamples) || 5)} samples). Mean ${Number(workflowPlayersDelayProcessed.mean || 0).toFixed(1)}s, stddev ${Number(workflowPlayersDelayProcessed.stddev || 0).toFixed(1)}s.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : workflowPlayersTab === 'unit-production-cadence' ? (
              <div className="workflow-card workflow-card-fingerprints">
                <div className="workflow-card-title"><span>Unit production cadence distribution</span></div>
                <div className="workflow-card-subtitle">
                  <span>How it is calculated</span>
                  <HelpTooltip text="For each replay-player we only inspect attacking-unit production command timestamps from 7:00 to 80% game time. We compute interval evenness (cvGap = std(gaps)/mean(gaps)) and production rate (units per minute), then combine them into cadenceScore = ratePerMin / (1 + cvGap). Each player contributes one point: average cadenceScore across eligible games." label="Unit production cadence methodology" />
                </div>
                <div className="workflow-subtle-note">
                  Rationale: stronger macro tends to keep attacking-unit production frequent and less clumped. This score rewards high sustained rate and penalizes bursty long gaps.
                </div>
                <div className="workflow-subtle-note">
                  Strict filter excludes workers/econ and support utility units to focus on combat production rhythm.
                </div>
                {workflowPlayersCadenceHistogramLoading ? <div className="chart-empty">Loading unit production cadence...</div> : null}
                {!workflowPlayersCadenceHistogramLoading && workflowPlayersCadenceHistogramError ? <div className="chart-empty">{workflowPlayersCadenceHistogramError}</div> : null}
                {!workflowPlayersCadenceHistogramLoading && !workflowPlayersCadenceHistogramError && workflowPlayersCadenceProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough cadence data to render this distribution yet.</div>
                ) : null}
                {!workflowPlayersCadenceHistogramLoading && !workflowPlayersCadenceHistogramError && workflowPlayersCadenceProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(4, Number(workflowPlayersCadenceMinGames) || 4)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="4"
                        max={String(Math.max(4, Number(workflowPlayersCadenceProcessed.maxGames) || 4))}
                        step="1"
                        value={String(Math.max(4, Number(workflowPlayersCadenceMinGames) || 4))}
                        onChange={(e) => setWorkflowPlayersCadenceMinGames(Math.max(4, Number(e.target.value) || 4))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: workflowPlayersCadenceProcessed.bins,
                        x_axis_label: 'Average cadence score',
                        y_axis_label: 'Density',
                        overlay_value_label: 'cadence',
                        overlay_count_label: 'games',
                        mean: workflowPlayersCadenceProcessed.mean,
                        stddev: workflowPlayersCadenceProcessed.stddev,
                        chart_height: 620,
                        overlay_points: workflowPlayersCadenceProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                          tooltip_lines: [
                            `${String(player.player_name || '')}`,
                            `Cadence score: ${Number(player.average_apm || 0).toFixed(3)}`,
                            `Rate per minute: ${Number(player.average_rate_per_min || 0).toFixed(2)}`,
                            `Gap CV: ${Number(player.average_cv_gap || 0).toFixed(2)}`,
                            `Burstiness: ${Number(player.average_burstiness || 0).toFixed(2)}`,
                            `Idle gap ratio (>=20s): ${(Number(player.average_idle20_ratio || 0) * 100).toFixed(1)}%`,
                            `Games used: ${Number(player.games_played || 0)}`,
                          ],
                        })),
                        on_overlay_point_click: openWorkflowPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(workflowPlayersCadenceProcessed.playersIncluded) || 0} players (>=${Math.max(4, Number(workflowPlayersCadenceMinGames) || 4)} games). Mean ${Number(workflowPlayersCadenceProcessed.mean || 0).toFixed(3)}, stddev ${Number(workflowPlayersCadenceProcessed.stddev || 0).toFixed(3)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : workflowPlayersTab === 'viewport-multitasking' ? (
              <div className="workflow-card workflow-card-fingerprints">
                <div className="workflow-card-title"><span>Viewport multitasking distribution</span></div>
                <div className="workflow-card-subtitle">
                  <span>How it is calculated</span>
                  <HelpTooltip text="For each replay-player we inspect coordinate-bearing commands from 7:00 until 80% of game length. A switch happens when the next command lands outside the previous viewport-sized box (22x16 tiles, 32 pixels per tile). The detector stores a JSON payload per replay-player with Viewport Switch Rate, SameViewportShare, and SameViewportMedianRun." label="Viewport multitasking methodology" />
                </div>
                <div className="workflow-subtle-note">
                  {VIEWPORT_SWITCH_RATE_CONFIG.interpretation}
                </div>
                {workflowPlayersViewportHistogramLoading ? <div className="chart-empty">Loading viewport multitasking...</div> : null}
                {!workflowPlayersViewportHistogramLoading && workflowPlayersViewportHistogramError ? <div className="chart-empty">{workflowPlayersViewportHistogramError}</div> : null}
                {!workflowPlayersViewportHistogramLoading && !workflowPlayersViewportHistogramError && workflowPlayersViewportProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough viewport multitasking data to render this distribution yet.</div>
                ) : null}
                {!workflowPlayersViewportHistogramLoading && !workflowPlayersViewportHistogramError && workflowPlayersViewportProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(4, Number(workflowPlayersViewportMinGames) || 4)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="4"
                        max={String(Math.max(4, Number(workflowPlayersViewportProcessed.maxGames) || 4))}
                        step="1"
                        value={String(Math.max(4, Number(workflowPlayersViewportMinGames) || 4))}
                        onChange={(e) => setWorkflowPlayersViewportMinGames(Math.max(4, Number(e.target.value) || 4))}
                      />
                    </div>
                    <div className="workflow-subtle-note">
                      {`Backend gate: at least ${Number(workflowPlayersViewportHistogram?.min_games || 0)} games. The slider only filters the already-loaded population client-side.`}
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: workflowPlayersViewportProcessed.bins,
                        x_axis_label: VIEWPORT_SWITCH_RATE_CONFIG.axisLabel,
                        y_axis_label: 'Density',
                        overlay_value_label: VIEWPORT_SWITCH_RATE_CONFIG.overlayValueLabel,
                        overlay_count_label: 'games',
                        mean: workflowPlayersViewportProcessed.mean,
                        stddev: workflowPlayersViewportProcessed.stddev,
                        chart_height: 620,
                        overlay_points: workflowPlayersViewportProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                          tooltip_lines: [
                            `${String(player.player_name || '')}`,
                            `${VIEWPORT_SWITCH_RATE_CONFIG.title}: ${VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(player.average_apm)}`,
                            `Games used: ${Number(player.games_played || 0)}`,
                          ],
                        })),
                        on_overlay_point_click: openWorkflowPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(workflowPlayersViewportProcessed.playersIncluded) || 0} players (>=${Math.max(4, Number(workflowPlayersViewportMinGames) || 4)} games after post-filter). Mean ${VIEWPORT_SWITCH_RATE_CONFIG.summaryFormatter(workflowPlayersViewportProcessed.mean)}, stddev ${VIEWPORT_SWITCH_RATE_CONFIG.summaryFormatter(workflowPlayersViewportProcessed.stddev)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
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
                  <button className={`btn-switch ${workflowGameTab === 'first-unit-efficiency' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('first-unit-efficiency')}>First Unit Efficiency</button>
                  <button className={`btn-switch ${workflowGameTab === 'unit-production-cadence' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('unit-production-cadence')}>Unit Production Cadence</button>
                  <button className={`btn-switch ${workflowGameTab === 'viewport-multitasking' ? 'workflow-nav-active' : ''}`} onClick={() => setWorkflowGameTab('viewport-multitasking')}>Viewport Multitasking</button>
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
                            {filterSummaryPillPatterns(player.detected_patterns).map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-${idx}`))}
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
                  <div className="workflow-card workflow-card-recent-games">
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
                  <div className="workflow-card workflow-card-chat-summary">
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
                            {workflowGamePlayers.map((player) => (
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
                              {workflowGamePlayers.map((player) => {
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
                {workflowGameTab === 'first-unit-efficiency' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-timing-notice">
                      Timing caveat: this metric uses command timestamps, so inefficiency can be inflated by worker travel after a build order is issued (e.g. SCV pathing before Barracks placement). Skilled players usually pre-position workers to reduce this delay. Network latency should not affect calculations. Game speed can affect timings, but most games are played on Fastest.
                    </div>
                    {workflowFirstUnitEfficiencyGroups.length > 0 ? (
                      workflowFirstUnitEfficiencyGroups.map((groupEntry) => (
                        <FirstUnitEfficiencyTimelineRows
                          key={`first-unit-eff-${groupEntry.id}`}
                          group={groupEntry}
                        />
                      ))
                    ) : (
                      <div className="workflow-card">
                        <div className="chart-empty">No first unit efficiency rows found for this game.</div>
                      </div>
                    )}
                  </div>
                )}
                {workflowGameTab === 'unit-production-cadence' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-card workflow-card-fingerprints">
                      <div className="workflow-card-title"><span>Unit production cadence (this game)</span></div>
                      <div className="workflow-card-subtitle">
                        <span>How it is calculated</span>
                        <HelpTooltip text="For each player in this replay: use attacking-unit Train/Unit Morph commands in [7:00, 80% game length], compute ratePerMin and gap evenness (cvGap), then cadenceScore = ratePerMin / (1 + cvGap)." label="Per-game cadence methodology" />
                      </div>
                      <div className="workflow-subtle-note">
                        Rationale: this captures sustained combat-unit production rhythm. Higher score means faster and less bursty production during the mid-game window.
                      </div>
                      {workflowGameCadenceProcessed.points.length > 0 ? (
                        <Histogram
                          data={[]}
                          config={{
                            style: 'monobell_relax',
                            precomputed_bins: workflowGameCadenceProcessed.bins,
                            x_axis_label: 'Cadence score',
                            y_axis_label: 'Density',
                            overlay_value_label: 'cadence',
                            overlay_count_label: 'units',
                            mean: workflowGameCadenceProcessed.mean,
                            stddev: workflowGameCadenceProcessed.stddev,
                            chart_height: 560,
                            overlay_points: workflowGameCadenceProcessed.points.map((player) => ({
                              value: Number(player.average_apm || 0),
                              label: String(player.player_name || ''),
                              player_key: String(player.player_key || ''),
                              games_played: Number(player.games_played || 0),
                              tooltip_lines: [
                                `${String(player.player_name || '')}`,
                                `Cadence score: ${Number(player.average_apm || 0).toFixed(3)}`,
                                `Rate per minute: ${Number(player.average_rate_per_min || 0).toFixed(2)}`,
                                `Gap CV: ${Number(player.average_cv_gap || 0).toFixed(2)}`,
                                `Burstiness: ${Number(player.average_burstiness || 0).toFixed(2)}`,
                                `Idle gap ratio (>=20s): ${(Number(player.average_idle20_ratio || 0) * 100).toFixed(1)}%`,
                                `Units counted in window: ${Number(player.games_played || 0)}`,
                                `Window length: ${formatDuration(Number(player.window_seconds || 0))}`,
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">No eligible players for this game cadence window yet.</div>
                      )}
                      <div className="workflow-card-subtitle"><span>Per-player breakdown</span></div>
                      {(workflowGame?.unit_production_cadence || []).map((entry) => (
                        <div key={`game-cadence-${entry.player_id}`} className="workflow-pattern-row">
                          <span style={playerAccentColor(entry.player_key) ? { color: playerAccentColor(entry.player_key), fontWeight: 600 } : undefined}>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? `rate=${Number(entry.rate_per_minute || 0).toFixed(2)}, cv=${Number(entry.cv_gap || 0).toFixed(2)}, burstiness=${Number(entry.burstiness || 0).toFixed(2)}, idle20=${(Number(entry.idle20_ratio || 0) * 100).toFixed(1)}%, units=${Number(entry.units_produced || 0)}, gaps=${Number(entry.gap_count || 0)}` : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? `${Number(entry.cadence_score || 0).toFixed(3)} cadence (${Number(entry.units_produced || 0)} units, ${formatDuration(Number(entry.window_seconds || 0))} window)`
                              : `N/A (${entry.ineligible_reason || 'insufficient data'})`}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {workflowGameTab === 'viewport-multitasking' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-card workflow-card-fingerprints">
                      <div className="workflow-card-title"><span>Viewport multitasking (this game)</span></div>
                      <div className="workflow-card-subtitle">
                        <span>How it is calculated</span>
                        <HelpTooltip text="For each player in this replay: use coordinate-bearing commands in [7:00, 80% game length]. A switch happens when the next command lands outside the previous viewport-sized box. The detector stores only the Viewport Switch Rate as a float per replay-player." label="Per-game viewport multitasking methodology" />
                      </div>
                      <div className="workflow-subtle-note">
                        {VIEWPORT_SWITCH_RATE_CONFIG.interpretation}
                      </div>
                      {workflowGameViewportProcessed.points.length > 0 ? (
                        <Histogram
                          data={[]}
                          config={{
                            style: 'monobell_relax',
                            precomputed_bins: workflowGameViewportProcessed.bins,
                            x_axis_label: VIEWPORT_SWITCH_RATE_CONFIG.axisLabel,
                            y_axis_label: 'Density',
                            overlay_value_label: VIEWPORT_SWITCH_RATE_CONFIG.overlayValueLabel,
                            overlay_count_label: 'player',
                            mean: workflowGameViewportProcessed.mean,
                            stddev: workflowGameViewportProcessed.stddev,
                            chart_height: 560,
                            overlay_points: workflowGameViewportProcessed.points.map((player) => ({
                              value: Number(player.average_apm || 0),
                              label: String(player.player_name || ''),
                              player_key: String(player.player_key || ''),
                              games_played: Number(player.games_played || 0),
                              tooltip_lines: [
                                `${String(player.player_name || '')}`,
                                `${VIEWPORT_SWITCH_RATE_CONFIG.title}: ${VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(player.average_apm)}`,
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">No eligible players for this game viewport multitasking window yet.</div>
                      )}
                      <div className="workflow-card-subtitle"><span>Per-player breakdown</span></div>
                      {(workflowGame?.viewport_multitasking || []).map((entry) => (
                        <div key={`game-viewport-${entry.player_id}`} className="workflow-pattern-row">
                          <span style={playerAccentColor(entry.player_key) ? { color: playerAccentColor(entry.player_key), fontWeight: 600 } : undefined}>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(entry.viewport_switch_rate) : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(entry.viewport_switch_rate)
                              : `N/A (${entry.ineligible_reason || 'insufficient data'})`}
                          </span>
                        </div>
                      ))}
                    </div>
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
                    <div className="workflow-card-title"><span>Population comparison</span></div>
                    {workflowPlayerInsightLoading ? <div className="chart-empty">Loading population comparisons...</div> : null}
                    {!workflowPlayerInsightLoading && workflowPlayerInsightErrors.length > 0 ? (
                      <div className="chart-empty">{workflowPlayerInsightErrors[0]}</div>
                    ) : null}
                    {!workflowPlayerInsightLoading && workflowPlayerInsightErrors.length === 0 ? (
                      <div className="workflow-insight-grid">
                        {workflowPlayerInsights.map((insight) => {
                          const percentile = Number(insight.performance_percentile || 0);
                          const accent = insightScoreColor(percentile);
                          return (
                            <button
                              type="button"
                              key={insight.insight_type}
                              className="workflow-insight-card workflow-insight-card-link"
                              style={insight.eligible ? { borderColor: `${accent}55`, boxShadow: `inset 0 0 0 1px ${accent}22` } : undefined}
                              onClick={() => openWorkflowPlayersSubview(playerInsightDestinationTab(insight.insight_type))}
                            >
                              <div className="workflow-insight-card-header">
                                <span>{insight.title}</span>
                              </div>
                              {insight.eligible ? (
                                <>
                                  <div className="workflow-insight-score-row">
                                    <span className="workflow-insight-score" style={{ color: accent }}>{insightSummaryLabel(percentile)}</span>
                                    <span className="workflow-insight-grade" style={{ backgroundColor: `${accent}22`, color: accent }}>{insightScoreLabel(percentile)}</span>
                                  </div>
                                  <div className="workflow-insight-value">{insight.player_value_label}</div>
                                  <div className="workflow-subtle-note">{`${insight.population_size} eligible players in population.`}</div>
                                </>
                              ) : (
                                <>
                                  <div className="workflow-insight-unavailable">Not enough data yet</div>
                                  <div className="workflow-subtle-note">{insight.ineligible_reason || 'This comparison is not available yet.'}</div>
                                </>
                              )}
                              <div className="workflow-insight-footer">
                                <span className="workflow-insight-link-hint">Open player population view</span>
                                <span className="workflow-insight-info-icon" aria-hidden="true">ⓘ</span>
                              </div>
                              <div className="workflow-insight-details">
                                <div className="workflow-subtle-note">{insight.description}</div>
                                <div className="workflow-insight-detail-list">
                                  {(insight.details || []).map((detail) => (
                                    <div key={`${insight.insight_type}-${detail.label}`} className="workflow-insight-detail-row">
                                      <span>{detail.label}</span>
                                      <span>{detail.value}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    ) : null}
                    <div className="workflow-card-subtitle"><span>Usage signals</span></div>
                    {workflowPlayerUsagePills.length === 0 ? (
                      <div className="workflow-subtle-note">No low-usage flags were triggered for hotkeys or queued orders.</div>
                    ) : (
                      <div className="workflow-pattern-pills">
                        {workflowPlayerUsagePills.map((pill) => (
                          <span key={pill.key} className={pill.className} title={pill.title}>{pill.label}</span>
                        ))}
                      </div>
                    )}
                    <div className="workflow-card-subtitle">
                      <span>Distinctive outliers</span>
                      <HelpTooltip text={PLAYER_OUTLIER_HELP} label="Outlier calculation explanation" />
                    </div>
                    <div className="workflow-subtle-note">Same-race, human-only baselines. Items are shown in one list and prefixed by command family.</div>
                    {workflowPlayerOutliersLoading ? <div className="chart-empty">Loading outliers...</div> : null}
                    {!workflowPlayerOutliersLoading && workflowPlayerOutliersError ? <div className="chart-empty">{workflowPlayerOutliersError}</div> : null}
                    {!workflowPlayerOutliersLoading && !workflowPlayerOutliersError && workflowPlayerOutlierItems.length === 0 ? (
                      <div className="chart-empty">No outliers crossed current thresholds.</div>
                    ) : null}
                    {!workflowPlayerOutliersLoading && !workflowPlayerOutliersError && workflowPlayerOutlierItems.map((item) => (
                      <div key={`${item.category}-${item.race}-${item.name}`} className="workflow-pattern-row">
                        <span>{`${item.category}: ${item.pretty_name}`}</span>
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
                  <div className="workflow-card workflow-card-recent-games">
                    <div className="workflow-card-title"><span>Recent games</span></div>
                    {workflowPlayerRecentGamesLoading ? <div className="chart-empty">Loading recent games...</div> : null}
                    {!workflowPlayerRecentGamesLoading && workflowPlayerRecentGamesError ? <div className="chart-empty">{workflowPlayerRecentGamesError}</div> : null}
                    {!workflowPlayerRecentGamesLoading && !workflowPlayerRecentGamesError && workflowPlayerRecentGames.length === 0 ? (
                      <div className="chart-empty">No recent games found for this player.</div>
                    ) : null}
                    {!workflowPlayerRecentGamesLoading && !workflowPlayerRecentGamesError && workflowPlayerRecentGames.slice(0, 6).map((g) => (
                      <button key={g.replay_id} className="workflow-recent-game-card" onClick={() => openWorkflowGame(g.replay_id)}>
                        <div className="workflow-recent-game-header">
                          <span>{formatRelativeReplayDate(g.replay_date)}</span>
                          <span>{g.map_name}</span>
                          {g.current_player?.race ? (
                            <span className="workflow-recent-game-race">
                              {getRaceIcon(g.current_player.race) ? (
                                <img
                                  src={getRaceIcon(g.current_player.race)}
                                  alt={g.current_player.race}
                                  className="unit-icon-inline workflow-recent-game-race-icon"
                                />
                              ) : null}
                              <span>{g.current_player.race}</span>
                            </span>
                          ) : (
                            <span className="workflow-empty-inline">-</span>
                          )}
                          <span>{formatDuration(g.duration_seconds)}</span>
                        </div>
                        <div className="workflow-subtle-note">{renderPlayersMatchup(g.players_label || '')}</div>
                        <div className="workflow-recent-game-meta">
                          {g.current_player?.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                        </div>
                        {filterSummaryPillPatterns(g.current_player?.detected_patterns).length > 0 ? (
                          <div className="workflow-pattern-pills workflow-pattern-pills-compact">
                            {filterSummaryPillPatterns(g.current_player?.detected_patterns).map((pattern, idx) => renderPatternPill(pattern, `recent-${g.replay_id}-${idx}`))}
                          </div>
                        ) : null}
                      </button>
                    ))}
                  </div>
                  <div className="workflow-card workflow-card-chat-summary">
                    <div className="workflow-card-title"><span>Chat Summary</span></div>
                    {workflowPlayerChatSummaryLoading ? <div className="chart-empty">Loading chat summary...</div> : null}
                    {!workflowPlayerChatSummaryLoading && workflowPlayerChatSummaryError ? <div className="chart-empty">{workflowPlayerChatSummaryError}</div> : null}
                    {!workflowPlayerChatSummaryLoading && !workflowPlayerChatSummaryError && (Number(workflowPlayerChatSummary?.total_messages) || 0) === 0 ? (
                      <div className="chart-empty">No chat messages found for this player in ingested games.</div>
                    ) : (
                      !workflowPlayerChatSummaryLoading && !workflowPlayerChatSummaryError && workflowPlayerChatSummary ? (
                      <>
                        <div className="workflow-subtle-note">
                          {`${workflowPlayerChatSummary?.total_messages || 0} messages across ${workflowPlayerChatSummary?.games_with_chat || 0} games, ${workflowPlayerChatSummary?.distinct_terms || 0} distinct terms after cleanup.`}
                        </div>
                        <div className="workflow-card-subtitle"><span>Top terms</span></div>
                        {(workflowPlayerChatSummary?.top_terms || []).length === 0 ? (
                          <div className="chart-empty">Not enough messages to infer common terms.</div>
                        ) : (
                          <div className="workflow-pattern-pills">
                            {(workflowPlayerChatSummary?.top_terms || []).map((item) => (
                              <span key={`player-chat-term-${item.term}`} className="workflow-pattern-pill">
                                <span>{item.term}</span>
                                <span>{`x${item.count}`}</span>
                              </span>
                            ))}
                          </div>
                        )}
                        <div className="workflow-card-subtitle"><span>Last 5 messages</span></div>
                        {(workflowPlayerChatSummary?.example_messages || []).map((msg, idx) => (
                          <div key={`player-chat-example-${idx}`} className="workflow-event-row">
                            <span>{msg}</span>
                          </div>
                        ))}
                      </>
                      ) : null
                    )}
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

      {showGlobalReplayFilter && (
        <GlobalReplayFilterModal
          config={globalReplayFilterConfig}
          options={globalReplayFilterOptions}
          saving={globalReplayFilterSaving}
          error={globalReplayFilterError}
          onClose={() => setShowGlobalReplayFilter(false)}
          onSave={handleSaveGlobalReplayFilter}
        />
      )}

      {showIngestPanel && (
        <IngestModal
          ingestForm={ingestForm}
          ingestMessage={ingestMessage}
          ingestStatus={ingestStatus}
          ingestLogs={ingestLogs}
          ingestInputDir={ingestInputDir}
          ingestInputDirDirty={String(ingestInputDir || '').trim() !== String(savedIngestInputDir || '').trim()}
          ingestSettingsLoading={ingestSettingsLoading}
          ingestSettingsSaving={ingestSettingsSaving}
          ingestSocketState={ingestSocketState}
          onClose={() => setShowIngestPanel(false)}
          onSubmit={handleIngestSubmit}
          onChange={setIngestForm}
          onInputDirChange={setIngestInputDir}
          onSaveInputDir={handleSaveIngestInputDir}
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

      {autoIngestNotice ? (
        <div className="ingest-toast">{autoIngestNotice}</div>
      ) : null}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null
            ? `${replayCount.toLocaleString()} replays in database. You can trigger an ingestion using the button above.`
            : 'Loading replay count...'}
        </div>
      </div>
    </div>
  );
}

export default App;
