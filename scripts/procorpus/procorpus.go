// Package procorpus is the shared labelling and joining logic for the offline
// progamer-corpus scripts (expert-mine, pro-pack). It turns a screpharvest
// harvest plus the scfingerprint pro registry into resolved (replay, player)
// rows inside a screpdb-ingested scratch database.
//
// Dev-only tooling: it never ships in the binary, so it reads files directly
// rather than through iofacade (scripts/ is on the enforcement-test skip list).
package procorpus

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HarvestRow is one line of screpharvest's replays.jsonl: a match seen from
// the perspective of the harvested account plus its opponent.
type HarvestRow struct {
	MatchID     string `json:"matchId"`
	Timestamp   int64  `json:"timestamp"`
	Duration    int    `json:"duration"`
	AuroraID    int64  `json:"auroraId"`
	Toon        string `json:"toon"`
	Gateway     int    `json:"gateway"`
	Race        string `json:"race"`
	MMR         int    `json:"mmr"`
	OppAuroraID int64  `json:"oppAuroraId"`
	OppToon     string `json:"oppToon"`
	OppGateway  int    `json:"oppGateway"`
	OppRace     string `json:"oppRace"`
	OppMMR      int    `json:"oppMmr"`
}

// ProSide is one labelled progamer player-game before the DB join.
type ProSide struct {
	MatchID   string
	Timestamp int64
	AuroraID  int64
	ProName   string // registry name (pros_merged.json key)
	Toon      string // may be empty (~32% of harvest rows)
	// AltNames are extra replay names that identify this side, e.g. the pack
	// key a previous pro-pack run already renamed the player row to.
	AltNames []string
	Gateway  int
	OppToon  string
	Race     string // full race name ("Zerg")
	MMR      int
}

var RaceByLetter = map[string]string{"Z": "Zerg", "T": "Terran", "P": "Protoss"}

// Registry maps enrolled aurora IDs to their progamer name, with the
// pro_exclusions.json entries already removed.
type Registry struct {
	ByAurora map[int64]string
	// AurorasByName lists every enrolled aurora ID per pro name.
	AurorasByName map[string][]int64
}

// LoadRegistry reads pros_merged.json and pro_exclusions.json from the
// scfingerprint corpus dir. Never trust the harvest's proName field: label by
// aurora ID only.
func LoadRegistry(corpusDir string) (*Registry, error) {
	var merged map[string][]int64
	if err := ReadJSON(filepath.Join(corpusDir, "pros_merged.json"), &merged); err != nil {
		return nil, err
	}
	var exclusions map[string]json.RawMessage
	if err := ReadJSON(filepath.Join(corpusDir, "pro_exclusions.json"), &exclusions); err != nil {
		return nil, err
	}
	excluded := map[int64]bool{}
	for name, raw := range exclusions {
		if name == "_comment" {
			continue
		}
		var byID map[string]string
		if err := json.Unmarshal(raw, &byID); err != nil {
			return nil, fmt.Errorf("pro_exclusions[%s]: %w", name, err)
		}
		for idStr := range byID {
			var id int64
			fmt.Sscan(idStr, &id)
			excluded[id] = true
		}
	}
	reg := &Registry{ByAurora: map[int64]string{}, AurorasByName: map[string][]int64{}}
	for name, ids := range merged {
		for _, id := range ids {
			if excluded[id] {
				continue
			}
			reg.ByAurora[id] = name
			reg.AurorasByName[name] = append(reg.AurorasByName[name], id)
		}
	}
	return reg, nil
}

// LabelCorpus reads <harvestDir>/replays.jsonl and returns every pro
// player-game: a side is pro iff its aurora ID is enrolled in the registry and
// the game lasted at least minDuration seconds. Both sides of a row are
// considered (pro-vs-pro games).
func LabelCorpus(harvestDir string, reg *Registry, minDuration int) ([]ProSide, error) {
	f, err := os.Open(filepath.Join(harvestDir, "replays.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := map[string]bool{} // matchID:auroraID
	var sides []ProSide
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		var r HarvestRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("bad jsonl row: %w", err)
		}
		if r.Duration < minDuration {
			continue
		}
		for _, cand := range []ProSide{
			{MatchID: r.MatchID, Timestamp: r.Timestamp, AuroraID: r.AuroraID, Toon: r.Toon, Gateway: r.Gateway, OppToon: r.OppToon, Race: RaceByLetter[r.Race], MMR: r.MMR},
			{MatchID: r.MatchID, Timestamp: r.Timestamp, AuroraID: r.OppAuroraID, Toon: r.OppToon, Gateway: r.OppGateway, OppToon: r.Toon, Race: RaceByLetter[r.OppRace], MMR: r.OppMMR},
		} {
			name, ok := reg.ByAurora[cand.AuroraID]
			if cand.AuroraID == 0 || !ok || cand.Race == "" {
				continue
			}
			key := cand.MatchID + ":" + fmt.Sprint(cand.AuroraID)
			if seen[key] {
				continue
			}
			seen[key] = true
			cand.ProName = name
			sides = append(sides, cand)
		}
	}
	return sides, sc.Err()
}

// GroupByMatch indexes sides by match ID.
func GroupByMatch(sides []ProSide) map[string][]ProSide {
	byMatch := map[string][]ProSide{}
	for _, s := range sides {
		byMatch[s.MatchID] = append(byMatch[s.MatchID], s)
	}
	return byMatch
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Stage copies every labelled match's .rep flat into stagedDir, skipping files
// already there. Returns how many are staged and how many were not on disk.
func Stage(harvestDir, stagedDir string, byMatch map[string][]ProSide) (staged, missing int) {
	for matchID := range byMatch {
		src := filepath.Join(harvestDir, "replays", matchID+".rep")
		dst := filepath.Join(stagedDir, matchID+".rep")
		if _, err := os.Stat(dst); err == nil {
			staged++
			continue
		}
		if err := copyFile(src, dst); err != nil {
			missing++
			continue
		}
		staged++
	}
	return staged, missing
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// JoinedPlayer is one resolved (replay, player) pro row.
type JoinedPlayer struct {
	ReplayID int64
	PlayerID int64
	FileName string
	Matchup  string
	Name     string
	Race     string
	Side     ProSide
}

type JoinTallies struct {
	ByToon, ByOppElim, ByRace, Unresolved, NotIngested int
}

func (t JoinTallies) String() string {
	return fmt.Sprintf("by-toon=%d by-opp-elim=%d by-race=%d unresolved=%d not-ingested=%d",
		t.ByToon, t.ByOppElim, t.ByRace, t.Unresolved, t.NotIngested)
}

type PlayerRow struct {
	ID   int64
	Name string
	Race string
}

// Join resolves each labelled side to a (replay, player) row in the ingested
// scratch DB: by toon, else by eliminating the opponent's toon (1v1 only),
// else by unique race. Unresolved sides are dropped, never guessed.
func Join(db *sql.DB, byMatch map[string][]ProSide) ([]JoinedPlayer, JoinTallies, error) {
	type replayRow struct {
		id       int64
		fileName string
		matchup  string
		players  []PlayerRow
	}
	rows, err := db.Query(`
		SELECT r.id, r.file_name, r.matchup, p.id, p.name, p.race
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE r.team_format = '1v1' AND p.is_observer = 0 AND p.type = 'Human'
		ORDER BY r.id, p.id`)
	if err != nil {
		return nil, JoinTallies{}, err
	}
	defer rows.Close()
	byFile := map[string]*replayRow{}
	for rows.Next() {
		var rid, pid int64
		var fileName, matchup, pname, prace string
		if err := rows.Scan(&rid, &fileName, &matchup, &pid, &pname, &prace); err != nil {
			return nil, JoinTallies{}, err
		}
		rr, ok := byFile[fileName]
		if !ok {
			rr = &replayRow{id: rid, fileName: fileName, matchup: matchup}
			byFile[fileName] = rr
		}
		rr.players = append(rr.players, PlayerRow{ID: pid, Name: pname, Race: prace})
	}
	if err := rows.Err(); err != nil {
		return nil, JoinTallies{}, err
	}

	var out []JoinedPlayer
	var t JoinTallies
	for matchID, sides := range byMatch {
		rr, ok := byFile[matchID+".rep"]
		if !ok {
			t.NotIngested += len(sides)
			continue
		}
		for _, s := range sides {
			p, method := ResolvePlayer(rr.players, s)
			switch method {
			case "toon":
				t.ByToon++
			case "opp":
				t.ByOppElim++
			case "race":
				t.ByRace++
			default:
				t.Unresolved++
				continue
			}
			out = append(out, JoinedPlayer{
				ReplayID: rr.id, PlayerID: p.ID, FileName: rr.fileName,
				Matchup: rr.matchup, Name: p.Name, Race: p.Race, Side: s,
			})
		}
	}
	return out, t, nil
}

// ResolvePlayer picks the replay player a labelled side refers to and reports
// the method used ("toon", "opp", "race"), or "" when it cannot be resolved.
func ResolvePlayer(players []PlayerRow, s ProSide) (PlayerRow, string) {
	var zero PlayerRow
	if s.Toon != "" {
		for _, p := range players {
			if p.Name == s.Toon {
				return p, "toon"
			}
		}
	}
	for _, alt := range s.AltNames {
		for _, p := range players {
			if strings.EqualFold(p.Name, alt) {
				return p, "toon"
			}
		}
	}
	if s.OppToon != "" && len(players) == 2 {
		if players[0].Name == s.OppToon {
			return players[1], "opp"
		}
		if players[1].Name == s.OppToon {
			return players[0], "opp"
		}
	}
	var match *PlayerRow
	count := 0
	for i := range players {
		if strings.EqualFold(players[i].Race, s.Race) {
			match = &players[i]
			count++
		}
	}
	if count == 1 {
		return *match, "race"
	}
	return zero, ""
}
