package worldstate

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/utils"
)

const (
	ownershipTimeoutSec = 90
	contestedSwitchSec  = 45
	rushWindowSec       = 300
	attackCooldownSec   = 60
	attackWindowSec     = 60
	attackMinCount      = 5
	attackCutoffSec     = 20 * 60
	neutralPID          = byte(255)
	commandRadiusMul    = 1.25
	radiusSafety        = 0.98
)

type NarrativeEntry struct {
	Type        string `json:"type"`
	Second      int    `json:"second"`
	Description string `json:"description"`
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
	DisplayName   string
}

type Engine struct {
	replay  *models.Replay
	players map[byte]*models.Player
	teams   map[byte]byte
	left    map[byte]bool

	bases        []base
	ownerByBase  []byte
	lastOwningAt []map[byte]int

	startBaseByPID map[byte]int
	playerExpanded map[byte]map[int]bool

	attackCountsByWindow map[string]int
	lastAttackEmitted    map[string]int

	entries []NarrativeEntry
}

func NewEngine(replay *models.Replay, players []*models.Player, mapCtx *models.ReplayMapContext) *Engine {
	e := &Engine{
		replay:               replay,
		players:              map[byte]*models.Player{},
		teams:                map[byte]byte{},
		left:                 map[byte]bool{},
		startBaseByPID:       map[byte]int{},
		playerExpanded:       map[byte]map[int]bool{},
		attackCountsByWindow: map[string]int{},
		lastAttackEmitted:    map[string]int{},
		entries:              make([]NarrativeEntry, 0, 256),
	}
	for _, p := range players {
		e.players[p.PlayerID] = p
		e.teams[p.PlayerID] = p.Team
	}

	if mapCtx == nil {
		return e
	}

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
	if len(e.bases) == 0 {
		return e
	}

	slotToPID := map[byte]byte{}
	for _, p := range players {
		slotToPID[byte(p.SlotID)] = p.PlayerID
	}
	for _, sl := range mapCtx.StartLocations {
		idx := nearestBase(float64(sl.X), float64(sl.Y), e.bases)
		if idx < 0 {
			continue
		}
		e.bases[idx].StartCount++
		e.bases[idx].IsStarting = true
		if pid, ok := slotToPID[sl.SlotID]; ok {
			e.startBaseByPID[pid] = idx
		}
	}

	assignPerBaseRadii(e.bases, radiusSafety)
	enlargeStartBaseRadii(e.bases, radiusSafety)

	e.ownerByBase = make([]byte, len(e.bases))
	e.lastOwningAt = make([]map[byte]int, len(e.bases))
	for i := range e.bases {
		e.ownerByBase[i] = neutralPID
		e.lastOwningAt[i] = map[byte]int{}
	}
	for pid, bi := range e.startBaseByPID {
		e.ownerByBase[bi] = pid
		e.lastOwningAt[bi][pid] = 0
	}

	for i := range e.bases {
		oc := utils.CalculateStartLocationOclock(
			int(replay.MapWidth),
			int(replay.MapHeight),
			int(math.Round(e.bases[i].CenterX)),
			int(math.Round(e.bases[i].CenterY)),
		)
		if e.bases[i].IsStarting {
			e.bases[i].DisplayName = fmt.Sprintf("at %d", oc)
		} else {
			e.bases[i].DisplayName = fmt.Sprintf("an expa near %d", oc)
		}
	}
	return e
}

func (e *Engine) Entries() []NarrativeEntry {
	out := make([]NarrativeEntry, len(e.entries))
	copy(out, e.entries)
	return out
}

func (e *Engine) ProcessCommand(command *models.Command) {
	if len(e.bases) == 0 || command == nil {
		return
	}

	sec := command.SecondsFromGameStart
	if sec < 0 {
		sec = 0
	}

	// Drop ownership on timeout/leave before applying command effects.
	for bi := range e.ownerByBase {
		owner := e.ownerByBase[bi]
		if owner == neutralPID {
			continue
		}
		if e.left[owner] {
			e.transitionOwnership(sec, bi, neutralPID, "left the game")
			continue
		}
		if sec-e.lastOwningAt[bi][owner] > ownershipTimeoutSec {
			e.transitionOwnership(sec, bi, neutralPID, "stopped owning actions")
		}
	}

	pid, ok := e.playerIDFromCommand(command)
	if !ok {
		return
	}
	if isLeaveAction(command.ActionType) {
		e.left[pid] = true
		e.emitEvent("leave", sec, fmt.Sprintf("%s leaves the game", e.playerName(pid)))
		for bi, owner := range e.ownerByBase {
			if owner == pid {
				e.transitionOwnership(sec, bi, neutralPID, "left the game")
			}
		}
		return
	}
	if e.left[pid] {
		return
	}

	x, y, hasCoords := commandCoords(command)
	if !hasCoords {
		return
	}

	// Build/Land command positions are tile coordinates.
	if isBuildLike(command.ActionType) {
		x = tileToPixel(x)
		y = tileToPixel(y)
	}

	biOwnership := nearestBase(x, y, e.bases)
	if biOwnership < 0 {
		return
	}
	biEvent := pointToEventBase(x, y, e.bases)
	if biEvent < 0 {
		biEvent = biOwnership
	}

	owner := e.ownerByBase[biOwnership]
	orderName := ""
	if command.OrderName != nil {
		orderName = *command.OrderName
	}
	unitType := ""
	if command.UnitType != nil {
		unitType = *command.UnitType
	}

	isAttackLike := isAttackAction(command.ActionType, command.OrderID, orderName)
	isBuild := isBuildLike(command.ActionType)
	isTownHall := isTownHallUnit(unitType)
	isRush := isRushBuilding(unitType)
	isDrop := isDropOrder(orderName)
	isRecall := isRecallOrder(orderName)
	isNuke := isNukeOrder(orderName)

	owningSignal := false
	if isBuild {
		owningSignal = true
	} else if owner == pid && !isAttackLike {
		owningSignal = true
	}
	if owningSignal {
		e.lastOwningAt[biOwnership][pid] = sec
	}

	if owningSignal {
		if owner == neutralPID {
			e.transitionOwnership(sec, biOwnership, pid, "ownership claim")
		} else if owner != pid {
			ownerLast := e.lastOwningAt[biOwnership][owner]
			if sec-ownerLast > contestedSwitchSec {
				e.transitionOwnership(sec, biOwnership, pid, "ownership transfer")
			}
		}
	}

	if isTownHall {
		if e.playerExpanded[pid] == nil {
			e.playerExpanded[pid] = map[int]bool{}
		}
		if !e.playerExpanded[pid][biOwnership] {
			if !e.bases[biOwnership].IsStarting {
				where := e.bases[biOwnership].DisplayName
				if strings.HasPrefix(where, "at ") {
					e.emitEvent("expansion", sec, fmt.Sprintf("%s expands %s", e.playerName(pid), where))
				} else {
					e.emitEvent("expansion", sec, fmt.Sprintf("%s expands to %s", e.playerName(pid), where))
				}
			}
			e.playerExpanded[pid][biOwnership] = true
		}
	}

	owner = e.ownerByBase[biOwnership]
	if owner == neutralPID || owner == pid || e.left[owner] || e.sameTeam(pid, owner) {
		return
	}

	if sec <= attackCutoffSec && isAttackLike {
		window := (sec / attackWindowSec) * attackWindowSec
		wk := fmt.Sprintf("%d|%d|%d|%d", pid, owner, biEvent, window)
		e.attackCountsByWindow[wk]++
		if e.attackCountsByWindow[wk] >= attackMinCount {
			k := fmt.Sprintf("%d|%d|%d", pid, owner, biEvent)
			last := e.lastAttackEmitted[k]
			if last == 0 || sec-last >= attackCooldownSec {
				e.lastAttackEmitted[k] = sec
				e.emitEvent("attack", sec, fmt.Sprintf("%s attacks %s %s", e.playerName(pid), e.playerName(owner), e.bases[biEvent].DisplayName))
			}
		}
	}
	if isDrop {
		e.emitEvent("drop", sec, fmt.Sprintf("%s drops on %s %s", e.playerName(pid), e.playerName(owner), e.bases[biEvent].DisplayName))
	}
	if isRecall {
		e.emitEvent("recall", sec, fmt.Sprintf("%s recalls into %s %s", e.playerName(pid), e.playerName(owner), e.bases[biEvent].DisplayName))
	}
	if isNuke {
		e.emitEvent("nuke", sec, fmt.Sprintf("%s nukes %s %s", e.playerName(pid), e.playerName(owner), e.bases[biEvent].DisplayName))
	}
	if isBuild && isRush && sec <= rushWindowSec {
		e.emitEvent("rush", sec, fmt.Sprintf("%s cannon/bunker rushes %s %s", e.playerName(pid), e.playerName(owner), e.bases[biEvent].DisplayName))
	}
}

func (e *Engine) transitionOwnership(sec int, baseIdx int, to byte, reason string) {
	from := e.ownerByBase[baseIdx]
	if from == to {
		return
	}
	e.ownerByBase[baseIdx] = to
	if to == neutralPID {
		// Keep neutralization in world state but do not emit as game event.
		return
	}
	_ = reason
	e.emitEvent("takeover", sec, fmt.Sprintf("%s takes over %s", e.playerName(to), e.bases[baseIdx].DisplayName))
}

func (e *Engine) emitEvent(eventType string, second int, description string) {
	if description == "" || eventType == "" {
		return
	}
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
	})
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

func isAttackAction(actionType string, orderID *byte, orderName string) bool {
	n := normalize(actionType)
	if n == "rightclick" {
		return true
	}
	if n != "targetedorder" {
		return false
	}
	if orderID != nil && repcmd.IsOrderIDKindAttack(*orderID) {
		return true
	}
	o := normalize(orderName)
	return strings.Contains(o, "attack") || strings.Contains(o, "psionicstorm")
}

func normalize(s string) string {
	x := strings.ToLower(s)
	x = strings.ReplaceAll(x, " ", "")
	x = strings.ReplaceAll(x, "_", "")
	return x
}

func pointToEventBase(x float64, y float64, bases []base) int {
	best := -1
	bestDist := math.MaxFloat64
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
