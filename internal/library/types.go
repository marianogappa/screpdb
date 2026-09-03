// Package library is the dashboard's in-memory replay corpus: compact
// immutable records, an atomically swapped snapshot with indexes, and
// filtered views that memoise aggregates.
package library

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNotFound is returned by lookups for ids that are not in the corpus.
var ErrNotFound = errors.New("library: not found")

type Race uint8

const (
	RaceUnknown Race = iota
	RaceZerg
	RaceTerran
	RaceProtoss
)

func ParseRace(s string) Race {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "zerg":
		return RaceZerg
	case "terran":
		return RaceTerran
	case "protoss":
		return RaceProtoss
	}
	return RaceUnknown
}

func (r Race) String() string {
	switch r {
	case RaceZerg:
		return "Zerg"
	case RaceTerran:
		return "Terran"
	case RaceProtoss:
		return "Protoss"
	}
	return ""
}

type PlayerType uint8

const (
	PlayerTypeUnknown PlayerType = iota
	PlayerTypeInactive
	PlayerTypeComputer
	PlayerTypeHuman
	PlayerTypeRescuePassive
	PlayerTypeUnused
	PlayerTypeComputerControlled
	PlayerTypeOpen
	PlayerTypeNeutral
	PlayerTypeClosed
)

var playerTypeNames = map[string]PlayerType{
	"inactive":            PlayerTypeInactive,
	"computer":            PlayerTypeComputer,
	"human":               PlayerTypeHuman,
	"rescue passive":      PlayerTypeRescuePassive,
	"(unused)":            PlayerTypeUnused,
	"computer controlled": PlayerTypeComputerControlled,
	"open":                PlayerTypeOpen,
	"neutral":             PlayerTypeNeutral,
	"closed":              PlayerTypeClosed,
}

func ParsePlayerType(s string) PlayerType {
	return playerTypeNames[strings.ToLower(strings.TrimSpace(s))]
}

func (t PlayerType) String() string {
	for name, v := range playerTypeNames {
		if v == t {
			return name
		}
	}
	return ""
}

// IsComputer mirrors the dashboard's global filter, which treats both
// 'computer' and 'computer controlled' as AI players.
func (t PlayerType) IsComputer() bool {
	return t == PlayerTypeComputer || t == PlayerTypeComputerControlled
}

type MapKind uint8

const (
	MapKindUnknown MapKind = iota
	MapKindRegular
	MapKindMoney
	MapKindUseMapSettings
)

func ParseMapKind(s string) MapKind {
	switch strings.TrimSpace(s) {
	case "Regular":
		return MapKindRegular
	case "Money":
		return MapKindMoney
	case "UseMapSettings":
		return MapKindUseMapSettings
	}
	return MapKindUnknown
}

func (k MapKind) String() string {
	switch k {
	case MapKindRegular:
		return "Regular"
	case MapKindMoney:
		return "Money"
	case MapKindUseMapSettings:
		return "UseMapSettings"
	}
	return ""
}

type Flags uint8

const (
	FlagTeamStacking Flags = 1 << iota
	FlagTeamInfoIncomplete
	FlagHasComputer
	FlagIsOneOnOne
	FlagIsAutosave
)

func (f Flags) Has(flag Flags) bool { return f&flag != 0 }

type PlayerFlags uint8

const (
	PlayerObserver PlayerFlags = 1 << iota
	PlayerWinner
	PlayerDropped
	PlayerLeft
	PlayerHasStartLocation
	PlayerHasViewport
)

func (f PlayerFlags) Has(flag PlayerFlags) bool { return f&flag != 0 }

// FileRef is one on-disk file holding a replay. Several files with the same
// checksum share one Replay record.
type FileRef struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// Replay is one parsed game. Records are immutable once committed to a
// snapshot; the committer replaces them with shallow copies when file paths
// change.
type Replay struct {
	ID       int64
	Checksum [32]byte
	Paths    []FileRef
	Date     time.Time
	Duration uint16
	Frames   uint32
	Flags    Flags
	MapKind  MapKind

	HomeTeamSize uint8
	AvailSlots   uint8
	MapWidth     uint16
	MapHeight    uint16

	Title         string
	Host          uint16
	Map           uint16
	GameType      uint16
	GameSpeed     uint16
	Engine        uint16
	EngineVersion uint16
	GameSource    uint16
	LobbyKind     uint16
	TeamFormat    uint16
	Matchup       uint16

	Players  []Player
	Markers  []Marker
	Events   []GameEvent
	Prod     ProdColumns
	Chat     []ChatLine
	Alliance *AllianceTimeline
	Layout   *MapLayout
}

// Path is the newest file holding this replay.
func (r *Replay) Path() string {
	if len(r.Paths) == 0 {
		return ""
	}
	return r.Paths[0].Path
}

func (r *Replay) FileName() string { return filepath.Base(r.Path()) }

func (r *Replay) ModTime() time.Time {
	if len(r.Paths) == 0 {
		return time.Time{}
	}
	return r.Paths[0].ModTime
}

func (r *Replay) PlayerID(ordinal uint8) int64 { return PlayerID(r.ID, ordinal) }

// OneOnOne mirrors the global filter's definition: exactly two non-observer
// players on two distinct teams.
func (r *Replay) OneOnOne() bool {
	count := 0
	var teams [2]uint8
	distinct := 0
	for i := range r.Players {
		p := &r.Players[i]
		if p.IsObserver() {
			continue
		}
		count++
		if count > 2 {
			return false
		}
		seen := false
		for j := 0; j < distinct; j++ {
			if teams[j] == p.Team {
				seen = true
			}
		}
		if !seen {
			teams[distinct] = p.Team
			distinct++
		}
	}
	return count == 2 && distinct == 2
}

// HasNonObserverComputer mirrors the global filter's "exclude computers" test.
func (r *Replay) HasNonObserverComputer() bool {
	for i := range r.Players {
		if !r.Players[i].IsObserver() && r.Players[i].Type.IsComputer() {
			return true
		}
	}
	return false
}

// SortPaths orders files newest ModTime first, then by path for determinism.
func SortPaths(paths []FileRef) {
	sort.SliceStable(paths, func(i, j int) bool {
		if !paths[i].ModTime.Equal(paths[j].ModTime) {
			return paths[i].ModTime.After(paths[j].ModTime)
		}
		return paths[i].Path < paths[j].Path
	})
}

// IsAutosavePath reports whether a file lives under StarCraft's Autosave
// folder, the only evidence that the user actually played the game.
func IsAutosavePath(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	return strings.Contains(strings.ToLower(normalized), "/autosave/")
}

// PlayerKey is the corpus-wide identity of a player name.
func PlayerKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

type Player struct {
	Name           string
	Key            string
	Race           Race
	Type           PlayerType
	Team           uint8
	Slot           uint8
	ReplayPlayerID uint8
	Flags          PlayerFlags
	Color          uint16
	APM            uint16
	EAPM           uint16
	StartX         uint16
	StartY         uint16
	StartOclock    uint8
	LeaveSec       uint16
	LeaveReason    uint16
	Viewport       float32
	HotkeyStream   []byte
	Fingerprint    *Fingerprint
	Cadence        *Cadence
}

func (p *Player) IsObserver() bool { return p.Flags.Has(PlayerObserver) }
func (p *Player) IsWinner() bool   { return p.Flags.Has(PlayerWinner) }
func (p *Player) IsHuman() bool    { return p.Type == PlayerTypeHuman }
func (p *Player) IsComputer() bool { return p.Type.IsComputer() }

// Fingerprint is one player's scfingerprint feature vector for a game.
type Fingerprint struct {
	Race           Race
	FeatureVersion uint16
	ModelTag       uint16
	Frames         int32
	CmdCount       int32
	Vector         []float64
}

// Cadence holds the army production rhythm metrics for one player in one
// game over the unit cadence window. Nil on Player when the window is empty
// or the player produced nothing eligible.
type Cadence struct {
	WindowSec     uint16
	Units         uint16
	Gaps          uint16
	RatePerMinute float64
	CVGap         float64
	Burstiness    float64
	Idle20Ratio   float64
	Score         float64
}

// NoPlayer marks a replay-level marker or an event with no source/target.
const NoPlayer uint8 = 255

type Marker struct {
	Feature uint16
	Label   uint16
	Sec     uint16
	Player  uint8
	Payload []byte
}

type CastCount struct {
	Order uint16
	Count uint16
}

type GameEvent struct {
	Type                  uint16
	Sec                   uint16
	LocationBaseType      uint16
	Source                uint8
	Target                uint8
	LocationOclock        uint8
	LocationNaturalOclock uint8
	LocationMineralOnly   bool
	AttackUnits           []uint16
	CastCounts            []CastCount
	Payload               []byte
}

type ProdKind uint8

const (
	ProdTrain ProdKind = iota + 1
	ProdUnitMorph
	ProdBuildingMorph
	ProdBuild
	ProdTech
	ProdUpgrade
	ProdCast
)

func (k ProdKind) String() string {
	switch k {
	case ProdTrain:
		return "Train"
	case ProdUnitMorph:
		return "Unit Morph"
	case ProdBuildingMorph:
		return "Building Morph"
	case ProdBuild:
		return "Build"
	case ProdTech:
		return "Tech"
	case ProdUpgrade:
		return "Upgrade"
	case ProdCast:
		return "Targeted Order"
	}
	return ""
}

// Table returns the symbol table that Subject ids of this kind index.
func (k ProdKind) Table() *Table {
	switch k {
	case ProdTech:
		return Techs
	case ProdUpgrade:
		return Upgrades
	case ProdCast:
		return Orders
	}
	return Units
}

// ProdColumns is the struct-of-arrays production stream (7 bytes per event),
// sorted by second with original command order preserved within a second.
type ProdColumns struct {
	Sec     []uint16
	Player  []uint8
	Kind    []ProdKind
	Subject []uint16
	Count   []uint8
}

func (p *ProdColumns) Len() int { return len(p.Sec) }

func (p *ProdColumns) Append(sec uint16, player uint8, kind ProdKind, subject uint16, count uint8) {
	p.Sec = append(p.Sec, sec)
	p.Player = append(p.Player, player)
	p.Kind = append(p.Kind, kind)
	p.Subject = append(p.Subject, subject)
	p.Count = append(p.Count, count)
}

func (p *ProdColumns) Swap(i, j int) {
	p.Sec[i], p.Sec[j] = p.Sec[j], p.Sec[i]
	p.Player[i], p.Player[j] = p.Player[j], p.Player[i]
	p.Kind[i], p.Kind[j] = p.Kind[j], p.Kind[i]
	p.Subject[i], p.Subject[j] = p.Subject[j], p.Subject[i]
	p.Count[i], p.Count[j] = p.Count[j], p.Count[i]
}

func (p *ProdColumns) Less(i, j int) bool { return p.Sec[i] < p.Sec[j] }

// SortBySecond orders events by second, keeping original order within a second.
func (p *ProdColumns) SortBySecond() { sort.Stable(p) }

// SubjectName resolves event i's subject through the table for its kind.
func (p *ProdColumns) SubjectName(i int) string {
	return p.Kind[i].Table().Name(p.Subject[i])
}

type ChatLine struct {
	Player uint8
	Sec    uint16
	Text   string
}

type AllianceTimeline struct {
	Snapshots []AllianceSnapshot
}

// AllianceSnapshot is one observed team topology; Teams hold player ordinals.
type AllianceSnapshot struct {
	Sec      uint16
	Stacking bool
	Teams    [][]uint8
}

// MapLayout is interned per map so replays on the same map share a pointer.
type MapLayout struct {
	Name        string
	WidthTiles  uint16
	HeightTiles uint16
	Bases       []MapBase
}

type MapBase struct {
	Name             string
	Kind             string
	NaturalExpansion string
	Clock            uint8
	MineralOnly      bool
	CenterX          int32
	CenterY          int32
	Polygon          []MapPoint
}

type MapPoint struct {
	X int32
	Y int32
}

// ClampU16 narrows an int to uint16, saturating at both ends.
func ClampU16(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return uint16(v)
}

// ClampU8 narrows an int to uint8, saturating at both ends.
func ClampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 0xFF {
		return 0xFF
	}
	return uint8(v)
}
