package detectors

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// GameEventsReplayDetector emits world-state game event entries.
type GameEventsReplayDetector struct {
	BaseReplayDetector
	valueString *string
}

func NewGameEventsReplayDetector() *GameEventsReplayDetector {
	return &GameEventsReplayDetector{}
}

func (d *GameEventsReplayDetector) Name() string {
	return "Game Events"
}

func (d *GameEventsReplayDetector) ProcessCommand(command *models.Command) bool {
	_ = command
	// Orchestrator-owned world state is updated independently.
	return false
}

func (d *GameEventsReplayDetector) Finalize() {
	d.SetFinished(true)

	ws := d.GetWorldState()
	if ws == nil {
		return
	}
	entries := ws.Entries()
	if len(entries) == 0 {
		return
	}

	payload, err := json.Marshal(entries)
	if err != nil {
		return
	}
	text := string(payload)
	d.valueString = &text
}

func (d *GameEventsReplayDetector) GetResult() *core.PatternResult {
	if !d.IsFinished() || d.valueString == nil {
		return nil
	}
	return d.BuildReplayResult(d.Name(), nil, nil, d.valueString, nil)
}

func (d *GameEventsReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.valueString != nil
}
