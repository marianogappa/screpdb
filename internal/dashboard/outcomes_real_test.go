package dashboard

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/digestcache"
	"github.com/marianogappa/screpdb/internal/enginepass"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/compact"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/parser"
)

// Checks the parser's and the engine's results against Battle.net's, which each player's own profile
// records. OUTCOME_LIST names a file with one replay per line; DIGESTS is a copy of engine_digests.bin
// (opening it may rewrite it), SCREPDB_ROOT the app-data folder holding the game archive.
func TestRealOutcomesAgainstBattleNet(t *testing.T) {
	list := os.Getenv("OUTCOME_LIST")
	if list == "" || os.Getenv("OPENBW_DATA") == "" || os.Getenv("DIGESTS") == "" || os.Getenv("SCREPDB_ROOT") == "" {
		t.Skip("OUTCOME_LIST, OPENBW_DATA, DIGESTS and SCREPDB_ROOT are not set")
	}
	data, err := engine.LoadData(os.Getenv("OPENBW_DATA"))
	if err != nil {
		t.Fatal(err)
	}
	var dataHash [32]byte
	if _, err := hex.Decode(dataHash[:], []byte(data.FilesHash())); err != nil {
		t.Fatal(err)
	}
	cache, err := digestcache.Open(os.Getenv("DIGESTS"), func(digestcache.Key) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	archive := persist.NewBnetGameArchive(os.Getenv("SCREPDB_ROOT"))
	if err := archive.Load(); err != nil {
		t.Fatal(err)
	}
	index := indexBnetGames(archive.GamesWithResults())
	f, err := os.Open(list)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	type tally struct{ agree, disagree, unknown int }
	var parsed, engined tally
	count := func(t *tally, got library.Player, want library.BnetResult) {
		switch {
		case !got.PlayerOutcome().Known():
			t.unknown++
		case got.PlayerOutcome() == want.Outcome:
			t.agree++
		default:
			t.disagree++
		}
	}
	games, matched, settled := 0, 0, 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		path := s.Text()
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsedData, err := parser.ParseReplayWithOptions(path, parser.CreateReplayFromFileInfo(path, filepath.Base(path), int64(len(b)), ""), parser.Options{})
		if err != nil {
			continue
		}
		rec, err := compact.FromReplayData(parsedData, compact.FileMeta{Path: path, Size: int64(len(b)), Checksum: sha256.Sum256(b), ModTime: time.Now()})
		if err != nil {
			continue
		}
		entry, found, err := cache.Get(digestcache.Key{Checksum: rec.Checksum, EngineVersion: engine.EngineVersion, DataHash: dataHash})
		if !found || err != nil || entry.Failed != "" {
			continue
		}
		d, err := engine.Decode(entry.Digest)
		if err != nil {
			continue
		}
		games++
		g, ok := bnetGameFor(rec, index)
		if !ok {
			continue
		}
		matched++
		lib := library.New(library.Options{})
		lib.Add(0, rec)
		lib.SetEngineData(rec.Checksum, enginepass.Summarize(d, false))
		lib.Flush()
		got, _ := lib.Snapshot().ByChecksum(rec.Checksum)
		if got.WinnerUndecided() {
			lib.SetBnetResults(rec.Checksum, bnetResultsOf(g), "")
			lib.Flush()
			if after, _ := lib.Snapshot().ByChecksum(rec.Checksum); !after.WinnerUndecided() {
				settled++
			}
		}
		lib.Close()
		for _, want := range bnetResultsOf(g) {
			for i := range rec.Players {
				if rec.Players[i].Key != want.Key {
					continue
				}
				count(&parsed, rec.Players[i], want)
				count(&engined, got.Players[i], want)
				if o := got.Players[i].PlayerOutcome(); o.Known() && o != want.Outcome {
					fmt.Printf("engine disagrees: %s %s: Battle.net %v, engine %v (%s)\n", filepath.Base(path), rec.Players[i].Name, want.Outcome, o, got.Players[i].PlayerOutcomeReason)
				}
			}
		}
	}
	fmt.Printf("%d engine games, %d on Battle.net, %d settled only by Battle.net\n", games, matched, settled)
	fmt.Printf("parser: %+v\nengine: %+v\n", parsed, engined)
	if engined.disagree > 0 {
		t.Errorf("the engine disagrees with Battle.net on %d results", engined.disagree)
	}
}
