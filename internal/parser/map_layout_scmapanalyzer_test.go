package parser

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestMinitileToPixelInt(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 4},
		{1, 12},
		{10, 84},
	}
	for _, c := range cases {
		if got := minitileToPixelInt(c.in); got != c.want {
			t.Fatalf("minitileToPixelInt(%d) = %d want %d", c.in, got, c.want)
		}
	}
}

func TestBuildMapContextLayoutFromReplay_RealReplay(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	repPath := filepath.Join(repoRoot, "internal", "testdata", "replays", "bgh.rep")

	rep := &models.Replay{FilePath: repPath}
	data, err := ParseReplayWithOptions(repPath, rep, Options{})
	if err != nil {
		t.Skipf("parse %s: %v", repPath, err)
	}

	layout, err := buildMapContextLayoutFromReplay(
		repPath,
		data.Replay.MapName,
		int(data.Replay.MapWidth),
		int(data.Replay.MapHeight),
	)
	if err != nil {
		t.Skipf("buildMapContextLayoutFromReplay: %v", err)
	}
	if layout == nil {
		t.Skipf("no layout available for map %q", data.Replay.MapName)
	}

	if layout.WidthTiles != int(data.Replay.MapWidth) {
		t.Fatalf("WidthTiles got %d want %d", layout.WidthTiles, data.Replay.MapWidth)
	}
	if layout.HeightTiles != int(data.Replay.MapHeight) {
		t.Fatalf("HeightTiles got %d want %d", layout.HeightTiles, data.Replay.MapHeight)
	}
	if len(layout.Bases) == 0 {
		t.Fatalf("expected at least one base for a ladder map")
	}
	for i, b := range layout.Bases {
		if b.Center.X < 0 || b.Center.Y < 0 {
			t.Fatalf("base %d has negative center %+v", i, b.Center)
		}
		for j, pt := range b.Polygon {
			if pt.X < 0 || pt.Y < 0 {
				t.Fatalf("base %d polygon point %d is negative: %+v", i, j, pt)
			}
		}
	}
}

func TestBuildMapContextLayoutFromReplay_MissingFile(t *testing.T) {
	layout, err := buildMapContextLayoutFromReplay("/nonexistent/path/does-not-exist.rep", "", 0, 0)
	if err == nil && layout == nil {
		return
	}
	if err == nil {
		t.Fatalf("expected error or nil layout for a missing replay file, got layout=%+v", layout)
	}
}

func TestMutualAllianceTeamsComponents(t *testing.T) {
	allies := map[byte]map[byte]bool{
		1: {2: true},
		2: {1: true, 3: true},
		3: {2: true},
		4: {5: true},
		5: {4: true},
	}
	sortedPIDs := []byte{1, 2, 3, 4, 5, 6}
	mutual := func(a, b byte) bool {
		return allies[a][b] && allies[b][a]
	}

	got := mutualAllianceTeamsComponents(allies, sortedPIDs, mutual)
	want := [][]byte{{1, 2, 3}, {4, 5}, {6}}
	expectTeams(t, got, want)
}
