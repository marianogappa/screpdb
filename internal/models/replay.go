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

	// Start location (if available)
	StartLocationX *int `json:"start_location_x,omitempty"`
	StartLocationY *int `json:"start_location_y,omitempty"`
}

// UnitInfo represents information about a unit instance in a command
type UnitInfo struct {
	UnitTag      uint16    `json:"unit_tag"`
	UnitType     string    `json:"unit_type"` // "Marine", "Zergling", etc.
	UnitID       uint16    `json:"unit_id"`   // The unit type ID
	PlayerID     int64     `json:"player_id"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedFrame int32     `json:"created_frame"`
	X            int       `json:"x"`
	Y            int       `json:"y"`
	IsAlive      bool      `json:"is_alive"`
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
	UnitID     *byte  `json:"unit_id,omitempty"` // Unit type ID (properly filled)
	TargetID   byte   `json:"target_id"`

	// Position data
	X int `json:"x"`
	Y int `json:"y"`

	// Command effectiveness
	Effective bool `json:"effective"`

	// Common fields (used by multiple command types)
	Queued    *bool   `json:"queued,omitempty"`
	OrderID   *byte   `json:"order_id,omitempty"`
	OrderName *string `json:"order_name,omitempty"`

	// Unit information (normalized fields)
	UnitType     *string `json:"unit_type,omitempty"`      // Single unit type
	UnitPlayerID *int64  `json:"unit_player_id,omitempty"` // Single unit player ID
	UnitTypes    *string `json:"unit_types,omitempty"`     // JSON array of unit types for multiple units
	UnitIDs      *string `json:"unit_ids,omitempty"`       // JSON array of unit IDs for multiple units

	// Select command fields (legacy - will be removed)
	SelectUnitTags  *string `json:"select_unit_tags,omitempty"`  // JSON array of unit tags
	SelectUnitTypes *string `json:"select_unit_types,omitempty"` // JSON map of unit tag -> unit type

	// Build command fields
	BuildUnitName *string `json:"build_unit_name,omitempty"`

	// Train command fields
	TrainUnitName *string `json:"train_unit_name,omitempty"`

	// Building Morph command fields
	BuildingMorphUnitName *string `json:"building_morph_unit_name,omitempty"`

	// Tech command fields
	TechName *string `json:"tech_name,omitempty"`

	// Upgrade command fields
	UpgradeName *string `json:"upgrade_name,omitempty"`

	// Hotkey command fields
	HotkeyType  *string `json:"hotkey_type,omitempty"`
	HotkeyGroup *byte   `json:"hotkey_group,omitempty"`

	// Game Speed command fields
	GameSpeed *string `json:"game_speed,omitempty"`

	// Chat command fields
	ChatSenderSlotID *byte   `json:"chat_sender_slot_id,omitempty"`
	ChatMessage      *string `json:"chat_message,omitempty"`

	// Vision command fields
	VisionSlotIDs *[]int `json:"vision_slot_ids,omitempty"` // Array of slot IDs

	// Alliance command fields
	AllianceSlotIDs *[]int `json:"alliance_slot_ids,omitempty"` // Array of slot IDs
	AlliedVictory   *bool  `json:"allied_victory,omitempty"`

	// Leave Game command fields
	LeaveReason *string `json:"leave_reason,omitempty"`

	// Minimap Ping command fields
	MinimapPingX *int `json:"minimap_ping_x,omitempty"`
	MinimapPingY *int `json:"minimap_ping_y,omitempty"`

	// General command fields (for unhandled commands)
	GeneralData *string `json:"general_data,omitempty"` // Hex string of raw data
}

// Unit represents a unit in the game
type Unit struct {
	ID           int64     `json:"id"`
	ReplayID     int64     `json:"replay_id"`
	PlayerID     int64     `json:"player_id"`
	UnitID       uint16    `json:"unit_id"`
	Type         string    `json:"type"` // Marine, Zealot, Zergling, etc.
	Created      time.Time `json:"created"`
	CreatedFrame int32     `json:"created_frame"`
}

// Building represents a building in the game
type Building struct {
	ID           int64     `json:"id"`
	ReplayID     int64     `json:"replay_id"`
	PlayerID     int64     `json:"player_id"`
	BuildingID   uint16    `json:"building_id"`
	Type         string    `json:"type"` // Command Center, Nexus, Hatchery, etc.
	Name         string    `json:"name"`
	Created      time.Time `json:"created"`
	CreatedFrame int32     `json:"created_frame"`
	X            int       `json:"x"`
	Y            int       `json:"y"`
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
	ID       int64  `json:"id"`
	ReplayID int64  `json:"replay_id"`
	PlayerID int64  `json:"player_id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

// ChatMessage represents an in-game chat message
type ChatMessage struct {
	ID           int64     `json:"id"`
	ReplayID     int64     `json:"replay_id"`
	PlayerID     int64     `json:"player_id"`
	SenderSlotID byte      `json:"sender_slot_id"`
	Message      string    `json:"message"`
	Frame        int32     `json:"frame"`
	Time         time.Time `json:"time"`
}

// LeaveGame represents a player leaving the game
type LeaveGame struct {
	ID       int64     `json:"id"`
	ReplayID int64     `json:"replay_id"`
	PlayerID int64     `json:"player_id"`
	Reason   string    `json:"reason"`
	Frame    int32     `json:"frame"`
	Time     time.Time `json:"time"`
}

// ReplayData represents the complete parsed replay data
type ReplayData struct {
	Replay         *Replay          `json:"replay"`
	Players        []*Player        `json:"players"`
	Commands       []*Command       `json:"commands"`
	Units          []*Unit          `json:"units"`
	Buildings      []*Building      `json:"buildings"`
	Resources      []*Resource      `json:"resources"`
	StartLocations []*StartLocation `json:"available_start_locations"`
	PlacedUnits    []*PlacedUnit    `json:"placed_units"`
	ChatMessages   []*ChatMessage   `json:"chat_messages"`
	LeaveGames     []*LeaveGame     `json:"leave_games"`
}
