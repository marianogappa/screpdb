package engine

// EngineVersion keys the digest cache: bump it when Pass or the digest format changes what a digest holds.
// Detector changes do not need a bump; detectors read digests at load time.
//
// A bump does not discard older digests: an engine pass costs seconds per replay, and a library of
// thousands would take hours to redo. Digests from OldestReadableEngineVersion on stay in use, and a feature
// that needs newer data checks for it (UnitOrdersKnown) and is simply missing on older games. Raise
// OldestReadableEngineVersion only when older digests would mislead the features that read them.
//
// Not every bump is worth hours of background passes: BackgroundUpgradeBelow is the oldest version the
// background pass leaves alone. Raise it to EngineVersion when a bump changes what library-wide figures
// read (aggregates, the games list); leave it when a bump only adds to one game's page, and a game with an
// older digest is redone when someone opens it.
const (
	EngineVersion               = 14
	OldestReadableEngineVersion = 7
	BackgroundUpgradeBelow      = 11
)

// UpgradeInBackground says whether the background pass redoes a digest of this version.
func UpgradeInBackground(version int32) bool { return version < BackgroundUpgradeBelow }

func Readable(version int32) bool {
	return version >= OldestReadableEngineVersion && version <= EngineVersion
}

const (
	EconomyEveryFrames = 24
	// Positions every 10 s are enough to detect events; the map at an event's exact second comes from
	// an on-demand engine pass (Snapshots), so the digest stays small for replays nobody opens.
	ArmyEveryFrames = 240
	ArmyCellPixels  = 16
	NoFrame         = -1
	NoCmd           = -1
)

type Digest struct {
	EngineVersion int32
	EndFrame      int32
	LastFrame     int32
	Players       []Player
	Initial       []InitialUnit
	Productions   []Production
	Research      []Research
	Deaths        []Death
	OwnerChanges  []OwnerChange
	Outcomes      []Outcome
	Economy       []EconomySample
	Army          []ArmySnapshot
	Unloads       []Unload
	Teleports     []Teleport
	Casts         []Cast
	Hotkeys       []HotkeyAssign
	Drift         Drift
}

type Player struct {
	Slot     int8
	PlayerID int32
	Race     uint8
}

// InitialUnit is a player's unit from the game start: town hall, workers, and Overlord or Larva.
type InitialUnit struct {
	Unit  uint32
	Type  uint16
	Owner int8
	X, Y  int32
}

type Production struct {
	Owner    int8
	Type     uint16
	From     uint16
	Unit     uint32
	Producer uint32
	X, Y     int32
	Start    int32
	Complete int32
	Cancel   int32
	Cmd      int32
}

type Research struct {
	Owner    int8
	Upgrade  bool
	ID       uint8
	Level    int32
	Producer uint32
	Start    int32
	Complete int32
	Cancel   int32
}

type Death struct {
	Frame       int32
	Unit        uint32
	Type        uint16
	Owner       int8
	Class       uint8
	X, Y        int32
	Reason      uint8
	KillerType  uint16
	KillerOwner int8
	// Killer is the unit that dealt the killing damage, 0 when none or when the digest predates EngineVersion 13.
	Killer uint32
}

type OwnerChange struct {
	Frame    int32
	Unit     uint32
	Type     uint16
	From, To int8
	Reason   uint8
}

type OutcomeKind uint8

const (
	OutcomeLeft OutcomeKind = iota
	OutcomeDropped
	OutcomeDefeated
	OutcomeWon
)

type Outcome struct {
	Frame int32
	Owner int8
	Kind  OutcomeKind
}

// EconomySample supply values are doubled, as the engine counts them (a Zergling is 1).
type EconomySample struct {
	Frame            int32
	Owner            int8
	Minerals, Gas    int32
	GatheredMinerals int32
	GatheredGas      int32
	SupplyUsed       int32
	SupplyMax        int32
	Workers          int32
	Larva            int32
}

type ArmySnapshot struct {
	Frame int32
	Units []ArmyUnit
}

// ArmyUnit coordinates are in ArmyCellPixels cells. ID follows one unit from snapshot to snapshot, and
// Order is its orders.dat order, so a unit standing idle can be told from one walking to a rally point.
type ArmyUnit struct {
	Owner  int8
	Type   uint16
	X, Y   int32
	ID     uint32
	Order  uint8
	Energy int32
	// Commanded is the frame of the player's last order to this unit, NoFrame when it never had one.
	Commanded int32
}

// Unload is a transport emptying some or all of its cargo within one second.
type Unload struct {
	Frame         int32
	Transport     uint32
	TransportType uint16
	Owner         int8
	X, Y          int32
	Cargo         []uint16
	// Units are the unloaded units' IDs, sorted; empty before EngineVersion 13.
	Units []uint32
	// LoadFrame and LoadX, LoadY are where the transport last picked up this cargo, or NoFrame when unknown.
	LoadFrame    int32
	LoadX, LoadY int32
}

// Teleport is a player's units jumping across the map within one second: a Recall or a Nydus Canal.
type Teleport struct {
	Frame        int32
	Owner        int8
	FromX, FromY int32
	ToX, ToY     int32
	Units        []uint16
}

// Cast is a spell order the engine accepted, with its target point. From EngineVersion 11 it also says
// which kind of unit cast it and which unit it targeted (Target 0 and TargetOwner NoOwner for a point).
type Cast struct {
	Frame       int32
	Owner       int8
	Order       uint8
	X, Y        int32
	CasterType  uint16
	Target      uint32
	TargetType  uint16
	TargetOwner int8
}

const NoOwner = int8(-1)

type Drift struct {
	EngineError      string
	ErrorFrame       int32
	Selects          int32
	MissingSelects   int32
	FirstMissing     int32
	DesyncFrame      int32
	UnlinkedCommands int32
	LinkedCommands   int32
}

type Confidence uint8

const (
	ConfidenceHigh Confidence = iota
	ConfidenceLow
)

func (d *Digest) Confidence() Confidence {
	if d.Drift.EngineError != "" || d.Drift.DesyncFrame != NoFrame {
		return ConfidenceLow
	}
	return ConfidenceHigh
}

// TrustedUntil is the first frame whose engine data may disagree with the real game.
// UnitOrdersKnown says the army snapshots carry each unit's ID, order and energy, which digests before
// EngineVersion 8 lack.
func (d *Digest) UnitOrdersKnown() bool { return d.EngineVersion >= 8 }

// CommandsKnown says army snapshots carry when the player last ordered each unit, which digests before
// EngineVersion 10 lack.
func (d *Digest) CommandsKnown() bool { return d.EngineVersion >= 10 }

// CastTargetsKnown says casts carry their caster's type and their target unit, which digests before
// EngineVersion 11 lack.
func (d *Digest) CastTargetsKnown() bool { return d.EngineVersion >= 12 }

// UnitIDsKnown says deaths name their killer and unloads name the units they set down, which digests before
// EngineVersion 13 lack.
func (d *Digest) UnitIDsKnown() bool { return d.EngineVersion >= 13 }

func (d *Digest) TrustedUntil() int32 {
	until := d.LastFrame + 1
	for _, f := range []int32{d.Drift.DesyncFrame, d.Drift.ErrorFrame} {
		if f != NoFrame && f < until {
			until = f
		}
	}
	return until
}
