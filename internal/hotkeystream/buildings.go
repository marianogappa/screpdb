package hotkeystream

// Building IDs are the stable wire values for annotated building assigns.
// Append-only: never renumber, or stored blobs change meaning.
var buildingNames = map[byte]string{
	1: "Command Center", 2: "Comsat Station", 3: "Nuclear Silo", 4: "Supply Depot",
	5: "Refinery", 6: "Barracks", 7: "Academy", 8: "Factory", 9: "Machine Shop",
	10: "Starport", 11: "Control Tower", 12: "Science Facility", 13: "Covert Ops",
	14: "Physics Lab", 15: "Engineering Bay", 16: "Armory", 17: "Missile Turret",
	18: "Bunker",
	20: "Nexus", 21: "Robotics Facility", 22: "Pylon", 23: "Assimilator",
	24: "Observatory", 25: "Gateway", 26: "Photon Cannon", 27: "Citadel of Adun",
	28: "Cybernetics Core", 29: "Templar Archives", 30: "Forge", 31: "Stargate",
	32: "Fleet Beacon", 33: "Arbiter Tribunal", 34: "Robotics Support Bay",
	35: "Shield Battery",
	40: "Hatchery", 41: "Lair", 42: "Hive", 43: "Nydus Canal", 44: "Hydralisk Den",
	45: "Defiler Mound", 46: "Greater Spire", 47: "Queen's Nest", 48: "Evolution Chamber",
	49: "Ultralisk Cavern", 50: "Spire", 51: "Spawning Pool", 52: "Creep Colony",
	53: "Spore Colony", 54: "Sunken Colony", 55: "Extractor",
}

var buildingIDs = func() map[string]byte {
	m := make(map[string]byte, len(buildingNames))
	for id, name := range buildingNames {
		m[name] = id
	}
	return m
}()

// BuildingID returns the wire ID for a building name (0 = unknown).
func BuildingID(name string) byte { return buildingIDs[name] }

// BuildingName returns the building name for a wire ID ("" = unknown).
func BuildingName(id byte) string { return buildingNames[id] }
