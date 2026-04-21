package dashboard

import (
	"testing"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func TestBattleTagLookupKeysWithDiscriminator(t *testing.T) {
	keys := battleTagLookupKeys("myname#1234")
	if len(keys) != 2 || keys[0] != "myname#1234" || keys[1] != "myname" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestBattleTagLookupKeysNoDiscriminator(t *testing.T) {
	keys := battleTagLookupKeys("myname")
	if len(keys) != 1 || keys[0] != "myname" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestDisplayNamesWithAliasRowsDiscriminatorMismatch(t *testing.T) {
	rows := []dashboarddb.PlayerAliasRow{
		{
			ID:                  1,
			CanonicalAlias:      "you",
			BattleTagNormalized: "player#9999",
			BattleTagRaw:        "Player#9999",
			Source:              aliasSourceYou,
		},
	}
	best := buildBestAliasRowByLookupKey(rows)
	names := []string{"Player"}
	got := displayNamesWithAliasRows(names, best)
	if got["Player"] != "Player (you)" {
		t.Fatalf("expected %q, got %q", "Player (you)", got["Player"])
	}
}

func TestDisplayNamesPrefersYouOverImportedOnSharedBase(t *testing.T) {
	rows := []dashboarddb.PlayerAliasRow{
		{
			ID:                  1,
			CanonicalAlias:      "Imported",
			BattleTagNormalized: "foo#1",
			Source:              aliasSourceImported,
			UpdatedAt:           "2020-01-01 00:00:00",
		},
		{
			ID:                  2,
			CanonicalAlias:      "you",
			BattleTagNormalized: "foo#2",
			Source:              aliasSourceYou,
			UpdatedAt:           "2020-01-01 00:00:00",
		},
	}
	best := buildBestAliasRowByLookupKey(rows)
	got := displayNamesWithAliasRows([]string{"foo"}, best)
	if got["foo"] != "foo (you)" {
		t.Fatalf("expected you to win on shared base, got %q", got["foo"])
	}
}
