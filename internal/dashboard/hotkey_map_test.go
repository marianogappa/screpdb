package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

func TestHotkeyBuildingsAtCutoff(t *testing.T) {
	events := []hotkeystream.Event{
		{Sec: 10, Type: hotkeystream.TypeAssignBuilding, Group: 5, Building: hotkeystream.BuildingID("Hatchery"), TileX: 10, TileY: 10},
		{Sec: 20, Type: hotkeystream.TypeSelect, Group: 5},
		{Sec: 100, Type: hotkeystream.TypeAssignUnits, Group: 1, Count: 8},
		// Group 5 is repurposed to units at minute 9: gone from the minute-10 map.
		{Sec: 540, Type: hotkeystream.TypeAssignBuilding, Group: 6, Building: hotkeystream.BuildingID("Comsat Station"), TileX: 30, TileY: 12},
		{Sec: 550, Type: hotkeystream.TypeAssignUnits, Group: 5, Count: 12},
		// Unknown tile and unknown building never render.
		{Sec: 560, Type: hotkeystream.TypeAssignBuilding, Group: 7, Building: hotkeystream.BuildingID("Nexus"), TileX: hotkeystream.TileUnknown, TileY: hotkeystream.TileUnknown},
		{Sec: 570, Type: hotkeystream.TypeAssignBuilding, Group: 8, Building: 0, TileX: 5, TileY: 5},
		// Beyond the cutoff: ignored.
		{Sec: 700, Type: hotkeystream.TypeAssignBuilding, Group: 9, Building: hotkeystream.BuildingID("Barracks"), TileX: 50, TileY: 50},
	}
	atMin5 := hotkeyBuildingsAtCutoff(events, 5*60)
	if len(atMin5) != 1 || atMin5[0].group != 5 || atMin5[0].building != "Hatchery" {
		t.Fatalf("minute 5: %+v", atMin5)
	}
	atMin10 := hotkeyBuildingsAtCutoff(events, 10*60)
	if len(atMin10) != 1 || atMin10[0].group != 6 || atMin10[0].building != "Comsat Station" {
		t.Fatalf("minute 10: %+v", atMin10)
	}
	if got := hotkeyBuildingsAtCutoff(events, 0); len(got) != 0 {
		t.Fatalf("minute 0 must have no buildings, got %+v", got)
	}
}

func TestHotkeyMapTileRect(t *testing.T) {
	buildings := []hotkeyMapBuilding{
		{group: 5, building: "Hatchery", tileX: 10, tileY: 10},
		{group: 2, building: "Comsat Station", tileX: 30, tileY: 12},
	}
	rect := hotkeyMapTileRect(buildings, 64, 64)
	want := tileRect{X: 4, Y: 4, W: 34, H: 16}
	if rect != want {
		t.Fatalf("rect = %+v, want %+v", rect, want)
	}
	// Buildings near the edge clamp to the map bounds.
	edge := []hotkeyMapBuilding{{group: 1, building: "Hatchery", tileX: 1, tileY: 62}}
	rect = hotkeyMapTileRect(edge, 64, 64)
	want = tileRect{X: 0, Y: 56, W: 11, H: 8}
	if rect != want {
		t.Fatalf("edge rect = %+v, want %+v", rect, want)
	}
}

func TestHotkeyMapData(t *testing.T) {
	buildings := []hotkeyMapBuilding{
		{group: 5, building: "Hatchery", tileX: 10, tileY: 10},
		{group: 2, building: "Comsat Station", tileX: 30, tileY: 12},
		{group: 0, building: "Hatchery", tileX: 10, tileY: 10},
		{group: 8, building: "Not A Real Building", tileX: 40, tileY: 40},
	}
	got := hotkeyMapData(buildings, 64, 64, 480)
	if got.Crop != (hotkeyMapRect{X: 4 * 32, Y: 4 * 32, W: 45 * 32, H: 44 * 32}) {
		t.Fatalf("crop = %+v", got.Crop)
	}
	if len(got.Buildings) != 3 {
		t.Fatalf("two groups on one Hatchery must share it: %+v", got.Buildings)
	}
	hatch := got.Buildings[0]
	if hatch.Building != "Hatchery" || hatch.Footprint != (hotkeyMapRect{X: 320, Y: 320, W: 128, H: 96}) || len(hatch.Groups) != 2 || hatch.Groups[0] != 5 || hatch.Groups[1] != 0 {
		t.Fatalf("hatchery = %+v", hatch)
	}
	if fp := got.Buildings[2].Footprint; fp.W != 96 || fp.H != 64 {
		t.Fatalf("unknown buildings fall back to a 3x2 footprint: %+v", fp)
	}
}

func TestKeyboardOrderSort(t *testing.T) {
	groups := []int{0, 9, 1, 5}
	keyboardOrder(groups)
	want := []int{1, 5, 9, 0}
	for i := range want {
		if groups[i] != want[i] {
			t.Fatalf("keyboardOrder = %v, want %v", groups, want)
		}
	}
}

// TestHotkeyMapEndToEnd drives the map data endpoint against real ingested replays, then renders the
// terrain under the first crop it gets.
func TestHotkeyMapEndToEnd(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	games := listTestGames(t, r)
	if len(games) == 0 {
		t.Fatal("no games ingested")
	}
	rendered := false
	for _, replayID := range games {
		for _, playerID := range listTestGamePlayerIDs(t, r, replayID) {
			path := "/api/custom/hotkeys/map-data?replay_id=" + itoa64(replayID) + "&player_id=" + itoa64(playerID) + "&minute=8"
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			switch rec.Code {
			case http.StatusOK:
				var data hotkeyMapWire
				if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil || len(data.Buildings) == 0 || data.Crop.W == 0 {
					t.Fatalf("map data %s: %v", rec.Body.String(), err)
				}
				if !rendered {
					crop := "/api/custom/game-assets/map-crop?replay_id=" + itoa64(replayID) + "&x=" + strconv.Itoa(data.Crop.X/32) + "&y=" + strconv.Itoa(data.Crop.Y/32) + "&w=" + strconv.Itoa(data.Crop.W/32) + "&h=" + strconv.Itoa(data.Crop.H/32)
					rec := httptest.NewRecorder()
					r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, crop, nil))
					if body := rec.Body.Bytes(); rec.Code != http.StatusOK || len(body) < 24 || string(body[1:4]) != "PNG" {
						t.Fatalf("%s: status %d", crop, rec.Code)
					}
				}
				rendered = true
			case http.StatusNotFound:
				// Player without located hotkeyed buildings at minute 8: fine.
			default:
				t.Fatalf("%s: unexpected status %d: %s", path, rec.Code, rec.Body.String())
			}
		}
	}
	if !rendered {
		t.Fatal("no player produced a hotkey map across the corpus")
	}
}

func listTestGames(t *testing.T, r http.Handler) []int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/games", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("games list: %d %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []struct {
			ReplayID int64 `json:"replay_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("games json: %v", err)
	}
	ids := make([]int64, 0, len(payload.Items))
	for _, it := range payload.Items {
		ids = append(ids, it.ReplayID)
	}
	return ids
}

func listTestGamePlayerIDs(t *testing.T, r http.Handler, replayID int64) []int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/games/"+itoa64(replayID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("game detail %d: %d %s", replayID, rec.Code, rec.Body.String())
	}
	var payload struct {
		Players []struct {
			PlayerID int64 `json:"player_id"`
		} `json:"players"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("game json: %v", err)
	}
	ids := make([]int64, 0, len(payload.Players))
	for _, p := range payload.Players {
		ids = append(ids, p.PlayerID)
	}
	return ids
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}
