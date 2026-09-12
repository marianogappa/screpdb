package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/icza/screp/repparser"
	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/propack"
)

// manifestIdentity is the part of a catalog identity this pass needs: the
// enrolled fingerprint, and the replays it was enrolled from.
type manifestIdentity struct {
	ID             string   `json:"id"`
	Fingerprint    string   `json:"fingerprint"`
	ReplayManifest []string `json:"replay_manifest"`
}

// metricsFromManifests computes pack metrics for identities the harvest could
// not label.
//
// The main pass attributes a replay to a pro by looking its toon up in the
// registry, which cannot work for a pro whose games were never played on a
// registered toon: FlaSh is enrolled upstream from community replay archives,
// and SoulKey has no registry toons at all. Both still carry a replay_manifest
// naming the exact files they were enrolled from, so this pass reads those
// directly and settles which slot is the pro by scoring each side against that
// identity's own enrolled fingerprint — the same evidence that enrolled them.
func metricsFromManifests(
	ctx context.Context,
	datasetDir, corpusDir, stagedDir string,
	missing map[string]roster,
) (map[string]dashboard.ProMetrics, error) {
	if len(missing) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		return nil, err
	}

	// packKeyByStagedFile and proNameByStagedFile together say, for one staged
	// replay, which slot is the pro and what to rename it to.
	packKeyByStagedFile := map[string]string{}
	proNameByStagedFile := map[string]string{}

	for name, r := range missing {
		ident, err := readManifestIdentity(datasetDir, r.id)
		if err != nil {
			log.Printf("manifest %s: %v", r.id, err)
			continue
		}
		if len(ident.ReplayManifest) == 0 || strings.TrimSpace(ident.Fingerprint) == "" {
			continue
		}
		dataset, err := singleIdentityDataset(ident.Fingerprint)
		if err != nil {
			log.Printf("manifest %s: %v", r.id, err)
			continue
		}
		staged := 0
		for _, rel := range ident.ReplayManifest {
			src := filepath.Join(corpusDir, rel)
			if _, err := os.Stat(src); err != nil {
				continue
			}
			slot, err := proSlotName(src, dataset)
			if err != nil || slot == "" {
				continue
			}
			dst := r.id + "__" + filepath.Base(rel)
			if err := copyFile(src, filepath.Join(stagedDir, dst)); err != nil {
				continue
			}
			packKeyByStagedFile[dst] = propack.Key(r.id)
			proNameByStagedFile[dst] = slot
			staged++
		}
		log.Printf("manifest %s (%s): staged %d/%d replays", r.id, name, staged, len(ident.ReplayManifest))
	}
	if len(packKeyByStagedFile) == 0 {
		return nil, nil
	}

	corpus, err := dashboard.LoadProCorpus(ctx, stagedDir, nil, -1)
	if err != nil {
		return nil, fmt.Errorf("read manifest replays: %w", err)
	}
	defer corpus.Close()

	rename := map[int64]string{}
	for _, replay := range corpus.Replays1v1() {
		key, ok := packKeyByStagedFile[replay.FileName]
		if !ok {
			continue
		}
		want := proNameByStagedFile[replay.FileName]
		for _, player := range replay.Players {
			if player.Name == want {
				rename[player.PlayerID] = key
			}
		}
	}
	log.Printf("manifest: analysed %d replays, renamed %d players", corpus.Len(), corpus.RenamePlayers(rename))

	keys := make([]string, 0, len(missing))
	for _, r := range missing {
		keys = append(keys, propack.Key(r.id))
	}
	return corpus.Metrics(ctx, keys)
}

func readManifestIdentity(datasetDir, id string) (manifestIdentity, error) {
	var ident manifestIdentity
	path := filepath.Join(datasetDir, "players", id+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ident, err
	}
	return ident, json.Unmarshal(raw, &ident)
}

func singleIdentityDataset(fingerprint string) (*scfingerprint.Dataset, error) {
	fp, err := scfingerprint.ParseFingerprint(fingerprint)
	if err != nil {
		return nil, fmt.Errorf("parsing fingerprint: %w", err)
	}
	dataset, err := scfingerprint.NewDataset()
	if err != nil {
		return nil, err
	}
	if err := dataset.Add(fp); err != nil {
		return nil, err
	}
	return dataset, nil
}

// proSlotName reports which player in a replay is the identity the dataset
// holds, by scoring every slot against it and taking the best. The manifest
// already asserts the pro played this game, so this only has to pick a side;
// the loser's score is not a claim about anyone.
func proSlotName(path string, dataset *scfingerprint.Dataset) (string, error) {
	replay, err := repparser.ParseFile(path)
	if err != nil {
		return "", err
	}
	replay.Compute()
	vectors, err := scfingerprint.Extract(replay)
	if err != nil {
		return "", err
	}
	bestZ := math.Inf(-1)
	best := ""
	for _, pv := range vectors {
		results, err := scfingerprint.MatchMany(
			[]scfingerprint.PlayerGame{{Vector: pv.Vector, Race: pv.Race}},
			dataset,
			scfingerprint.WithMinZ(math.Inf(-1)),
		)
		if err != nil || len(results) == 0 {
			continue
		}
		if results[0].Z > bestZ {
			bestZ, best = results[0].Z, pv.Name
		}
	}
	return best, nil
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
