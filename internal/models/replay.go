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
	CreatedAt    time.Time `json:"created_at"`

	// StarCraft: Brood War specific fields
	ReplayDate      time.Time `json:"replay_date"`
	Title           string    `json:"title"`
	Host            string    `json:"host"`
	MapName         string    `json:"map_name"`
	MapWidth        uint16    `json:"map_width"`
	MapHeight       uint16    `json:"map_height"`
	DurationSeconds int       `json:"duration_seconds"` // in seconds
	FrameCount      int32     `json:"frame_count"`
	EngineVersion   string    `json:"engine_version"`
	Engine          string    `json:"engine"`     // StarCraft or Brood War
	GameSpeed       string    `json:"game_speed"` // Slowest, Slower, Slow, Normal, Fast, Faster, Fastest
	GameType        string    `json:"game_type"`  // Melee, FFA, 1on1, CTF, etc.
	AvailSlotsCount byte      `json:"avail_slots_count"`
	// On Melee & Free for all this is always 1, and on Top vs Bottom it's what the game creator set for the home team.
	HomeTeamSize uint16 `json:"home_team_size"` // Team size

	Players []*Player `json:"-"`
}

// Player represents a player in the replay
type Player struct {
	ID         int64  `json:"id"`
	ReplayID   int64  `json:"replay_id"`
	SlotID     uint16 `json:"slot_id"`
	PlayerID   byte   `json:"player_id"` // This is the replay's player_id (not the database player id). Computer players all have ID=255
	Name       string `json:"name"`
	Race       string `json:"race"`  // Terran, Protoss, Zerg
	Type       string `json:"type"`  // Human, Computer, Inactive, etc.
	Color      string `json:"color"` // Red, Blue, Teal, etc.
	Team       byte   `json:"team"`
	IsObserver bool   `json:"is_observer"`

	// Computed fields
	APM      int  `json:"apm"`
	EAPM     int  `json:"eapm"` // Effective APM (APM excluding actions deemed ineffective)
	IsWinner bool `json:"is_winner"`

	// Start location (if available)
	StartLocationX      *int `json:"start_location_x,omitempty"`
	StartLocationY      *int `json:"start_location_y,omitempty"`
	StartLocationOclock *int `json:"start_location_oclock,omitempty"` // Clock position: 11, 12, 1, 3, 5, 6, 7, 9

	Replay *Replay `json:"-"`
}

// Command represents a player command/action in the game
type Command struct {
	ID                   int64     `json:"id"`
	ReplayID             int64     `json:"replay_id"`
	PlayerID             int64     `json:"player_id"`
	Frame                int32     `json:"frame"`
	RunAt                time.Time `json:"run_at"`
	SecondsFromGameStart int       `json:"secondsFromGameStart"`

	// Command details
	ActionType string `json:"action_type"`       // Build, Move, Attack, etc.
	UnitID     *byte  `json:"unit_id,omitempty"` // Unit type ID (properly filled)

	// Position data
	X *int `json:"x"`
	Y *int `json:"y"`

	// Common fields (used by multiple command types)
	IsQueued  *bool   `json:"is_queued,omitempty"`
	OrderID   *byte   `json:"order_id,omitempty"`
	OrderName *string `json:"order_name,omitempty"`

	// Unit information (normalized fields)
	UnitType  *string `json:"unit_type,omitempty"`  // Single unit type
	UnitTypes *string `json:"unit_types,omitempty"` // JSON array of unit types for multiple units
	UnitIDs   *string `json:"unit_ids,omitempty"`   // JSON array of unit IDs for multiple units

	// Tech command fields
	TechName *string `json:"tech_name,omitempty"`

	// Upgrade command fields
	UpgradeName *string `json:"upgrade_name,omitempty"`

	// Hotkey command fields
	HotkeyType  *string `json:"hotkey_type,omitempty"`
	HotkeyGroup *byte   `json:"hotkey_group,omitempty"`

	// Game Speed command fields
	GameSpeed *string `json:"game_speed,omitempty"`

	VisionPlayerIDs *[]int64 `json:"vision_player_ids,omitempty"` // Array of player IDs

	// Alliance command fields
	AlliancePlayerIDs *[]int64 `json:"alliance_player_ids,omitempty"` // Array of player IDs
	IsAlliedVictory   *bool    `json:"is_allied_victory,omitempty"`

	// General command fields (for unhandled commands)
	GeneralData *string `json:"general_data,omitempty"` // Hex string of raw data

	// Chat and leave game fields
	ChatMessage *string `json:"chat_message,omitempty"` // Chat message content
	LeaveReason *string `json:"leave_reason,omitempty"` // Reason for leaving game

	Replay *Replay `json:"-"`
	Player *Player `json:"-"`
}

// ReplayData represents the complete parsed replay data
type ReplayData struct {
	Replay   *Replay    `json:"replay"`
	Players  []*Player  `json:"players"`
	Commands []*Command `json:"commands"`
}
