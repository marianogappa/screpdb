// generate-demo-assets pre-renders unit/building icon PNGs for the WASM demo.
// Run at build time with the native Go toolchain:
//
//	go run scripts/generate-demo-assets/main.go <output-dir>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
)

// Unique scmapanalyzer display names for all known unit/building icons.
// Derived from gameAssetIconScmapQueries in internal/dashboard/game_asset_icon_queries.go.
// Exact copy of gameAssetIconScmapQueries from
// internal/dashboard/game_asset_icon_queries.go — every key the frontend can
// request must have a pre-rendered PNG.
var iconQueries = map[string]string{
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
	"commandcenter":            "Terran Command Center",
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: generate-demo-assets <output-dir>\n")
		os.Exit(1)
	}
	outDir := os.Args[1]

	iconsDir := filepath.Join(outDir, "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", iconsDir, err)
		os.Exit(1)
	}

	// Render each unique display name once, then write a file for every key.
	rendered := map[string][]byte{}
	for _, displayName := range iconQueries {
		if _, ok := rendered[displayName]; ok {
			continue
		}
		png, err := scmapanalyzer.UnitOrBuildingImagePNG(displayName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "icon %q: %v\n", displayName, err)
			continue
		}
		rendered[displayName] = png
	}

	generated := 0
	for key, displayName := range iconQueries {
		png, ok := rendered[displayName]
		if !ok {
			continue
		}
		outPath := filepath.Join(iconsDir, key+".png")
		if err := os.WriteFile(outPath, png, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
			continue
		}
		generated++
	}

	fmt.Printf("Generated %d icon PNGs\n", generated)
}
