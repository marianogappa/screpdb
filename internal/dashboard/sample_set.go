package dashboard

import (
	"path/filepath"
	"strings"
)

const sampleSetDirName = "sample_replays"

// samePath compares two folder settings the way the user means them: the same
// directory reached by a different spelling is the same directory.
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(strings.TrimSpace(a))
	absB, errB := filepath.Abs(strings.TrimSpace(b))
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}
