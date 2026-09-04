package db

// ReplayPlayerHotkeyStreamRow is one player's encoded hotkey stream in a game.
type ReplayPlayerHotkeyStreamRow struct {
	PlayerID     int64
	Name         string
	Race         string
	Team         int64
	HotkeyStream []byte
}

// PlayerHotkeyStreamRow is one game's stream for a player, for signature
// aggregation.
type PlayerHotkeyStreamRow struct {
	Name            string
	Race            string
	HotkeyStream    []byte
	DurationSeconds int64
}

// ReplayPlayerHotkeyStream is one player's stream fetched for the map
// composite.
type ReplayPlayerHotkeyStream struct {
	PlayerID     int64
	Name         string
	Race         string
	SlotID       int64
	HotkeyStream []byte
}
