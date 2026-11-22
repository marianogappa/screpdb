package propack

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestLoadPack(t *testing.T) {
	pack, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pack.DetectorVersion != core.DetectorVersion {
		t.Fatalf("pack was generated at DetectorVersion %d but the code is at %d: regenerate it with scripts/pro-pack so built-in profiles stay comparable with local players", pack.DetectorVersion, core.DetectorVersion)
	}
	for _, pro := range pack.Pros {
		if pro.Label == "" {
			t.Errorf("%s: empty label", pro.ID)
		}
		if pro.GamesSampled <= 0 {
			t.Errorf("%s: no sampled games", pro.ID)
		}
		if pro.Photo != "" {
			if _, _, ok := Photo(&pro); !ok {
				t.Errorf("%s: photo %q is referenced but not embedded", pro.ID, pro.Photo)
			}
		}
		if got := pack.ByID(pro.ID); got == nil || got.ID != pro.ID {
			t.Errorf("%s: ByID lookup failed", pro.ID)
		}
		if got := pack.ByLabel(pro.Label); got == nil {
			t.Errorf("%s: ByLabel(%q) lookup failed", pro.ID, pro.Label)
		}
		if got := pack.ByID(pro.ID); got.Key() != KeyPrefix+pro.ID {
			t.Errorf("%s: key %q", pro.ID, got.Key())
		}
	}
}

func TestKeyRoundTrip(t *testing.T) {
	id, ok := IDFromKey(Key("Bisu"))
	if !ok || id != "bisu" {
		t.Fatalf("IDFromKey(Key(Bisu)) = %q, %v", id, ok)
	}
	if _, ok := IDFromKey("bisu"); ok {
		t.Fatal("plain key must not be a pro key")
	}
	if _, ok := IDFromKey("pro:"); ok {
		t.Fatal("empty id must not be a pro key")
	}
}

func TestHotkeyOdditiesAttach(t *testing.T) {
	pack, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	found := 0
	for _, pro := range pack.Pros {
		for race, oddities := range pro.HotkeyOddities {
			if pro.Races[race] == 0 {
				t.Errorf("%s: oddities for %s, a race the pack never sampled", pro.ID, race)
			}
			found += len(oddities)
		}
	}
	if found == 0 {
		t.Fatal("no hotkey oddities attached; is data/hotkey_oddities.json embedded?")
	}
	if pack.ByID("rich").OdditiesFor("protoss") == nil {
		t.Error("rich: expected Protoss oddities")
	}
}

// Every habit line of a race says "of N built-in players"; they must all count the same players.
func TestHotkeyOdditiesShareOnePoolPerRace(t *testing.T) {
	pack, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	populations := map[string]map[int][]string{}
	for _, pro := range pack.Pros {
		for race, oddities := range pro.HotkeyOddities {
			for _, o := range oddities {
				if populations[race] == nil {
					populations[race] = map[int][]string{}
				}
				populations[race][o.Population] = append(populations[race][o.Population], pro.ID+" "+o.Kind)
			}
		}
	}
	for race, byPopulation := range populations {
		if len(byPopulation) > 1 {
			t.Errorf("%s habit lines count different players: %v", race, byPopulation)
		}
	}
}
