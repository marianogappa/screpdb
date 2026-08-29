package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/repparser"
)

const framesPerMin = 1428.6
const framesPerSec = 23.81

type acc struct {
	name, race string

	frames []float64

	typeCounts map[string]float64
	total      float64

	hkAssignGroup [10]float64
	hkSelectGroup [10]float64
	hkAssigns     float64
	hkSelects     float64
	hkDoubleTaps  float64
	lastHKSelGrp  int
	lastHKSelFrm  float64

	selectSizes []float64

	queueable    float64
	queued       float64
	posDists     []float64
	lastPosX     float64
	lastPosY     float64
	hasLastPos   bool
	minimapPings float64
	chats        float64

	earlyCmds, midCmds, lateCmds float64

	effective float64

	bigram        [10][10]float64
	biICI         map[int][]float64
	prevClass     int
	firstAssigns  []int
	iciFineBins   [24]float64
	hkSelTrans    [10][10]float64
	pendingAssign map[int]float64
	a2sLatencies  []float64
	dblTapGaps    []float64
	classICI      [10][]float64
	burstRuns     []float64
	curRun        float64
}

func newAcc() *acc {
	return &acc{typeCounts: map[string]float64{}, lastHKSelGrp: -1, prevClass: -1,
		pendingAssign: map[int]float64{}, biICI: map[int][]float64{}}
}

var coreClasses = []int{0, 3, 4, 5, 7}

var typeBuckets = []string{
	"select", "select_add", "select_remove", "hotkey_assign", "hotkey_select",
	"rightclick", "targeted_order", "build", "train", "unit_morph",
	"building_morph", "tech", "upgrade", "stop_hold", "return_cargo",
	"cancel", "burrow_siege_cloak", "unload", "liftoff_land", "other",
}

func pct(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func featureNames() []string {
	names := []string{"apm", "eapm", "redundancy", "apm_early", "apm_mid", "apm_late"}
	for _, t := range typeBuckets {
		names = append(names, "frac_"+t)
	}
	for i := 0; i < 10; i++ {
		names = append(names, fmt.Sprintf("hk_assign_g%d", i))
	}
	for i := 0; i < 10; i++ {
		names = append(names, fmt.Sprintf("hk_select_g%d", i))
	}
	names = append(names,
		"hk_per_min", "hk_assigns_per_min", "hk_sel_assign_ratio", "hk_double_tap_rate",
		"sel_size_mean", "sel_size_p90",
		"queued_frac",
		"ici_p10", "ici_p25", "ici_p50", "ici_p75", "ici_p90", "ici_mean",
		"ici_frac_0", "ici_frac_le2", "ici_frac_ge24",
		"dist_p50", "dist_p90", "dist_frac_far",
		"pings_per_min", "chats_per_min",
	)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			names = append(names, fmt.Sprintf("bigram_%d_%d", i, j))
		}
	}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			names = append(names, fmt.Sprintf("hktrans_%d_%d", i, j))
		}
	}
	names = append(names, "a2s_lat_p25", "a2s_lat_p50", "a2s_lat_p75", "dbltap_gap_med")
	for b := 0; b < len(iciBins); b++ {
		names = append(names, fmt.Sprintf("ici_hist_%d", b))
	}
	for c := 0; c < 10; c++ {
		names = append(names, fmt.Sprintf("preici_med_c%d", c))
	}
	names = append(names, "burst_run_mean")
	for _, i := range coreClasses {
		for _, j := range coreClasses {
			names = append(names, fmt.Sprintf("bici_%d_%d", i, j))
		}
	}
	for b := 0; b < 13; b++ {
		names = append(names, fmt.Sprintf("dblgap_h%d", b))
	}
	for b := 0; b < len(a2sBins); b++ {
		names = append(names, fmt.Sprintf("a2s_h%d", b))
	}
	for g := 0; g < 10; g++ {
		names = append(names, fmt.Sprintf("firstassign_g%d", g))
	}
	names = append(names, "ici_mode", "burst_cadence")
	for b := 0; b < len(distBins); b++ {
		names = append(names, fmt.Sprintf("dist_h%d", b))
	}
	return names
}

var a2sBins = []float64{6, 12, 24, 48, 120, math.Inf(1)}
var distBins = []float64{16, 64, 160, 320, 640, 1280, math.Inf(1)}

var iciBins = []float64{1, 2, 3, 4, 6, 8, 12, 16, 24, 36, 48, 72, 120, 240, math.Inf(1)}

func (a *acc) features(gameFrames float64) []float64 {
	durMin := gameFrames / framesPerMin
	lastFrame := a.frames[len(a.frames)-1]
	activeMin := lastFrame / framesPerMin
	if activeMin < 0.5 {
		activeMin = 0.5
	}
	apm := a.total / activeMin
	eapm := a.effective / activeMin
	red := 0.0
	if a.total > 0 {
		red = 1 - a.effective/a.total
	}

	earlyMin := math.Min(activeMin, 3)
	midMin := math.Max(0.001, math.Min(activeMin, 8)-3)
	lateMin := math.Max(0.001, activeMin-8)
	apmEarly := a.earlyCmds / earlyMin
	apmMid := 0.0
	if activeMin > 3 {
		apmMid = a.midCmds / midMin
	}
	apmLate := 0.0
	if activeMin > 8 {
		apmLate = a.lateCmds / lateMin
	}

	fs := []float64{apm, eapm, red, apmEarly, apmMid, apmLate}
	for _, t := range typeBuckets {
		fs = append(fs, a.typeCounts[t]/a.total)
	}
	for i := 0; i < 10; i++ {
		v := 0.0
		if a.hkAssigns > 0 {
			v = a.hkAssignGroup[i] / a.hkAssigns
		}
		fs = append(fs, v)
	}
	for i := 0; i < 10; i++ {
		v := 0.0
		if a.hkSelects > 0 {
			v = a.hkSelectGroup[i] / a.hkSelects
		}
		fs = append(fs, v)
	}
	selAssignRatio := 0.0
	if a.hkAssigns > 0 {
		selAssignRatio = a.hkSelects / a.hkAssigns
	}
	dblRate := 0.0
	if a.hkSelects > 0 {
		dblRate = a.hkDoubleTaps / a.hkSelects
	}
	fs = append(fs, (a.hkAssigns+a.hkSelects)/activeMin, a.hkAssigns/activeMin, selAssignRatio, dblRate)

	sort.Float64s(a.selectSizes)
	fs = append(fs, mean(a.selectSizes), pct(a.selectSizes, 0.9))

	qf := 0.0
	if a.queueable > 0 {
		qf = a.queued / a.queueable
	}
	fs = append(fs, qf)

	icis := make([]float64, 0, len(a.frames)-1)
	n0, nle2, nge24 := 0.0, 0.0, 0.0
	for i := 1; i < len(a.frames); i++ {
		d := a.frames[i] - a.frames[i-1]
		icis = append(icis, d)
		if d == 0 {
			n0++
		}
		if d <= 2 {
			nle2++
		}
		if d >= 24 {
			nge24++
		}
	}
	sort.Float64s(icis)
	nn := float64(len(icis))
	if nn == 0 {
		nn = 1
	}
	fs = append(fs, pct(icis, 0.1), pct(icis, 0.25), pct(icis, 0.5), pct(icis, 0.75), pct(icis, 0.9), mean(icis),
		n0/nn, nle2/nn, nge24/nn)

	sort.Float64s(a.posDists)
	far := 0.0
	for _, d := range a.posDists {
		if d > 512 {
			far++
		}
	}
	nd := float64(len(a.posDists))
	if nd == 0 {
		nd = 1
	}
	fs = append(fs, pct(a.posDists, 0.5), pct(a.posDists, 0.9), far/nd)

	fs = append(fs, a.minimapPings/activeMin, a.chats/activeMin)

	bigramTotal := 0.0
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			bigramTotal += a.bigram[i][j]
		}
	}
	if bigramTotal == 0 {
		bigramTotal = 1
	}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			fs = append(fs, a.bigram[i][j]/bigramTotal)
		}
	}
	transTotal := 0.0
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			transTotal += a.hkSelTrans[i][j]
		}
	}
	if transTotal == 0 {
		transTotal = 1
	}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			fs = append(fs, a.hkSelTrans[i][j]/transTotal)
		}
	}
	sort.Float64s(a.a2sLatencies)
	fs = append(fs, pct(a.a2sLatencies, 0.25), pct(a.a2sLatencies, 0.5), pct(a.a2sLatencies, 0.75))
	sort.Float64s(a.dblTapGaps)
	fs = append(fs, pct(a.dblTapGaps, 0.5))

	histCounts := make([]float64, len(iciBins))
	for _, d := range icis {
		for b, edge := range iciBins {
			if d < edge {
				histCounts[b]++
				break
			}
		}
	}
	for _, h := range histCounts {
		fs = append(fs, h/nn)
	}
	for c := 0; c < 10; c++ {
		sort.Float64s(a.classICI[c])
		fs = append(fs, pct(a.classICI[c], 0.5))
	}
	fs = append(fs, mean(a.burstRuns))

	for _, i := range coreClasses {
		for _, j := range coreClasses {
			xs := a.biICI[i*10+j]
			sort.Float64s(xs)
			fs = append(fs, pct(xs, 0.5))
		}
	}
	dg := make([]float64, 13)
	for _, g := range a.dblTapGaps {
		b := int(g)
		if b > 12 {
			b = 12
		}
		dg[b]++
	}
	dgn := math.Max(float64(len(a.dblTapGaps)), 1)
	for _, v := range dg {
		fs = append(fs, v/dgn)
	}
	ah := make([]float64, len(a2sBins))
	for _, l := range a.a2sLatencies {
		for b, edge := range a2sBins {
			if l < edge {
				ah[b]++
				break
			}
		}
	}
	ahn := math.Max(float64(len(a.a2sLatencies)), 1)
	for _, v := range ah {
		fs = append(fs, v/ahn)
	}
	fa := make([]float64, 10)
	for _, g := range a.firstAssigns {
		fa[g]++
	}
	fan := math.Max(float64(len(a.firstAssigns)), 1)
	for _, v := range fa {
		fs = append(fs, v/fan)
	}
	modeBin, modeCount := 0.0, -1.0
	for b, c := range a.iciFineBins {
		if b >= 1 && c > modeCount {
			modeCount = c
			modeBin = float64(b)
		}
	}
	fs = append(fs, modeBin)
	var burstICIs []float64
	for i := 1; i < len(a.frames); i++ {
		d := a.frames[i] - a.frames[i-1]
		if d >= 1 && d <= 4 {
			burstICIs = append(burstICIs, d)
		}
	}
	sort.Float64s(burstICIs)
	fs = append(fs, pct(burstICIs, 0.5))
	dh := make([]float64, len(distBins))
	for _, dd := range a.posDists {
		for b, edge := range distBins {
			if dd < edge {
				dh[b]++
				break
			}
		}
	}
	dhn := math.Max(float64(len(a.posDists)), 1)
	for _, v := range dh {
		fs = append(fs, v/dhn)
	}
	_ = durMin
	return fs
}

func (a *acc) addPos(x, y float64) {
	if a.hasLastPos {
		dx, dy := x-a.lastPosX, y-a.lastPosY
		a.posDists = append(a.posDists, math.Hypot(dx, dy))
	}
	a.lastPosX, a.lastPosY, a.hasLastPos = x, y, true
}

func processReplay(path string) ([][]string, error) {
	r, err := repparser.ParseFileConfig(path, repparser.Config{Commands: true, MapData: false})
	if err != nil {
		return nil, err
	}
	r.Compute()

	players := map[byte]*acc{}
	numHumans := 0
	for _, p := range r.Header.Players {
		if p.Observer || p.Type == nil || p.Type.Name != "Human" {
			continue
		}
		numHumans++
		a := newAcc()
		a.name = p.Name
		if p.Race != nil {
			a.race = p.Race.Name
		}
		players[p.ID] = a
	}

	gameFrames := float64(r.Header.Frames)

	for _, cmd := range r.Commands.Cmds {
		base := cmd.BaseCmd()
		a, ok := players[base.PlayerID]
		if !ok {
			continue
		}
		f := float64(base.Frame)
		hasPrev := len(a.frames) > 0
		prevF := 0.0
		if hasPrev {
			prevF = a.frames[len(a.frames)-1]
		}
		a.frames = append(a.frames, f)
		a.total++
		cls := 9
		if base.IneffKind == 0 {
			a.effective++
		}
		switch {
		case f < 3*framesPerMin:
			a.earlyCmds++
		case f < 8*framesPerMin:
			a.midCmds++
		default:
			a.lateCmds++
		}

		switch c := cmd.(type) {
		case *repcmd.SelectCmd:
			switch base.Type.ID {
			case repcmd.TypeIDSelect, repcmd.TypeIDSelect121:
				a.typeCounts["select"]++
				a.selectSizes = append(a.selectSizes, float64(len(c.UnitTags)))
				cls = 0
			case repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectAdd121:
				a.typeCounts["select_add"]++
				cls = 1
			default:
				a.typeCounts["select_remove"]++
				cls = 1
			}
		case *repcmd.HotkeyCmd:
			g := int(c.Group)
			if g > 9 {
				g = 9
			}
			if c.HotkeyType.ID == repcmd.HotkeyTypeIDAssign || c.HotkeyType.ID == repcmd.HotkeyTypeIDAdd {
				a.typeCounts["hotkey_assign"]++
				a.hkAssigns++
				a.hkAssignGroup[g]++
				a.pendingAssign[g] = f
				cls = 2
			} else {
				a.typeCounts["hotkey_select"]++
				a.hkSelects++
				a.hkSelectGroup[g]++
				if a.lastHKSelGrp == g && f-a.lastHKSelFrm <= 8 {
					a.hkDoubleTaps++
				}
				if a.lastHKSelGrp == g && f-a.lastHKSelFrm <= 12 {
					a.dblTapGaps = append(a.dblTapGaps, f-a.lastHKSelFrm)
				}
				if a.lastHKSelGrp >= 0 {
					a.hkSelTrans[a.lastHKSelGrp][g]++
				}
				if af, ok := a.pendingAssign[g]; ok {
					a.a2sLatencies = append(a.a2sLatencies, f-af)
					delete(a.pendingAssign, g)
				}
				a.lastHKSelGrp = g
				a.lastHKSelFrm = f
				cls = 3
			}
		case *repcmd.RightClickCmd:
			a.typeCounts["rightclick"]++
			cls = 4
			a.queueable++
			if c.Queued {
				a.queued++
			}
			a.addPos(float64(c.Pos.X), float64(c.Pos.Y))
		case *repcmd.TargetedOrderCmd:
			a.typeCounts["targeted_order"]++
			cls = 5
			a.queueable++
			if c.Queued {
				a.queued++
			}
			a.addPos(float64(c.Pos.X), float64(c.Pos.Y))
		case *repcmd.BuildCmd:
			a.typeCounts["build"]++
			a.addPos(float64(c.Pos.X), float64(c.Pos.Y))
			cls = 6
		case *repcmd.TrainCmd:
			if base.Type.ID == repcmd.TypeIDTrain {
				a.typeCounts["train"]++
			} else {
				a.typeCounts["unit_morph"]++
			}
			cls = 7
		case *repcmd.BuildingMorphCmd:
			a.typeCounts["building_morph"]++
			cls = 7
		case *repcmd.TechCmd:
			a.typeCounts["tech"]++
			cls = 7
		case *repcmd.UpgradeCmd:
			a.typeCounts["upgrade"]++
			cls = 7
		case *repcmd.QueueableCmd:
			cls = 8
			a.queueable++
			if c.Queued {
				a.queued++
			}
			switch base.Type.ID {
			case repcmd.TypeIDStop, repcmd.TypeIDHoldPosition:
				a.typeCounts["stop_hold"]++
			case repcmd.TypeIDReturnCargo:
				a.typeCounts["return_cargo"]++
			case repcmd.TypeIDBurrow, repcmd.TypeIDUnburrow, repcmd.TypeIDSiege, repcmd.TypeIDUnsiege, repcmd.TypeIDCloack, repcmd.TypeIDDecloack:
				a.typeCounts["burrow_siege_cloak"]++
			default:
				a.typeCounts["other"]++
			}
		case *repcmd.MinimapPingCmd:
			a.minimapPings++
			a.typeCounts["other"]++
		case *repcmd.ChatCmd:
			a.chats++
			a.typeCounts["other"]++
		case *repcmd.UnloadCmd:
			a.typeCounts["unload"]++
		case *repcmd.LiftOffCmd:
			a.typeCounts["liftoff_land"]++
			a.addPos(float64(c.Pos.X), float64(c.Pos.Y))
		case *repcmd.LandCmd:
			a.typeCounts["liftoff_land"]++
			a.addPos(float64(c.Pos.X), float64(c.Pos.Y))
		default:
			switch base.Type.ID {
			case repcmd.TypeIDCancelBuild, repcmd.TypeIDCancelMorph, repcmd.TypeIDCancelTrain, repcmd.TypeIDCancelNuke, repcmd.TypeIDCancelTech, repcmd.TypeIDCancelUpgrade, repcmd.TypeIDCancelAddon:
				a.typeCounts["cancel"]++
			default:
				a.typeCounts["other"]++
			}
		}

		if a.prevClass >= 0 {
			a.bigram[a.prevClass][cls]++
			if hasPrev {
				a.biICI[a.prevClass*10+cls] = append(a.biICI[a.prevClass*10+cls], f-prevF)
			}
		}
		a.prevClass = cls
		if cls == 2 && len(a.firstAssigns) < 5 {
			if hc, ok := cmd.(*repcmd.HotkeyCmd); ok {
				g := int(hc.Group)
				if g > 9 {
					g = 9
				}
				a.firstAssigns = append(a.firstAssigns, g)
			}
		}
		if hasPrev {
			d := f - prevF
			a.classICI[cls] = append(a.classICI[cls], d)
			if d < 24 {
				a.iciFineBins[int(d)]++
			}
			if d <= 2 {
				a.curRun++
			} else {
				if a.curRun > 0 {
					a.burstRuns = append(a.burstRuns, a.curRun+1)
				}
				a.curRun = 0
			}
		}
	}

	matchup := r.Header.Matchup()
	mapName := strings.TrimSpace(r.Header.Map)
	var rows [][]string
	for _, a := range players {
		if a.total < 150 || gameFrames < 2*framesPerMin {
			continue
		}
		row := []string{
			filepath.Base(path), a.name, a.race, matchup, mapName,
			r.Header.StartTime.UTC().Format("2006-01-02T15:04:05"),
			fmt.Sprintf("%.2f", gameFrames/framesPerMin),
			fmt.Sprintf("%d", numHumans),
		}
		for _, v := range a.features(gameFrames) {
			row = append(row, fmt.Sprintf("%.5f", v))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func main() {
	root := os.Args[1]
	out := os.Args[2]

	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".rep") {
			files = append(files, path)
		}
		return nil
	})
	fmt.Fprintf(os.Stderr, "found %d replays\n", len(files))

	type result struct {
		rows [][]string
		err  error
		path string
	}
	jobs := make(chan string, len(files))
	results := make(chan result, len(files))
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							results <- result{nil, fmt.Errorf("panic: %v", rec), p}
						}
					}()
					rows, err := processReplay(p)
					results <- result{rows, err, p}
				}()
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	go func() { wg.Wait(); close(results) }()

	fo, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer fo.Close()
	w := csv.NewWriter(fo)
	header := []string{"file", "player", "race", "matchup", "map", "start_time", "duration_min", "num_humans"}
	header = append(header, featureNames()...)
	w.Write(header)

	nerr, nrows := 0, 0
	for res := range results {
		if res.err != nil {
			nerr++
			if nerr <= 5 {
				fmt.Fprintf(os.Stderr, "error %s: %v\n", res.path, res.err)
			}
			continue
		}
		for _, row := range res.rows {
			w.Write(row)
			nrows++
		}
	}
	w.Flush()
	fmt.Fprintf(os.Stderr, "wrote %d rows, %d errors\n", nrows, nerr)
}
