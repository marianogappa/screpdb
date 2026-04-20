package dashboard

import (
	"strings"
	"unicode"
)

// gameAssetIconScmapQueries maps keys produced by [normalizeGameAssetIconKey] (same rules as
// frontend normalizeUnitName) to display names accepted by scmapanalyzer.UnitOrBuildingImagePNG.
var gameAssetIconScmapQueries = map[string]string{
	"probe":                    "Protoss Probe",
	"scv":                      "Terran SCV",
	"drone":                    "Zerg Drone",
	"arbiter":                  "Protoss Arbiter",
	"protossarbiter":           "Protoss Arbiter",
	"corsair":                  "Protoss Corsair",
	"protosscorsair":           "Protoss Corsair",
	"scout":                    "Protoss Scout",
	"protossscout":             "Protoss Scout",
	"reaver":                   "Protoss Reaver",
	"protossreaver":            "Protoss Reaver",
	"overlord":                 "Zerg Overlord",
	"zergoverlord":             "Zerg Overlord",
	"scourge":                  "Zerg Scourge",
	"zergscourge":              "Zerg Scourge",
	"observer":                 "Protoss Observer",
	"protossobserver":          "Protoss Observer",
	"carrier":                  "Protoss Carrier",
	"battlecruiser":            "Terran Battlecruiser",
	"terranbattlecruiser":      "Terran Battlecruiser",
	"dropship":                 "Terran Dropship",
	"terrandropship":           "Terran Dropship",
	"sciencevessel":            "Terran Science Vessel",
	"terransciencevessel":      "Terran Science Vessel",
	"wraith":                   "Terran Wraith",
	"terranwraith":             "Terran Wraith",
	"marine":                   "Terran Marine",
	"siegetank":                "Terran Siege Tank",
	"siegetanktankmode":        "Terran Siege Tank",
	"siegetankturrettankmode":  "Terran Siege Tank",
	"terransiegetanksiegemode": "Terran Siege Tank",
	"siegetankturretsiegemode": "Terran Siege Tank",
	"zealot":                   "Protoss Zealot",
	"dragoon":                  "Protoss Dragoon",
	"zergling":                 "Zerg Zergling",
	"hydralisk":                "Zerg Hydralisk",
	"mutalisk":                 "Zerg Mutalisk",
	"ultralisk":                "Zerg Ultralisk",
	"goliath":                  "Terran Goliath",
	"vulture":                  "Terran Vulture",
	"medic":                    "Terran Medic",
	"defiler":                  "Zerg Defiler",
	"zergdefiler":              "Zerg Defiler",
	"firebat":                  "Terran Firebat",
	"darktemplar":              "Protoss Dark Templar",
	"hightemplar":              "Protoss High Templar",
	"lurker":                   "Zerg Lurker",
	"archon":                   "Protoss Archon",
	"ghost":                    "Terran Ghost",
	"valkyrie":                 "Terran Valkyrie",
	"devourer":                 "Zerg Devourer",
	"darkarchon":               "Protoss Dark Archon",
	"guardian":                 "Zerg Guardian",
	"infestedterran":           "Infested Terran",
	"queen":                    "Zerg Queen",
	"shuttle":                  "Protoss Shuttle",
	"academy":                  "Terran Academy",
	"arbitertribunal":          "Protoss Arbiter Tribunal",
	"armory":                   "Terran Armory",
	"assimilator":              "Protoss Assimilator",
	"barracks":                 "Terran Barracks",
	"bunker":                   "Terran Bunker",
	"citadelofadun":            "Protoss Citadel of Adun",
	"comsat":                   "Terran Comsat Station",
	"controltower":             "Terran Control Tower",
	"covertops":                "Terran Covert Ops",
	"creepcolony":              "Zerg Creep Colony",
	"cyberneticscore":          "Protoss Cybernetics Core",
	"defilermound":             "Zerg Defiler Mound",
	"engineeringbay":           "Terran Engineering Bay",
	"evolutionchamber":         "Zerg Evolution Chamber",
	"extractor":                "Zerg Extractor",
	"factory":                  "Terran Factory",
	"fleetbeacon":              "Protoss Fleet Beacon",
	"forge":                    "Protoss Forge",
	"gateway":                  "Protoss Gateway",
	"greaterspire":             "Zerg Greater Spire",
	"hatchery":                 "Zerg Hatchery",
	"hive":                     "Zerg Hive",
	"hydraliskden":             "Zerg Hydralisk Den",
	"infestedcc":               "Infested Command Center",
	"lair":                     "Zerg Lair",
	"machineshop":              "Terran Machine Shop",
	"missileturret":            "Terran Missile Turret",
	"nexus":                    "Protoss Nexus",
	"nyduscanal":               "Zerg Nydus Canal",
	"observatory":              "Protoss Observatory",
	"photoncannon":             "Protoss Photon Cannon",
	"physicslab":               "Terran Physics Lab",
	"pylon":                    "Protoss Pylon",
	"queensnest":               "Zerg Queen's Nest",
	"refinery":                 "Terran Refinery",
	"roboticsfacility":         "Protoss Robotics Facility",
	"roboticssupportbay":       "Protoss Robotics Support Bay",
	"sciencefacility":          "Terran Science Facility",
	"shieldbattery":            "Protoss Shield Battery",
	"spawningpool":             "Zerg Spawning Pool",
	"spire":                    "Zerg Spire",
	"sporecolony":              "Zerg Spore Colony",
	"stargate":                 "Protoss Stargate",
	"starport":                 "Terran Starport",
	"sunkencolony":             "Zerg Sunken Colony",
	"supplydepot":              "Terran Supply Depot",
	"templararchives":          "Protoss Templar Archives",
	"ultraliskcavern":          "Zerg Ultralisk Cavern",
}

func normalizeGameAssetIconKey(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case unicode.IsSpace(r):
			continue
		default:
			continue
		}
	}
	return b.String()
}

func stripRacePrefixGameAssetKey(normalized string) string {
	prefixes := []string{"terran", "protoss", "zerg"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) && len(normalized) > len(prefix) {
			return normalized[len(prefix):]
		}
	}
	return normalized
}

func stripGameAssetUnitModeSuffix(withoutRace string) string {
	s := withoutRace
	for {
		prev := s
		s = strings.TrimSuffix(s, "siegemode")
		s = strings.TrimSuffix(s, "tankmode")
		s = strings.TrimSuffix(s, "turret")
		if s == prev {
			return s
		}
	}
}

func resolveGameAssetIconQuery(name string) (cacheKey string, scmapQuery string, ok bool) {
	normalized := normalizeGameAssetIconKey(name)
	if normalized == "" {
		return "", "", false
	}
	if q, hit := gameAssetIconScmapQueries[normalized]; hit {
		return normalized, q, true
	}
	withoutRace := stripRacePrefixGameAssetKey(normalized)
	if q, hit := gameAssetIconScmapQueries[withoutRace]; hit {
		return withoutRace, q, true
	}
	withoutMode := stripGameAssetUnitModeSuffix(withoutRace)
	if q, hit := gameAssetIconScmapQueries[withoutMode]; hit {
		return withoutMode, q, true
	}
	return "", "", false
}
