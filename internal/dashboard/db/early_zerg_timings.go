package db

// EarlyZergTimingsRow is one Zerg player's morph / build timings in the
// early game window. DroneMorphSecs is the full ordered list (1st, 2nd,
// ...); the building / unit times are first-only.
type EarlyZergTimingsRow struct {
	PlayerID         int64
	DroneMorphSecs   []int
	FirstOverlordSec *int
	FirstPoolSec     *int
	FirstHatcherySec *int
}
