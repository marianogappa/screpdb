//go:build !js

package dashboard

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

// handlerMapTerrainCrop serves the terrain of a rectangle of a game's map (in tiles) at native
// resolution, 32 pixels per tile. The full-map image is downsampled, which is fine for the whole map
// and blurry for a close view; GameMap draws this over it for close views. Terrain only: everything
// on top is drawn by the browser.
func (d *Dashboard) handlerMapTerrainCrop(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	replayID, err := strconv.ParseInt(strings.TrimSpace(q.Get("replay_id")), 10, 64)
	if err != nil || replayID <= 0 {
		http.Error(w, "replay_id is required", http.StatusBadRequest)
		return
	}
	var rect [4]int
	for i, name := range []string{"x", "y", "w", "h"} {
		v, err := strconv.Atoi(strings.TrimSpace(q.Get(name)))
		if err != nil || v < 0 {
			http.Error(w, name+" is required", http.StatusBadRequest)
			return
		}
		rect[i] = v
	}
	summary, err := d.dbStore.GetReplaySummary(r.Context(), replayID)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to load replay", http.StatusInternalServerError)
		return
	}
	if rect[2] == 0 || rect[3] == 0 || rect[0]+rect[2] > summary.MapTileWidth || rect[1]+rect[3] > summary.MapTileHeight {
		http.Error(w, "rectangle outside the map", http.StatusBadRequest)
		return
	}
	path := strings.TrimSpace(summary.FilePath)
	if path == "" {
		http.Error(w, "replay file path unknown", http.StatusNotFound)
		return
	}
	// layer=terrain is the floor alone and layer=resources the minerals and geysers alone, so creep can go between them.
	crop := scmapanalyzer.MapImagePNGCropFromReplayFile
	switch q.Get("layer") {
	case "terrain":
		crop = scmapanalyzer.MapTerrainPNGCropFromReplayFile
	case "resources":
		crop = scmapanalyzer.MapResourcesPNGCropFromReplayFile
	}
	terrain, err := crop(path, scmapanalyzer.TileRect{X: rect[0], Y: rect[1], W: rect[2], H: rect[3]})
	if err != nil {
		log.Printf("map terrain crop replay=%d: %v", replayID, err)
		http.Error(w, "map render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(terrain)
}
