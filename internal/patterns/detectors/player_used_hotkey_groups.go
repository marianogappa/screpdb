package detectors

import (
	"sort"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// UsedHotkeyGroupsPlayerDetector detects which hotkey groups a player used.
// It stores a comma-separated sorted list, e.g. "1,3,5".
type UsedHotkeyGroupsPlayerDetector struct {
	BasePlayerDetector
	groups map[int]struct{}
}

func NewUsedHotkeyGroupsPlayerDetector() *UsedHotkeyGroupsPlayerDetector {
	return &UsedHotkeyGroupsPlayerDetector{
		groups: map[int]struct{}{},
	}
}

func (d *UsedHotkeyGroupsPlayerDetector) Name() string {
	return "Used Hotkey Groups"
}

func (d *UsedHotkeyGroupsPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}
	if command.HotkeyGroup != nil {
		d.groups[int(*command.HotkeyGroup)] = struct{}{}
	}
	return false
}

func (d *UsedHotkeyGroupsPlayerDetector) GetResult() *core.PatternResult {
	if !d.IsFinished() || len(d.groups) == 0 {
		return nil
	}
	hotkeys := make([]int, 0, len(d.groups))
	for group := range d.groups {
		hotkeys = append(hotkeys, group)
	}
	sort.Ints(hotkeys)
	parts := make([]string, 0, len(hotkeys))
	for _, group := range hotkeys {
		parts = append(parts, strconv.Itoa(group))
	}
	value := strings.Join(parts, ",")
	return d.BuildPlayerResult(d.Name(), nil, nil, &value, nil)
}

func (d *UsedHotkeyGroupsPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && len(d.groups) > 0
}
