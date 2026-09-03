package compact

import (
	"math"
	"strings"

	"github.com/marianogappa/screpdb/internal/gamerules"
	"github.com/marianogappa/screpdb/internal/library"
)

var cadenceExcludedUnits = func() map[string]struct{} {
	set := make(map[string]struct{}, len(gamerules.UnitCadenceExcludedUnits))
	for _, name := range gamerules.UnitCadenceExcludedUnits {
		set[name] = struct{}{}
	}
	return set
}()

// cadenceFor reproduces the dashboard's unit cadence CTE for one player:
// Train / Unit Morph commands of non-excluded units inside
// [start, int(0.8*duration)], gaps between consecutive commands, population
// standard deviation, and the 9999 coefficient-of-variation fallback when the
// deviation is undefined. Nil when the window is empty or nothing was produced.
func cadenceFor(r *library.Replay, ordinal uint8) *library.Cadence {
	const start = gamerules.UnitCadenceStartSeconds
	windowEnd := int(gamerules.UnitCadenceEndFraction * float64(r.Duration))
	if windowEnd <= start {
		return nil
	}

	var times []int
	for i := 0; i < r.Prod.Len(); i++ {
		if r.Prod.Player[i] != ordinal {
			continue
		}
		if kind := r.Prod.Kind[i]; kind != library.ProdTrain && kind != library.ProdUnitMorph {
			continue
		}
		name := library.Units.Name(r.Prod.Subject[i])
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, excluded := cadenceExcludedUnits[name]; excluded {
			continue
		}
		sec := int(r.Prod.Sec[i])
		if sec < start || sec > windowEnd {
			continue
		}
		times = append(times, sec)
	}
	if len(times) == 0 {
		return nil
	}

	window := windowEnd - start
	rate := float64(len(times)) * 60.0 / float64(window)
	c := &library.Cadence{
		WindowSec:     library.ClampU16(window),
		Units:         library.ClampU16(len(times)),
		Gaps:          library.ClampU16(len(times) - 1),
		RatePerMinute: rate,
	}
	if len(times) < 2 {
		c.Score = rate / (1.0 + 9999.0)
		return c
	}

	var sum, sumSquares float64
	idle := 0
	for i := 1; i < len(times); i++ {
		gap := float64(times[i] - times[i-1])
		sum += gap
		sumSquares += gap * gap
		if gap >= gamerules.UnitCadenceIdleGapSeconds {
			idle++
		}
	}
	n := float64(len(times) - 1)
	mean := sum / n
	variance := sumSquares/n - mean*mean
	c.Idle20Ratio = float64(idle) / n
	if variance < 0 || mean == 0 {
		c.Score = rate / (1.0 + 9999.0)
		return c
	}
	cv := math.Sqrt(variance) / mean
	c.CVGap = cv
	c.Burstiness = (cv - 1.0) / (cv + 1.0)
	c.Score = rate / (1.0 + cv)
	return c
}
