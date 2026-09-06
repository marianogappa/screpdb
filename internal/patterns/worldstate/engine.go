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
	// Bunker rushes on standard (non-BGH) maps need a longer window than cannon
	// rushes: the SCV walks cross-map, so the bunker lands later than a proxy
	// cannon. The marine-trained + enemy-start gates keep it from over-firing.
	bunkerRushWindowSec = 5 * 60
	// Snap radius for a rush build outside any base polygon (map polygons often
	// miss ramp cannons).
	rushBuildSnapToEnemyBaseCenterPx = 10 * 32
	proxyFactoryWindowSec            = 5 * 60
	// A proxy 2-port Starport lands later than a proxy Gateway/Barracks (1 Rax /
	// 1 Fac precede it), so it gets the same 5:00 window as a proxy Factory.
	proxyStarportWindowSec = 5 * 60
	// Build/train evidence for an attack at second S is collected from a window
	// centered at S - offset. ~45s before matches the "units are already on their
	// way" delay; the past arm covers an army massed then sent out, the short
	// future arm catches reinforcements.
	attackUnitsEpicenterOffsetSec = 45
	attackUnitsPastSec            = 120
	attackUnitsFutureSec          = 30
	// Bounds the per-attacker cast sample buffer: older casts are well outside any
	// plausible attack pressure window.
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
	// AttackCastCounts tallies aggressive casts landing inside the attack pressure
	// window, keyed by canonical cast subject ("PsionicStorm"). Nil for non-attacks.
	AttackCastCounts map[string]int
	// Payload ships event-specific detail the structured columns don't cover (the
	// alliance events' team-sizes summary and ally-name lists). Nil when unused.
	Payload *string
}

type attackUnitSample struct {
	Second   int
	UnitType string
}

// castSample records one aggressive cast, used to derive ground-truth
// caster-unit presence and per-cast cardinalities for attack events.
type castSample struct {
	Second    int
	OrderName string
}

type zergRushCandidate struct {
	DetectedSecond     int
	ZerglingCount      int
	AttackCountsByBase map[int]int
}

// Below three Zerglings the early pressure isn't a rush — a scouting pair
// shouldn't trip the chip.
const minZerglingsForRush = 3

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

	// Batch pipeline state: ProcessCommand appends here, Finalize runs the passes.
	stream         []cmdenrich.EnrichedCommand
	streamCommands []*models.Command
	polygonGeoms   []PolygonGeom
	leaveSec       map[byte]int
	leaveReason    map[byte]string
	lastCmdSec     map[byte]int
	finalized      bool

	// use1v1Attacks routes "attack" emission through the bilateral-fight model for
	// 1v1 games, and emitAttackCandidates then skips its pressure-tracker path.
	use1v1Attacks bool

	// productionSignals feeds BuildOwnership. Nil when the caller never set them
	// (e.g. the debug map-layout endpoint), and ownership then behaves as before.
	productionSignals []ProductionSignal

	mutaHarass []MutaHarassCandidate

	// townHallBuilds holds per-player Build seconds of REAL expansion town halls
	// (halls whose tag morphed larvae), phantom/cancelled placements excluded, so
	// the "N Hatch <tech>" base count can't be inflated. Source is internal/unittags.
	townHallBuilds map[byte][]int

	// massDisconnect marks a replay ending in a saver disconnect (issue #358): the
	// leave cluster is an artifact of the saver's connection dying, not real
	// departures.
	massDisconnect *massDisconnectEnd
}

type massDisconnectEnd struct {
	saverPID   byte
	clusterSec int
}

// Must be called before Finalize. Source is internal/unittags.
func (e *Engine) SetProductionSignals(signals []ProductionSignal) {
	e.productionSignals = signals
}

// Must be called before Finalize.
func (e *Engine) SetMassDisconnectEnd(saverPID byte, clusterSecond int) {
	e.massDisconnect = &massDisconnectEnd{saverPID: saverPID, clusterSec: clusterSecond}
}

// Must be called before results are read.
func (e *Engine) SetTownHallBuilds(builds map[byte][]int) {
	e.townHallBuilds = builds
}

// The starting hall is excluded — it has no Build command. Nil when the caller
// never supplied the data.
func (e *Engine) TownHallBuildSeconds(pid byte) []int {
	return e.townHallBuilds[pid]
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
		lastCmdSec:            map[byte]int{},
		leaveReason:           map[byte]string{},
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

// LastCommandSecond distinguishes "player had no time" from "player had time
// but didn't act" for per-player marker gates. leaveSec is unreliable here:
// some replays record an early spurious leave_game for players who keep
// playing. Returns ok=false for players who issued zero commands.
func (e *Engine) LastCommandSecond(pid byte) (int, bool) {
	sec, ok := e.lastCmdSec[pid]
	return sec, ok
}

// EnrichedStream is for markers needing cross-player visibility at Finalize:
// the per-(player × marker) framework only feeds Observe the current player's
// commands, so gating on opponent activity means walking this stream.
func (e *Engine) EnrichedStream() []cmdenrich.EnrichedCommand {
	out := make([]cmdenrich.EnrichedCommand, len(e.stream))
	copy(out, e.stream)
	return out
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

// AppendReplayEvents lets the parser push alliance-derived events into the same
// channel the storage layer drains. Finalize sorts the combined list by Second.
func (e *Engine) AppendReplayEvents(events []ReplayEvent) {
	e.replayEvents = append(e.replayEvents, events...)
}

// Finalize runs the batch pipeline: ownership, attacks, then rush/proxy/
// race-change over the buffered stream. Idempotent, so the lazy-finalize entry
// points can all call it; a no-op before any commands with no bases.

// Finalized lets callers that must not trigger a premature Finalize (e.g. a
// marker finishing mid-stream) check first.
func (e *Engine) Finalized() bool { return e.finalized }

func (e *Engine) Finalize() {
	if e.finalized {
		return
	}
	e.finalized = true

	// Re-snapshot teams from the player pointers: NewEngine captured p.Team at
	// Initialize, but the parser's alliance-fallback pass rewrites it afterwards for
	// FFA / BGH replays where screp left everyone on team 0. Without this,
	// BuildAttacks sees the stale all-zero snapshot — read as "unknown" — and
	// reports allied players attacking each other (issue #146).
	for pid, p := range e.players {
		e.teams[pid] = p.Team
	}

	var ownership []PolyOwnership
	var candidates []CandidateAttack

	var dropClusters []DropCluster
	var nydusClusters []NydusCluster

	if len(e.bases) > 0 {
		e.polygonGeoms = polygonGeomFromBases(e.bases)
		starts := e.buildPlayerStarts()

		durationSec := 0
		if e.replay != nil {
			durationSec = e.replay.DurationSeconds
		}

		ownership = BuildOwnership(e.stream, e.polygonGeoms, starts, e.productionSignals, durationSec)
		candidates = BuildAttacks(e.stream, e.polygonGeoms, ownership, e.teams)

		// Feed drop candidates back into the shared list so cross-event inference
		// (Recall's attack-coincidence pass) keeps seeing drops as attack-class.
		dropClusters = BuildDrops(e.stream, e.polygonGeoms, e.bases, ownership, e.teams)
		candidates = append(candidates, dropClustersToCandidateAttacks(dropClusters)...)

		nydusClusters = e.buildNydusClusters(ownership, candidates)

		e.emitPlayerStartEvents()
	}

	// Safe with empty bases: race-switch detection needs none, and the rush/proxy
	// emits guard on base lookups returning -1.
	e.runRushPass(ownership)

	e.emitFirstTechTimingEvents()

	if len(e.bases) > 0 {
		e.emitOwnershipTransitions(ownership)
		e.emitRecallEvents(ownership, candidates)
		e.emitDropEvents(ownership, dropClusters, candidates)
		e.emitNydusEvents(nydusClusters)
	}
	e.emitLeaveGameEvents()
	if aPID, bPID, ok := e.singleOpponents(); ok && len(e.bases) > 0 {
		e.use1v1Attacks = true
		e.emitAttackCandidates(candidates)
		e.emit1v1Attacks(ownership, aPID, bPID)
	} else {
		e.emitAttackCandidates(candidates)
	}

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

// DebugBase mirrors the internal base struct for read-only external inspection,
// keeping the internal fields unexported.
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

// DebugSnapshot compares engine-resolved structure against raw scmapanalyzer
// data, which is how location misclassifications get debugged.
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

func (e *Engine) NaturalExpansionForPlayer(playerID byte) (string, bool) {
	baseIdx, ok := e.naturalBaseByPID[playerID]
	if !ok || baseIdx < 0 || baseIdx >= len(e.bases) {
		return "", false
	}
	return e.bases[baseIdx].DisplayName, true
}

// FirstEventSecondForPlayer matches by (EventType, SourceReplayPlayerID); the
// legacy description-prefix variants fold through the same path. Finalizes lazily.
func (e *Engine) FirstEventSecondForPlayer(playerID byte, eventType string) *int {
	e.Finalize()
	switch eventType {
	case "drop", "recall", "nuke", "became_terran", "became_zerg",
		"cliff_drop", "scout", "attack", "nydus_attack",
		"cannon_rush", "bunker_rush", "zergling_rush",
		"proxy_gate", "proxy_rax", "proxy_factory", "proxy_starport", "manner_pylon",
		"expansion", "takeover", "location_inactive",
		"player_start", "leave_game", "player_dropped", "mass_disconnect":
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

// ProcessCommand only buffers; real work happens in Finalize. Lifecycle
// bookkeeping (left / leaveSec) stays here because rush detection later in the
// pass needs it for opponent-aware gating.
func (e *Engine) ProcessCommand(command *models.Command) {
	if command == nil {
		return
	}
	sec := command.SecondsFromGameStart
	if sec < 0 {
		sec = 0
	}
	if pid, ok := e.playerIDFromCommand(command); ok {
		if sec > e.lastCmdSec[pid] {
			e.lastCmdSec[pid] = sec
		}
	}
	if isLeaveAction(command.ActionType) {
		if pid, ok := e.playerIDFromCommand(command); ok {
			e.left[pid] = true
			if _, exists := e.leaveSec[pid]; !exists {
				e.leaveSec[pid] = sec
				if command.LeaveReason != nil {
					e.leaveReason[pid] = *command.LeaveReason
				}
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
	// Recalls are explicit per-cast: a recall combo onto an enemy main is the whole
	// point of surfacing them, so the 60s dedup must not collapse them.
	if eventType == "recall" {
		return false
	}
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

// Non-aggressive utility casts (Restoration, Hallucination, ScannerSweep,
// DefensiveMatrix) are skipped — they don't represent combat presence.
func (e *Engine) recordRecentCast(pid byte, second int, ec cmdenrich.EnrichedCommand) {
	if ec.Kind != cmdenrich.KindCast {
		return
	}
	if !castIsAggressive(ec.OrderName) {
		return
	}
	samples := e.castsByPID[pid]
	cutoff := second - castSamplesRetentionSec
	// Too old to be useful to any future window (the longest reach is the
	// build/train epicenter past arm).
	for len(samples) > 0 && samples[0].Second < cutoff {
		samples = samples[1:]
	}
	samples = append(samples, castSample{Second: second, OrderName: ec.OrderName})
	e.castsByPID[pid] = samples
}

// Window: [attackSec - epicOffset - past, attackSec - epicOffset + future].
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

// Cast evidence comes first because it is ground truth — a Storm cast proves a
// High Templar existed then — and is looked up over the full pressure window;
// build/train history fills the rest from the epicenter window. Sorted for
// stable downstream comparison.
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

// Keys are the canonical cast subject (Cast prefix stripped, normalized), the
// same shape the importance filter's spell-novelty check uses.
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

// castSubjectFromOrderName strips the "Cast" prefix so storage keys match the
// canonical Subject form used elsewhere ("CastPsionicStorm" → "PsionicStorm").
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

// A "starting"-kind polygon nobody started at (extra start locations on an
// N-player map played 1v1) is an expansion, not a main, so the location label
// must not call it "starting".
func (e *Engine) isActualPlayerStart(baseIdx int) bool {
	for _, idx := range e.startBaseByPID {
		if idx == baseIdx {
			return true
		}
	}
	return false
}

func (e *Engine) locationForBase(baseIdx int) (*string, *int, *int, *bool) {
	if baseIdx < 0 || baseIdx >= len(e.bases) {
		return nil, nil, nil, nil
	}
	base := e.bases[baseIdx]
	baseTypeValue := "expansion"
	if base.IsStarting && e.isActualPlayerStart(baseIdx) {
		baseTypeValue = "starting"
	} else if _, ok := e.naturalOwnerByBase[baseIdx]; ok {
		baseTypeValue = "natural"
	}
	baseType := &baseTypeValue
	// Always emit the base's clock so the dashboard overlay can match by (kind,
	// clock) even for bases scmapanalyzer flagged with an out-of-range clock. The
	// same value lands in overlayBaseMetasFromLayout, so equal-on-both-sides is
	// enough — no need to enforce the 0..12 dial range here.
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
	return sameTeamByMap(e.teams, a, b)
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
	if e.zergRushEmitted[pid] {
		return
	}
	if !command.IsUnitBuild() || command.UnitType == nil || *command.UnitType != models.GeneralUnitZergling {
		return
	}
	candidate := e.zergRushCandidates[pid]
	if candidate == nil {
		// Only the FIRST morph is timing-gated: once a candidate is open, later morphs
		// keep counting inside the observation window, so a real 9-pool rush whose
		// 2nd/3rd morphs land just past the cutoff isn't lost.
		if sec > zerglingRushSec {
			return
		}
		candidate = &zergRushCandidate{
			DetectedSecond:     sec,
			AttackCountsByBase: map[int]int{},
		}
		e.zergRushCandidates[pid] = candidate
	} else if sec > candidate.DetectedSecond+zergRushObserveSec {
		return
	}
	// One Zergling-morph command spawns two (Zerg morphs pair-wise). The exact
	// count doesn't matter — only a monotone tally crossing the threshold.
	candidate.ZerglingCount += 2
}

func (e *Engine) recordMarineTraining(pid byte, sec int, command *models.Command) {
	if sec > bunkerRushWindowSec || command == nil || !command.IsUnitBuild() || command.UnitType == nil {
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
	owner, ok := e.rushTargetEnemyForBase(pid, baseIdx)
	if !ok {
		return
	}
	// Attacks before the target leaves still count: a successful rush often forces
	// the target out, and rejecting that would lose the strongest rush signal.
	if leaveSec, left := e.leaveSec[owner]; left && sec > leaveSec {
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
		// Triple gate: ≥3 Zerglings, attack on a confirmed enemy base, and the early
		// window already enforced by processZerglingRushEvent.
		if targetBaseIdx >= 0 && maxCount > 0 && candidate.ZerglingCount >= minZerglingsForRush {
			var target *NarrativePlayerRef
			if owner, ok := e.rushTargetEnemyForBase(pid, targetBaseIdx); ok {
				target = e.playerRef(owner)
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
	if command == nil || command.UnitType == nil {
		return
	}
	unitType := strings.TrimSpace(*command.UnitType)
	unitNorm := normalize(unitType)
	rushType := ""
	window := rushBuildWindowSec
	switch {
	case strings.Contains(unitNorm, "photoncannon"):
		rushType = "cannon_rush"
	case strings.Contains(unitNorm, "bunker"):
		if e.marineTrainCountByPID[pid] <= 0 {
			return
		}
		rushType = "bunker_rush"
		window = bunkerRushWindowSec
	default:
		return
	}
	if sec > window {
		return
	}
	enemyBaseIdx := e.enemyBaseIdxAtPoint(pid, x, y)
	if enemyBaseIdx < 0 && rushType == "bunker_rush" {
		// On standard (non-BGH) maps the bunker lands on the opponent's not-yet-taken
		// natural, which reads as neutral early and is skipped by the enemy-owned
		// lookup above, so fall back to the polygon's static owner (#195, cf. #196).
		// Bounded, because on money maps NaturalRadius is large and an unbounded snap
		// pulls an open-ground proxy bunker onto a base it never threatened.
		enemyBaseIdx = pointToEventBaseBounded(x, y, e.bases, rushBuildSnapToEnemyBaseCenterPx)
	}
	if enemyBaseIdx < 0 && strings.Contains(unitNorm, "photoncannon") {
		enemyBaseIdx = e.nearestEnemyBaseIdxForRush(pid, x, y, rushBuildSnapToEnemyBaseCenterPx)
	}
	if enemyBaseIdx < 0 {
		return
	}
	enemyPID, ok := e.rushTargetEnemyForBase(pid, enemyBaseIdx)
	if !ok {
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
	case strings.Contains(unitNorm, "starport"):
		proxyType = "proxy_starport"
		window = proxyStarportWindowSec
	default:
		return
	}
	if sec > window || !e.proxyPlacementAllowed(pid, x, y) {
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

// A manner pylon is an early worker-harass placement; 8:00 catches delayed ones.
const mannerPylonWindowSec = 8 * 60

// The opposite of a proxy: a proxy sits between the bases, a manner pylon sits
// inside the opponent's main. Two-human games only.
func (e *Engine) tryEmitMannerPylonEvent(command *models.Command, pid byte, sec int, xPx, yPx float64) {
	if command == nil || command.UnitType == nil || !e.isTwoHumanGame() {
		return
	}
	if !strings.Contains(normalize(strings.TrimSpace(*command.UnitType)), "pylon") {
		return
	}
	if sec > mannerPylonWindowSec {
		return
	}
	a, b := e.humanPlayerIDs[0], e.humanPlayerIDs[1]
	enemyPID := a
	switch pid {
	case a:
		enemyPID = b
	case b:
		enemyPID = a
	default:
		return
	}
	// A manner pylon is impossible against Zerg — creep spread prevents building
	// inside their base — so firing there is always a false positive.
	if p, ok := e.players[enemyPID]; ok && p != nil && p.Race == "Zerg" {
		return
	}
	enemyStart, ok := e.startBaseByPID[enemyPID]
	if !ok || enemyStart < 0 || enemyStart >= len(e.bases) {
		return
	}
	// Require polygon containment rather than pointToEventBase's nearest-base
	// fallback, which would accept a pylon at the player's OWN natural that merely
	// lands near the enemy's base index (e.g. a proxy Gateway at one's own expa).
	if !pointInBasePolygon(xPx, yPx, e.bases[enemyStart]) {
		return
	}
	e.emitEvent(
		"manner_pylon",
		sec,
		fmt.Sprintf("%s manner pylons at %s's main", e.playerName(pid), e.playerName(enemyPID)),
		e.playerRef(pid),
		e.playerRef(enemyPID),
		enemyStart,
		[]string{*command.UnitType},
	)
}

// Only a first Reaver / Corsair / Zealot-speed before 10:00 is a meaningful
// opener signal worth a timeline item.
const firstTechTimingWindowSec = 10 * 60

func (e *Engine) opponentRace(pid byte) string {
	if len(e.humanPlayerIDs) != 2 {
		return ""
	}
	other := e.humanPlayerIDs[0]
	if other == pid {
		other = e.humanPlayerIDs[1]
	}
	if p, ok := e.players[other]; ok && p != nil {
		return p.Race
	}
	return ""
}

// The timeline counterpart of the First Reaver / First Corsair / Speedlot pills.
func (e *Engine) emitFirstTechTimingEvents() {
	if !e.isTwoHumanGame() {
		return
	}
	type firstKey struct {
		pid       byte
		eventType string
	}
	firstSec := map[firstKey]int{}
	order := []firstKey{}
	note := func(pid byte, eventType string, sec int) {
		k := firstKey{pid, eventType}
		if _, seen := firstSec[k]; seen {
			return
		}
		firstSec[k] = sec
		order = append(order, k)
	}
	for i, ec := range e.stream {
		cmd := e.streamCommands[i]
		pid, ok := e.playerIDFromCommand(cmd)
		if !ok {
			continue
		}
		switch {
		case ec.Kind == cmdenrich.KindMakeUnit && ec.Subject == models.GeneralUnitReaver:
			note(pid, "first_reaver", ec.Second)
		case ec.Kind == cmdenrich.KindMakeUnit && ec.Subject == models.GeneralUnitCorsair:
			note(pid, "first_corsair", ec.Second)
		case ec.Kind == cmdenrich.KindUpgrade && ec.Subject == models.UpgradeLegEnhancementZealotSpeed:
			note(pid, "speedlot", ec.Second)
		}
	}
	for _, k := range order {
		sec := firstSec[k]
		if sec >= firstTechTimingWindowSec {
			continue
		}
		player, ok := e.players[k.pid]
		if !ok || player == nil || !strings.EqualFold(player.Race, "Protoss") {
			continue
		}
		opp := e.opponentRace(k.pid)
		var desc, icon string
		switch k.eventType {
		case "first_reaver":
			if !strings.EqualFold(opp, "Protoss") && !strings.EqualFold(opp, "Terran") {
				continue
			}
			desc, icon = fmt.Sprintf("%s trains their first Reaver", e.playerName(k.pid)), models.GeneralUnitReaver
		case "first_corsair":
			if !strings.EqualFold(opp, "Zerg") {
				continue
			}
			desc, icon = fmt.Sprintf("%s trains their first Corsair", e.playerName(k.pid)), models.GeneralUnitCorsair
		case "speedlot":
			if !strings.EqualFold(opp, "Zerg") {
				continue
			}
			desc, icon = fmt.Sprintf("%s starts Zealot Speed research", e.playerName(k.pid)), models.GeneralUnitZealot
		default:
			continue
		}
		e.emitEvent(k.eventType, sec, desc, e.playerRef(k.pid), nil, -1, []string{icon})
	}
}

func (e *Engine) hasKnownEnemyTeam(a byte, b byte) bool {
	ta, oka := e.teams[a]
	tb, okb := e.teams[b]
	return oka && okb && ta != 0 && tb != 0 && ta != tb
}

// isRushTargetEnemy is like hasKnownEnemyTeam but treats two humans in a 1v1 as
// opponents when replay teams are missing or zero, as some replays leave them.
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

// rushTargetEnemyForBase prefers the base's dynamic owner but falls back to its
// static start/natural assignment while still neutral-owned: on standard maps
// the rusher attack-moves through the opponent's not-yet-taken natural, which
// reads as neutral early, so dynamic ownership alone would drop the strongest
// rush signal.
func (e *Engine) rushTargetEnemyForBase(pid byte, baseIdx int) (byte, bool) {
	if baseIdx < 0 || baseIdx >= len(e.ownerByBase) {
		return 0, false
	}
	owner := e.ownerByBase[baseIdx]
	if owner == neutralPID {
		owner = e.staticBaseOwner(baseIdx)
	}
	if owner == neutralPID || owner == pid || !e.isRushTargetEnemy(pid, owner) {
		return 0, false
	}
	return owner, true
}

// staticBaseOwner ignores in-game ownership transitions. neutralPID if none.
func (e *Engine) staticBaseOwner(baseIdx int) byte {
	for pid, bi := range e.startBaseByPID {
		if bi == baseIdx {
			return pid
		}
	}
	for pid, bi := range e.naturalBaseByPID {
		if bi == baseIdx {
			return pid
		}
	}
	return neutralPID
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

func (e *Engine) proxyPlacementAllowed(pid byte, x float64, y float64) bool {
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
	// Resolve own vs enemy main from the placing player so the gate can tell a home
	// build from a forward proxy: far enough from the builder's own main
	// (>= 0.7 * half) and within reach of the enemy's (<= 1.3 * half). Dropping the
	// old "<= 1.3 * half from BOTH mains" cap is what admits an at-the-enemy proxy
	// (a 2-Rax BBS on the doorstep), not only the symmetric midfield one.
	ownStart, enemyStart := startA, startB
	if pid == e.humanPlayerIDs[1] {
		ownStart, enemyStart = startB, startA
	}
	halfDist := startDist / 2
	minDist := halfDist * 0.7
	maxDist := halfDist * 1.3
	distOwn := dist(x, y, e.bases[ownStart].CenterX, e.bases[ownStart].CenterY)
	distEnemy := dist(x, y, e.bases[enemyStart].CenterX, e.bases[enemyStart].CenterY)
	return distOwn >= minDist && distEnemy <= maxDist
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
		"commandcenter", "supplydepot", "barracks", "engineeringbay", "academy",
		"bunker", "missileturret", "factory", "starport", "armory", "refinery",
		"sciencefacility", "covertops", "physicslab", "nuclearsilo",
		"machineshop", "comsat", "controltower":
		return "Terran"
	case
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
		// Clock==0 is scmapanalyzer's "center base" marker; label it explicitly because
		// the "at N" / "an expa near N" templates don't accommodate the center concept.
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
		e.bases[i].DisplayName = fmt.Sprintf("an expansion near %d", oc)
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
	return pointToEventBaseBounded(x, y, bases, math.MaxFloat64)
}

// pointToEventBaseBounded caps only the "just outside a base" radius snap;
// polygon containment is always honoured. Without the cap, a build in open
// ground on a money map (large NaturalRadius) is attributed to a distant base
// it never threatened. Pass math.MaxFloat64 for no cap.
func pointToEventBaseBounded(x float64, y float64, bases []base, maxFallbackPx float64) int {
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
		if opRadius > maxFallbackPx {
			opRadius = maxFallbackPx
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
