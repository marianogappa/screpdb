package propack

import (
	"encoding/json"
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

func TestShieldBatteryAccounts(t *testing.T) {
	pack, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	raw, err := dataFS.ReadFile("data/shieldbattery.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Pros map[string][]SBAccount `json:"pros"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Pros) == 0 {
		t.Fatal("no ShieldBattery accounts embedded")
	}
	owner := map[uint32]string{}
	for id, accounts := range file.Pros {
		pro := pack.ByID(id)
		if pro == nil {
			t.Errorf("shieldbattery.json lists unknown pro %q", id)
			continue
		}
		if len(pro.ShieldBattery) != len(accounts) {
			t.Errorf("%s: %d accounts attached, want %d", id, len(pro.ShieldBattery), len(accounts))
		}
		for _, a := range accounts {
			if prev, dup := owner[a.ID]; dup {
				t.Errorf("ShieldBattery user %d listed for both %s and %s", a.ID, prev, id)
			}
			owner[a.ID] = id
			if got, ok := pack.ProBySBUserID(a.ID); !ok || got.ID != id {
				t.Errorf("ProBySBUserID(%d) = %v, %v; want %s", a.ID, got, ok, id)
			}
		}
	}

	bishop, ok := pack.ProBySBUserID(17185)
	if !ok || bishop.ID != "bishop" {
		t.Fatalf("ProBySBUserID(17185) = %v, %v; want bishop", bishop, ok)
	}
	if len(bishop.ShieldBattery) != 1 || bishop.ShieldBattery[0].Name != "K_BishOp" {
		t.Fatalf("bishop accounts = %+v", bishop.ShieldBattery)
	}
	if _, ok := pack.ProBySBUserID(0); ok {
		t.Fatal("user 0 must not resolve")
	}
	var nilPack *Pack
	if _, ok := nilPack.ProBySBUserID(17185); ok {
		t.Fatal("nil pack must not resolve")
	}
}
