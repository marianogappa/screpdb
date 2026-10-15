package engine

import "github.com/marianogappa/screpsim"

const (
	actionSelect      = 0x09
	actionShiftSelect = 0x0a

	desyncWindowFrames   = 240
	desyncWindowMisses   = 3
	desyncMinTailMisses  = 20
	desyncTailMissPerMil = 50
)

type selectResult struct {
	frame   int32
	missing bool
}

func (p *passState) commandResult(e *screpsim.CommandResult) {
	if e.ActionID != actionSelect && e.ActionID != actionShiftSelect {
		return
	}
	missing := e.MissingTags > 0
	p.d.Drift.Selects++
	if missing {
		if p.d.Drift.MissingSelects == 0 {
			p.d.Drift.FirstMissing = e.Frame
		}
		p.d.Drift.MissingSelects++
	}
	p.selects = append(p.selects, selectResult{frame: e.Frame, missing: missing})
}

// desyncOnset is the first select of a lasting run of selects naming units the engine does not have.
// A player who selects a unit that dies in the same frame makes a stray miss; in-sync games stay under
// 0.5% of selects, desynced ones are 5% or more from the onset on (a 160-replay sample, 1v1 and team).
func desyncOnset(selects []selectResult) int32 {
	tailMisses := make([]int, len(selects)+1)
	for i := len(selects) - 1; i >= 0; i-- {
		tailMisses[i] = tailMisses[i+1]
		if selects[i].missing {
			tailMisses[i]++
		}
	}
	windowEnd := 0
	for i, s := range selects {
		if !s.missing {
			continue
		}
		for windowEnd < len(selects) && selects[windowEnd].frame < s.frame+desyncWindowFrames {
			windowEnd++
		}
		inWindow := tailMisses[i] - tailMisses[windowEnd]
		tail := len(selects) - i
		if inWindow >= desyncWindowMisses && tailMisses[i] >= desyncMinTailMisses && tailMisses[i]*1000 >= desyncTailMissPerMil*tail {
			return s.frame
		}
	}
	return NoFrame
}
