// Package cmdenrich normalizes raw models.Command into a compact EnrichedCommand
// so downstream analysis never re-discriminates action types or re-normalizes
// game quirks: Kind is flattened from both ActionType and nested OrderName,
// Subject absorbs the Zerg "Drone → Spawning Pool" morph quirk, and Aggression
// is a tunable tri-state kept in one table.
//
// Locations stay raw (X, Y). Resolving them against worldstate + scmapanalyzer
// lives near worldstate, so cmdenrich stays cheap and free of the map layer.
package cmdenrich

import (
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// Kind is the flattened command category: targeted-order subtypes collapse into
// their nearest semantic category here so predicates don't have to.
type Kind int

const (
	// KindUnknown is the zero value: unclassifiable or uninteresting.
	KindUnknown Kind = iota
	KindMakeBuilding
	// KindMakeUnit absorbs the unit-morph quirk.
	KindMakeUnit
	KindAttackMove
	KindAttackUnit
	KindMove
	KindPatrol
	KindHold
	KindStop
	// KindRightClick: contextual right-click, order not explicit.
	KindRightClick
	KindTech
	KindUpgrade
	// KindHotkey: Subject is the group number as a string ("0".."9").
	KindHotkey
	// KindCast: Subject is the spell name with any "Cast" prefix stripped, or the
	// raw OrderName when there is none (e.g. "NuclearStrike").
	KindCast
	// KindUnloadAll carries no spatial coords; the worldstate drop detector
	// backfills position from the engagement layer.
	KindUnloadAll
	// Zerg burrow toggles (queueable).
	KindBurrow
	KindUnburrow
	// Terran siege toggles (queueable).
	KindSiege
	KindUnsiege
	// KindLoad is a right-click onto a transport, synthesized from Right Click by
	// the load-classify pass. The transport's tag rides on the source Command's
	// TargetUnitTag, which is how the drop detector pairs Loads with later Unloads.
	KindLoad
	// KindLoadBunker follows the same flow as KindLoad but stays distinct because
	// bunkers aren't transports: they produce garrison signals, not drop events.
	KindLoadBunker
	// KindBuildNydusExit carries the exit's pixel position, so the offensive-nydus
	// pass can classify a forward exit and synthesize an attack event there.
	KindBuildNydusExit
	// KindEnterNydusCanal is one command per selected wave. The position targets a
	// canal end, so the pass relies on timing and count rather than these coords.
	KindEnterNydusCanal
	// KindLayMine: Subject is the raw order name, for the first-mine timing marker.
	KindLayMine
)

// Aggression is populated by Classify from Kind; tune aggressionByKind below
// rather than at each call site.
type Aggression int

const (
	AggressionUnknown Aggression = iota
	// Aggressive: the action, in isolation, signals offensive intent.
	Aggressive
	// NonAggressive: economic, defensive, or neutral.
	NonAggressive
	// Ambiguous: context-dependent (a Move into the enemy's natural is aggressive,
	// a Move at home isn't), so predicates should lean on Location.
	Ambiguous
)

// EnrichedCommand is the normalized view of one models.Command. Kind + Subject
// + Second is enough for build-order detection; Location and Aggression exist
// for predicates that care where or whether the action is hostile.
type EnrichedCommand struct {
	Kind     Kind
	Subject  string // canonical unit/building name, post-normalization
	Frame    int32
	Second   int
	PlayerID int64

	// X, Y are uniformly in PIXELS: Build's tile coordinates are normalized in
	// Classify so consumers never re-convert. Nil for non-spatial actions.
	X, Y *int

	Aggression Aggression

	Queued bool

	// Count is >1 only for a Zerg larva-morph that morphed several selected larvae
	// at once (the early filter resolves the real number). Unit-counting predicates
	// must add Count rather than 1, or multi-larva morphs get undercounted.
	Count int

	// OrderName preserves the raw order for KindCast and any KindRightClick that
	// carried an explicit one, so worldstate can tell PsionicStorm from Recall from
	// Restoration without re-walking the raw stream. Empty otherwise.
	OrderName string

	// TargetUnitTag is the tag of the unit being right-clicked, populated for
	// KindLoad / KindLoadBunker and any order whose source Command carries one. The
	// drop detector tracks transport positions through it.
	TargetUnitTag *uint16
}

// Tweak here; every caller picks up the change.
var aggressionByKind = map[Kind]Aggression{
	KindMakeBuilding:    NonAggressive,
	KindMakeUnit:        NonAggressive,
	KindTech:            NonAggressive,
	KindUpgrade:         NonAggressive,
	KindStop:            NonAggressive,
	KindHold:            NonAggressive,
	KindHotkey:          NonAggressive,
	KindBurrow:          NonAggressive,
	KindUnburrow:        NonAggressive,
	KindUnsiege:         NonAggressive,
	KindSiege:           Ambiguous,
	KindPatrol:          Ambiguous,
	KindMove:            Ambiguous,
	KindRightClick:      Ambiguous,
	KindAttackMove:      Aggressive,
	KindAttackUnit:      Aggressive,
	KindCast:            Aggressive,
	KindUnloadAll:       Aggressive,
	KindLoad:            NonAggressive,
	KindLoadBunker:      NonAggressive,
	KindBuildNydusExit:  Ambiguous,
	KindEnterNydusCanal: Aggressive,
	KindLayMine:         Aggressive,
}

// Classify returns false when the command isn't a recognized action (Sync,
// Chat, …) and callers can skip it entirely.
func Classify(cmd *models.Command) (EnrichedCommand, bool) {
	if cmd == nil {
		return EnrichedCommand{}, false
	}
	kind := classifyKind(cmd)
	if kind == KindUnknown {
		return EnrichedCommand{}, false
	}
	subject := strings.TrimSpace(stringPtr(cmd.UnitType))
	// Surface the hotkey group as Subject so predicates read it off the common field.
	if kind == KindHotkey {
		if cmd.HotkeyGroup == nil {
			return EnrichedCommand{}, false
		}
		subject = strconv.Itoa(int(*cmd.HotkeyGroup))
	}
	// Strip the "Cast" prefix so Subject is the canonical spell name; nuke variants
	// that lack the prefix pass through unchanged.
	if kind == KindCast {
		on := stringPtr(cmd.OrderName)
		subject = strings.TrimPrefix(on, "Cast")
	}
	// Tech / Upgrade carry their canonical name in TechName / UpgradeName, not
	// UnitType. Before this, Subject was empty for these kinds and every
	// phase/composition/timing rule reading it silently misclassified them. The
	// Never-Upgraded / Never-Researched predicates key on this Subject to split HP
	// upgrades from research-grade ones.
	if kind == KindTech {
		subject = strings.TrimSpace(stringPtr(cmd.TechName))
	}
	if kind == KindUpgrade {
		subject = strings.TrimSpace(stringPtr(cmd.UpgradeName))
	}
	// Lay-mine carries no UnitType, so surface the order name for the first-mine
	// timing marker to match on.
	if kind == KindLayMine {
		subject = strings.TrimSpace(stringPtr(cmd.OrderName))
	}
	// Surface the transport's type from the source Command's TargetUnitType so
	// worldstate can read it off the canonical field.
	if kind == KindLoad || kind == KindLoadBunker {
		subject = strings.TrimSpace(stringPtr(cmd.TargetUnitType))
	}
	orderName := stringPtr(cmd.OrderName)
	// Prefer the Player pointer: test fixtures and some parser paths populate it
	// but leave PlayerID at zero. Mirror of engine.playerIDFromCommand.
	playerID := cmd.PlayerID
	if cmd.Player != nil {
		playerID = int64(cmd.Player.PlayerID)
	}
	// Normalize to PIXELS. Only Build commands carry TILE coordinates in the raw
	// stream; everything spatial else is already pixels. Converting once here is
	// what keeps a missed per-consumer conversion from recurring — that is exactly
	// what put a building's tile coords into pixel logic and landed a "drop" in the
	// map corner. BuildNydusExit is a build issued via TargetedOrder, so it is in
	// tiles too.
	xPx, yPx := cmd.X, cmd.Y
	if (kind == KindMakeBuilding || kind == KindBuildNydusExit) && xPx != nil && yPx != nil {
		px, py := *xPx*32+16, *yPx*32+16
		xPx, yPx = &px, &py
	}
	count := cmd.MorphUnitCount
	if count < 1 {
		count = 1
	}
	fact := EnrichedCommand{
		Kind:          kind,
		Subject:       subject,
		Frame:         cmd.Frame,
		Second:        cmd.SecondsFromGameStart,
		PlayerID:      playerID,
		X:             xPx,
		Y:             yPx,
		Aggression:    aggressionByKind[kind],
		Queued:        boolPtr(cmd.IsQueued),
		OrderName:     orderName,
		TargetUnitTag: cmd.TargetUnitTag,
		Count:         count,
	}
	return fact, true
}

// FromAction is the DB-side constructor for callers holding only action type,
// subject and second (the dashboard reading detected_patterns rows). Kind comes
// from the action-type string alone; Location and Aggression stay zero.
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

func classifyKind(cmd *models.Command) Kind {
	// Nydus orders take precedence over ActionType: BuildNydusExit arrives as
	// ActionType="Build", so without this it classifies as a plain
	// KindMakeBuilding and the offensive-nydus pass never sees it.
	// EnterNydusCanal is here for completeness though screp rarely emits it.
	if cmd.OrderName != nil {
		switch *cmd.OrderName {
		case models.UnitOrderBuildNydusExit:
			return KindBuildNydusExit
		case models.UnitOrderEnterNydusCanal:
			return KindEnterNydusCanal
		case models.UnitOrderPlaceMine, models.UnitOrderVultureMine:
			return KindLayMine
		}
	}
	if k := kindFromActionType(cmd.ActionType); k != KindUnknown {
		return k
	}
	// Right Click and Targeted Order carry the real semantic in OrderName, so
	// flatten it into a Kind rather than leaving callers to re-parse. Matched on a
	// space-stripped lowercase form so a fixture's "Attack Move" resolves the same
	// as the parser's "AttackMove".
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
		on := *cmd.OrderName
		onNorm := strings.ToLower(strings.ReplaceAll(on, " ", ""))
		switch onNorm {
		case "attackmove":
			return KindAttackMove
		case "attackunit", "attack1", "attack2", "attacktile", "attackfixedrange", "attack":
			return KindAttackUnit
		case "move":
			return KindMove
		case "patrol":
			return KindPatrol
		case "holdposition", "hold":
			return KindHold
		case "stop":
			return KindStop
		}
		// The nuke family (NukeLaunch, NuclearStrike, …) lacks the "Cast" prefix but
		// the engine treats it as casts for routing.
		if strings.HasPrefix(on, "Cast") ||
			strings.HasPrefix(on, "Nuke") ||
			on == "NuclearStrike" {
			return KindCast
		}
		// Unload variants on TargetedOrder, needed for spatial drop detection. The
		// QueueableCmd UnloadAll path also lands on KindUnloadAll but carries no X/Y,
		// so it is filtered out downstream.
		if strings.Contains(strings.ToLower(on), "unload") {
			return KindUnloadAll
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
	case "Unload All":
		return KindUnloadAll
	case "Burrow":
		return KindBurrow
	case "Unburrow":
		return KindUnburrow
	case "Siege":
		return KindSiege
	case "Unsiege":
		return KindUnsiege
	case "Load":
		return KindLoad
	case "LoadBunker":
		return KindLoadBunker
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
