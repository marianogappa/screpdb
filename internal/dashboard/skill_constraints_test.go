package dashboard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/enginepass"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/compact"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/skill"
)

type skillConstraint struct {
	File        string  `json:"file"`
	Player      string  `json:"player"`
	Metric      string  `json:"metric"`
	MaxPosition float64 `json:"max_position"`
	Open        bool    `json:"open"`
	Why         string  `json:"why"`
}

type skillConstraints struct {
	Cases   []skillConstraint `json:"cases"`
	Session struct {
		Dir                 string  `json:"dir"`
		MaxStripPosition    float64 `json:"max_strip_position"`
		MaxHeadlinePosition float64 `json:"max_headline_position"`
		Open                bool    `json:"open"`
		Why                 string  `json:"why"`
	} `json:"session"`
}

// TestSkillConstraints replays the games the owner flagged against the current model. It needs their
// replays, so it only runs with SKILL_CONSTRAINTS_DIR (the AutoSave folder) and SKILL_BWDATA set:
//
//	SKILL_CONSTRAINTS_DIR=".../Maps/Replays/AutoSave" SKILL_BWDATA=".../screpdb/bwdata" go test ./internal/dashboard -run TestSkillConstraints -v
func TestSkillConstraints(t *testing.T) {
	dir, bwdata := os.Getenv("SKILL_CONSTRAINTS_DIR"), os.Getenv("SKILL_BWDATA")
	if dir == "" || bwdata == "" {
		t.Skip("set SKILL_CONSTRAINTS_DIR and SKILL_BWDATA to check the flagged games")
	}
	var cs skillConstraints
	b, err := os.ReadFile("testdata/skill_constraints.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	data, err := engine.LoadData(bwdata)
	if err != nil {
		t.Fatal(err)
	}
	rated := map[string]map[string]skill.Result{}
	rate := func(file string) map[string]skill.Result {
		if r, ok := rated[file]; ok {
			return r
		}
		r := rateReplayFile(t, filepath.Join(dir, file), model, data)
		rated[file] = r
		return r
	}
	for _, c := range cs.Cases {
		res, ok := rate(c.File)[c.Player]
		var p skill.Placement
		var measured bool
		if ok {
			p, measured = res.Metrics[c.Metric]
		}
		holds := !measured || p.Position <= c.MaxPosition
		reading := "no reading"
		if measured {
			reading = formatPlacement(p)
		}
		switch {
		case holds && c.Open:
			t.Logf("NOW HOLDS (mark it fixed): %s %s %s: %s", c.File, c.Player, c.Metric, reading)
		case holds:
			t.Logf("ok: %s %s %s: %s", c.File, c.Player, c.Metric, reading)
		case c.Open:
			t.Logf("open: %s %s %s: %s, want at most %.2f (%s)", c.File, c.Player, c.Metric, reading, c.MaxPosition, c.Why)
		default:
			t.Errorf("REGRESSED: %s %s %s: %s, want at most %.2f (%s)", c.File, c.Player, c.Metric, reading, c.MaxPosition, c.Why)
		}
	}
	if cs.Session.Dir == "" {
		return
	}
	files, _ := filepath.Glob(filepath.Join(dir, cs.Session.Dir, "*.rep"))
	var breaks []string
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		for player, res := range rate(rel) {
			for key, p := range res.Metrics {
				if p.Position > cs.Session.MaxStripPosition {
					breaks = append(breaks, rel+" "+player+" "+key+" "+formatPlacement(p))
				}
			}
			if res.Combined != nil && res.Combined.Position > cs.Session.MaxHeadlinePosition {
				breaks = append(breaks, rel+" "+player+" headline "+formatPlacement(*res.Combined))
			}
		}
	}
	for _, br := range breaks {
		if cs.Session.Open {
			t.Logf("session open: %s", br)
		} else {
			t.Errorf("session REGRESSED: %s (%s)", br, cs.Session.Why)
		}
	}
	if len(breaks) == 0 && cs.Session.Open {
		t.Logf("session NOW HOLDS (mark it fixed): %s", cs.Session.Why)
	}
}

func formatPlacement(p skill.Placement) string {
	return []string{"Beginner", "Intermediate", "Advanced", "Pro"}[min(skill.TierPro, int(p.Position))] + " at " + strings.TrimRight(strings.TrimRight(jsonFloat(p.Position), "0"), ".") + ", value " + jsonFloat(p.Value)
}

func jsonFloat(f float64) string {
	b, _ := json.Marshal(float64(int(f*1000+0.5)) / 1000)
	return string(b)
}

func rateReplayFile(t *testing.T, path string, model *skill.Model, data *engine.Data) map[string]skill.Result {
	t.Helper()
	out := map[string]skill.Result{}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return out
	}
	replay := parser.CreateReplayFromFileInfo(path, filepath.Base(path), int64(len(b)), "")
	parsed, err := parser.ParseReplayWithOptions(path, replay, parser.Options{})
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return out
	}
	rec, err := compact.FromReplayData(parsed, compact.FileMeta{Path: path, Size: int64(len(b))})
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return out
	}
	d, err := engine.Pass(context.Background(), b, data)
	if err != nil {
		t.Errorf("%s: engine: %v", path, err)
		return out
	}
	rec.EngineData = enginepass.Summarize(d, rec.MapKind == library.MapKindMoney)
	for ordinal, r := range rateGame(model, rec).players {
		out[rec.Players[ordinal].Name] = r.result
	}
	return out
}
