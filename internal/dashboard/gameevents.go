package dashboard

import (
	"math"
	"strings"

	"github.com/marianogappa/screpdb/internal/gameevents"
	"github.com/marianogappa/screpdb/internal/models"
)

// A game's events come from exactly one of two sources, never mixed:
//
//	engine digest trusted? ──yes──> gameevents_engine.go   (moments.Detect: exact positions, deaths)
//	          │
//	          no ───────────────> gameevents_commands.go (worldstate rows guessed from commands at ingest)
//
// Both keep the command meta events (openers, leaves, alliances). Whatever the source, every event then
// gets the same treatment here: owner-named naturals, a map region of a steady shape, and a key flag
// from gameevents.Select, which picks the few events the tab shows by default.
func (d *Dashboard) populateGameEvents(detail *workflowGameDetail, mapLayout *models.MapContextLayout, startClockByPlayerID map[int64]int, displayByName map[string]string) error {
	if err := d.commandGameEvents(detail, mapLayout, startClockByPlayerID, displayByName); err != nil {
		return err
	}
	d.applyEngineEvents(detail, mapLayout, startClockByPlayerID)
	detail.GameEvents = resolveNaturalOwnerLabels(detail.GameEvents, detail.Players, startClockByPlayerID)
	return nil
}

// regionAspect is the width over height every event region is widened to, so the map frame keeps one
// shape from event to event instead of jumping between tall and wide.
const regionAspect = 4.0 / 3.0

// curateGameEvents runs last, once every event (the openers included) is on the list.
func curateGameEvents(detail *workflowGameDetail) {
	humans := len(detail.Players)
	cands := make([]gameevents.Candidate, len(detail.GameEvents))
	for i, e := range detail.GameEvents {
		c := gameevents.Candidate{Type: strings.ToLower(e.Type), Second: int(e.Second)}
		if e.Actor != nil {
			c.Actor = e.Actor.PlayerID
		}
		if o := e.Outcome; o != nil {
			c.Magnitude = gameevents.Magnitude{Traded: int(o.Killed + o.Lost), Workers: int(o.Workers), TownHall: o.TownHall, First: o.First}
		}
		cands[i] = c
	}
	gameevents.Number(cands)
	key := gameevents.Select(cands, gameevents.Game{DurationSeconds: int(detail.DurationSeconds), Players: humans})
	for i := range detail.GameEvents {
		e := &detail.GameEvents[i]
		e.Key = key[i]
		if e.Engine != nil && e.Engine.Region != nil {
			e.Engine.Region = steadyRegion(*e.Engine.Region, detail.MapWidthPixels, detail.MapHeightPixels)
		}
	}
}

// steadyRegion widens a region to regionAspect around its centre, then slides it inside the map.
func steadyRegion(r workflowEngineRegion, mapW, mapH int64) *workflowEngineRegion {
	cx, cy := float64(r.X)+float64(r.W)/2, float64(r.Y)+float64(r.H)/2
	w, h := float64(r.W), float64(r.H)
	if w/h < regionAspect {
		w = h * regionAspect
	} else {
		h = w / regionAspect
	}
	if mapW > 0 && mapH > 0 {
		scale := math.Min(1, math.Min(float64(mapW)/w, float64(mapH)/h))
		w, h = w*scale, h*scale
	}
	x, y := cx-w/2, cy-h/2
	if mapW > 0 && mapH > 0 {
		x = math.Max(0, math.Min(x, float64(mapW)-w))
		y = math.Max(0, math.Min(y, float64(mapH)-h))
	}
	return &workflowEngineRegion{X: int64(math.Round(x)), Y: int64(math.Round(y)), W: int64(math.Round(w)), H: int64(math.Round(h))}
}
