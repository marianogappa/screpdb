package dashboard

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"golang.org/x/sync/singleflight"
)

var gameAssetFlight singleflight.Group

func (d *Dashboard) gameAssetsCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "screpdb", "game-assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (d *Dashboard) writeGameAssetCacheFile(absPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return err
	}
	tmp := absPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, absPath)
}

func (d *Dashboard) handlerGameAssetUnit(w http.ResponseWriter, r *http.Request) {
	d.serveGameAssetIcon(w, r)
}

func (d *Dashboard) handlerGameAssetBuilding(w http.ResponseWriter, r *http.Request) {
	d.serveGameAssetIcon(w, r)
}

func (d *Dashboard) serveGameAssetIcon(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	cacheKey, scmapQuery, ok := resolveGameAssetIconQuery(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	cacheRoot, err := d.gameAssetsCacheDir()
	if err != nil {
		log.Printf("game asset icon cache dir: %v", err)
		http.Error(w, "cache unavailable", http.StatusInternalServerError)
		return
	}
	cachePath := filepath.Join(cacheRoot, "icons", cacheKey+".png")

	if data, readErr := os.ReadFile(cachePath); readErr == nil && len(data) > 0 {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(data)
		return
	}

	v, err, _ := gameAssetFlight.Do("icon:"+cacheKey, func() (any, error) {
		if data, readErr := os.ReadFile(cachePath); readErr == nil && len(data) > 0 {
			return data, nil
		}
		pngBytes, genErr := scmapanalyzer.UnitOrBuildingImagePNG(scmapQuery)
		if genErr != nil {
			return nil, genErr
		}
		if writeErr := d.writeGameAssetCacheFile(cachePath, pngBytes); writeErr != nil {
			return nil, writeErr
		}
		return pngBytes, nil
	})
	if err != nil {
		log.Printf("game asset icon %q: %v", cacheKey, err)
		http.NotFound(w, r)
		return
	}
	pngBytes := v.([]byte)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(pngBytes)
}

func (d *Dashboard) handlerGameAssetMap(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(r.URL.Query().Get("replay_id"))
	replayID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || replayID <= 0 {
		http.Error(w, "replay_id is required", http.StatusBadRequest)
		return
	}

	summary, err := d.dbStore.GetReplaySummary(r.Context(), replayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("game asset map summary: %v", err)
		http.Error(w, "failed to load replay", http.StatusInternalServerError)
		return
	}
	replayPath := strings.TrimSpace(summary.FilePath)
	if replayPath == "" {
		http.NotFound(w, r)
		return
	}

	cacheRoot, err := d.gameAssetsCacheDir()
	if err != nil {
		log.Printf("game asset map cache dir: %v", err)
		http.Error(w, "cache unavailable", http.StatusInternalServerError)
		return
	}
	cacheKey := scmapanalyzer.NormalizeMapKey(summary.MapName)
	if cacheKey == "" {
		cacheKey = "unknown-map"
	}
	cachePath := filepath.Join(cacheRoot, "maps", cacheKey+".png")

	if data, readErr := os.ReadFile(cachePath); readErr == nil && len(data) > 0 {
		w.Header().Set("Content-Type", "image/png")
		// Do not let browsers disk-cache: replay_id URL is stable while map bytes can change (reingest, file swap).
		w.Header().Set("Cache-Control", "no-store, no-cache, max-age=0, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		_, _ = w.Write(data)
		return
	}

	v, err, _ := gameAssetFlight.Do("map:"+cacheKey, func() (any, error) {
		if data, readErr := os.ReadFile(cachePath); readErr == nil && len(data) > 0 {
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
		log.Printf("game asset map replay_id=%d: %v", replayID, err)
		http.Error(w, "map render failed", http.StatusInternalServerError)
		return
	}
	pngBytes := v.([]byte)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store, no-cache, max-age=0, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(pngBytes)
}
