package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// handlerDebugMapLayout serves /api/custom/debug/map-layout/{replayID}.
//
// Emits both the raw scmapanalyzer layout and the engine-resolved view
// (start → player, natural → player). Flags (kind, clock) collisions that
// the render-time lookup would otherwise collapse. This is the primary
// diagnostic surface for location misclassifications where scmapanalyzer is
// known to be correct but polygons land on the wrong base. Not part of the
// OpenAPI contract — lives under /api/custom/ so it bypasses the strict
// handler and schema validation.
func (d *Dashboard) handlerDebugMapLayout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	replayIDStr := vars["replayID"]
	replayID, err := strconv.ParseInt(strings.TrimSpace(replayIDStr), 10, 64)
	if err != nil {
		http.Error(w, "invalid replayID", http.StatusBadRequest)
		return
	}

	summary, err := d.dbStore.GetReplaySummary(r.Context(), replayID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if summary == nil || strings.TrimSpace(summary.FilePath) == "" {
		http.Error(w, "replay has no file path", http.StatusNotFound)
		return
	}

	payload := debugMapLayoutResponse{
		ReplayID: replayID,
		MapName:  summary.MapName,
	}

	layout, layoutErr := buildDashboardMapContextLayoutFromReplay(summary.FilePath)
	if layoutErr != nil {
		payload.LayoutError = layoutErr.Error()
	}
	if layout != nil {
		payload.RawLayout = buildDebugRawLayout(layout)
		payload.ClockCollisions = buildClockCollisions(layout)
	}

	// Re-parse replay to reconstruct the engine view (start/natural assignments,
	// display names). This is the slow path — debug only.
	replay := &models.Replay{FilePath: summary.FilePath}
	data, parseErr := parser.ParseReplay(summary.FilePath, replay)
	if parseErr != nil {
		payload.ParseError = parseErr.Error()
	} else if data != nil {
		if data.MapContext != nil && data.MapContext.Layout == nil && layout != nil {
			data.MapContext.Layout = layout
		}
		engine := worldstate.NewEngine(data.Replay, data.Players, data.MapContext)
		bases, startBaseByPID, naturalBaseByPID, naturalOwnerByBase := engine.DebugSnapshot()
		payload.EngineBases = bases
		payload.StartBaseByPID = stringifyByteKeys(startBaseByPID)
		payload.NaturalBaseByPID = stringifyByteKeys(naturalBaseByPID)
		payload.NaturalOwnerByBase = stringifyIntKeysByte(naturalOwnerByBase)
		if data.Replay != nil {
			payload.MapWidthTiles = int(data.Replay.MapWidth)
			payload.MapHeightTiles = int(data.Replay.MapHeight)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(payload); encErr != nil {
		http.Error(w, encErr.Error(), http.StatusInternalServerError)
		return
	}
}

type debugMapLayoutResponse struct {
	ReplayID           int64                    `json:"replay_id"`
	MapName            string                   `json:"map_name,omitempty"`
	MapWidthTiles      int                      `json:"map_width_tiles,omitempty"`
	MapHeightTiles     int                      `json:"map_height_tiles,omitempty"`
	LayoutError        string                   `json:"layout_error,omitempty"`
	ParseError         string                   `json:"parse_error,omitempty"`
	RawLayout          *debugRawLayout          `json:"raw_layout,omitempty"`
	EngineBases        []worldstate.DebugBase   `json:"engine_bases,omitempty"`
	StartBaseByPID     map[string]int           `json:"start_base_by_player_id,omitempty"`
	NaturalBaseByPID   map[string]int           `json:"natural_base_by_player_id,omitempty"`
	NaturalOwnerByBase map[string]int           `json:"natural_owner_by_base,omitempty"`
	ClockCollisions    []debugClockCollisionRow `json:"clock_collisions,omitempty"`
}

type debugRawLayout struct {
	WidthTiles  int             `json:"width_tiles"`
	HeightTiles int             `json:"height_tiles"`
	Bases       []debugRawBase  `json:"bases"`
}

type debugRawBase struct {
	Name             string                 `json:"name"`
	Kind             string                 `json:"kind"`
	Clock            int                    `json:"clock"`
	Center           models.MapResourcePosition `json:"center"`
	Polygon          []models.MapPolygonPoint   `json:"polygon"`
	MineralOnly      bool                   `json:"mineral_only"`
	NaturalExpansion string                 `json:"natural_expansion,omitempty"`
}

type debugClockCollisionRow struct {
	Kind  string              `json:"kind"`
	Clock int                 `json:"clock"`
	Bases []debugCollisionRef `json:"bases"`
}

type debugCollisionRef struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

func buildDebugRawLayout(layout *models.MapContextLayout) *debugRawLayout {
	out := &debugRawLayout{
		WidthTiles:  layout.WidthTiles,
		HeightTiles: layout.HeightTiles,
		Bases:       make([]debugRawBase, 0, len(layout.Bases)),
	}
	for _, base := range layout.Bases {
		polyCopy := make([]models.MapPolygonPoint, len(base.Polygon))
		copy(polyCopy, base.Polygon)
		out.Bases = append(out.Bases, debugRawBase{
			Name:             base.Name,
			Kind:             base.Kind,
			Clock:            base.Clock,
			Center:           base.Center,
			Polygon:          polyCopy,
			MineralOnly:      base.MineralOnly,
			NaturalExpansion: base.NaturalExpansion,
		})
	}
	return out
}

// buildClockCollisions groups bases by (kind, clock) and returns every pair
// with more than one base — the set that the legacy clock-only lookup would
// collapse and paint incorrectly. Two players' naturals at the same clock is
// the common case; an expa sharing a clock with a natural is the other.
func buildClockCollisions(layout *models.MapContextLayout) []debugClockCollisionRow {
	type key struct {
		Kind  string
		Clock int
	}
	groups := map[key][]debugCollisionRef{}
	for i, base := range layout.Bases {
		k := key{Kind: strings.ToLower(strings.TrimSpace(base.Kind)), Clock: base.Clock}
		groups[k] = append(groups[k], debugCollisionRef{Index: i, Name: base.Name})
	}
	out := make([]debugClockCollisionRow, 0)
	for k, refs := range groups {
		if len(refs) < 2 {
			continue
		}
		out = append(out, debugClockCollisionRow{Kind: k.Kind, Clock: k.Clock, Bases: refs})
	}
	return out
}

func stringifyByteKeys(in map[byte]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[fmt.Sprintf("%d", k)] = v
	}
	return out
}

func stringifyIntKeysByte(in map[int]byte) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[fmt.Sprintf("%d", k)] = int(v)
	}
	return out
}

