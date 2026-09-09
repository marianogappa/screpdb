//go:build !js

package dashboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestWriteGameAssetCacheFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SCREPDB_APPDATA_DIR", root)
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)

	d := &Dashboard{}
	path := filepath.Join(root, "game-assets", "maps", "v"+gameAssetMapRenderVersion, "fighting-spirit.jpg")
	if err := d.writeGameAssetCacheFile(path, []byte("jpeg-bytes")); err != nil {
		t.Fatalf("writeGameAssetCacheFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached file: %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Fatalf("cached data: got %q", string(data))
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind, stat err=%v", err)
	}
}
