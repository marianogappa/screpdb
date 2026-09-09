//go:build !js

package dashboard

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

// gameAssetMapCacheBudgetBytes caps the current map cache dir after pruning:
// roughly 80 maps at ~1.2 MB each. Oldest files by mtime go first — for a
// cache this cheap to regenerate, insertion order is a good enough proxy for
// value, and atime is unreliable across platforms and mount options.
const gameAssetMapCacheBudgetBytes int64 = 100 << 20

// PruneGameAssetCache reclaims disk from the game-asset cache at startup:
// every entry under maps/ and icons/ that is not the current render-version
// dir is deleted (which also clears the pre-versioning lossless map PNGs),
// then the current map dir is capped to a size budget. Failures are logged
// and ignored — a cold cache just re-renders on demand.
func PruneGameAssetCache() {
	root, err := appdata.Path("game-assets")
	if err != nil {
		log.Printf("game asset cache prune: %v", err)
		return
	}
	pruneStaleCacheVersions(filepath.Join(root, "maps"), "v"+gameAssetMapRenderVersion)
	pruneStaleCacheVersions(filepath.Join(root, "icons"), gameAssetIconRenderVersion)
	capCacheDirSize(filepath.Join(root, "maps", "v"+gameAssetMapRenderVersion), gameAssetMapCacheBudgetBytes)
}

func pruneStaleCacheVersions(dir, keep string) {
	entries, err := iofacade.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == keep {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := iofacade.RemoveAll(path); err != nil {
			log.Printf("game asset cache prune %s: %v", path, err)
		}
	}
}

func capCacheDirSize(dir string, budget int64) {
	type cacheFile struct {
		path  string
		size  int64
		mtime time.Time
	}
	var files []cacheFile
	var total int64
	if err := iofacade.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, cacheFile{path: path, size: info.Size(), mtime: info.ModTime()})
		total += info.Size()
		return nil
	}); err != nil || total <= budget {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.Before(files[j].mtime) })
	for _, f := range files {
		if total <= budget {
			return
		}
		if err := iofacade.Remove(f.path); err != nil {
			log.Printf("game asset cache cap %s: %v", f.path, err)
			continue
		}
		total -= f.size
	}
}
