package propack

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MacroRange is the spread of one macro scorecard metric over the pros of a race.
type MacroRange struct {
	P10   float64 `json:"p10"`
	P25   float64 `json:"p25"`
	P50   float64 `json:"p50"`
	P75   float64 `json:"p75"`
	P90   float64 `json:"p90"`
	Games int     `json:"games"`
}

// MacroRanges holds, per race, the pro spread of every engine macro scorecard metric, keyed by the
// metric's JSON name in the game detail. scripts/pro-pack writes it from engine passes over the
// labelled pro corpus; the app never computes it.
type MacroRanges struct {
	GeneratedAt     string                           `json:"generated_at"`
	EngineVersion   int                              `json:"engine_version"`
	DetectorVersion int                              `json:"detector_version"`
	Source          string                           `json:"source,omitempty"`
	Games           map[string]int                   `json:"games"`
	Races           map[string]map[string]MacroRange `json:"races"`
}

// ForRace returns the ranges of a race named as the game detail names it, or nil.
func (r *MacroRanges) ForRace(race string) map[string]MacroRange {
	if r == nil {
		return nil
	}
	return r.Races[strings.ToLower(strings.TrimSpace(race))]
}

var loadMacroRanges = sync.OnceValues(func() (*MacroRanges, error) {
	raw, err := dataFS.ReadFile("data/macro_ranges.json")
	if err != nil {
		return nil, fmt.Errorf("propack: reading embedded macro ranges: %w", err)
	}
	var ranges MacroRanges
	if err := json.Unmarshal(raw, &ranges); err != nil {
		return nil, fmt.Errorf("propack: parsing embedded macro ranges: %w", err)
	}
	return &ranges, nil
})

// LoadMacroRanges returns the embedded macro ranges, parsed once.
func LoadMacroRanges() (*MacroRanges, error) { return loadMacroRanges() }
