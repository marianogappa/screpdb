package hotkeystream

import "sort"

// RateWindowSeconds limits hotkey rates to the first 12 minutes, so long games and short ones weigh the same
// stretch of play.
const RateWindowSeconds = 720

// Rates are a player's hotkey tempo: presses of a hotkey, and units assigned to one (Ctrl or Shift + number).
type Rates struct {
	PressesPerMinute float64 `json:"presses_per_minute"`
	RebindsPerMinute float64 `json:"rebinds_per_minute"`
}

// GameRates measures one game's hotkey tempo over its first RateWindowSeconds.
func GameRates(events []Event, gameSeconds int) (Rates, bool) {
	window := min(gameSeconds, RateWindowSeconds)
	if window <= 0 {
		return Rates{}, false
	}
	presses, rebinds := 0, 0
	for _, e := range events {
		if e.Group > 9 || int(e.Sec) > window {
			continue
		}
		switch e.Type {
		case TypeSelect:
			presses++
		case TypeAssignUnits, TypeAdd:
			rebinds++
		}
	}
	minutes := float64(window) / 60
	return Rates{PressesPerMinute: float64(presses) / minutes, RebindsPerMinute: float64(rebinds) / minutes}, true
}

// MedianRates is the per-field median of several games' rates.
func MedianRates(games []Rates) Rates {
	presses := make([]float64, len(games))
	rebinds := make([]float64, len(games))
	for i, g := range games {
		presses[i], rebinds[i] = g.PressesPerMinute, g.RebindsPerMinute
	}
	return Rates{PressesPerMinute: medianFloat(presses), RebindsPerMinute: medianFloat(rebinds)}
}

func medianFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	if len(xs)%2 == 1 {
		return xs[len(xs)/2]
	}
	return (xs[len(xs)/2-1] + xs[len(xs)/2]) / 2
}
