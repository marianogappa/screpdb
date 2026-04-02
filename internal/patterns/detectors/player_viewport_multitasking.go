package detectors

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

const viewportMultitaskingWindowEndPercent = 80

type ViewportMultitaskingPlayerDetector struct {
	BasePlayerDetector
	coordinateCommands int
	viewportSwitches   int
	hasPrevious        bool
	previousX          int
	previousY          int
	windowSeconds      int
}

func NewViewportMultitaskingPlayerDetector() *ViewportMultitaskingPlayerDetector {
	return &ViewportMultitaskingPlayerDetector{}
}

func (d *ViewportMultitaskingPlayerDetector) Name() string {
	return models.PatternNameViewportMultitasking
}

func (d *ViewportMultitaskingPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || command.X == nil || command.Y == nil {
		return false
	}
	replay := d.GetReplay()
	if replay == nil {
		return false
	}
	windowEndSecond := (replay.DurationSeconds * viewportMultitaskingWindowEndPercent) / 100
	if windowEndSecond <= models.ViewportMultitaskingWindowStartSecond {
		return false
	}
	if command.SecondsFromGameStart < models.ViewportMultitaskingWindowStartSecond || command.SecondsFromGameStart > windowEndSecond {
		return false
	}
	d.windowSeconds = windowEndSecond - models.ViewportMultitaskingWindowStartSecond
	d.coordinateCommands++
	if !d.hasPrevious {
		d.previousX = *command.X
		d.previousY = *command.Y
		d.hasPrevious = true
		return false
	}

	if isViewportSwitchPosition(d.previousX, d.previousY, *command.X, *command.Y) {
		d.viewportSwitches++
	}
	d.previousX = *command.X
	d.previousY = *command.Y
	return false
}

func (d *ViewportMultitaskingPlayerDetector) Finalize() {
	d.SetFinished(true)
}

func (d *ViewportMultitaskingPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	value := fmt.Sprintf("%.6f", d.viewportSwitchRate())
	return d.BuildPlayerResult(d.Name(), nil, nil, &value, nil)
}

func (d *ViewportMultitaskingPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.windowSeconds > 0 && d.coordinateCommands > 1
}

func isViewportSwitchPosition(prevX, prevY, currentX, currentY int) bool {
	return absInt(currentX-prevX) > models.ViewportWidthPixels || absInt(currentY-prevY) > models.ViewportHeightPixels
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (d *ViewportMultitaskingPlayerDetector) viewportSwitchRate() float64 {
	if d.windowSeconds <= 0 {
		return 0
	}
	return float64(d.viewportSwitches) / (float64(d.windowSeconds) / 60)
}
