package worldstate

import "strings"

// Building categories for future ownership / commitment heuristics. Contested takeover in the
// engine still counts all build-like commands; these helpers document intent and support tests.

func normalizeUnitKey(unitName string) string {
	x := strings.ToLower(strings.TrimSpace(unitName))
	x = strings.ReplaceAll(x, " ", "")
	x = strings.ReplaceAll(x, "_", "")
	return x
}

// IsCommitmentBuild reports production / tech structures that usually imply economic control at a
// location (town halls, core production, tech, supply).
func IsCommitmentBuild(unitName string) bool {
	n := normalizeUnitKey(unitName)
	if n == "" {
		return false
	}
	if isTownHallUnit(unitName) {
		return true
	}
	for _, frag := range commitmentBuildFragments {
		if strings.Contains(n, frag) {
			return true
		}
	}
	return false
}

// IsAmbiguousAggressiveDefenseBuild reports structures often used offensively (proxy, rush).
func IsAmbiguousAggressiveDefenseBuild(unitName string) bool {
	n := normalizeUnitKey(unitName)
	for _, frag := range ambiguousDefenseFragments {
		if strings.Contains(n, frag) {
			return true
		}
	}
	return false
}

// Substrings matched against normalized unit type names (no spaces/underscores).
var commitmentBuildFragments = []string{
	"commandcenter", "supplydepot", "refinery", "barracks", "academy", "factory", "starport",
	"sciencefacility", "comsatstation", "nuclearmissile", "machineshop", "controltower",
	"physicslab", "covertops", "missileturret", "engineeringbay", "armory",
	"nexus", "pylon", "assimilator", "gateway", "forge", "cyberneticscore", "photoncannon",
	"shieldbattery", "roboticsfacility", "stargate", "citadelofadun", "roboticssupportbay",
	"observatory", "fleetbeacon", "templararchives",
	"hatchery", "lair", "hive", "extractor", "spawningpool", "hydraliskden", "spire",
	"greaterspire", "nyduscanal", "ultraliskcavern", "defilermound", "queensnest",
	"evolutionchamber", "infestedcommandcenter",
}

var ambiguousDefenseFragments = []string{
	"photoncannon", "sunkencolony", "bunker", "creepcolony",
}
