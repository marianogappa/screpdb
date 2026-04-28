package markers

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// This file hosts CustomEvaluator implementations — the escape hatch markers
// use when the predicate DSL can't express their rule. Today this means
// markers that read worldstate-synthesized narrative events, or that need
// spatial/time-window statistics.

// -----------------------------------------------------------------------------
// worldstateFirstEventEvaluator: migrates playerFirstEventByTypeDetector
// (Made drops / Made recalls / Threw Nukes / Became Terran / Became Zerg).
// Observe is a no-op — the match is entirely a worldstate query at Finalize.
// -----------------------------------------------------------------------------

type worldstateFirstEventEvaluator struct {
	eventType string // "drop", "recall", "nuke", "became_terran", "became_zerg"
}

func (e *worldstateFirstEventEvaluator) Observe(cmdenrich.EnrichedCommand) {}

func (e *worldstateFirstEventEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if ctx.WorldState == nil {
		return CustomResult{}
	}
	sec := ctx.WorldState.FirstEventSecondForPlayer(ctx.ReplayPlayerID, e.eventType)
	if sec == nil {
		return CustomResult{}
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: *sec,
	}
}

// -----------------------------------------------------------------------------
// firstCastEvaluator: matches the first second the player issues a cast
// whose canonical Subject equals the configured value. Used by "Made
// recalls" — recall casts no longer surface as standalone game events
// (folded into the attack-pressure path), so the marker reads the
// command stream directly.
// -----------------------------------------------------------------------------

type firstCastEvaluator struct {
	subject  string
	firstSec int
	matched  bool
}

func (e *firstCastEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	if e.matched {
		return
	}
	if f.Kind != cmdenrich.KindCast {
		return
	}
	if f.Subject != e.subject {
		return
	}
	e.firstSec = f.Second
	e.matched = true
}

func (e *firstCastEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	if !e.matched {
		return CustomResult{}
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: e.firstSec,
	}
}

// -----------------------------------------------------------------------------
// viewportMultitaskingEvaluator: migrates ViewportMultitaskingPlayerDetector.
// Tracks viewport-level coordinate jumps in the middle window of the replay.
// Emits a formatted switches-per-minute string (ValueString), matching the
// previous detector's storage shape so FE rendering stays identical.
// -----------------------------------------------------------------------------

const viewportMultitaskingWindowEndPercent = 80

type viewportMultitaskingEvaluator struct {
	windowStartSec     int
	coordinateCommands int
	viewportSwitches   int
	hasPrevious        bool
	previousX          int
	previousY          int
	// windowEndSec is derived from replay duration at Finalize; we stash
	// window state per-Observe because we need the per-command coordinate
	// trace. A small late-fill for windowSeconds happens in Finalize.
	observed []viewportSample
}

type viewportSample struct {
	second int
	x, y   int
}

func newViewportMultitaskingEvaluator() CustomEvaluator {
	return &viewportMultitaskingEvaluator{windowStartSec: models.ViewportMultitaskingWindowStartSecond}
}

func (v *viewportMultitaskingEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	if f.X == nil || f.Y == nil {
		return
	}
	if f.Second < v.windowStartSec {
		return
	}
	v.observed = append(v.observed, viewportSample{second: f.Second, x: *f.X, y: *f.Y})
}

func (v *viewportMultitaskingEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if ctx.Replay == nil {
		return CustomResult{}
	}
	windowEndSec := (ctx.Replay.DurationSeconds * viewportMultitaskingWindowEndPercent) / 100
	if windowEndSec <= v.windowStartSec {
		return CustomResult{}
	}
	windowSeconds := windowEndSec - v.windowStartSec
	for _, s := range v.observed {
		if s.second > windowEndSec {
			break
		}
		v.coordinateCommands++
		if !v.hasPrevious {
			v.previousX = s.x
			v.previousY = s.y
			v.hasPrevious = true
			continue
		}
		if isViewportSwitch(v.previousX, v.previousY, s.x, s.y) {
			v.viewportSwitches++
		}
		v.previousX = s.x
		v.previousY = s.y
	}
	if windowSeconds <= 0 || v.coordinateCommands <= 1 {
		return CustomResult{}
	}
	rate := float64(v.viewportSwitches) / (float64(windowSeconds) / 60.0)
	payload, err := json.Marshal(map[string]float64{"switches_per_minute": rate})
	if err != nil {
		return CustomResult{}
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: windowEndSec,
		Payload:          payload,
	}
}

func isViewportSwitch(prevX, prevY, currentX, currentY int) bool {
	return absInt(currentX-prevX) > models.ViewportWidthPixels ||
		absInt(currentY-prevY) > models.ViewportHeightPixels
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// -----------------------------------------------------------------------------
// usedHotkeyGroupsEvaluator: migrates UsedHotkeyGroupsPlayerDetector.
// Accumulates the set of hotkey groups the player used. Finalize emits a
// sorted comma-separated string (e.g. "1,3,5") as ValueString so the existing
// DB rows + FE checks keep working.
// -----------------------------------------------------------------------------

type usedHotkeyGroupsEvaluator struct {
	groups map[int]struct{}
}

func newUsedHotkeyGroupsEvaluator() CustomEvaluator {
	return &usedHotkeyGroupsEvaluator{groups: map[int]struct{}{}}
}

func (e *usedHotkeyGroupsEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	if f.Kind != cmdenrich.KindHotkey {
		return
	}
	g, err := strconv.Atoi(f.Subject)
	if err != nil {
		return
	}
	e.groups[g] = struct{}{}
}

func (e *usedHotkeyGroupsEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if len(e.groups) == 0 {
		return CustomResult{}
	}
	keys := make([]int, 0, len(e.groups))
	for g := range e.groups {
		keys = append(keys, g)
	}
	sort.Ints(keys)
	payload, err := json.Marshal(map[string][]int{"groups": keys})
	if err != nil {
		return CustomResult{}
	}
	// Commit at end-of-replay — user-confirmed convention for this marker.
	detectedAtSecond := 0
	if ctx.Replay != nil {
		detectedAtSecond = ctx.Replay.DurationSeconds
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: detectedAtSecond,
		Payload:          payload,
	}
}
