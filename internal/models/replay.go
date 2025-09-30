package models

import (
	"time"
)

// Replay represents the main replay metadata
type Replay struct {
	ID           int64     `json:"id"`
	FilePath     string    `json:"file_path"`
	FileChecksum string    `json:"file_checksum"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	CreatedAt    time.Time `json:"created_at"`

	// StarCraft: Brood War specific fields
	ReplayDate      time.Time `json:"replay_date"`
	Title           string    `json:"title"`
	Host            string    `json:"host"`
	MapName         string    `json:"map_name"`
	MapWidth        uint16    `json:"map_width"`
	MapHeight       uint16    `json:"map_height"`
	Duration        int       `json:"duration"` // in seconds
	FrameCount      int32     `json:"frame_count"`
	Version         string    `json:"version"`
	Engine          string    `json:"engine"`    // StarCraft or Brood War
	Speed           string    `json:"speed"`     // Slowest, Slower, Slow, Normal, Fast, Faster, Fastest
	GameType        string    `json:"game_type"` // Melee, FFA, 1on1, CTF, etc.
	SubType         uint16    `json:"sub_type"`  // Team size
	AvailSlotsCount byte      `json:"avail_slots_count"`
}

// Player represents a player in the replay
type Player struct {
	ID       int64  `json:"id"`
	ReplayID int64  `json:"replay_id"`
	SlotID   uint16 `json:"slot_id"`
	PlayerID byte   `json:"player_id"` // Computer players all have ID=255
	Name     string `json:"name"`
	Race     string `json:"race"`  // Terran, Protoss, Zerg
	Type     string `json:"type"`  // Human, Computer, Inactive, etc.
	Color    string `json:"color"` // Red, Blue, Teal, etc.
	Team     byte   `json:"team"`
	Observer bool   `json:"observer"`

	// Computed fields
	APM      int  `json:"apm"`
	SPM      int  `json:"spm"` // Supply per minute
	IsWinner bool `json:"is_winner"`
}

// Command represents a player command/action in the game
type Command struct {
	ID       int64     `json:"id"`
	ReplayID int64     `json:"replay_id"`
	PlayerID int64     `json:"player_id"`
	Frame    int32     `json:"frame"`
	Time     time.Time `json:"time"`

	// Command details
	ActionType string `json:"action_type"` // Build, Move, Attack, etc.
	ActionID   byte   `json:"action_id"`
	UnitID     byte   `json:"unit_id"`
	TargetID   byte   `json:"target_id"`

	// Position data
	X int `json:"x"`
	Y int `json:"y"`

	// Additional data
	Data string `json:"data"` // Additional JSON data

	// Command effectiveness
	Effective bool `json:"effective"`
}

// Unit represents a unit in the game
type Unit struct {
	ID        int64      `json:"id"`
	ReplayID  int64      `json:"replay_id"`
	PlayerID  int64      `json:"player_id"`
	UnitID    uint16     `json:"unit_id"`
	Type      string     `json:"type"` // Marine, Zealot, Zergling, etc.
	Name      string     `json:"name"`
	Created   time.Time  `json:"created"`
	Destroyed *time.Time `json:"destroyed,omitempty"`
	X         int        `json:"x"`
	Y         int        `json:"y"`
	HP        int        `json:"hp"`
	MaxHP     int        `json:"max_hp"`
	Shield    int        `json:"shield"`
	MaxShield int        `json:"max_shield"`
	Energy    int        `json:"energy"`
	MaxEnergy int        `json:"max_energy"`
}

// Building represents a building in the game
type Building struct {
	ID         int64      `json:"id"`
	ReplayID   int64      `json:"replay_id"`
	PlayerID   int64      `json:"player_id"`
	BuildingID uint16     `json:"building_id"`
	Type       string     `json:"type"` // Command Center, Nexus, Hatchery, etc.
	Name       string     `json:"name"`
	Created    time.Time  `json:"created"`
	Destroyed  *time.Time `json:"destroyed,omitempty"`
	X          int        `json:"x"`
	Y          int        `json:"y"`
	HP         int        `json:"hp"`
	MaxHP      int        `json:"max_hp"`
	Shield     int        `json:"shield"`
	MaxShield  int        `json:"max_shield"`
	Energy     int        `json:"energy"`
	MaxEnergy  int        `json:"max_energy"`
}

// Resource represents mineral fields and geysers on the map
type Resource struct {
	ID       int64  `json:"id"`
	ReplayID int64  `json:"replay_id"`
	Type     string `json:"type"` // "mineral" or "geyser"
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Amount   int    `json:"amount"`
}

// StartLocation represents starting positions on the map
type StartLocation struct {
	ID       int64 `json:"id"`
	ReplayID int64 `json:"replay_id"`
	X        int   `json:"x"`
	Y        int   `json:"y"`
}

// PlacedUnit represents units placed on the map at game start
type PlacedUnit struct {
	ID        int64  `json:"id"`
	ReplayID  int64  `json:"replay_id"`
	PlayerID  int64  `json:"player_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	HP        int    `json:"hp"`
	MaxHP     int    `json:"max_hp"`
	Shield    int    `json:"shield"`
	MaxShield int    `json:"max_shield"`
	Energy    int    `json:"energy"`
	MaxEnergy int    `json:"max_energy"`
}

// ReplayData represents the complete parsed replay data
type ReplayData struct {
	Replay         *Replay          `json:"replay"`
	Players        []*Player        `json:"players"`
	Commands       []*Command       `json:"commands"`
	Units          []*Unit          `json:"units"`
	Buildings      []*Building      `json:"buildings"`
	Resources      []*Resource      `json:"resources"`
	StartLocations []*StartLocation `json:"start_locations"`
	PlacedUnits    []*PlacedUnit    `json:"placed_units"`
}
