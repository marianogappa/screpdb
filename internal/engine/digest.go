package engine

// EngineVersion keys the digest cache: bump it when Pass or the digest format changes what a digest holds.
// Detector changes do not need a bump; detectors read digests at load time.
const EngineVersion = 2

const (
	EconomyEveryFrames = 24
	ArmyEveryFrames    = 240
	ArmyCellPixels     = 16
	NoFrame            = -1
	NoCmd              = -1
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

// ArmyUnit coordinates are in ArmyCellPixels cells.
type ArmyUnit struct {
	Owner int8
	Type  uint16
	X, Y  int32
}

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
func (d *Digest) TrustedUntil() int32 {
	until := d.LastFrame + 1
	for _, f := range []int32{d.Drift.DesyncFrame, d.Drift.ErrorFrame} {
		if f != NoFrame && f < until {
			until = f
		}
	}
	return until
}
