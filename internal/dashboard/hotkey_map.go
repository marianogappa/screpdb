package dashboard

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

// The hotkey map shows the part of the map holding a player's hotkeyed buildings at a chosen moment.
// The server only says what to draw (the crop and each building with its groups, in map pixels); the
// browser draws it with the shared map component. Group contents come entirely from the stored
// hotkey_stream blob: building assigns carry the building type and its build tile.

// hotkeyMapDefaultMinute is the snapshot minute when the request omits one.
const hotkeyMapDefaultMinute = 8

// hotkeyBuildingFootprints maps a building to its build-tile footprint (W, H).
var hotkeyBuildingFootprints = map[string][2]int{
	"Hatchery": {4, 3}, "Lair": {4, 3}, "Hive": {4, 3},
	"Command Center": {4, 3}, "Nexus": {4, 3},
	"Barracks": {4, 3}, "Factory": {4, 3}, "Starport": {4, 3}, "Science Facility": {4, 3},
	"Engineering Bay": {4, 3}, "Academy": {3, 2}, "Armory": {3, 2},
	"Gateway": {4, 3}, "Stargate": {4, 3}, "Robotics Facility": {3, 2},
	"Forge": {3, 2}, "Cybernetics Core": {3, 2}, "Citadel of Adun": {3, 2},
	"Templar Archives": {3, 2}, "Observatory": {3, 2}, "Arbiter Tribunal": {3, 2},
	"Fleet Beacon": {3, 2}, "Robotics Support Bay": {3, 2},
	"Spawning Pool": {3, 2}, "Evolution Chamber": {3, 2}, "Hydralisk Den": {3, 2},
	"Spire": {2, 2}, "Greater Spire": {2, 2}, "Queen's Nest": {3, 2},
	"Defiler Mound": {4, 2}, "Ultralisk Cavern": {3, 2}, "Extractor": {4, 2},
	"Comsat Station": {2, 2}, "Nuclear Silo": {2, 2}, "Machine Shop": {2, 2}, "Control Tower": {2, 2},
}

const (
	tilePixels              = 32
	hotkeyMapTileRectMargin = 6
)

type hotkeyMapBuilding struct {
	group    int
	building string
	tileX    int
	tileY    int
}

type tileRect struct{ X, Y, W, H int }

func footprintOf(building string) [2]int {
	if fp, ok := hotkeyBuildingFootprints[building]; ok {
		return fp
	}
	return [2]int{3, 2}
}

func hotkeyBuildingsAtCutoff(events []hotkeystream.Event, cutoff int32) []hotkeyMapBuilding {
	last := map[byte]*hotkeystream.Event{}
	for i := range events {
		e := &events[i]
		if e.Sec > cutoff || e.Group > 9 {
			continue
		}
		if e.Type == hotkeystream.TypeAssignBuilding || e.Type == hotkeystream.TypeAssignUnits {
			last[e.Group] = e
		}
	}
	var out []hotkeyMapBuilding
	for group, e := range last {
		if e.Type != hotkeystream.TypeAssignBuilding || e.TileX == hotkeystream.TileUnknown {
			continue
		}
		name := hotkeystream.BuildingName(e.Building)
		if name == "" {
			continue
		}
		out = append(out, hotkeyMapBuilding{group: int(group), building: name, tileX: int(e.TileX), tileY: int(e.TileY)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].group < out[j].group })
	return out
}

func hotkeyMapTileRect(buildings []hotkeyMapBuilding, mapTilesW, mapTilesH int) tileRect {
	minTX, minTY, maxTX, maxTY := 1<<30, 1<<30, 0, 0
	for _, b := range buildings {
		fp := footprintOf(b.building)
		minTX, minTY = min(minTX, b.tileX), min(minTY, b.tileY)
		maxTX, maxTY = max(maxTX, b.tileX+fp[0]), max(maxTY, b.tileY+fp[1])
	}
	x0 := clampInt(minTX-hotkeyMapTileRectMargin, 0, mapTilesW)
	y0 := clampInt(minTY-hotkeyMapTileRectMargin, 0, mapTilesH)
	x1 := clampInt(maxTX+hotkeyMapTileRectMargin, 0, mapTilesW)
	y1 := clampInt(maxTY+hotkeyMapTileRectMargin, 0, mapTilesH)
	return tileRect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// keyboardOrder sorts hotkey groups as they sit on the keyboard: 1 to 9, then 0.
func keyboardOrder(groups []int) {
	order := func(g int) int {
		if g == 0 {
			return 10
		}
		return g
	}
	sort.Slice(groups, func(i, j int) bool { return order(groups[i]) < order(groups[j]) })
}

type hotkeyMapRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// hotkeyMapWireBuilding is one hotkeyed building: its footprint in map pixels and its groups in
// keyboard order (one building can sit on several hotkeys).
type hotkeyMapWireBuilding struct {
	Building  string        `json:"building"`
	Footprint hotkeyMapRect `json:"footprint"`
	Groups    []int         `json:"groups"`
}

type hotkeyMapWire struct {
	Second    int64                   `json:"second"`
	Crop      hotkeyMapRect           `json:"crop"`
	Buildings []hotkeyMapWireBuilding `json:"buildings"`
}

func hotkeyMapData(buildings []hotkeyMapBuilding, mapTilesW, mapTilesH int, second int64) hotkeyMapWire {
	rect := hotkeyMapTileRect(buildings, mapTilesW, mapTilesH)
	out := hotkeyMapWire{
		Second:    second,
		Crop:      hotkeyMapRect{X: rect.X * tilePixels, Y: rect.Y * tilePixels, W: rect.W * tilePixels, H: rect.H * tilePixels},
		Buildings: []hotkeyMapWireBuilding{},
	}
	byTile := map[[2]int]int{}
	for _, b := range buildings {
		key := [2]int{b.tileX, b.tileY}
		if i, ok := byTile[key]; ok {
			out.Buildings[i].Groups = append(out.Buildings[i].Groups, b.group)
			continue
		}
		fp := footprintOf(b.building)
		byTile[key] = len(out.Buildings)
		out.Buildings = append(out.Buildings, hotkeyMapWireBuilding{
			Building:  b.building,
			Footprint: hotkeyMapRect{X: b.tileX * tilePixels, Y: b.tileY * tilePixels, W: fp[0] * tilePixels, H: fp[1] * tilePixels},
			Groups:    []int{b.group},
		})
	}
	for i := range out.Buildings {
		keyboardOrder(out.Buildings[i].Groups)
	}
	return out
}

func hotkeyMapCutoff(r *http.Request) (int64, bool) {
	cutoffSec := int64(hotkeyMapDefaultMinute * 60)
	if raw := strings.TrimSpace(r.URL.Query().Get("minute")); raw != "" {
		minute, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || minute < 0 || minute > 180 {
			return 0, false
		}
		cutoffSec = minute * 60
	}
	// An exact-second cutoff overrides the minute: the frontend notches the slider at building-assign
	// moments, not at whole minutes.
	if raw := strings.TrimSpace(r.URL.Query().Get("second")); raw != "" {
		second, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || second < 0 || second > 180*60 {
			return 0, false
		}
		cutoffSec = second
	}
	return cutoffSec, true
}

func (d *Dashboard) handlerHotkeyMapData(w http.ResponseWriter, r *http.Request) {
	replayID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("replay_id")), 10, 64)
	if err != nil || replayID <= 0 {
		http.Error(w, "replay_id is required", http.StatusBadRequest)
		return
	}
	playerID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("player_id")), 10, 64)
	if err != nil || playerID <= 0 {
		http.Error(w, "player_id is required", http.StatusBadRequest)
		return
	}
	cutoffSec, ok := hotkeyMapCutoff(r)
	if !ok {
		http.Error(w, "minute or second out of range", http.StatusBadRequest)
		return
	}
	summary, err := d.dbStore.GetReplaySummary(r.Context(), replayID)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("hotkey map summary replay=%d: %v", replayID, err)
		http.Error(w, "failed to load replay", http.StatusInternalServerError)
		return
	}
	player, err := d.dbStore.GetReplayPlayerHotkeyStream(r.Context(), replayID, playerID)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("hotkey map player replay=%d player=%d: %v", replayID, playerID, err)
		http.Error(w, "failed to load player", http.StatusInternalServerError)
		return
	}
	events, err := hotkeystream.Decode(player.HotkeyStream)
	if err != nil || len(events) == 0 {
		http.Error(w, "no hotkey stream for player", http.StatusNotFound)
		return
	}
	buildings := hotkeyBuildingsAtCutoff(events, int32(cutoffSec))
	if len(buildings) == 0 {
		http.Error(w, "no located hotkeyed buildings at that moment", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(hotkeyMapData(buildings, summary.MapTileWidth, summary.MapTileHeight, cutoffSec))
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
