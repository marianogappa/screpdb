package botdetect

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icza/screp/rep"
	"github.com/marianogappa/screpdb/internal/screp"
)

func parse(t *testing.T, path string) *rep.Replay {
	t.Helper()
	r, err := screp.ParseFile(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return r
}

func playerID(t *testing.T, r *rep.Replay, name string) byte {
	t.Helper()
	for _, p := range r.Header.Players {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("no player %q", name)
	return 0
}

// The fixtures are cwal.gg ladder games whose bot side cwal flags as a bot.
func TestDetectFlagsBots(t *testing.T) {
	cases := []struct {
		file, player string
		want         Signal
	}{
		{"all_signals_ComfyUI.rep", "ComfyUI", SignalSameFrame | SignalSelectOrderBursts | SignalFarJumps | SignalSustainedAPM},
		{"no_bursts_bcmzcvnmzxczmcv.rep", "bcmzcvnmzxczmcv", SignalSameFrame | SignalFarJumps},
		{"bursts_only_karinaangang.rep", "karinaangang", SignalSelectOrderBursts},
	}
	for _, c := range cases {
		t.Run(c.player, func(t *testing.T) {
			r := parse(t, filepath.Join("testdata", "bots", c.file))
			got := Detect(r.Commands.Cmds, int32(r.Header.Frames))
			if s := got[playerID(t, r, c.player)]; s != c.want {
				t.Fatalf("signals = %04b, want %04b", s, c.want)
			}
		})
	}
}

func TestDetectFlagsNoHumanFixture(t *testing.T) {
	roots := []string{
		"../testdata/replays",
		"../patterns/markers/testdata/replays",
		"../patterns/worldstate/testdata/replays",
		"../builddedup/testdata/replays",
	}
	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".rep") {
				return err
			}
			r, perr := screp.ParseFile(path)
			if perr != nil || r.Commands == nil {
				return nil
			}
			checked++
			for id, s := range Detect(r.Commands.Cmds, int32(r.Header.Frames)) {
				t.Errorf("%s: player %d flagged as bot (%04b)", path, id, s)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 200 {
		t.Fatalf("checked only %d human replays", checked)
	}
}

func TestDetectSkipsShortGames(t *testing.T) {
	r := parse(t, filepath.Join("testdata", "bots", "all_signals_ComfyUI.rep"))
	if got := Detect(r.Commands.Cmds, minGameFrames-1); got != nil {
		t.Fatalf("got %v for a game under four minutes", got)
	}
}

func TestDetectNeedsEnoughCommands(t *testing.T) {
	r := parse(t, filepath.Join("testdata", "bots", "all_signals_ComfyUI.rep"))
	if got := Detect(r.Commands.Cmds[:minCommands/2], int32(r.Header.Frames)); len(got) != 0 {
		t.Fatalf("got %v from %d commands", got, minCommands/2)
	}
}

func TestSignalHas(t *testing.T) {
	s := SignalSameFrame | SignalFarJumps
	if !s.Has(SignalFarJumps) || s.Has(SignalSustainedAPM) {
		t.Fatalf("Has misreports %04b", s)
	}
}
