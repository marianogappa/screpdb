package screp

import (
	"fmt"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/repparser"
)

// ParseFile parses a StarCraft: Brood War replay file using the real screp library
func ParseFile(filePath string) (*rep.Replay, error) {
	// Parse the replay file using the real screp library
	replay, err := repparser.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay file: %w", err)
	}

	// Compute derived data
	replay.Compute()

	return replay, nil
}
