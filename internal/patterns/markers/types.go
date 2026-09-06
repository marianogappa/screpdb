// Package markers is the single source of truth for Marker definitions: a
// classifier attached to a (replay × player) that reports something interesting
// about the player's play.
//
// Each Marker carries a Rule or Custom evaluator (the match) and, for openers,
// Expert milestones (the "progamer ideal" the Build Orders tab compares against).
// Both live here so detectors and the dashboard share one definition, and adding
// a marker stays a single-file change in definitions.go.
package markers

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

type Race string

const (
	RaceZerg    Race = "Zerg"
	RaceProtoss Race = "Protoss"
	RaceTerran  Race = "Terran"
)

// BuildDedupGapSeconds collapses rapid repeat Builds of the same subject at the
// same build tile: progamers double-tap a placement (spam / misclick) and the
// earlier order is effectively cancelled. Repeats at *different* tiles are left
// alone — the time-only heuristic this replaced wrongly merged ~55% of those.
const BuildDedupGapSeconds = 3

// BuildDedupMaxSecond stops dedup firing past this second: a building destroyed
// or lifted mid-game can legitimately be rebuilt on the same spot much later.
// All opening-build-order markers finalize well before this cap.
const BuildDedupMaxSecond = 4 * 60

// TriState is the monotone decision a PredicateState reports. Once committed to
// Matched or Rejected it must stay there; Finalize collapses Pending to Rejected.
type TriState int

const (
	Pending TriState = iota
	Matched
	Rejected
)

// PredicateState is the streaming evaluator a Predicate produces. Contract:
//
//   - Observe must be idempotent once committed (further facts are ignored).
//   - Decision(now) may return Pending before the predicate's intrinsic deadline.
//   - Finalize forces a final commitment: Pending collapses to the
//     "event-never-happened" answer, which Not inverts.
type PredicateState interface {
	Observe(f cmdenrich.EnrichedCommand)
	Decision(now int) TriState
	Finalize() TriState
}

// Predicate is a factory producing a fresh PredicateState per call, so marker
// authors compose with All / Any / Not and never see the state machinery.
type Predicate func() PredicateState

// Eval is for tests and one-shot callers; the streaming detector path drives
// PredicateState directly.
func (p Predicate) Eval(facts []cmdenrich.EnrichedCommand) bool {
	if p == nil {
		return false
	}
	st := p()
	for _, f := range facts {
		st.Observe(f)
	}
	return st.Finalize() == Matched
}

// Custom evaluators, for markers that can't be expressed as a bool predicate
// (worldstate-sourced events, spatial/ratio stats).

// MarkerValue carries optional extras for a Custom evaluator's verdict. The
// presence of a replay_events row is itself the "matched" signal, so most
// markers need no value; those with auxiliary data set Payload.
type MarkerValue struct {
	Payload json.RawMessage
}

// CustomEvalContext carries replay-scoped state a Custom evaluator may need at
// Finalize. Rule-based markers only observe the command stream.
type CustomEvalContext struct {
	ReplayPlayerID byte
	Replay         *models.Replay
	WorldState     *worldstate.Engine
}

// CustomResult is the verdict plus optional extras a Custom evaluator returns
// at Finalize. Absence markers and hotkey/viewport windows each document their
// own source for DetectedAtSecond.
type CustomResult struct {
	Matched          bool
	DetectedAtSecond int
	Payload          json.RawMessage
}

// CustomEvaluator is the streaming evaluator for a Custom marker. Purely
// worldstate-sourced markers can leave Observe a no-op; Finalize is called at
// RuleDeadline or end-of-replay, whichever comes first.
type CustomEvaluator interface {
	Observe(f cmdenrich.EnrichedCommand)
	Finalize(ctx CustomEvalContext) CustomResult
}

// Tolerance describes acceptable deviation around an expert target second.
// Construct with Sym or Asym.
type Tolerance struct {
	EarlySeconds int
	LateSeconds  int
}

func Sym(v int) Tolerance { return Tolerance{EarlySeconds: v, LateSeconds: v} }

func Asym(early, late int) Tolerance { return Tolerance{EarlySeconds: early, LateSeconds: late} }

// ExpertEvent is one milestone in the progamer template. Key is the UI label
// (e.g. "Spawning Pool"); Match selects which fact counts as the occurrence.
type ExpertEvent struct {
	Key          string
	Match        FactMatcher
	TargetSecond int
	Tolerance    Tolerance
}

type FactMatcher struct {
	Kind            cmdenrich.Kind
	Subject         string
	OccurrenceIndex int // 1-indexed; defaults to 1 when zero.
}

func MatchBuild(subject string) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeBuilding, Subject: subject, OccurrenceIndex: 1}
}

func MatchNthBuild(subject string, n int) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeBuilding, Subject: subject, OccurrenceIndex: n}
}

func MatchFirstProduce(unit string) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeUnit, Subject: unit, OccurrenceIndex: 1}
}

func (m FactMatcher) Resolve(facts []cmdenrich.EnrichedCommand) (int, bool) {
	n := m.OccurrenceIndex
	if n <= 0 {
		n = 1
	}
	count := 0
	for _, f := range facts {
		if f.Kind != m.Kind || f.Subject != m.Subject {
			continue
		}
		count++
		if count == n {
			return f.Second, true
		}
	}
	return 0, false
}

// PillStyle selects the frontend's visual variant: Strong for truthy signature
// pills, Negative for absence pills, Inline for pills embedding a sub-icon.
type PillStyle string

const (
	PillStyleDefault  PillStyle = ""
	PillStyleStrong   PillStyle = "strong"
	PillStyleNegative PillStyle = "negative"
	PillStyleInline   PillStyle = "inline"
)

// SubjectKind picks where a pill's {subject} placeholder reads its value from:
// a fixed string, or a named field of the marker's payload JSON.
type SubjectKind string

const (
	SubjectKindStatic       SubjectKind = "static"
	SubjectKindPayloadField SubjectKind = "payload_field"
)

// Subject describes how the frontend resolves a {subject} placeholder in a
// Pill's Label or IconKey. All fields are optional.
type Subject struct {
	Kind  SubjectKind `json:"kind"`
	Value string      `json:"value,omitempty"`
	Field string      `json:"field,omitempty"`
}

func StaticSubject(value string) *Subject {
	return &Subject{Kind: SubjectKindStatic, Value: value}
}

func PayloadFieldSubject(field string) *Subject {
	return &Subject{Kind: SubjectKindPayloadField, Field: field}
}

// Pill describes how a marker renders on one UI surface: a non-nil pointer on
// the Marker means "show here", nil means "hide". Label and IconKey support two
// placeholders, {subject} (resolved via Subject) and {minute} (derived from
// replay_events.seconds_from_game_start at render time), e.g. "Quick {subject}"
// or "Drops at min {minute}". IconKey names a sprite resolved via getUnitIcon().
type Pill struct {
	Label   string    `json:"label,omitempty"`
	IconKey string    `json:"icon_key,omitempty"`
	Subject *Subject  `json:"subject,omitempty"`
	Style   PillStyle `json:"style,omitempty"`
	Title   string    `json:"title,omitempty"` // optional tooltip
}

// Kind lets mutually-exclusive families (openers) coexist in the registry
// alongside overlap-permitted ones (signatures, absences, worldstate events).
type Kind string

const (
	// At most one KindInitialBuildOrder may match per player (fuzz-enforced mutex).
	KindInitialBuildOrder Kind = "initial_build_order"
	// Multiple KindMarker entries may match a player at once, including alongside
	// a KindInitialBuildOrder.
	KindMarker Kind = "marker"
)

// Opener tiers (see Marker.Tier). Lower wins. Every KindInitialBuildOrder
// marker must set one; the fuzz test asserts it.
const (
	TierPreferred = 1
	TierBackup    = 2
	TierResidual  = 3
)

type Marker struct {
	// Name doubles as the pattern-name suffix stored in the DB for openers.
	Name string

	Kind Kind

	// Tier ranks competing KindInitialBuildOrder markers when several match the
	// same player: the lowest wins and is the only opener persisted (see
	// Orchestrator.GetResults), so a scene-named opener takes precedence over the
	// broad bucket it overlaps and the residual catch-all, while every classifiable
	// player still resolves to exactly one opener. Mutual exclusion is fuzz-enforced
	// only WITHIN a (race, matchup, tier) tuple; across tiers overlap is expected.
	// An unset Tier normalizes to TierBackup at registry build time.
	Tier int

	// PatternName is stored in detected_patterns_replay_player. Openers use
	// "Build Order: <Name>"; KindMarker entries use bare names, to preserve
	// existing frontend checks and DB-row compatibility.
	PatternName string

	// FeatureKey is the stable identifier used by the games-list "Featuring"
	// filter and the frontend pill registry (e.g. "bo_9_pool", "made_drops").
	FeatureKey string

	// Empty means "any race".
	Race Race

	// Matchup restricts this marker to replays whose replays.matchup is one of the
	// listed values; empty means any. Combined with Race it gates per (race,
	// matchup) tuple, which is also the granularity the opener mutex is fuzzed at.
	Matchup []string

	// MapKind restricts this marker to replays whose replays.map_kind is one of the
	// listed values; empty means any. For markers that only make sense on specific
	// economies — "10+ Scouts" is only a pattern on Money maps.
	MapKind []string

	// MinReplaySeconds gates on replay duration (0 = no gate), for "never X"
	// markers that would otherwise trip on short games.
	MinReplaySeconds int

	// MinReplaySecondsByMatchup gates on duration per (own race, opp race), and is
	// consulted for 1v1 replays only; team games and FFA use MinReplaySeconds, as do
	// missing entries. Exists because a "valid game" length depends on the matchup —
	// ZvZ first research lands far earlier than PvP, so a flat floor over-fires.
	MinReplaySecondsByMatchup map[Race]map[Race]int

	// Rule is the predicate-DSL path; exactly one of Rule / Custom is expected.
	Rule Predicate

	// Custom is the evaluator path for markers that can't be expressed as a bool
	// predicate, typically sourced from worldstate.
	Custom func() CustomEvaluator

	// RequireWorldstateEvent layers a spatial confirmation on a Rule match: the
	// marker saves only if the Rule matched AND the worldstate produced this event
	// type for the player, letting a topology opener add a location signal the fact
	// stream can't express (a defensive sim-city bunker no longer reads as a rush).
	// Markers setting this MUST use the endOfReplaySentinel RuleDeadline, because
	// the worldstate event list only exists once the full stream is processed.
	RequireWorldstateEvent string

	// Modifiers tag a matched build order with orthogonal facts that don't change
	// WHICH opener it is but materially change what it means (an "expand" 1 Gate
	// Reaver vs the one-base pressure variant). Evaluated only once the BO matches,
	// and they never gate the match — they only annotate it.
	Modifiers []Modifier

	// RuleDeadline is the last in-game second that could still change the answer;
	// past it the detector finalizes. Set to the tightest upper bound across all
	// sub-predicates.
	RuleDeadline int

	// Expert is only populated for KindInitialBuildOrder markers.
	Expert []ExpertEvent

	// A non-nil pill means "show on this surface"; nil means hide.
	SummaryPlayer *Pill

	// SummaryReplay is for markers characterising the whole game ("Threw Nukes").
	SummaryReplay *Pill

	// GamesList doubles as the featuring-filter entry.
	GamesList *Pill

	EventsList *Pill
}

// Modifier is an orthogonal tag on a matched build order. It holds when either
// its Rule matches over the same dedup'd stream the BO rule sees, or its
// WorldstateEvent was produced for the player. Exactly one of the two is set.
type Modifier struct {
	Name            string
	Rule            Predicate
	WorldstateEvent string
}
