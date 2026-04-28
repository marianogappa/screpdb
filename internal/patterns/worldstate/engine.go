package worldstate

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/utils"
	"github.com/samber/lo"
)

const (
	rushWindowSec      = 300
	zerglingRushSec    = 140
	zergRushObserveSec = 120
	rushBuildWindowSec = 4 * 60
	// Max distance from a rush build command to an enemy base center when the point is outside the base polygon (map polygons often miss ramp cannons).
	rushBuildSnapToEnemyBaseCenterPx = 10 * 32
	proxyFactoryWindowSec            = 5 * 60
	// Build/train evidence for an attack at second S is collected from
	// the window centered at S - attackUnitsEpicenterOffsetSec, expanded
	// attackUnitsPastSec into the past and attackUnitsFutureSec into the
	// future. The epicenter ~45s before the attack matches the typical
	// "units are already on their way" delay; the past arm reaches deep
	// enough to cover an army that was massed and then sent out, while
	// the short future arm catches reinforcements.
	attackUnitsEpicenterOffsetSec = 45
	attackUnitsPastSec            = 120
	attackUnitsFutureSec          = 30
	// castSamplesRetentionSec bounds memory for the per-attacker cast
	// sample buffer. Casts older than this many seconds before the
	// current stream second are dropped — well outside any plausible
	// attack pressure window.
	castSamplesRetentionSec = 600
	eventDedupWindowSec     = 60
	neutralPID              = byte(255)
	commandRadiusMul        = 1.25
	radiusSafety            = 0.98
)

type NarrativeEntry struct {
	Type        string               `json:"type"`
	Second      int                  `json:"second"`
	Description string               `json:"description"`
	Actor       *NarrativePlayerRef  `json:"actor,omitempty"`
	Target      *NarrativePlayerRef  `json:"target,omitempty"`
	Base        *NarrativeBaseRef    `json:"base,omitempty"`
	ActorOrigin *NarrativePoint      `json:"actor_origin,omitempty"`
	Ownership   []NarrativeOwnership `json:"ownership,omitempty"`
}

type NarrativePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type NarrativePlayerRef struct {
	PlayerID int64  `json:"player_id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
}

type NarrativeBaseRef struct {
	Name        string           `json:"name"`
	Kind        string           `json:"kind,omitempty"`
	Clock       int              `json:"clock,omitempty"`
	MineralOnly *bool            `json:"mineral_only,omitempty"`
	Center      NarrativePoint   `json:"center"`
	Polygon     []NarrativePoint `json:"polygon,omitempty"`
}

type NarrativeOwnership struct {
	Base  NarrativeBaseRef    `json:"base"`
	Owner *NarrativePlayerRef `json:"owner,omitempty"`
}

type ReplayEvent struct {
	EventType              string
	Second                 int
	SourceReplayPlayerID   *byte
	TargetReplayPlayerID   *byte
	LocationBaseType       *string
	LocationBaseOclock     *int
	LocationNaturalOfClock *int
	LocationMineralOnly    *bool
	AttackUnitTypes        []string
	// AttackCastCounts is a per-cast-name tally of every aggressive cast
	// that landed inside the attack pressure window for "attack" events.
	// Keyed by the canonical cast subject (e.g. "PsionicStorm", "Plague",
	// "Recall"). Empty / nil for non-attack events.
	AttackCastCounts map[string]int
}

type attackUnitSample struct {
	Second   int
	UnitType string
}

// castSample records a single aggressive cast issued by a given attacker
// at a given stream second. Used to derive ground-truth caster-unit
// presence (and per-cast cardinalities) for attack events.
type castSample struct {
	Second    int
	OrderName string
}

type zergRushCandidate struct {
	DetectedSecond     int
	AttackCountsByBase map[int]int
}

type point struct {
	X float64
	Y float64
}

type base struct {
	CenterX       float64
	CenterY       float64
	NaturalRadius float64
	GeoRadius     float64
	StartCount    int
	IsStarting    bool
	MineralOnly   bool
	Name          string
	Kind          string
	Clock         int
	Polygon       []point
	DisplayName   string
}

type Engine struct {
	replay  *models.Replay
	players map[byte]*models.Player
	teams   map[byte]byte
	left    map[byte]bool

	bases       []base
	ownerByBase []byte

	startBaseByPID     map[byte]int
	naturalBaseByPID   map[byte]int
	naturalOwnerByBase map[int]byte
	playerBecameRace   map[byte]map[string]bool

	attackUnitsByPID      map[byte][]attackUnitSample
	castsByPID            map[byte][]castSample
	lastEventByKey        map[string]int
	zergRushCandidates    map[byte]*zergRushCandidate
	zergRushEmitted       map[byte]bool
	marineTrainCountByPID map[byte]int
	humanPlayerIDs        []byte

	entries      []NarrativeEntry
	replayEvents []ReplayEvent

	// Batch pipeline state. ProcessCommand appends to stream /
	// streamCommands; Finalize runs the ownership / attacks / rush passes
	// and populates entries / replayEvents.
	stream         []cmdenrich.EnrichedCommand
	streamCommands []*models.Command
	polygonGeoms   []PolygonGeom
	leaveSec       map[byte]int
	finalized      bool
}

func NewEngine(replay *models.Replay, players []*models.Player, mapCtx *models.ReplayMapContext) *Engine {
	e := &Engine{
		replay:                replay,
		players:               map[byte]*models.Player{},
		teams:                 map[byte]byte{},
		left:                  map[byte]bool{},
		startBaseByPID:        map[byte]int{},
		naturalBaseByPID:      map[byte]int{},
		naturalOwnerByBase:    map[int]byte{},
		playerBecameRace:      map[byte]map[string]bool{},
		attackUnitsByPID:      map[byte][]attackUnitSample{},
		castsByPID:            map[byte][]castSample{},
		lastEventByKey:        map[string]int{},
		zergRushCandidates:    map[byte]*zergRushCandidate{},
		zergRushEmitted:       map[byte]bool{},
		marineTrainCountByPID: map[byte]int{},
		humanPlayerIDs:        []byte{},
		entries:               make([]NarrativeEntry, 0, 256),
		replayEvents:          make([]ReplayEvent, 0, 256),
		leaveSec:              map[byte]int{},
	}
	for _, p := range players {
		e.players[p.PlayerID] = p
		e.teams[p.PlayerID] = p.Team
		if p.IsNonObserverHuman() {
			e.humanPlayerIDs = append(e.humanPlayerIDs, p.PlayerID)
		}
	}

	if mapCtx == nil {
		return e
	}

	e.bases = basesFromLayout(mapCtx)
	if len(e.bases) == 0 {
		points := make([]point, 0, len(mapCtx.MineralFields)+len(mapCtx.Geysers))
		for _, m := range mapCtx.MineralFields {
			points = append(points, point{X: float64(m.X), Y: float64(m.Y)})
		}
		for _, g := range mapCtx.Geysers {
			points = append(points, point{X: float64(g.X), Y: float64(g.Y)})
		}
		if len(points) == 0 {
			return e
		}

		_, _, _, _, labels := chooseMSTLabels(points)
		e.bases = makeBases(points, labels)
	}
	if len(e.bases) == 0 {
		return e
	}

	slotToPID := map[byte]byte{}
	for _, p := range players {
		slotToPID[byte(p.SlotID)] = p.PlayerID
	}
	for _, sl := range mapCtx.StartLocations {
		idx := pointToOwnershipBase(float64(sl.X), float64(sl.Y), e.bases)
		if idx < 0 {
			continue
		}
		e.bases[idx].StartCount++
		e.bases[idx].IsStarting = true
		if pid, ok := slotToPID[sl.SlotID]; ok {
			e.startBaseByPID[pid] = idx
		}
	}
	if mapCtx.Layout != nil {
		e.assignNaturalBasesFromLayoutByName(mapCtx.Layout)
	}

	assignPerBaseRadii(e.bases, radiusSafety)
	enlargeStartBaseRadii(e.bases, radiusSafety)

	e.ownerByBase = make([]byte, len(e.bases))
	for i := range e.bases {
		e.ownerByBase[i] = neutralPID
	}
	for pid, bi := range e.startBaseByPID {
		e.ownerByBase[bi] = pid
	}

	e.assignDisplayNames()
	return e
}

func (e *Engine) Entries() []NarrativeEntry {
	e.Finalize()
	out := make([]NarrativeEntry, len(e.entries))
	copy(out, e.entries)
	return out
}

func (e *Engine) ReplayEvents() []ReplayEvent {
	e.Finalize()
	out := make([]ReplayEvent, len(e.replayEvents))
	copy(out, e.replayEvents)
	return out
}

// Finalize runs the v2 batch pipeline: ownership pass, attacks pass,
// rush/proxy/race-change pass over the buffered stream, then composes
// final entries / replayEvents with the attack-importance filter.
//
// Idempotent — safe to call from multiple lazy-finalize entry points.
// No-op if invoked before any commands were buffered AND there are no
// bases (the legacy path returned empty in that case anyway).
func (e *Engine) Finalize() {
	if e.finalized {
		return
	}
	e.finalized = true

	var ownership []PolyOwnership
	var candidates []CandidateAttack

	if len(e.bases) > 0 {
		e.polygonGeoms = polygonGeomFromBases(e.bases)
		starts := e.buildPlayerStarts()

		durationSec := 0
		if e.replay != nil {
			durationSec = e.replay.DurationSeconds
		}

		ownership = BuildOwnership(e.stream, e.polygonGeoms, starts, durationSec)
		candidates = BuildAttacks(e.stream, e.polygonGeoms, ownership)

		e.emitPlayerStartEvents()
	}

	// runRushPass walks the buffered stream. Race-switch detection works
	// without bases; the rush/proxy emits inside guard on base lookups
	// returning -1, so the pass is safe to run even with empty bases.
	e.runRushPass(ownership)

	if len(e.bases) > 0 {
		e.emitOwnershipTransitions(ownership)
	}
	e.emitLeaveGameEvents()
	e.emitAttackCandidates(candidates)

	endSec := 0
	if e.replay != nil {
		endSec = e.replay.DurationSeconds
	}
	e.finalizeZergRushCandidates(endSec, true)

	sort.SliceStable(e.entries, func(i, j int) bool {
		return e.entries[i].Second < e.entries[j].Second
	})
	sort.SliceStable(e.replayEvents, func(i, j int) bool {
		return e.replayEvents[i].Second < e.replayEvents[j].Second
	})
}

func (e *Engine) buildPlayerStarts() []PlayerStart {
	out := make([]PlayerStart, 0, len(e.startBaseByPID))
	for pid, idx := range e.startBaseByPID {
		if idx < 0 || idx >= len(e.bases) {
			continue
		}
		b := e.bases[idx]
		out = append(out, PlayerStart{
			PlayerID: pid,
			X:        int(b.CenterX),
			Y:        int(b.CenterY),
		})
	}
	return out
}

// DebugBase mirrors the internal `base` struct for read-only external
// inspection (e.g. the /api/debug/map-layout endpoint). Kept separate so
// internal fields stay unexported.
type DebugBase struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Clock       int     `json:"clock"`
	CenterX     float64 `json:"center_x"`
	CenterY     float64 `json:"center_y"`
	IsStarting  bool    `json:"is_starting"`
	MineralOnly bool    `json:"mineral_only"`
	DisplayName string  `json:"display_name"`
}

// DebugSnapshot exposes the resolved world-state base layout for diagnostics.
// Used by the /api/debug/map-layout endpoint to compare engine-resolved
// structure (start/natural ownership, display names) against the raw
// scmapanalyzer data — critical for debugging location misclassifications.
func (e *Engine) DebugSnapshot() (bases []DebugBase, startBaseByPID map[byte]int, naturalBaseByPID map[byte]int, naturalOwnerByBase map[int]byte) {
	bases = make([]DebugBase, 0, len(e.bases))
	for i, b := range e.bases {
		bases = append(bases, DebugBase{
			Index:       i,
			Name:        b.Name,
			Kind:        b.Kind,
			Clock:       b.Clock,
			CenterX:     b.CenterX,
			CenterY:     b.CenterY,
			IsStarting:  b.IsStarting,
			MineralOnly: b.MineralOnly,
			DisplayName: b.DisplayName,
		})
	}
	startBaseByPID = make(map[byte]int, len(e.startBaseByPID))
	for k, v := range e.startBaseByPID {
		startBaseByPID[k] = v
	}
	naturalBaseByPID = make(map[byte]int, len(e.naturalBaseByPID))
	for k, v := range e.naturalBaseByPID {
		naturalBaseByPID[k] = v
	}
	naturalOwnerByBase = make(map[int]byte, len(e.naturalOwnerByBase))
	for k, v := range e.naturalOwnerByBase {
		naturalOwnerByBase[k] = v
	}
	return bases, startBaseByPID, naturalBaseByPID, naturalOwnerByBase
}

// NaturalExpansionForPlayer returns the player's natural expansion display name.
func (e *Engine) NaturalExpansionForPlayer(playerID byte) (string, bool) {
	baseIdx, ok := e.naturalBaseByPID[playerID]
	if !ok || baseIdx < 0 || baseIdx >= len(e.bases) {
		return "", false
	}
	return e.bases[baseIdx].DisplayName, true
}

// FirstEventSecondForPlayer returns the first second where the given event
// type appears for the provided player in the replay-events stream.
//
// Matches by (EventType, SourceReplayPlayerID) for the typed event_type
// values markers consume; the legacy description-prefix variants are
// folded through the same path. Calls Finalize lazily.
func (e *Engine) FirstEventSecondForPlayer(playerID byte, eventType string) *int {
	e.Finalize()
	switch eventType {
	case "drop", "recall", "nuke", "became_terran", "became_zerg",
		"reaver_drop", "dt_drop", "scout", "attack",
		"cannon_rush", "bunker_rush", "zergling_rush",
		"proxy_gate", "proxy_rax", "proxy_factory",
		"expansion", "takeover", "location_inactive",
		"player_start", "leave_game":
	default:
		return nil
	}

	for _, ev := range e.replayEvents {
		if ev.EventType != eventType {
			continue
		}
		if ev.SourceReplayPlayerID == nil || *ev.SourceReplayPlayerID != playerID {
			continue
		}
		sec := ev.Second
		return &sec
	}
	return nil
}

// FirstExpansionForPlayer returns the first expansion second and location text
// for a player based on existing narrative expansion entries.
func (e *Engine) FirstExpansionForPlayer(playerID byte) (*int, *string) {
	e.Finalize()
	name := e.playerName(playerID)
	prefixExpands := name + " expands "
	prefixExpandsTo := name + " expands to "

	for _, entry := range e.entries {
		if entry.Type != "expansion" {
			continue
		}
		desc := entry.Description
		if strings.HasPrefix(desc, prefixExpandsTo) {
			sec := entry.Second
			where := strings.TrimPrefix(desc, prefixExpandsTo)
			return &sec, &where
		}
		if strings.HasPrefix(desc, prefixExpands) {
			sec := entry.Second
			where := strings.TrimPrefix(desc, prefixExpands)
			return &sec, &where
		}
	}
	return nil, nil
}

// ProcessCommand buffers commands for the v2 batch pipeline. Real work
// happens in Finalize. Keeps lifecycle bookkeeping (left / leaveSec) here
// because rush detection later in the pass needs them for opponent-aware
// gating.
func (e *Engine) ProcessCommand(command *models.Command) {
	if command == nil {
		return
	}
	sec := command.SecondsFromGameStart
	if sec < 0 {
		sec = 0
	}
	if isLeaveAction(command.ActionType) {
		if pid, ok := e.playerIDFromCommand(command); ok {
			e.left[pid] = true
			if _, exists := e.leaveSec[pid]; !exists {
				e.leaveSec[pid] = sec
			}
		}
		return
	}
	ec, ok := cmdenrich.Classify(command)
	if !ok {
		return
	}
	e.stream = append(e.stream, ec)
	e.streamCommands = append(e.streamCommands, command)
}

func (e *Engine) emitEvent(eventType string, second int, description string, actor *NarrativePlayerRef, target *NarrativePlayerRef, baseIdx int, attackUnitTypes []string) {
	if description == "" || eventType == "" {
		return
	}
	if e.shouldSuppressEvent(eventType, second, actor, target, baseIdx, attackUnitTypes) {
		return
	}
	base := e.baseRef(baseIdx)
	if len(e.entries) > 0 {
		last := e.entries[len(e.entries)-1]
		if last.Second == second && last.Type == eventType && last.Description == description {
			return
		}
	}
	e.entries = append(e.entries, NarrativeEntry{
		Type:        eventType,
		Second:      second,
		Description: description,
		Actor:       actor,
		Target:      target,
		Base:        base,
		ActorOrigin: e.actorOrigin(actor, base),
		Ownership:   e.ownershipSnapshot(),
	})
	e.replayEvents = append(e.replayEvents, e.toReplayEvent(eventType, second, actor, target, baseIdx, attackUnitTypes))
}

func (e *Engine) emitPlayerStartEvents() {
	for pid, startIdx := range e.startBaseByPID {
		player := e.playerRef(pid)
		if player == nil {
			continue
		}
		e.emitEvent("player_start", 0, fmt.Sprintf("%s starts at %s", e.playerName(pid), e.bases[startIdx].DisplayName), player, nil, startIdx, nil)
	}
}

func (e *Engine) toReplayEvent(eventType string, second int, actor *NarrativePlayerRef, target *NarrativePlayerRef, baseIdx int, attackUnitTypes []string) ReplayEvent {
	baseType, baseOclock, naturalOfClock, mineralOnly := e.locationForBase(baseIdx)
	var sourceReplayPlayerID *byte
	if actor != nil {
		pid := byte(actor.PlayerID)
		sourceReplayPlayerID = &pid
	}
	var targetReplayPlayerID *byte
	if target != nil {
		pid := byte(target.PlayerID)
		targetReplayPlayerID = &pid
	}
	unitTypes := make([]string, 0, len(attackUnitTypes))
	for _, unitType := range attackUnitTypes {
		trimmed := strings.TrimSpace(unitType)
		if trimmed == "" {
			continue
		}
		unitTypes = append(unitTypes, trimmed)
	}
	if len(unitTypes) == 0 {
		unitTypes = nil
	}
	return ReplayEvent{
		EventType:              eventType,
		Second:                 second,
		SourceReplayPlayerID:   sourceReplayPlayerID,
		TargetReplayPlayerID:   targetReplayPlayerID,
		LocationBaseType:       baseType,
		LocationBaseOclock:     baseOclock,
		LocationNaturalOfClock: naturalOfClock,
		LocationMineralOnly:    mineralOnly,
		AttackUnitTypes:        unitTypes,
	}
}

func (e *Engine) shouldSuppressEvent(eventType string, second int, actor *NarrativePlayerRef, target *NarrativePlayerRef, baseIdx int, attackUnitTypes []string) bool {
	sourceID := int64(0)
	if actor != nil {
		sourceID = actor.PlayerID
	}
	targetID := int64(0)
	if target != nil {
		targetID = target.PlayerID
	}
	normalizedAttackUnits := normalizeUnitTypes(attackUnitTypes)
	key := fmt.Sprintf("%s|%d|%d|%d|%s", eventType, sourceID, targetID, baseIdx, strings.Join(normalizedAttackUnits, ","))
	lastSecond, exists := e.lastEventByKey[key]
	if exists && second-lastSecond < eventDedupWindowSec {
		return true
	}
	e.lastEventByKey[key] = second
	return false
}

func (e *Engine) recordRecentAttackUnit(pid byte, second int, command *models.Command) {
	if command == nil || !command.IsAttackingUnitBuild() {
		return
	}
	unitType := strings.TrimSpace(command.UnitBuildName())
	if unitType == "" {
		return
	}
	e.attackUnitsByPID[pid] = append(e.attackUnitsByPID[pid], attackUnitSample{Second: second, UnitType: unitType})
}

// recordRecentCast appends an aggressive cast to the per-attacker cast
// sample buffer. Non-aggressive utility casts (Restoration, Hallucination,
// ScannerSweep, DefensiveMatrix) are skipped — they don't represent
// combat presence.
func (e *Engine) recordRecentCast(pid byte, second int, ec cmdenrich.EnrichedCommand) {
	if ec.Kind != cmdenrich.KindCast {
		return
	}
	if !castIsAggressive(ec.OrderName) {
		return
	}
	samples := e.castsByPID[pid]
	cutoff := second - castSamplesRetentionSec
	// Drop samples too old to ever be useful for any future attack-window
	// query (the longest reach is the build/train epicenter past arm).
	for len(samples) > 0 && samples[0].Second < cutoff {
		samples = samples[1:]
	}
	samples = append(samples, castSample{Second: second, OrderName: ec.OrderName})
	e.castsByPID[pid] = samples
}

// buildUnitsInEpicenterWindow returns distinct attacking-unit-build names
// the given attacker issued inside the epicenter window for an attack at
// `attackSec`. Window: [attackSec - epicOffset - past, attackSec - epicOffset + future].
func (e *Engine) buildUnitsInEpicenterWindow(pid byte, attackSec int) []string {
	epicenter := attackSec - attackUnitsEpicenterOffsetSec
	lo := epicenter - attackUnitsPastSec
	hi := epicenter + attackUnitsFutureSec
	seen := map[string]struct{}{}
	out := []string{}
	for _, s := range e.attackUnitsByPID[pid] {
		if s.Second < lo || s.Second > hi {
			continue
		}
		if _, ok := seen[s.UnitType]; ok {
			continue
		}
		seen[s.UnitType] = struct{}{}
		out = append(out, s.UnitType)
	}
	return out
}

// attackUnitsCombined builds the attack event's unit_types list. Cast
// evidence comes first (ground-truth: a Storm cast proves a High Templar
// existed at that moment) — looked up over the full attack pressure
// window. Build/train history fills in the rest from the epicenter
// window. Result is sorted for stable downstream comparison.
func (e *Engine) attackUnitsCombined(c CandidateAttack) []string {
	seen := map[string]struct{}{}
	out := []string{}
	lo, hi := c.OpenSec, c.CloseSec
	if hi < lo {
		hi = lo
	}
	for _, cs := range e.castsByPID[c.Attacker] {
		if cs.Second < lo || cs.Second > hi {
			continue
		}
		unit, ok := casterUnitForCast(cs.OrderName)
		if !ok || unit == "" {
			continue
		}
		if _, exists := seen[unit]; exists {
			continue
		}
		seen[unit] = struct{}{}
		out = append(out, unit)
	}
	for _, u := range e.buildUnitsInEpicenterWindow(c.Attacker, c.Second) {
		if _, exists := seen[u]; exists {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// attackCastCounts tallies aggressive casts by the attacker inside the
// attack pressure window. Keys are the canonical cast subject (Cast
// prefix stripped, normalized name) — same shape used by the importance
// filter's spell-novelty check.
func (e *Engine) attackCastCounts(c CandidateAttack) map[string]int {
	lo, hi := c.OpenSec, c.CloseSec
	if hi < lo {
		hi = lo
	}
	counts := map[string]int{}
	for _, cs := range e.castsByPID[c.Attacker] {
		if cs.Second < lo || cs.Second > hi {
			continue
		}
		key := castSubjectFromOrderName(cs.OrderName)
		if key == "" {
			continue
		}
		counts[key]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

// castSubjectFromOrderName strips the "Cast" prefix from raw OrderName
// values so storage keys match the canonical Subject form used elsewhere
// (e.g. "CastPsionicStorm" → "PsionicStorm", "CastRecall" → "Recall",
// "NukeLaunch" → "NukeLaunch").
func castSubjectFromOrderName(orderName string) string {
	trimmed := strings.TrimSpace(orderName)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "Cast") && len(trimmed) > len("Cast") {
		return trimmed[len("Cast"):]
	}
	return trimmed
}

func normalizeUnitTypes(unitTypes []string) []string {
	if len(unitTypes) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(unitTypes))
	for _, unitType := range unitTypes {
		trimmed := strings.TrimSpace(unitType)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func (e *Engine) locationForBase(baseIdx int) (*string, *int, *int, *bool) {
	if baseIdx < 0 || baseIdx >= len(e.bases) {
		return nil, nil, nil, nil
	}
	base := e.bases[baseIdx]
	baseTypeValue := "expansion"
	if base.IsStarting {
		baseTypeValue = "starting"
	} else if _, ok := e.naturalOwnerByBase[baseIdx]; ok {
		baseTypeValue = "natural"
	}
	baseType := &baseTypeValue
	// Always emit the base's clock so the dashboard's overlay lookup can
	// match by (kind, clock) even for bases scmapanalyzer flagged with an
	// out-of-range / unknown clock (negative values or >12). The same clock
	// value lands in overlayBaseMetasFromLayout, so equal-on-both-sides is
	// enough — no need to semantically enforce the 0..12 dial range here.
	clock := base.Clock
	baseOclock := &clock
	var naturalOfClock *int
	if ownerPID, ok := e.naturalOwnerByBase[baseIdx]; ok {
		if ownerBaseIdx, hasStart := e.startBaseByPID[ownerPID]; hasStart && ownerBaseIdx >= 0 && ownerBaseIdx < len(e.bases) {
			ownerClock := e.bases[ownerBaseIdx].Clock
			if ownerClock >= 0 && ownerClock <= 12 {
				naturalOfClock = &ownerClock
			}
		}
	}
	var mineralOnly *bool
	if base.MineralOnly {
		value := true
		mineralOnly = &value
	}
	return baseType, baseOclock, naturalOfClock, mineralOnly
}

func unitTypesFromCommand(command *models.Command) []string {
	if command == nil {
		return nil
	}
	unitTypes := []string{}
	if command.UnitType != nil {
		trimmed := strings.TrimSpace(*command.UnitType)
		if trimmed != "" {
			unitTypes = append(unitTypes, trimmed)
		}
	}
	if command.UnitTypes != nil && strings.TrimSpace(*command.UnitTypes) != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(*command.UnitTypes), &parsed); err == nil {
			unitTypes = append(unitTypes, parsed...)
		}
	}
	return normalizeUnitTypes(unitTypes)
}

func hasUnitType(unitTypes []string, unitType string) bool {
	if len(unitTypes) == 0 {
		return false
	}
	unitNorm := normalize(unitType)
	for _, candidate := range unitTypes {
		if normalize(candidate) == unitNorm {
			return true
		}
	}
	return false
}

func (e *Engine) ownershipSnapshot() []NarrativeOwnership {
	if len(e.bases) == 0 {
		return nil
	}
	out := make([]NarrativeOwnership, 0, len(e.bases))
	for idx := range e.bases {
		baseRef := e.baseRef(idx)
		if baseRef == nil {
			continue
		}
		baseValue := *baseRef
		owner := e.ownerByBase[idx]
		var ownerRef *NarrativePlayerRef
		if owner != neutralPID {
			ownerRef = e.playerRef(owner)
		}
		out = append(out, NarrativeOwnership{
			Base:  baseValue,
			Owner: ownerRef,
		})
	}
	return out
}

func (e *Engine) actorOrigin(actor *NarrativePlayerRef, targetBase *NarrativeBaseRef) *NarrativePoint {
	if actor == nil {
		return nil
	}
	pid := byte(actor.PlayerID)
	if startIdx, ok := e.startBaseByPID[pid]; ok && startIdx >= 0 && startIdx < len(e.bases) {
		return &NarrativePoint{X: e.bases[startIdx].CenterX, Y: e.bases[startIdx].CenterY}
	}
	if targetBase != nil {
		targetCenter := targetBase.Center
		bestIdx := -1
		bestDist := math.MaxFloat64
		for idx, owner := range e.ownerByBase {
			if owner != pid {
				continue
			}
			d := dist(e.bases[idx].CenterX, e.bases[idx].CenterY, targetCenter.X, targetCenter.Y)
			if d < bestDist {
				bestDist = d
				bestIdx = idx
			}
		}
		if bestIdx >= 0 {
			return &NarrativePoint{X: e.bases[bestIdx].CenterX, Y: e.bases[bestIdx].CenterY}
		}
	}
	return nil
}

func (e *Engine) playerName(pid byte) string {
	if pid == neutralPID {
		return "neutral"
	}
	if p, ok := e.players[pid]; ok && p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("player-%d", pid)
}

func (e *Engine) workerUnitForPlayer(pid byte) string {
	player, ok := e.players[pid]
	if !ok || player == nil {
		return ""
	}
	switch normalize(player.Race) {
	case "terran":
		return models.GeneralUnitSCV
	case "protoss":
		return models.GeneralUnitProbe
	case "zerg":
		return models.GeneralUnitDrone
	default:
		return ""
	}
}

// scoutPayloadUnitsFromCommand picks replay-event unit payload for early "scout" classification.
// Zerg Overlords can satisfy the same early-pressure heuristic as workers.
func (e *Engine) scoutPayloadUnitsFromCommand(pid byte, commandUnitType string) []string {
	u := strings.TrimSpace(commandUnitType)
	if u != "" {
		n := normalize(u)
		if strings.Contains(n, "overlord") {
			return []string{models.GeneralUnitOverlord}
		}
	}
	if w := e.workerUnitForPlayer(pid); w != "" {
		return []string{w}
	}
	return nil
}

func (e *Engine) sameTeam(a byte, b byte) bool {
	ta, oka := e.teams[a]
	tb, okb := e.teams[b]
	return oka && okb && ta != 0 && ta == tb
}

func (e *Engine) playerIDFromCommand(command *models.Command) (byte, bool) {
	if command.Player != nil {
		return command.Player.PlayerID, true
	}
	if command.PlayerID < 0 || command.PlayerID > 255 {
		return 0, false
	}
	return byte(command.PlayerID), true
}

func isLeaveAction(actionType string) bool {
	return normalize(actionType) == "leavegame"
}

func isBuildLike(actionType string) bool {
	n := normalize(actionType)
	return n == "build" || n == "land"
}

func commandCoords(command *models.Command) (float64, float64, bool) {
	if command.X == nil || command.Y == nil {
		return 0, 0, false
	}
	return float64(*command.X), float64(*command.Y), true
}

func tileToPixel(v float64) float64 {
	return v*32 + 16
}

func isTownHallUnit(unitName string) bool {
	n := normalize(unitName)
	return strings.Contains(n, "commandcenter") || strings.Contains(n, "hatchery") || strings.Contains(n, "nexus")
}

func isRushBuilding(unitName string) bool {
	n := normalize(unitName)
	return strings.Contains(n, "photoncannon") || strings.Contains(n, "bunker")
}

func isDropOrder(orderName string) bool {
	return strings.Contains(normalize(orderName), "unload")
}

func isRecallOrder(orderName string) bool {
	n := normalize(orderName)
	return strings.Contains(n, "castrecall") || strings.Contains(n, "recall")
}

func isNukeOrder(orderName string) bool {
	n := normalize(orderName)
	return strings.Contains(n, "nukelaunch") || strings.Contains(n, "nuke")
}

func normalize(s string) string {
	x := strings.ToLower(s)
	x = strings.ReplaceAll(x, " ", "")
	x = strings.ReplaceAll(x, "_", "")
	return x
}

func (e *Engine) processRaceSwitchEvent(command *models.Command, pid byte, sec int) {
	if !isBuildLike(command.ActionType) || command.UnitType == nil {
		return
	}
	player, ok := e.players[pid]
	if !ok || !strings.EqualFold(player.Race, "Protoss") {
		return
	}
	race := nonProtossBuildingRace(*command.UnitType)
	if race == "" {
		return
	}
	raceKey := strings.ToLower(race)
	if e.playerBecameRace[pid] == nil {
		e.playerBecameRace[pid] = map[string]bool{}
	}
	if e.playerBecameRace[pid][raceKey] {
		return
	}
	e.playerBecameRace[pid][raceKey] = true
	e.emitEvent("became_"+raceKey, sec, fmt.Sprintf("%s became %s", e.playerName(pid), race), e.playerRef(pid), nil, -1, nil)
}

func (e *Engine) processZerglingRushEvent(command *models.Command, pid byte, sec int) {
	if e.zergRushEmitted[pid] || e.zergRushCandidates[pid] != nil || sec > zerglingRushSec {
		return
	}
	if !command.IsUnitBuild() || command.UnitType == nil || *command.UnitType != models.GeneralUnitZergling {
		return
	}
	e.zergRushCandidates[pid] = &zergRushCandidate{
		DetectedSecond:     sec,
		AttackCountsByBase: map[int]int{},
	}
}

func (e *Engine) recordMarineTraining(pid byte, sec int, command *models.Command) {
	if sec > rushBuildWindowSec || command == nil || !command.IsUnitBuild() || command.UnitType == nil {
		return
	}
	if normalize(*command.UnitType) == normalize(models.GeneralUnitMarine) {
		e.marineTrainCountByPID[pid]++
	}
}

func (e *Engine) recordZergRushAttack(pid byte, sec int, baseIdx int) {
	candidate := e.zergRushCandidates[pid]
	if candidate == nil || baseIdx < 0 || baseIdx >= len(e.ownerByBase) {
		return
	}
	if sec < candidate.DetectedSecond || sec > candidate.DetectedSecond+zergRushObserveSec {
		return
	}
	owner := e.ownerByBase[baseIdx]
	if owner == neutralPID || owner == pid || e.left[owner] || e.sameTeam(pid, owner) {
		return
	}
	candidate.AttackCountsByBase[baseIdx]++
}

func (e *Engine) finalizeZergRushCandidates(currentSec int, force bool) {
	for pid, candidate := range e.zergRushCandidates {
		if candidate == nil {
			delete(e.zergRushCandidates, pid)
			continue
		}
		if !force && currentSec < candidate.DetectedSecond+zergRushObserveSec {
			continue
		}
		targetBaseIdx := -1
		maxCount := 0
		for baseIdx, count := range candidate.AttackCountsByBase {
			if count > maxCount || (count == maxCount && (targetBaseIdx < 0 || baseIdx < targetBaseIdx)) {
				targetBaseIdx = baseIdx
				maxCount = count
			}
		}
		if targetBaseIdx >= 0 && maxCount > 0 {
			var target *NarrativePlayerRef
			if targetBaseIdx < len(e.ownerByBase) {
				owner := e.ownerByBase[targetBaseIdx]
				if owner != neutralPID && owner != pid && !e.sameTeam(pid, owner) {
					target = e.playerRef(owner)
				}
			}
			e.emitEvent(
				"zergling_rush",
				candidate.DetectedSecond,
				fmt.Sprintf("%s Zergling rushes", e.playerName(pid)),
				e.playerRef(pid),
				target,
				targetBaseIdx,
				[]string{models.GeneralUnitZergling},
			)
			e.zergRushEmitted[pid] = true
		}
		delete(e.zergRushCandidates, pid)
	}
}

func (e *Engine) tryEmitRushBuildEvents(command *models.Command, pid byte, sec int, x float64, y float64) {
	if command == nil || command.UnitType == nil || sec > rushBuildWindowSec {
		return
	}
	unitType := strings.TrimSpace(*command.UnitType)
	unitNorm := normalize(unitType)
	rushType := ""
	switch {
	case strings.Contains(unitNorm, "photoncannon"):
		rushType = "cannon_rush"
	case strings.Contains(unitNorm, "bunker"):
		if e.marineTrainCountByPID[pid] <= 0 {
			return
		}
		rushType = "bunker_rush"
	default:
		return
	}
	enemyBaseIdx := e.enemyBaseIdxAtPoint(pid, x, y)
	if enemyBaseIdx < 0 && strings.Contains(unitNorm, "photoncannon") {
		enemyBaseIdx = e.nearestEnemyBaseIdxForRush(pid, x, y, rushBuildSnapToEnemyBaseCenterPx)
	}
	if enemyBaseIdx < 0 {
		return
	}
	enemyPID := e.ownerByBase[enemyBaseIdx]
	if !e.isRushTargetEnemy(pid, enemyPID) {
		return
	}
	payload := []string{unitType}
	if rushType == "bunker_rush" {
		payload = append(payload, models.GeneralUnitMarine)
	}
	e.emitEvent(
		rushType,
		sec,
		fmt.Sprintf("%s %s rushes %s %s", e.playerName(pid), strings.ReplaceAll(rushType, "_", " "), e.playerName(enemyPID), e.bases[enemyBaseIdx].DisplayName),
		e.playerRef(pid),
		e.playerRef(enemyPID),
		enemyBaseIdx,
		payload,
	)
}

func (e *Engine) tryEmitProxyBuildEvents(command *models.Command, pid byte, sec int, x float64, y float64, baseIdx int) {
	if command == nil || command.UnitType == nil || !e.isTwoHumanGame() {
		return
	}
	unitType := strings.TrimSpace(*command.UnitType)
	unitNorm := normalize(unitType)
	proxyType := ""
	window := rushBuildWindowSec
	switch {
	case strings.Contains(unitNorm, "gateway"):
		proxyType = "proxy_gate"
	case strings.Contains(unitNorm, "barracks"):
		proxyType = "proxy_rax"
	case strings.Contains(unitNorm, "factory"):
		proxyType = "proxy_factory"
		window = proxyFactoryWindowSec
	default:
		return
	}
	if sec > window || !e.proxyPlacementAllowed(x, y) {
		return
	}
	targetBaseIdx := baseIdx
	if targetBaseIdx < 0 {
		targetBaseIdx = pointToEventBase(x, y, e.bases)
	}
	e.emitEvent(
		proxyType,
		sec,
		fmt.Sprintf("%s proxies %s near %s", e.playerName(pid), strings.ToLower(unitType), e.baseDisplayName(targetBaseIdx)),
		e.playerRef(pid),
		nil,
		targetBaseIdx,
		[]string{unitType},
	)
}

func (e *Engine) hasKnownEnemyTeam(a byte, b byte) bool {
	ta, oka := e.teams[a]
	tb, okb := e.teams[b]
	return oka && okb && ta != 0 && tb != 0 && ta != tb
}

// isRushTargetEnemy is like hasKnownEnemyTeam but treats two humans in a 1v1 as opponents when replay teams are missing or zero (common in some replays).
func (e *Engine) isRushTargetEnemy(pid, owner byte) bool {
	if owner == neutralPID || owner == pid {
		return false
	}
	if e.hasKnownEnemyTeam(pid, owner) {
		return true
	}
	return e.rushOpponentWhenTeamsAmbiguous(pid, owner)
}

func (e *Engine) rushOpponentWhenTeamsAmbiguous(pid, owner byte) bool {
	if len(e.humanPlayerIDs) != 2 {
		return false
	}
	a := e.humanPlayerIDs[0]
	b := e.humanPlayerIDs[1]
	if !((pid == a && owner == b) || (pid == b && owner == a)) {
		return false
	}
	if e.sameTeam(pid, owner) {
		return false
	}
	return true
}

func (e *Engine) enemyBaseIdxAtPoint(pid byte, x float64, y float64) int {
	bestIdx := -1
	bestDist := math.MaxFloat64
	for baseIdx, owner := range e.ownerByBase {
		if owner == neutralPID || owner == pid || !e.isRushTargetEnemy(pid, owner) {
			continue
		}
		if !pointInBasePolygon(x, y, e.bases[baseIdx]) {
			continue
		}
		d := dist(x, y, e.bases[baseIdx].CenterX, e.bases[baseIdx].CenterY)
		if d < bestDist {
			bestDist = d
			bestIdx = baseIdx
		}
	}
	return bestIdx
}

func (e *Engine) nearestEnemyBaseIdxForRush(pid byte, x, y, maxCenterDist float64) int {
	bestIdx := -1
	bestDist := math.MaxFloat64
	for baseIdx, owner := range e.ownerByBase {
		if owner == neutralPID || owner == pid || !e.isRushTargetEnemy(pid, owner) {
			continue
		}
		b := e.bases[baseIdx]
		d := dist(x, y, b.CenterX, b.CenterY)
		if d > maxCenterDist {
			continue
		}
		if d < bestDist {
			bestDist = d
			bestIdx = baseIdx
		}
	}
	return bestIdx
}

func (e *Engine) isTwoHumanGame() bool {
	return len(e.humanPlayerIDs) == 2
}

func (e *Engine) proxyPlacementAllowed(x float64, y float64) bool {
	if len(e.humanPlayerIDs) != 2 {
		return false
	}
	startA, okA := e.startBaseByPID[e.humanPlayerIDs[0]]
	startB, okB := e.startBaseByPID[e.humanPlayerIDs[1]]
	natA, hasNatA := e.naturalBaseByPID[e.humanPlayerIDs[0]]
	natB, hasNatB := e.naturalBaseByPID[e.humanPlayerIDs[1]]
	if !okA || !okB || !hasNatA || !hasNatB {
		return false
	}
	if pointInBasePolygon(x, y, e.bases[startA]) || pointInBasePolygon(x, y, e.bases[startB]) || pointInBasePolygon(x, y, e.bases[natA]) || pointInBasePolygon(x, y, e.bases[natB]) {
		return false
	}
	startDist := dist(e.bases[startA].CenterX, e.bases[startA].CenterY, e.bases[startB].CenterX, e.bases[startB].CenterY)
	if startDist <= 0 {
		return false
	}
	halfDist := startDist / 2
	minDist := halfDist * 0.7
	maxDist := halfDist * 1.3
	distA := dist(x, y, e.bases[startA].CenterX, e.bases[startA].CenterY)
	distB := dist(x, y, e.bases[startB].CenterX, e.bases[startB].CenterY)
	return distA >= minDist && distA <= maxDist && distB >= minDist && distB <= maxDist
}

func (e *Engine) baseDisplayName(baseIdx int) string {
	if baseIdx >= 0 && baseIdx < len(e.bases) {
		return e.bases[baseIdx].DisplayName
	}
	return "unknown location"
}

func (e *Engine) playerRef(pid byte) *NarrativePlayerRef {
	if pid == neutralPID {
		return nil
	}
	name := e.playerName(pid)
	color := ""
	if player, ok := e.players[pid]; ok {
		color = strings.TrimSpace(player.Color)
	}
	return &NarrativePlayerRef{
		PlayerID: int64(pid),
		Name:     name,
		Color:    color,
	}
}

func (e *Engine) baseRef(baseIdx int) *NarrativeBaseRef {
	if baseIdx < 0 || baseIdx >= len(e.bases) {
		return nil
	}
	base := e.bases[baseIdx]
	polygon := make([]NarrativePoint, 0, len(base.Polygon))
	for _, vertex := range base.Polygon {
		polygon = append(polygon, NarrativePoint{X: vertex.X, Y: vertex.Y})
	}
	return &NarrativeBaseRef{
		Name:        base.DisplayName,
		Kind:        base.Kind,
		Clock:       base.Clock,
		MineralOnly: lo.Ternary(base.MineralOnly, lo.ToPtr(true), nil),
		Center:      NarrativePoint{X: base.CenterX, Y: base.CenterY},
		Polygon:     polygon,
	}
}

func nonProtossBuildingRace(unitName string) string {
	switch normalize(unitName) {
	case
		// Terran buildings.
		"commandcenter", "supplydepot", "barracks", "engineeringbay", "academy",
		"bunker", "missileturret", "factory", "starport", "armory", "refinery",
		"sciencefacility", "covertops", "physicslab", "nuclearsilo",
		"machineshop", "comsat", "controltower":
		return "Terran"
	case
		// Zerg buildings.
		"hatchery", "lair", "hive", "nyduscanal", "hydraliskden", "defilermound",
		"greaterspire", "queensnest", "evolutionchamber", "ultraliskcavern",
		"spire", "spawningpool", "creepcolony", "sporecolony", "sunkencolony",
		"extractor":
		return "Zerg"
	default:
		return ""
	}
}

func basesFromLayout(mapCtx *models.ReplayMapContext) []base {
	if mapCtx == nil || mapCtx.Layout == nil || len(mapCtx.Layout.Bases) == 0 {
		return nil
	}
	out := make([]base, 0, len(mapCtx.Layout.Bases))
	for _, src := range mapCtx.Layout.Bases {
		if len(src.Polygon) < 3 {
			continue
		}
		polygon := make([]point, 0, len(src.Polygon))
		maxRadius := 0.0
		centerX := float64(src.Center.X)
		centerY := float64(src.Center.Y)
		for _, vertex := range src.Polygon {
			px := float64(vertex.X)
			py := float64(vertex.Y)
			polygon = append(polygon, point{X: px, Y: py})
			d := dist(centerX, centerY, px, py)
			if d > maxRadius {
				maxRadius = d
			}
		}
		if maxRadius <= 0 {
			maxRadius = 120
		}
		out = append(out, base{
			CenterX:       centerX,
			CenterY:       centerY,
			NaturalRadius: maxRadius,
			MineralOnly:   src.MineralOnly,
			Name:          src.Name,
			Kind:          src.Kind,
			Clock:         src.Clock,
			Polygon:       polygon,
			IsStarting:    strings.EqualFold(src.Kind, "start"),
		})
	}
	return out
}

func (e *Engine) assignNaturalBasesFromLayoutByName(layout *models.MapContextLayout) {
	if layout == nil || len(layout.Bases) == 0 {
		return
	}
	baseByName := map[string]int{}
	for i := range e.bases {
		name := strings.TrimSpace(e.bases[i].Name)
		if name == "" {
			continue
		}
		baseByName[name] = i
	}

	naturalByStartName := map[string]string{}
	for _, src := range layout.Bases {
		if !strings.EqualFold(src.Kind, "start") {
			continue
		}
		naturalName := strings.TrimSpace(src.NaturalExpansion)
		if naturalName == "" {
			continue
		}
		naturalByStartName[strings.TrimSpace(src.Name)] = naturalName
	}

	for pid, startIdx := range e.startBaseByPID {
		if startIdx < 0 || startIdx >= len(e.bases) {
			continue
		}
		startName := strings.TrimSpace(e.bases[startIdx].Name)
		if startName == "" {
			continue
		}
		naturalName, hasNaturalName := naturalByStartName[startName]
		if !hasNaturalName {
			continue
		}
		naturalIdx, hasNatural := baseByName[naturalName]
		if !hasNatural {
			continue
		}
		e.bases[naturalIdx].IsStarting = false
		e.naturalBaseByPID[pid] = naturalIdx
		e.naturalOwnerByBase[naturalIdx] = pid
	}
}

func (e *Engine) assignDisplayNames() {
	for i := range e.bases {
		oc := e.bases[i].Clock
		// Clock==0 is scmapanalyzer's "center base" marker. Label it
		// explicitly so sentences like "P1 expands to center base" read
		// correctly — the existing "at N" / "an expa near N" templates
		// don't accommodate the center concept.
		if oc == 0 {
			e.bases[i].DisplayName = "center base"
			continue
		}
		if oc < 0 {
			oc = utils.CalculateStartLocationOclock(
				int(e.replay.MapWidth),
				int(e.replay.MapHeight),
				int(math.Round(e.bases[i].CenterX)),
				int(math.Round(e.bases[i].CenterY)),
			)
			if oc == 0 {
				e.bases[i].DisplayName = "center base"
				continue
			}
		}
		if e.bases[i].IsStarting {
			e.bases[i].DisplayName = fmt.Sprintf("at %d", oc)
			continue
		}
		e.bases[i].DisplayName = fmt.Sprintf("an expa near %d", oc)
	}
}

func (e *Engine) decorateBaseDescriptionForPlayer(pid byte, baseIdx int, baseLabel string) string {
	if naturalIdx, ok := e.naturalBaseByPID[pid]; ok && naturalIdx == baseIdx {
		return baseLabel + " (their natural expansion)"
	}
	if naturalPID, ok := e.naturalOwnerByBase[baseIdx]; ok {
		ownerStartIdx, hasStart := e.startBaseByPID[naturalPID]
		if hasStart && ownerStartIdx >= 0 && ownerStartIdx < len(e.bases) {
			return fmt.Sprintf("%s (natural expansion of %s)", baseLabel, e.bases[ownerStartIdx].DisplayName)
		}
	}
	return baseLabel
}

func pointToOwnershipBase(x float64, y float64, bases []base) int {
	best := -1
	bestDist := math.MaxFloat64
	for i, b := range bases {
		if pointInBasePolygon(x, y, b) {
			d := dist(x, y, b.CenterX, b.CenterY)
			if d < bestDist {
				bestDist = d
				best = i
			}
		}
	}
	if best >= 0 {
		return best
	}
	return nearestBase(x, y, bases)
}

func pointInBasePolygon(x float64, y float64, b base) bool {
	if len(b.Polygon) < 3 {
		return false
	}
	inside := false
	j := len(b.Polygon) - 1
	for i := 0; i < len(b.Polygon); i++ {
		xi, yi := b.Polygon[i].X, b.Polygon[i].Y
		xj, yj := b.Polygon[j].X, b.Polygon[j].Y
		intersects := ((yi > y) != (yj > y)) &&
			(x < (xj-xi)*(y-yi)/(yj-yi+1e-9)+xi)
		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}

func pointToEventBase(x float64, y float64, bases []base) int {
	best := -1
	bestDist := math.MaxFloat64
	for i, b := range bases {
		if pointInBasePolygon(x, y, b) {
			d := dist(x, y, b.CenterX, b.CenterY)
			if d < bestDist {
				bestDist = d
				best = i
			}
		}
	}
	if best >= 0 {
		return best
	}

	best = -1
	bestDist = math.MaxFloat64
	for i, b := range bases {
		opRadius := b.NaturalRadius * commandRadiusMul
		if opRadius < 120 {
			opRadius = 120
		}
		d := dist(x, y, b.CenterX, b.CenterY)
		if d <= opRadius && d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

func nearestBase(x float64, y float64, bases []base) int {
	if len(bases) == 0 {
		return -1
	}
	best := 0
	bestD := math.MaxFloat64
	for i, b := range bases {
		d := dist(x, y, b.CenterX, b.CenterY)
		if d < bestD {
			bestD = d
			best = i
		}
	}
	return best
}

func dist(x1 float64, y1 float64, x2 float64, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return math.Sqrt(dx*dx + dy*dy)
}

// --- Geofence clustering internals ---

type mstEdge struct {
	A int
	B int
	W float64
}

func chooseMSTLabels(points []point) (float64, float64, int, float64, []int) {
	bestAlpha := 1.9
	bestBeta := 2.3
	bestK := 0
	bestLabels := []int{}
	bestSil := -1.0
	bestScore := -math.MaxFloat64

	alphas := []float64{1.5, 1.7, 1.9, 2.1, 2.3}
	betas := []float64{2.0, 2.3, 2.6, 2.9}
	for _, alpha := range alphas {
		for _, beta := range betas {
			labels, k := labelsFromMSTCuts(points, 3, alpha, beta)
			if k < 4 {
				continue
			}
			sil := silhouetteScore(points, labels, k)
			score := sil
			sizes := clusterSizes(labels, k)
			for _, size := range sizes {
				if size < 5 {
					score -= 0.04 * float64(5-size)
				}
				if size > 22 {
					score -= 0.03 * float64(size-22)
				}
			}
			if k < 8 {
				score -= 0.06 * float64(8-k)
			}
			if k > 24 {
				score -= 0.04 * float64(k-24)
			}
			if score > bestScore {
				bestScore = score
				bestSil = sil
				bestAlpha = alpha
				bestBeta = beta
				bestK = k
				bestLabels = labels
			}
		}
	}
	if bestK == 0 {
		labels, k := labelsFromMSTCuts(points, 3, 1.9, 2.3)
		return 1.9, 2.3, k, silhouetteScore(points, labels, maxInt(k, 1)), labels
	}
	return bestAlpha, bestBeta, bestK, bestSil, bestLabels
}

func labelsFromMSTCuts(points []point, kNN int, alpha float64, beta float64) ([]int, int) {
	n := len(points)
	if n == 0 {
		return []int{}, 0
	}
	if n == 1 {
		return []int{0}, 1
	}
	localScale := kthNeighborDistances(points, kNN)
	medianScale := percentile(localScale, 0.5)
	mst := primMST(points)

	uf := newUnionFind(n)
	for _, e := range mst {
		localThreshold := alpha * math.Max(localScale[e.A], localScale[e.B])
		globalThreshold := beta * medianScale
		if e.W <= localThreshold && e.W <= globalThreshold {
			uf.union(e.A, e.B)
		}
	}

	components := map[int][]int{}
	for i := 0; i < n; i++ {
		root := uf.find(i)
		components[root] = append(components[root], i)
	}

	minComponentSize := 4
	bigRoots := make([]int, 0, len(components))
	for root, members := range components {
		if len(members) >= minComponentSize {
			bigRoots = append(bigRoots, root)
		}
	}
	sort.Ints(bigRoots)
	if len(bigRoots) == 0 {
		for root := range components {
			bigRoots = append(bigRoots, root)
		}
		sort.Ints(bigRoots)
	}

	rootCenters := map[int][2]float64{}
	for _, root := range bigRoots {
		rootCenters[root] = centroid(points, components[root])
	}
	pointRoot := make([]int, n)
	for i := 0; i < n; i++ {
		pointRoot[i] = uf.find(i)
	}
	for _, members := range components {
		if len(members) >= minComponentSize || len(bigRoots) == 0 {
			continue
		}
		targetRoot := bigRoots[0]
		best := math.MaxFloat64
		for _, bRoot := range bigRoots {
			c := rootCenters[bRoot]
			d := averageDistanceToPoint(points, members, c[0], c[1])
			if d < best {
				best = d
				targetRoot = bRoot
			}
		}
		for _, idx := range members {
			pointRoot[idx] = targetRoot
		}
	}

	labelByRoot := map[int]int{}
	labels := make([]int, n)
	next := 0
	for i := 0; i < n; i++ {
		root := pointRoot[i]
		lbl, ok := labelByRoot[root]
		if !ok {
			lbl = next
			labelByRoot[root] = lbl
			next++
		}
		labels[i] = lbl
	}
	return labels, next
}

func makeBases(points []point, labels []int) []base {
	per := map[int][]int{}
	for i, l := range labels {
		if l < 0 {
			continue
		}
		per[l] = append(per[l], i)
	}
	ids := make([]int, 0, len(per))
	for id := range per {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]base, 0, len(ids))
	for _, id := range ids {
		members := per[id]
		if len(members) < 4 {
			continue
		}
		c := centroid(points, members)
		natural := 0.0
		for _, mi := range members {
			d := dist(c[0], c[1], points[mi].X, points[mi].Y)
			if d > natural {
				natural = d
			}
		}
		out = append(out, base{
			CenterX:       c[0],
			CenterY:       c[1],
			NaturalRadius: natural,
		})
	}
	return out
}

func assignPerBaseRadii(bases []base, safety float64) {
	for i := range bases {
		minHalfDist := math.MaxFloat64
		for j := range bases {
			if i == j {
				continue
			}
			d := dist(bases[i].CenterX, bases[i].CenterY, bases[j].CenterX, bases[j].CenterY)
			if d/2 < minHalfDist {
				minHalfDist = d / 2
			}
		}
		if len(bases) == 1 {
			minHalfDist = bases[i].NaturalRadius
		}
		capR := minHalfDist * safety
		if bases[i].NaturalRadius < capR {
			bases[i].GeoRadius = bases[i].NaturalRadius
		} else {
			bases[i].GeoRadius = capR
		}
	}
}

func enlargeStartBaseRadii(bases []base, safety float64) {
	startIdx := make([]int, 0, len(bases))
	for i, b := range bases {
		if b.StartCount > 0 {
			startIdx = append(startIdx, i)
		}
	}
	if len(startIdx) == 0 {
		return
	}
	sort.Ints(startIdx)
	steps := []float64{64, 16, 4, 1, 0.25}
	for _, step := range steps {
		for turns := 0; turns < 20000; turns++ {
			progress := false
			for _, i := range startIdx {
				if canGrowBaseRadius(bases, i, step, safety) {
					bases[i].GeoRadius += step
					progress = true
				}
			}
			if !progress {
				break
			}
		}
	}
}

func canGrowBaseRadius(bases []base, idx int, step float64, safety float64) bool {
	newR := bases[idx].GeoRadius + step
	for j := range bases {
		if j == idx {
			continue
		}
		d := dist(bases[idx].CenterX, bases[idx].CenterY, bases[j].CenterX, bases[j].CenterY)
		if newR+bases[j].GeoRadius > d*safety {
			return false
		}
	}
	return true
}

func kthNeighborDistances(points []point, k int) []float64 {
	n := len(points)
	if n == 0 {
		return []float64{}
	}
	if k < 1 {
		k = 1
	}
	if k >= n {
		k = n - 1
	}
	res := make([]float64, n)
	for i := 0; i < n; i++ {
		ds := make([]float64, 0, n-1)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			ds = append(ds, dist(points[i].X, points[i].Y, points[j].X, points[j].Y))
		}
		sort.Float64s(ds)
		res[i] = ds[k-1]
	}
	return res
}

func primMST(points []point) []mstEdge {
	n := len(points)
	if n <= 1 {
		return []mstEdge{}
	}
	inTree := make([]bool, n)
	minDist := make([]float64, n)
	parent := make([]int, n)
	for i := 0; i < n; i++ {
		minDist[i] = math.MaxFloat64
		parent[i] = -1
	}
	minDist[0] = 0
	edges := make([]mstEdge, 0, n-1)
	for step := 0; step < n; step++ {
		u := -1
		best := math.MaxFloat64
		for i := 0; i < n; i++ {
			if !inTree[i] && minDist[i] < best {
				best = minDist[i]
				u = i
			}
		}
		if u == -1 {
			break
		}
		inTree[u] = true
		if parent[u] >= 0 {
			edges = append(edges, mstEdge{A: parent[u], B: u, W: best})
		}
		for v := 0; v < n; v++ {
			if inTree[v] || u == v {
				continue
			}
			w := dist(points[u].X, points[u].Y, points[v].X, points[v].Y)
			if w < minDist[v] {
				minDist[v] = w
				parent[v] = u
			}
		}
	}
	return edges
}

func silhouetteScore(points []point, labels []int, k int) float64 {
	clusters := make([][]int, k)
	for i, l := range labels {
		clusters[l] = append(clusters[l], i)
	}
	if len(points) == 0 {
		return 0
	}
	total := 0.0
	for i := range points {
		my := labels[i]
		a := 0.0
		if len(clusters[my]) > 1 {
			for _, j := range clusters[my] {
				if i == j {
					continue
				}
				a += dist(points[i].X, points[i].Y, points[j].X, points[j].Y)
			}
			a /= float64(len(clusters[my]) - 1)
		}
		b := math.MaxFloat64
		for c := 0; c < k; c++ {
			if c == my || len(clusters[c]) == 0 {
				continue
			}
			avg := 0.0
			for _, j := range clusters[c] {
				avg += dist(points[i].X, points[i].Y, points[j].X, points[j].Y)
			}
			avg /= float64(len(clusters[c]))
			if avg < b {
				b = avg
			}
		}
		if b == math.MaxFloat64 {
			continue
		}
		den := math.Max(a, b)
		if den == 0 {
			continue
		}
		total += (b - a) / den
	}
	return total / float64(len(points))
}

func clusterSizes(labels []int, k int) []int {
	sizes := make([]int, k)
	for _, l := range labels {
		sizes[l]++
	}
	return sizes
}

func centroid(points []point, members []int) [2]float64 {
	sx, sy := 0.0, 0.0
	for _, mi := range members {
		sx += points[mi].X
		sy += points[mi].Y
	}
	return [2]float64{sx / float64(len(members)), sy / float64(len(members))}
}

func averageDistanceToPoint(points []point, members []int, x float64, y float64) float64 {
	if len(members) == 0 {
		return math.MaxFloat64
	}
	total := 0.0
	for _, mi := range members {
		total += dist(points[mi].X, points[mi].Y, x, y)
	}
	return total / float64(len(members))
}

func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	x := make([]float64, len(vals))
	copy(x, vals)
	sort.Float64s(x)
	if p <= 0 {
		return x[0]
	}
	if p >= 1 {
		return x[len(x)-1]
	}
	pos := p * float64(len(x)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return x[lo]
	}
	frac := pos - float64(lo)
	return x[lo]*(1-frac) + x[hi]*frac
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	r := make([]int, n)
	for i := 0; i < n; i++ {
		p[i] = i
	}
	return &unionFind{parent: p, rank: r}
}

func (u *unionFind) find(x int) int {
	if u.parent[x] != x {
		u.parent[x] = u.find(u.parent[x])
	}
	return u.parent[x]
}

func (u *unionFind) union(a int, b int) {
	ra := u.find(a)
	rb := u.find(b)
	if ra == rb {
		return
	}
	if u.rank[ra] < u.rank[rb] {
		u.parent[ra] = rb
		return
	}
	if u.rank[ra] > u.rank[rb] {
		u.parent[rb] = ra
		return
	}
	u.parent[rb] = ra
	u.rank[ra]++
}
