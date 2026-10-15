//go:build !js

package dashboard

import (
	"sync"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/library"
)

const recentDigests = 8

// digestMemo keeps the last few full digests: a game page asks for several views of one game.
type digestMemo struct {
	mu    sync.Mutex
	order [][32]byte
	byKey map[[32]byte]*engine.Digest
}

func (m *digestMemo) get(checksum [32]byte, load func() (*engine.Digest, bool)) *engine.Digest {
	m.mu.Lock()
	if d, ok := m.byKey[checksum]; ok {
		m.mu.Unlock()
		return d
	}
	m.mu.Unlock()
	d, ok := load()
	if !ok {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byKey == nil {
		m.byKey = map[[32]byte]*engine.Digest{}
	}
	if len(m.order) >= recentDigests {
		delete(m.byKey, m.order[0])
		m.order = m.order[1:]
	}
	m.byKey[checksum] = d
	m.order = append(m.order, checksum)
	return d
}

// engineDigest is the full digest of a replay in engine mode with high confidence. A desynced game keeps
// its command-inferred views: engine data after the desync is wrong, and half of each would mislead.
func (d *Dashboard) engineDigest(replayID int64) (*engine.Digest, *library.Replay) {
	if d.enginePass == nil || d.library == nil {
		return nil, nil
	}
	rec, ok := d.library.lib.Snapshot().ByID(replayID)
	if !ok || rec.EngineData == nil || rec.EngineData.LowConfidence {
		return nil, nil
	}
	runner := d.enginePass
	dg := d.digests.get(rec.Checksum, func() (*engine.Digest, bool) { return runner.Digest(rec.Checksum) })
	return dg, rec
}

func frameSecond(frame int32) int64 { return int64(frame) * 42 / 1000 }

// applyEngineToGameDetail swaps the build order, the production timeline and the per-slice unit counts
// for what the game engine says was made: no phantom builds, real completion times and cancels.
func (d *Dashboard) applyEngineToGameDetail(detail *workflowGameDetail) {
	dg, rec := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	playerBySlot := map[int8]int64{}
	for ordinal, p := range rec.Players {
		playerBySlot[int8(p.Slot)] = library.PlayerID(rec.ID, uint8(ordinal))
	}
	rows := make([]unitProductionRow, 0, len(dg.Productions))
	for _, pr := range dg.Productions {
		playerID, ok := playerBySlot[pr.Owner]
		if !ok {
			continue
		}
		unit := repcmd.UnitByID(pr.Type)
		if unit == nil || unit.Name == "" {
			continue
		}
		row := unitProductionRow{PlayerID: playerID, Second: frameSecond(pr.Start), UnitType: unit.Name, Count: 1, Cancelled: pr.Cancel != engine.NoFrame}
		if pr.Complete != engine.NoFrame {
			completed := frameSecond(pr.Complete)
			row.CompletedSecond = &completed
		}
		rows = append(rows, row)
	}
	buildUnitsBySlice(detail, rows)
	populateSupplyCurves(detail, dg, playerBySlot)
}

const supplyCapDoubled = 400

func populateSupplyCurves(detail *workflowGameDetail, dg *engine.Digest, playerBySlot map[int8]int64) {
	curves := map[int64][]workflowSupplyPoint{}
	last := map[int64][2]int32{}
	for _, s := range dg.Economy {
		playerID, ok := playerBySlot[s.Owner]
		if !ok {
			continue
		}
		state := [2]int32{s.SupplyUsed, min(s.SupplyMax, supplyCapDoubled)}
		if prev, seen := last[playerID]; seen && prev == state {
			continue
		}
		last[playerID] = state
		curves[playerID] = append(curves[playerID], workflowSupplyPoint{Second: frameSecond(s.Frame), Used: float64(state[0]) / 2, Cap: float64(state[1]) / 2})
	}
	for i := range detail.Players {
		if c, ok := curves[detail.Players[i].PlayerID]; ok {
			detail.Players[i].SupplyCurve = c
		}
	}
}
