package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSnapshotsRoundTrip(t *testing.T) {
	want := map[int]Snapshot{
		60:  {Second: 60, Units: []SnapshotUnit{{Type: 7, Owner: 0, X: 100, Y: 200, Completed: true}, {Type: 106, Owner: 0, X: 300, Y: 300, Completed: true}}},
		300: {Second: 300, Units: []SnapshotUnit{{Type: 103, Owner: 1, X: 900, Y: 950, Completed: true, Burrowed: true}, {Type: 61, Owner: 1, X: 10, Y: 10, Completed: true, Cloaked: true}}},
	}
	b, err := EncodeSnapshots(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSnapshots(b)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: %v\n got %+v\nwant %+v", err, got, want)
	}
	if _, err := DecodeSnapshots(b[:len(b)/2]); err == nil {
		t.Fatal("decoded a truncated record")
	}
}

// SomaJyJ.rep is a ZvT: at 6:00 both players have workers on minerals and the Zerg a Hatchery.
func TestSnapshotsOfARealGame(t *testing.T) {
	if os.Getenv("OPENBW_DATA") == "" {
		t.Skip("OPENBW_DATA is not set")
	}
	data, err := testData()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "testdata", "replays", "SomaJyJ.rep"))
	if err != nil {
		t.Fatal(err)
	}
	snaps, err := Snapshots(context.Background(), b, data, []int{360, 60, 99999})
	if err != nil {
		t.Fatal(err)
	}
	if _, past := snaps[99999]; past || len(snaps) != 2 {
		t.Fatalf("seconds %v", len(snaps))
	}
	counts := map[uint16]int{}
	for _, u := range snaps[360].Units {
		counts[u.Type]++
	}
	if counts[41] < 10 || counts[7] < 10 || counts[131] == 0 && counts[132] == 0 {
		t.Fatalf("at 6:00 want Drones, SCVs and a Hatchery or Lair: %v", counts)
	}
}
