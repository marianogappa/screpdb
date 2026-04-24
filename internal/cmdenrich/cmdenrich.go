// Package cmdenrich normalizes raw models.Command into a compact, enriched
// form (EnrichedCommand) that downstream analysis (marker detection, future
// predicates) can consume without re-discriminating action types or
// re-normalizing game quirks.
//
// An EnrichedCommand answers questions the raw command can't answer cheaply:
//
//   - Is this a MakeBuilding, a MakeUnit, an Attack? (Kind, flattened from
//     both ActionType and nested OrderName fields so callers never have to
//     peer into targeted-order subtypes).
//   - What's the canonical subject? (Zerg unit morph's "Drone → Spawning
//     Pool" quirk is absorbed; callers see a single KindMakeBuilding fact.)
//   - Is it aggressive? (tri-state: Aggressive / NonAggressive / Ambiguous;
//     the per-action mapping lives here and is tunable in one place.)
//
// Locations are exposed as raw (X, Y). A follow-up enrichment step can
// resolve them against worldstate + scmapanalyzer into structured bases /
// regions; that deeper resolution lives near worldstate so cmdenrich stays
// cheap and dependency-free from the map layer.
package cmdenrich

import (
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// Kind is the flattened command category. One canonical kind per conceptual
// action — targeted-order subtypes collapse into their nearest semantic
// category here so predicates don't have to.
type Kind int

const (
	// KindUnknown is the zero value; set when the command wasn't interesting
	// enough to classify or didn't fit any known bucket.
	KindUnknown Kind = iota
	// KindMakeBuilding: a building is placed / starts construction.
	KindMakeBuilding
	// KindMakeUnit: a unit is trained or morphed (unit-morph quirk absorbed).
	KindMakeUnit
	// KindAttackMove: explicit attack-move or attack-tile order.
	KindAttackMove
	// KindAttackUnit: attack ordered at a specific target.
	KindAttackUnit
	// KindMove: plain move order.
	KindMove
	// KindPatrol: patrol order.
	KindPatrol
	// KindHold: hold-position order.
	KindHold
	// KindStop: stop order.
	KindStop
	// KindRightClick: contextual right-click (order not explicit).
	KindRightClick
	// KindTech: research / tech command.
	KindTech
	// KindUpgrade: upgrade command.
	KindUpgrade
	// KindHotkey: hotkey-group assign / select / add. Subject is the group
	// number as a string ("0".."9"); callers that care parse it.
	KindHotkey
)

// Aggression tri-state. Populated by Classify based on Kind; tune the mapping
// in the aggressionByKind table below rather than at each call site.
type Aggression int

const (
	// AggressionUnknown is the zero value for facts we haven't categorized.
	AggressionUnknown Aggression = iota
	// Aggressive: the action, in isolation, signals offensive intent.
	Aggressive
	// NonAggressive: the action is economic, defensive, or neutral.
	NonAggressive
	// Ambiguous: context-dependent (e.g. a Move into the enemy's natural
	// may be aggressive, a Move at home isn't). Predicates should lean on
	// Location to disambiguate.
	Ambiguous
)

// EnrichedCommand is the normalized view of one models.Command.
//
// Design note: callers rarely need every field. Kind + Subject + Second is
// enough for build-order detection. Location / Aggression are there for
// predicates that care about where or whether the action is hostile.
type EnrichedCommand struct {
	Kind     Kind
	Subject  string // canonical unit/building name, post-normalization
	Second   int
	PlayerID int64

	X, Y *int // raw coordinates when applicable; nil for non-spatial actions

	Aggression Aggression

	// Queued is true if the player shift-queued this order.
	Queued bool
}

// aggressionByKind is the tunable mapping from Kind to Aggression default.
// Tweak here; every caller picks up the change.
var aggressionByKind = map[Kind]Aggression{
	KindMakeBuilding:       NonAggressive,
	KindMakeUnit:     NonAggressive,
	KindTech:        NonAggressive,
	KindUpgrade:     NonAggressive,
	KindStop:        NonAggressive,
	KindHold:        NonAggressive,
	KindHotkey:      NonAggressive,
	KindPatrol:      Ambiguous,
	KindMove:        Ambiguous,
	KindRightClick:  Ambiguous,
	KindAttackMove:  Aggressive,
	KindAttackUnit:  Aggressive,
}

// Classify returns the EnrichedCommand for a raw command. The second return
// value is false when the command isn't a recognized action (Sync, Chat,
// etc.) and callers can skip it entirely.
func Classify(cmd *models.Command) (EnrichedCommand, bool) {
	if cmd == nil {
		return EnrichedCommand{}, false
	}
	kind := classifyKind(cmd)
	if kind == KindUnknown {
		return EnrichedCommand{}, false
	}
	subject := strings.TrimSpace(stringPtr(cmd.UnitType))
	// Hotkey commands carry the group in HotkeyGroup; surface it as Subject
	// ("0".."9") so predicates / evaluators can read it off the common field.
	if kind == KindHotkey {
		if cmd.HotkeyGroup == nil {
			return EnrichedCommand{}, false
		}
		subject = strconv.Itoa(int(*cmd.HotkeyGroup))
	}
	fact := EnrichedCommand{
		Kind:       kind,
		Subject:    subject,
		Second:     cmd.SecondsFromGameStart,
		PlayerID:   cmd.PlayerID,
		X:          cmd.X,
		Y:          cmd.Y,
		Aggression: aggressionByKind[kind],
		Queued:     boolPtr(cmd.IsQueued),
	}
	return fact, true
}

// FromAction is the DB-side constructor: callers that have the raw action
// type + subject + second (dashboard reading from detected_patterns DB rows)
// use this to get a fact without the full Command struct.
//
// It resolves Kind from the action-type string only — Location and
// Aggression stay at their zero values. Intended for one-shot dashboard
// reads that don't need those fields.
func FromAction(actionType, subject string, second int, playerID int64) (EnrichedCommand, bool) {
	kind := kindFromActionType(strings.TrimSpace(actionType))
	if kind == KindUnknown {
		return EnrichedCommand{}, false
	}
	return EnrichedCommand{
		Kind:       kind,
		Subject:    strings.TrimSpace(subject),
		Second:     second,
		PlayerID:   playerID,
		Aggression: aggressionByKind[kind],
	}, true
}

// classifyKind looks at ActionType, then OrderName when ActionType is
// something ambiguous like "Right Click".
func classifyKind(cmd *models.Command) Kind {
	if k := kindFromActionType(cmd.ActionType); k != KindUnknown {
		return k
	}
	// Right Click commands carry the real semantic in OrderName (Move,
	// AttackMove, Patrol, …) — flatten that into a Kind rather than
	// leaving callers to re-parse.
	if cmd.OrderName != nil {
		switch *cmd.OrderName {
		case models.UnitOrderAttackMove:
			return KindAttackMove
		case models.UnitOrderAttackUnit, models.UnitOrderAttack1, models.UnitOrderAttack2,
			models.UnitOrderAttackTile, models.UnitOrderAttackFixedRange:
			return KindAttackUnit
		case models.UnitOrderMove:
			return KindMove
		case models.UnitOrderPatrol:
			return KindPatrol
		case models.UnitOrderHoldPosition:
			return KindHold
		case models.UnitOrderStop:
			return KindStop
		}
	}
	if strings.EqualFold(cmd.ActionType, "Right Click") {
		return KindRightClick
	}
	return KindUnknown
}

func kindFromActionType(actionType string) Kind {
	switch actionType {
	case models.ActionTypeBuild:
		return KindMakeBuilding
	case models.ActionTypeTrain, models.ActionTypeUnitMorph:
		return KindMakeUnit
	case "Tech":
		return KindTech
	case "Upgrade":
		return KindUpgrade
	case "Attack Move":
		return KindAttackMove
	case "Attack":
		return KindAttackUnit
	case "Move":
		return KindMove
	case "Hold Position":
		return KindHold
	case "Patrol":
		return KindPatrol
	case "Stop":
		return KindStop
	case "Right Click":
		return KindRightClick
	case "Hotkey":
		return KindHotkey
	}
	return KindUnknown
}

func stringPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolPtr(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
