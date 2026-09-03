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
	if pack.AlgorithmVersion != core.AlgorithmVersion {
		t.Fatalf("pack was generated at AlgorithmVersion %d but the code is at %d: regenerate it with scripts/pro-pack so built-in profiles stay comparable with local players", pack.AlgorithmVersion, core.AlgorithmVersion)
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
