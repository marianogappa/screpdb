package dashboard

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/marianogappa/screpdb/internal/dashboard/db"
)

// cohesion_vectors.json holds real ladder feature vectors from the #345 study:
// 20 Zerg games of one account, a held-out game of the same account, and a
// Zerg game by a different account.
type cohesionFixture struct {
	References    [][]float64 `json:"references"`
	OwnProbe      []float64   `json:"own_probe"`
	StrangerProbe []float64   `json:"stranger_probe"`
}

func loadCohesionFixture(t *testing.T) cohesionFixture {
	t.Helper()
	b, err := os.ReadFile("testdata/cohesion_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var f cohesionFixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.References) != cohesionMinReferenceGames {
		t.Fatalf("fixture has %d references, want %d", len(f.References), cohesionMinReferenceGames)
	}
	return f
}

func cohesionRows(refs [][]float64, refRace string, probe []float64, probeFrames int32) []db.PlayerFingerprintVectorRow {
	rows := make([]db.PlayerFingerprintVectorRow, 0, len(refs)+1)
	for i, v := range refs {
		rows = append(rows, db.PlayerFingerprintVectorRow{ReplayID: int64(100 + i), Frames: 20000, Race: refRace, Vector: v})
	}
	return append(rows, db.PlayerFingerprintVectorRow{ReplayID: 1, Frames: probeFrames, Race: "Zerg", Vector: probe})
}

func TestGamePlaysUnlikeOthers(t *testing.T) {
	f := loadCohesionFixture(t)
	for _, tc := range []struct {
		name string
		rows []db.PlayerFingerprintVectorRow
		want bool
	}{
		{"own game coheres", cohesionRows(f.References, "Zerg", f.OwnProbe, 20000), false},
		{"another human's game is flagged", cohesionRows(f.References, "Zerg", f.StrangerProbe, 20000), true},
		{"too few reference games stays silent", cohesionRows(f.References[1:], "Zerg", f.StrangerProbe, 20000), false},
		{"short game stays silent", cohesionRows(f.References, "Zerg", f.StrangerProbe, cohesionMinGameFrames-1), false},
		{"other-race references do not count", cohesionRows(f.References, "Terran", f.StrangerProbe, 20000), false},
		{"game outside the handle's eligible games stays silent", cohesionRows(f.References, "Zerg", f.StrangerProbe, 20000)[:len(f.References)], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := gamePlaysUnlikeOthers(tc.rows, 1); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
