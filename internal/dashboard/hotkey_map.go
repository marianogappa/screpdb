package dashboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/iofacade"
	xdraw "golang.org/x/image/draw"
)

// The hotkey map composite paints the crop of the map that holds a player's
// hotkeyed buildings at a chosen minute: terrain from scmapanalyzer, building
// sprites at true footprint scale, the hotkey number badged on top. Group
// contents come entirely from the stored hotkey_stream blob — building assigns
// carry the building type and its build tile.

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

// hotkeySpriteRace prefixes building names for scmapanalyzer's HD sprite
// registry ("Zerg Hatchery", "Terran Comsat Station", ...).
var hotkeySpriteRace = map[string]string{
	"Hatchery": "Zerg", "Lair": "Zerg", "Hive": "Zerg", "Spawning Pool": "Zerg",
	"Evolution Chamber": "Zerg", "Hydralisk Den": "Zerg", "Spire": "Zerg",
	"Greater Spire": "Zerg", "Queen's Nest": "Zerg", "Defiler Mound": "Zerg",
	"Ultralisk Cavern": "Zerg", "Extractor": "Zerg",
	"Command Center": "Terran", "Barracks": "Terran", "Factory": "Terran",
	"Starport": "Terran", "Science Facility": "Terran", "Engineering Bay": "Terran",
	"Academy": "Terran", "Armory": "Terran", "Comsat Station": "Terran",
	"Machine Shop": "Terran", "Control Tower": "Terran", "Nuclear Silo": "Terran",
	"Nexus": "Protoss", "Gateway": "Protoss", "Forge": "Protoss",
	"Cybernetics Core": "Protoss", "Citadel of Adun": "Protoss",
	"Robotics Facility": "Protoss", "Stargate": "Protoss",
	"Templar Archives": "Protoss", "Observatory": "Protoss",
	"Arbiter Tribunal": "Protoss", "Fleet Beacon": "Protoss",
	"Robotics Support Bay": "Protoss",
}

// Overlay chrome matches the dashboard's accent and keycap styling.
var (
	hotkeyOutlineColor    = color.RGBA{0xe0, 0xb8, 0x4c, 0xff}
	hotkeyChipFillColor   = color.RGBA{0x18, 0x1e, 0x25, 0xff}
	hotkeyChipBorderColor = color.RGBA{0x6c, 0x78, 0x84, 0xff}
)

type hotkeyMapBuilding struct {
	group    int
	building string
	tileX    int
	tileY    int
}

func (d *Dashboard) handlerHotkeyMap(w http.ResponseWriter, r *http.Request) {
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
	cutoffSec := int64(hotkeyMapDefaultMinute * 60)
	if raw := strings.TrimSpace(r.URL.Query().Get("minute")); raw != "" {
		minute, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || minute < 0 || minute > 180 {
			http.Error(w, "minute out of range", http.StatusBadRequest)
			return
		}
		cutoffSec = minute * 60
	}
	// An exact-second cutoff overrides the minute: the frontend notches the
	// slider at building-assign moments, not at whole minutes.
	if raw := strings.TrimSpace(r.URL.Query().Get("second")); raw != "" {
		second, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || second < 0 || second > 180*60 {
			http.Error(w, "second out of range", http.StatusBadRequest)
			return
		}
		cutoffSec = second
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

	terrain, err := d.mapTerrainPNG(strings.TrimSpace(summary.FilePath), summary.MapName)
	if err != nil {
		log.Printf("hotkey map terrain replay=%d: %v", replayID, err)
		http.Error(w, "map render failed", http.StatusInternalServerError)
		return
	}
	composite, err := renderHotkeyMapComposite(terrain, buildings)
	if err != nil {
		log.Printf("hotkey map composite replay=%d player=%d: %v", replayID, playerID, err)
		http.Error(w, "map render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store, no-cache, max-age=0, must-revalidate")
	_, _ = w.Write(composite)
}

// hotkeyBuildingsAtCutoff reduces a decoded stream to the located buildings
// each group held at the cutoff second: the last assign at or before it wins.
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
	return out
}

// mapTerrainPNG returns the full-map terrain render, sharing the on-disk cache
// and singleflight key with handlerGameAssetMap.
func (d *Dashboard) mapTerrainPNG(replayPath, mapName string) ([]byte, error) {
	if replayPath == "" {
		return nil, errors.New("replay file path unknown")
	}
	cacheRoot, err := d.gameAssetsCacheDir()
	if err != nil {
		return nil, err
	}
	cacheKey := scmapanalyzer.NormalizeMapKey(mapName)
	if cacheKey == "" {
		cacheKey = "unknown-map"
	}
	cachePath := filepath.Join(cacheRoot, "maps", cacheKey+".png")
	if data, readErr := iofacade.ReadFile(cachePath); readErr == nil && len(data) > 0 {
		return data, nil
	}
	v, err, _ := gameAssetFlight.Do("map:"+cacheKey, func() (any, error) {
		if data, readErr := iofacade.ReadFile(cachePath); readErr == nil && len(data) > 0 {
			return data, nil
		}
		pngBytes, genErr := scmapanalyzer.MapImagePNGFromReplayFile(replayPath)
		if genErr != nil {
			return nil, genErr
		}
		if writeErr := d.writeGameAssetCacheFile(cachePath, pngBytes); writeErr != nil {
			return nil, writeErr
		}
		return pngBytes, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// renderHotkeyMapComposite overlays building sprites, footprint outlines and
// group badges on the terrain, cropped to the buildings' bounding box.
func renderHotkeyMapComposite(terrainPNG []byte, buildings []hotkeyMapBuilding) ([]byte, error) {
	terrain, err := png.Decode(bytes.NewReader(terrainPNG))
	if err != nil {
		return nil, fmt.Errorf("decode terrain: %w", err)
	}
	canvas := image.NewRGBA(terrain.Bounds())
	draw.Draw(canvas, canvas.Bounds(), terrain, image.Point{}, draw.Src)

	minTX, minTY, maxTX, maxTY := 1<<30, 1<<30, 0, 0
	spriteTop := map[[2]int]int{}
	for _, b := range buildings {
		fp, ok := hotkeyBuildingFootprints[b.building]
		if !ok {
			fp = [2]int{3, 2}
		}
		px, py := b.tileX*32, b.tileY*32
		w, h := fp[0]*32, fp[1]*32
		top := py
		if spriteName := hotkeySpriteName(b.building); spriteName != "" {
			if spritePNG, err := scmapanalyzer.UnitOrBuildingImagePNG(spriteName); err == nil {
				if sprite, err := png.Decode(bytes.NewReader(spritePNG)); err == nil {
					// Preserve the sprite's aspect ratio: fit its width to the
					// footprint and let it stand up to twice the footprint's
					// height, anchored bottom center (stretching to the
					// footprint box made near-square sprites look fat).
					sb := sprite.Bounds()
					scale := float64(w) / float64(sb.Dx())
					if maxH := float64(2 * h); float64(sb.Dy())*scale > maxH {
						scale = maxH / float64(sb.Dy())
					}
					drawnW, drawnH := int(float64(sb.Dx())*scale), int(float64(sb.Dy())*scale)
					x0, y0 := px+(w-drawnW)/2, py+h-drawnH
					xdraw.CatmullRom.Scale(canvas, image.Rect(x0, y0, x0+drawnW, y0+drawnH), sprite, sb, xdraw.Over, nil)
					top = min(top, y0)
				}
			}
		}
		key := [2]int{b.tileX, b.tileY}
		if cur, ok := spriteTop[key]; !ok || top < cur {
			spriteTop[key] = top
		}
		drawFootprintOutline(canvas, px, py, w, h, hotkeyOutlineColor)
		minTX, minTY = min(minTX, b.tileX), min(minTY, b.tileY)
		maxTX, maxTY = max(maxTX, b.tileX+fp[0]), max(maxTY, b.tileY+fp[1])
	}
	// Keycap chips last so they sit on top: one small uniform chip per group,
	// in a row above the sprite so they never cover it.
	byTile := map[[2]int][]int{}
	for _, b := range buildings {
		k := [2]int{b.tileX, b.tileY}
		byTile[k] = append(byTile[k], b.group)
	}
	for tile, groups := range byTile {
		keyboardOrder(groups)
		fpw := 3 * 32
		for _, b := range buildings {
			if b.tileX == tile[0] && b.tileY == tile[1] {
				if fp, ok := hotkeyBuildingFootprints[b.building]; ok {
					fpw = fp[0] * 32
				}
				break
			}
		}
		drawGroupChips(canvas, tile[0]*32+fpw/2, spriteTop[tile]-6, groups)
	}

	const marginTiles = 6
	bounds := canvas.Bounds()
	crop := image.Rect(
		clampInt((minTX-marginTiles)*32, 0, bounds.Max.X),
		clampInt((minTY-marginTiles)*32, 0, bounds.Max.Y),
		clampInt((maxTX+marginTiles)*32, 0, bounds.Max.X),
		clampInt((maxTY+marginTiles)*32, 0, bounds.Max.Y),
	)
	cropped := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(cropped, cropped.Bounds(), canvas, crop.Min, draw.Src)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, cropped); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hotkeySpriteName(building string) string {
	race, ok := hotkeySpriteRace[building]
	if !ok {
		return ""
	}
	return race + " " + building
}

// keyboardOrder sorts hotkey numbers with 0 after 9, matching the keyboard row.
func keyboardOrder(groups []int) {
	order := func(g int) int {
		if g == 0 {
			return 10
		}
		return g
	}
	for i := 1; i < len(groups); i++ {
		for j := i; j > 0 && order(groups[j]) < order(groups[j-1]); j-- {
			groups[j], groups[j-1] = groups[j-1], groups[j]
		}
	}
}

func drawFootprintOutline(img *image.RGBA, x, y, w, h int, col color.RGBA) {
	for t := 0; t < 3; t++ {
		for i := x - t; i <= x+w+t; i++ {
			setPixel(img, i, y-t, col)
			setPixel(img, i, y+h+t, col)
		}
		for j := y - t; j <= y+h+t; j++ {
			setPixel(img, x-t, j, col)
			setPixel(img, x+w+t, j, col)
		}
	}
}

func setPixel(img *image.RGBA, x, y int, col color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, col)
	}
}

// hotkeyDigitFont is a 3x5 bitmap font for the group chips.
var hotkeyDigitFont = map[rune][5][3]byte{
	'0': {{1, 1, 1}, {1, 0, 1}, {1, 0, 1}, {1, 0, 1}, {1, 1, 1}},
	'1': {{0, 1, 0}, {1, 1, 0}, {0, 1, 0}, {0, 1, 0}, {1, 1, 1}},
	'2': {{1, 1, 1}, {0, 0, 1}, {1, 1, 1}, {1, 0, 0}, {1, 1, 1}},
	'3': {{1, 1, 1}, {0, 0, 1}, {0, 1, 1}, {0, 0, 1}, {1, 1, 1}},
	'4': {{1, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 0, 1}, {0, 0, 1}},
	'5': {{1, 1, 1}, {1, 0, 0}, {1, 1, 1}, {0, 0, 1}, {1, 1, 1}},
	'6': {{1, 1, 1}, {1, 0, 0}, {1, 1, 1}, {1, 0, 1}, {1, 1, 1}},
	'7': {{1, 1, 1}, {0, 0, 1}, {0, 1, 0}, {0, 1, 0}, {0, 1, 0}},
	'8': {{1, 1, 1}, {1, 0, 1}, {1, 1, 1}, {1, 0, 1}, {1, 1, 1}},
	'9': {{1, 1, 1}, {1, 0, 1}, {1, 1, 1}, {0, 0, 1}, {1, 1, 1}},
}

// drawGroupChips paints one small keycap-style chip per hotkey group,
// centered on (cx, bottomY): a dark rounded square with a white digit.
func drawGroupChips(img *image.RGBA, cx, bottomY int, groups []int) {
	const (
		chipSize = 26
		chipGap  = 4
		scale    = 4
	)
	rowW := len(groups)*chipSize + (len(groups)-1)*chipGap
	x0 := cx - rowW/2
	y0 := max(bottomY-chipSize, 2)
	white := color.RGBA{255, 255, 255, 255}
	for i, g := range groups {
		x := x0 + i*(chipSize+chipGap)
		for dy := 0; dy < chipSize; dy++ {
			for dx := 0; dx < chipSize; dx++ {
				corner := (dx == 0 || dx == chipSize-1) && (dy == 0 || dy == chipSize-1)
				if corner {
					continue
				}
				c := hotkeyChipFillColor
				if dx == 0 || dy == 0 || dx == chipSize-1 || dy == chipSize-1 {
					c = hotkeyChipBorderColor
				}
				setPixel(img, x+dx, y0+dy, c)
			}
		}
		glyph, ok := hotkeyDigitFont[rune('0'+g)]
		if !ok {
			continue
		}
		gx := x + (chipSize-3*scale)/2
		gy := y0 + (chipSize-5*scale)/2
		for row := 0; row < 5; row++ {
			for colIdx := 0; colIdx < 3; colIdx++ {
				if glyph[row][colIdx] == 0 {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						setPixel(img, gx+colIdx*scale+dx, gy+row*scale+dy, white)
					}
				}
			}
		}
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
