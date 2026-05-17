package models

// UpgradeCompletionSec returns the second at which an upgrade started at
// startSec will have finished, based on the static upgrade metadata. Used by
// the worldstate drop detector to gate Overlord loads on Ventral Sacs
// completion.
//
// SecondsFromGameStart on commands is derived from raw frame count via
// icza/screp's Frame.Seconds() — Fastest-speed seconds, which matches the
// units in upgradeTable.Levels[].DurationS. No game-speed scaling needed.
//
// Returns (startSec, false) for unknown upgrade names; callers should treat
// that as "not yet completed" (i.e. don't gate on the upgrade).
func UpgradeCompletionSec(startSec int, upgradeName string) (int, bool) {
	meta, ok := LookupUpgrade(upgradeName)
	if !ok {
		return startSec, false
	}
	if meta.MaxLevel < 1 {
		return startSec, false
	}
	return startSec + int(meta.Levels[0].DurationS+0.5), true
}
