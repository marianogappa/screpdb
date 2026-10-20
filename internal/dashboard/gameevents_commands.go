package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/samber/lo"
)

// The command source of game events: what the ingest-time worldstate detectors guessed from the
// command stream and persisted per replay. A game the engine has not run takes its events from here.

type overlayBaseMeta struct {
	Base       workflowGameEventBase
	IsStarting bool
}

func replayEventsFromRows(rows []db.ReplayEventRow, mapLayout *models.MapContextLayout, startClockByPlayerID map[int64]int) []workflowGameEvent {
	baseMetas := overlayBaseMetasFromLayout(mapLayout)
	baseByKey := map[string]workflowGameEventBase{}
	ownershipByBaseKey := map[string]*workflowGameEventPlayer{}
	events := make([]workflowGameEvent, 0, len(rows))
	for _, row := range rows {
		event := workflowGameEvent{
			Type:             row.EventType,
			Second:           row.Second,
			Ownership:        []workflowGameOwnership{},
			AttackUnitTypes:  parseAttackUnitTypes(row.AttackUnitTypes),
			AttackCastCounts: parseAttackCastCounts(row.AttackCastCounts),
		}
		if row.SourcePlayerID != nil {
			event.Actor = &workflowGameEventPlayer{
				PlayerID: *row.SourcePlayerID,
				Name:     row.SourcePlayerName,
				Color:    row.SourcePlayerColor,
			}
		}
		if row.TargetPlayerID != nil {
			event.Target = &workflowGameEventPlayer{
				PlayerID: *row.TargetPlayerID,
				Name:     row.TargetPlayerName,
				Color:    row.TargetPlayerColor,
			}
		}
		if row.LocationBaseType != nil || row.LocationBaseOclock != nil {
			if matchedBase, ok := lookupOverlayBase(baseMetas, row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock); ok {
				baseCopy := matchedBase
				if label := baseLabel(row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock); strings.TrimSpace(label) != "" {
					baseCopy.Name = label
				}
				baseCopy.NaturalOfClock = row.LocationNaturalOfClock
				if row.LocationMineralOnly != nil && *row.LocationMineralOnly {
					baseCopy.MineralOnly = row.LocationMineralOnly
				}
				event.Base = &baseCopy
				baseByKey[baseKeyForEvent(&event)] = baseCopy
			}
		}
		if event.Base == nil && (row.LocationBaseType != nil || row.LocationBaseOclock != nil) {
			base := workflowGameEventBase{
				Name:           baseLabel(row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock),
				NaturalOfClock: row.LocationNaturalOfClock,
				Center: workflowGameEventPoint{
					X: 0,
					Y: 0,
				},
			}
			if row.LocationBaseType != nil {
				base.Kind = *row.LocationBaseType
			}
			if row.LocationBaseOclock != nil {
				base.Clock = *row.LocationBaseOclock
			}
			if row.LocationMineralOnly != nil && *row.LocationMineralOnly {
				base.MineralOnly = row.LocationMineralOnly
			}
			event.Base = &base
			baseByKey[baseKeyForEvent(&event)] = base
		}
		// Open-field 1v1 attacks (issue #186) have no base columns but carry a
		// relational location in the payload; surface it as the event's label.
		if event.Base == nil && row.EventType == "attack" && row.Payload != nil {
			if loc, x, y, ok := parseAttackRelativeLocation(*row.Payload); ok {
				event.Base = &workflowGameEventBase{
					Name:   loc,
					Kind:   "open_field",
					Center: workflowGameEventPoint{X: x, Y: y},
				}
			}
		}
		if event.Actor != nil {
			if startClock, ok := startClockByPlayerID[event.Actor.PlayerID]; ok {
				event.ActorStartClock = lo.ToPtr(int64(startClock))
				if startBase, startBaseOK := lookupOverlayBaseByClock(baseMetas, int64(startClock)); startBaseOK {
					startCenter := startBase.Center
					event.ActorOrigin = &startCenter
				}
			}
		}
		if event.Target != nil {
			if startClock, ok := startClockByPlayerID[event.Target.PlayerID]; ok {
				event.TargetStartClock = lo.ToPtr(int64(startClock))
			}
		}
		applyOwnershipTransition(&event, ownershipByBaseKey)
		event.Ownership = ownershipSnapshot(ownershipByBaseKey, baseByKey)
		if event.ActorOrigin == nil && event.Actor != nil {
			if fallbackBase, ok := fallbackActorOriginFromOwnership(event.Actor.PlayerID, ownershipByBaseKey, baseByKey); ok {
				center := fallbackBase.Center
				event.ActorOrigin = &center
			}
		}
		if event.Type == "recall" && row.Payload != nil && *row.Payload != "" {
			applyRecallPayload(&event, *row.Payload, baseMetas)
		}
		if isDropEventType(event.Type) && row.Payload != nil && *row.Payload != "" {
			applyDropPayload(&event, *row.Payload, baseMetas)
		}
		// nydus_attack reuses the drop payload shape (source = home, target = forward
		// exit), so the same source/target/base stamping applies.
		if event.Type == "nydus_attack" && row.Payload != nil && *row.Payload != "" {
			applyDropPayload(&event, *row.Payload, baseMetas)
		}
		if event.Type == "late_alliance" && row.Payload != nil && *row.Payload != "" {
			applyAlliancePayload(&event, *row.Payload)
		}
		events = append(events, event)
	}
	return events
}

// applyAlliancePayload stamps name-only entries; PlayerID and Color are filled
// by resolveAllianceTeamPlayers, which has access to the player roster.
func applyAlliancePayload(event *workflowGameEvent, raw string) {
	var pl struct {
		Teams [][]string `json:"teams"`
	}
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	teams := make([][]workflowGameEventPlayer, 0, len(pl.Teams))
	for _, team := range pl.Teams {
		if len(team) < 2 {
			continue
		}
		row := make([]workflowGameEventPlayer, 0, len(team))
		for _, name := range team {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			row = append(row, workflowGameEventPlayer{Name: name})
		}
		if len(row) >= 2 {
			teams = append(teams, row)
		}
	}
	if len(teams) > 0 {
		event.AllianceTeams = teams
	}
}

// resolveAllianceTeamPlayers fills PlayerID and Color on every AllianceTeams
// entry. It is a post-pass because applyAlliancePayload runs inside
// replayEventsFromRows, where the roster isn't available.
//
// Lookups are keyed by BOTH raw and display name: detail.Players carries the
// display name, so a raw payload entry like "chobo86" would otherwise miss a
// player whose display name is "chobo86 (you)".
func resolveAllianceTeamPlayers(events []workflowGameEvent, players []workflowGamePlayer, displayByName map[string]string) []workflowGameEvent {
	if len(events) == 0 {
		return events
	}
	byName := make(map[string]*workflowGamePlayer, len(players)*2)
	for i := range players {
		byName[players[i].Name] = &players[i]
	}
	for raw, display := range displayByName {
		if p, ok := byName[display]; ok {
			byName[raw] = p
		}
	}
	for i := range events {
		if events[i].Type != "late_alliance" || len(events[i].AllianceTeams) == 0 {
			continue
		}
		for ti := range events[i].AllianceTeams {
			team := events[i].AllianceTeams[ti]
			for pi := range team {
				if p, ok := byName[team[pi].Name]; ok {
					team[pi].PlayerID = p.PlayerID
					team[pi].Color = p.Color
				}
			}
		}
	}
	return events
}

func isDropEventType(t string) bool {
	switch t {
	case "drop", "cliff_drop":
		return true
	}
	return false
}

func applyDropPayload(event *workflowGameEvent, raw string, baseMetas []overlayBaseMeta) {
	var pl recallEventPayload
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	if len(pl.S) == 2 {
		event.SourcePoint = &workflowGameEventPoint{X: float64(pl.S[0]), Y: float64(pl.S[1])}
	}
	if len(pl.T) == 2 {
		event.TargetPoint = &workflowGameEventPoint{X: float64(pl.T[0]), Y: float64(pl.T[1])}
	}
	if pl.N > 1 {
		event.DropCount = int64(pl.N)
	}
	if pl.LE > 0 {
		event.DropLastSecond = int64(pl.LE)
	}
	event.DropTargetVia = pl.TV
	if pl.SB != nil {
		var baseTypePtr *string
		if pl.SB.K != "" {
			k := pl.SB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.SB.O
		var oclockPtr *int64
		if pl.SB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.SB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.SB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.SB.NO
			if pl.SB.MO != nil && *pl.SB.MO {
				baseCopy.MineralOnly = pl.SB.MO
			}
			event.SourceBase = &baseCopy
		}
	}
	if pl.TB != nil {
		var baseTypePtr *string
		if pl.TB.K != "" {
			k := pl.TB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.TB.O
		var oclockPtr *int64
		if pl.TB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.TB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.TB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.TB.NO
			if pl.TB.MO != nil && *pl.TB.MO {
				baseCopy.MineralOnly = pl.TB.MO
			}
			event.TargetBase = &baseCopy
		}
	}
}

// recallEventPayload mirrors the JSON worldstate.emitRecallEvents produces;
// short keys keep storage cost down. The drop payload reuses this shape with an
// extra SB field, because recall's source IS event.base while drops need a
// separate source ref.
type recallEventPayload struct {
	N  int                       `json:"n,omitempty"`
	LE int                       `json:"le,omitempty"`
	S  []int                     `json:"s,omitempty"`
	SB *recallEventTargetBaseRef `json:"sb,omitempty"`
	T  []int                     `json:"t,omitempty"`
	TB *recallEventTargetBaseRef `json:"tb,omitempty"`
	TP int64                     `json:"tp,omitempty"`
	TV string                    `json:"tv,omitempty"`
}

type recallEventTargetBaseRef struct {
	K  string `json:"k,omitempty"`
	O  int64  `json:"o,omitempty"`
	NO *int64 `json:"no,omitempty"`
	MO *bool  `json:"mo,omitempty"`
}

// The target_owner field needs the per-replay player list to resolve PID →
// name/color, so it is filled in a second pass after replayEventsFromRows.
func applyRecallPayload(event *workflowGameEvent, raw string, baseMetas []overlayBaseMeta) {
	var pl recallEventPayload
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	if len(pl.S) == 2 {
		event.SourcePoint = &workflowGameEventPoint{X: float64(pl.S[0]), Y: float64(pl.S[1])}
	}
	if len(pl.T) == 2 {
		event.TargetPoint = &workflowGameEventPoint{X: float64(pl.T[0]), Y: float64(pl.T[1])}
	}
	if pl.N > 1 {
		event.RecallCount = int64(pl.N)
	}
	if pl.LE > 0 {
		event.RecallLastSecond = int64(pl.LE)
	}
	event.RecallTargetVia = pl.TV
	if pl.TB != nil {
		var baseTypePtr *string
		if pl.TB.K != "" {
			k := pl.TB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.TB.O
		var oclockPtr *int64
		if pl.TB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.TB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.TB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.TB.NO
			if pl.TB.MO != nil && *pl.TB.MO {
				baseCopy.MineralOnly = pl.TB.MO
			}
			event.TargetBase = &baseCopy
		}
	}
}

// resolveNaturalOwnerLabels relabels a natural expansion with its owner's name
// ("Skins_'s natural") instead of the start-clock form ("7's natural") when the
// natural_of_clock matches a player's start location (issue #186). Falls through
// unchanged when no owner is found, preserving the clock-based label.
func resolveNaturalOwnerLabels(events []workflowGameEvent, players []workflowGamePlayer, startClockByPlayerID map[int64]int) []workflowGameEvent {
	if len(events) == 0 || len(startClockByPlayerID) == 0 {
		return events
	}
	nameByID := make(map[int64]string, len(players))
	for _, p := range players {
		nameByID[p.PlayerID] = p.Name
	}
	nameByClock := map[int64]string{}
	for id, clock := range startClockByPlayerID {
		if name, ok := nameByID[id]; ok {
			nameByClock[int64(clock)] = name
		}
	}
	for i := range events {
		b := events[i].Base
		if b == nil || strings.ToLower(b.Kind) != "natural" || b.NaturalOfClock == nil {
			continue
		}
		if name, ok := nameByClock[*b.NaturalOfClock]; ok && strings.TrimSpace(name) != "" {
			b.Name = name + "'s natural"
		}
	}
	return events
}

func resolveRecallTargetOwners(events []workflowGameEvent, players []workflowGamePlayer) []workflowGameEvent {
	if len(events) == 0 || len(players) == 0 {
		return events
	}
	playerByID := make(map[int64]workflowGamePlayer, len(players))
	for _, p := range players {
		playerByID[p.PlayerID] = p
	}
	for i := range events {
		if events[i].Type != "recall" && !isDropEventType(events[i].Type) {
			continue
		}
		if events[i].TargetBase == nil {
			continue
		}
		// The payload is gone at this layer, so re-derive target_owner from
		// event.Target, populated upstream from the row's target_player_id. Worldstate
		// writes Target only when the destination's owner is hostile, so for both
		// recall and drop, Target ⇔ TargetOwner.
		if events[i].Target != nil {
			cp := *events[i].Target
			if p, ok := playerByID[cp.PlayerID]; ok {
				cp.Name = p.Name
				cp.Color = p.Color
			}
			events[i].TargetOwner = &cp
		}
	}
	return events
}

func overlayBaseMetasFromLayout(layout *models.MapContextLayout) []overlayBaseMeta {
	if layout == nil || len(layout.Bases) == 0 {
		return nil
	}
	// Build natural_base_name -> start_clock so NaturalOfClock can be stamped onto
	// the natural's overlay metadata. Without it, a natural and an unrelated expa
	// at the same o'clock collapse to one (kind, clock) key and paint the wrong
	// polygon.
	startClockByNaturalName := map[string]int64{}
	for _, base := range layout.Bases {
		if !strings.EqualFold(strings.TrimSpace(base.Kind), "start") {
			continue
		}
		naturalName := strings.TrimSpace(base.NaturalExpansion)
		if naturalName == "" {
			continue
		}
		startClockByNaturalName[naturalName] = int64(base.Clock)
	}
	out := make([]overlayBaseMeta, 0, len(layout.Bases))
	for _, base := range layout.Bases {
		polygon := make([]workflowGameEventPoint, 0, len(base.Polygon))
		for _, vertex := range base.Polygon {
			polygon = append(polygon, workflowGameEventPoint{X: float64(vertex.X), Y: float64(vertex.Y)})
		}
		kind := strings.TrimSpace(base.Kind)
		prettyName := strings.TrimSpace(base.Name)
		if prettyName == "" {
			if base.Clock == 0 {
				prettyName = "center base"
			} else if base.Clock >= 1 && base.Clock <= 12 {
				prettyName = fmt.Sprintf("%d", base.Clock)
			}
		}
		isStarting := strings.EqualFold(kind, "start") || strings.EqualFold(kind, "starting")
		var naturalOfClock *int64
		if !isStarting {
			if clock, ok := startClockByNaturalName[strings.TrimSpace(base.Name)]; ok {
				clockCopy := clock
				naturalOfClock = &clockCopy
			}
		}
		out = append(out, overlayBaseMeta{
			Base: workflowGameEventBase{
				Name:           prettyName,
				Kind:           kind,
				Clock:          int64(base.Clock),
				NaturalOfClock: naturalOfClock,
				MineralOnly:    lo.Ternary(base.MineralOnly, lo.ToPtr(true), nil),
				Center:         workflowGameEventPoint{X: float64(base.Center.X), Y: float64(base.Center.Y)},
				Polygon:        polygon,
			},
			IsStarting: isStarting,
		})
	}
	return out
}

func lookupOverlayBase(baseMetas []overlayBaseMeta, baseType *string, baseOclock *int64, naturalOfOclock *int64) (workflowGameEventBase, bool) {
	if baseOclock == nil {
		return workflowGameEventBase{}, false
	}
	targetClock := *baseOclock
	targetType := strings.ToLower(strings.TrimSpace(nullableString(baseType)))
	// Primary pass: (kind, clock[, natural_of_clock]), where natural_of_clock is
	// what separates a natural from a coincident expa at the same clock.
	for _, candidate := range baseMetas {
		if candidate.Base.Clock != targetClock {
			continue
		}
		switch targetType {
		case "starting":
			if !candidate.IsStarting {
				continue
			}
		case "natural":
			if candidate.IsStarting {
				continue
			}
			if candidate.Base.NaturalOfClock == nil {
				continue
			}
			if naturalOfOclock == nil || *candidate.Base.NaturalOfClock != *naturalOfOclock {
				continue
			}
		default: // "expansion" and anything else
			if candidate.IsStarting {
				continue
			}
			if candidate.Base.NaturalOfClock != nil {
				continue
			}
		}
		return candidate.Base, true
	}
	// Kind-agnostic clock fallback, preserving prior behaviour when the primary
	// pass fails (layout missing, natural-of-clock unmapped) so rendering stays
	// best-effort rather than dropping the polygon.
	for _, candidate := range baseMetas {
		if candidate.Base.Clock != targetClock {
			continue
		}
		if targetType == "starting" && !candidate.IsStarting {
			continue
		}
		if targetType != "starting" && candidate.IsStarting {
			continue
		}
		return candidate.Base, true
	}
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == targetClock {
			return candidate.Base, true
		}
	}
	return workflowGameEventBase{}, false
}

func lookupOverlayBaseByClock(baseMetas []overlayBaseMeta, clock int64) (workflowGameEventBase, bool) {
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == clock && candidate.IsStarting {
			return candidate.Base, true
		}
	}
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == clock {
			return candidate.Base, true
		}
	}
	return workflowGameEventBase{}, false
}

func baseKeyForEvent(event *workflowGameEvent) string {
	if event == nil || event.Base == nil {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(event.Base.Kind))
	// Two players' naturals can sit at the same clock, and an expa can share a
	// clock with a natural, so without natural_of_clock in the key ownership
	// bookkeeping collapses them.
	if kind == "natural" && event.Base.NaturalOfClock != nil {
		return fmt.Sprintf("natural|%d|%d", *event.Base.NaturalOfClock, event.Base.Clock)
	}
	if event.Base.Clock >= 0 && (event.Base.Clock > 0 || hasValidCenterBaseKind(kind)) {
		return fmt.Sprintf("%s|%d", kind, event.Base.Clock)
	}
	return fmt.Sprintf("%s|%s", kind, strings.ToLower(strings.TrimSpace(event.Base.Name)))
}

// Base kinds that can legitimately carry clock=0 (scmapanalyzer's "center
// base"). Without this they silently fall through to name-based keying.
func hasValidCenterBaseKind(kind string) bool {
	switch kind {
	case "start", "starting", "natural", "expansion", "expa":
		return true
	}
	return false
}

func applyOwnershipTransition(event *workflowGameEvent, ownership map[string]*workflowGameEventPlayer) {
	if event == nil {
		return
	}
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	switch eventType {
	case "player_start", "expansion", "takeover":
		baseKey := baseKeyForEvent(event)
		if baseKey != "" && event.Actor != nil {
			ownerCopy := *event.Actor
			ownership[baseKey] = &ownerCopy
		}
	case "location_inactive":
		baseKey := baseKeyForEvent(event)
		if baseKey != "" {
			delete(ownership, baseKey)
		}
	case "leave_game", "player_dropped":
		if event.Actor == nil {
			return
		}
		for key, owner := range ownership {
			if owner != nil && owner.PlayerID == event.Actor.PlayerID {
				delete(ownership, key)
			}
		}
	}
}

func ownershipSnapshot(ownership map[string]*workflowGameEventPlayer, baseByKey map[string]workflowGameEventBase) []workflowGameOwnership {
	if len(ownership) == 0 {
		return []workflowGameOwnership{}
	}
	keys := make([]string, 0, len(ownership))
	for key := range ownership {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]workflowGameOwnership, 0, len(keys))
	for _, key := range keys {
		owner := ownership[key]
		base, ok := baseByKey[key]
		if !ok {
			continue
		}
		var ownerCopy *workflowGameEventPlayer
		if owner != nil {
			value := *owner
			ownerCopy = &value
		}
		out = append(out, workflowGameOwnership{Base: base, Owner: ownerCopy})
	}
	return out
}

func fallbackActorOriginFromOwnership(playerID int64, ownership map[string]*workflowGameEventPlayer, baseByKey map[string]workflowGameEventBase) (workflowGameEventBase, bool) {
	for key, owner := range ownership {
		if owner == nil || owner.PlayerID != playerID {
			continue
		}
		base, ok := baseByKey[key]
		if !ok {
			continue
		}
		if strings.EqualFold(base.Kind, "start") || strings.EqualFold(base.Kind, "starting") {
			return base, true
		}
	}
	for key, owner := range ownership {
		if owner == nil || owner.PlayerID != playerID {
			continue
		}
		base, ok := baseByKey[key]
		if ok {
			return base, true
		}
	}
	return workflowGameEventBase{}, false
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func baseLabel(baseType *string, baseOclock *int64, naturalOf *int64) string {
	if baseType == nil {
		return ""
	}
	// oclock==0 is scmapanalyzer's "center base". None of the templated labels read
	// right with 0 inserted, so short-circuit to a clear literal.
	isCenter := func(v *int64) bool { return v != nil && *v == 0 }
	switch strings.ToLower(strings.TrimSpace(*baseType)) {
	case "starting":
		if isCenter(baseOclock) {
			return "center base"
		}
		if baseOclock != nil {
			return fmt.Sprintf("%d", *baseOclock)
		}
		return "starting base"
	case "natural":
		if isCenter(baseOclock) {
			if isCenter(naturalOf) {
				return "center base"
			}
			if naturalOf != nil {
				return fmt.Sprintf("%d's natural (center base)", *naturalOf)
			}
			return "center base"
		}
		if naturalOf != nil {
			if baseOclock != nil && *baseOclock != *naturalOf {
				return fmt.Sprintf("%d's natural near %d", *naturalOf, *baseOclock)
			}
			return fmt.Sprintf("%d's natural", *naturalOf)
		}
		if baseOclock != nil {
			return fmt.Sprintf("%d's natural", *baseOclock)
		}
		return "natural expansion"
	default:
		if isCenter(baseOclock) {
			return "center base"
		}
		if baseOclock != nil {
			return fmt.Sprintf("an expansion near %d", *baseOclock)
		}
		return "expansion"
	}
}

// ok is false for any payload that doesn't carry a label (issue #186).
func parseAttackRelativeLocation(raw string) (loc string, x, y float64, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", 0, 0, false
	}
	var p struct {
		Loc string  `json:"loc"`
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
	}
	if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
		return "", 0, 0, false
	}
	label := strings.TrimSpace(p.Loc)
	if label == "" {
		return "", 0, 0, false
	}
	return label, p.X, p.Y, true
}

func parseAttackUnitTypes(raw *string) []string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	var unitTypes []string
	if err := json.Unmarshal([]byte(trimmed), &unitTypes); err != nil {
		return nil
	}
	filtered := make([]string, 0, len(unitTypes))
	seen := map[string]struct{}{}
	for _, unitType := range unitTypes {
		name := strings.TrimSpace(unitType)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		filtered = append(filtered, name)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func parseAttackCastCounts(raw *string) map[string]int64 {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	var counts map[string]int64
	if err := json.Unmarshal([]byte(trimmed), &counts); err != nil {
		return nil
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

// commandGameEvents reads the events the command-stream detectors persisted for a replay, labelled
// with the players' display names.
func (d *Dashboard) commandGameEvents(detail *workflowGameDetail, mapLayout *models.MapContextLayout, startClockByPlayerID map[int64]int, displayByName map[string]string) error {
	eventRows, err := d.dbStore.ListReplayEvents(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query replay events: %w", err)
	}
	detail.GameEvents = replayEventsFromRows(eventRows, mapLayout, startClockByPlayerID)
	detail.GameEvents = resolveNaturalOwnerLabels(detail.GameEvents, detail.Players, startClockByPlayerID)
	detail.GameEvents = resolveRecallTargetOwners(detail.GameEvents, detail.Players)
	detail.GameEvents = resolveAllianceTeamPlayers(detail.GameEvents, detail.Players, displayByName)
	for i := range detail.GameEvents {
		event := &detail.GameEvents[i]
		if event.Actor != nil {
			if displayName, ok := displayByName[event.Actor.Name]; ok {
				event.Actor.Name = displayName
			}
		}
		if event.Target != nil {
			if displayName, ok := displayByName[event.Target.Name]; ok {
				event.Target.Name = displayName
			}
		}
		if event.TargetOwner != nil {
			if displayName, ok := displayByName[event.TargetOwner.Name]; ok {
				event.TargetOwner.Name = displayName
			}
		}
		for ti := range event.AllianceTeams {
			team := event.AllianceTeams[ti]
			for pi := range team {
				if displayName, ok := displayByName[team[pi].Name]; ok {
					team[pi].Name = displayName
				}
			}
		}
	}

	return nil
}
