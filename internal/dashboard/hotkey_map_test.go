package dashboard

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

func testTerrainPNG(t *testing.T, widthTiles, heightTiles int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, widthTiles*32, heightTiles*32))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, color.RGBA{40, 44, 48, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode terrain: %v", err)
	}
	return buf.Bytes()
}

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
	want := scmapanalyzer.TileRect{X: 4, Y: 4, W: 34, H: 16}
	if rect != want {
		t.Fatalf("rect = %+v, want %+v", rect, want)
	}
	// Buildings near the edge clamp to the map bounds.
	edge := []hotkeyMapBuilding{{group: 1, building: "Hatchery", tileX: 1, tileY: 62}}
	rect = hotkeyMapTileRect(edge, 64, 64)
	want = scmapanalyzer.TileRect{X: 0, Y: 56, W: 11, H: 8}
	if rect != want {
		t.Fatalf("edge rect = %+v, want %+v", rect, want)
	}
}

func TestRenderHotkeyMapComposite(t *testing.T) {
	buildings := []hotkeyMapBuilding{
		{group: 5, building: "Hatchery", tileX: 10, tileY: 10},
		{group: 2, building: "Comsat Station", tileX: 30, tileY: 12},
		// Two groups on one tile share a badge; unknown footprint falls back.
		{group: 8, building: "Hatchery", tileX: 10, tileY: 10},
		{group: 0, building: "Not A Real Building", tileX: 40, tileY: 40},
	}
	rect := hotkeyMapTileRect(buildings, 64, 64)
	terrain := testTerrainPNG(t, rect.W, rect.H)
	out, err := renderHotkeyMapComposite(terrain, rect, buildings)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode composite: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != rect.W*32 || b.Dy() != rect.H*32 {
		t.Fatalf("composite bounds %v, want %dx%d", b, rect.W*32, rect.H*32)
	}
	// The footprint outline for tile (10,10) paints over terrain at the
	// building's rect-relative corner.
	corner := img.At((10-rect.X)*32, (10-rect.Y)*32)
	r, g, bl, _ := corner.RGBA()
	if r == 40<<8 && g == 44<<8 && bl == 48<<8 {
		t.Fatal("overlay not painted over terrain")
	}

	if _, err := renderHotkeyMapComposite([]byte("not a png"), rect, buildings); err == nil {
		t.Fatal("expected error for invalid terrain")
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

func TestHotkeySpriteName(t *testing.T) {
	if got := hotkeySpriteName("Hatchery"); got != "Zerg Hatchery" {
		t.Fatalf("hotkeySpriteName(Hatchery) = %q", got)
	}
	if got := hotkeySpriteName("Comsat Station"); got != "Terran Comsat Station" {
		t.Fatalf("hotkeySpriteName(Comsat Station) = %q", got)
	}
	if got := hotkeySpriteName("Unknown"); got != "" {
		t.Fatalf("hotkeySpriteName(Unknown) = %q", got)
	}
}

// TestHotkeyMapEndToEnd drives the PNG endpoint against real ingested replays:
// find a game and player with located buildings, render, and assert PNG bytes.
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
			path := "/api/custom/hotkeys/map?replay_id=" + itoa64(replayID) + "&player_id=" + itoa64(playerID) + "&minute=8"
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			switch rec.Code {
			case http.StatusOK:
				body := rec.Body.Bytes()
				if len(body) < 24 || string(body[1:4]) != "PNG" {
					t.Fatalf("expected PNG, got %q", body[:16])
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
