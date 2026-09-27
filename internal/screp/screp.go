package screp

import (
	"fmt"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/repparser"
)

func ParseFile(filePath string) (*rep.Replay, error) {
	replay, err := repparser.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay file: %w", err)
	}

	replay.Compute()

	return replay, nil
}

func ParseFileWithDebug(filePath string) (*rep.Replay, error) {
	replay, err := repparser.ParseFileConfig(filePath, repparser.Config{
		Commands: true,
		MapData:  true,
		Debug:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay file: %w", err)
	}

	replay.Compute()

	return replay, nil
}
