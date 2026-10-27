package dashboard

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseWorkflowGamesListFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/?player=Bisu,Flash&map=Fighting+Spirit&duration=short&featuring=cheese&matchup=pvz&map_kind=regular", nil)
	got := parseWorkflowGamesListFilters(req)
	if !reflect.DeepEqual(got.PlayerKeys, []string{"bisu", "flash"}) {
		t.Errorf("PlayerKeys = %v", got.PlayerKeys)
	}
	if !reflect.DeepEqual(got.MapNames, []string{"Fighting Spirit"}) {
		t.Errorf("MapNames should preserve case = %v", got.MapNames)
	}
	if !reflect.DeepEqual(got.DurationBuckets, []string{"short"}) {
		t.Errorf("DurationBuckets = %v", got.DurationBuckets)
	}
	if !reflect.DeepEqual(got.MatchupKeys, []string{"pvz"}) {
		t.Errorf("MatchupKeys = %v", got.MatchupKeys)
	}
	if !reflect.DeepEqual(got.MapKindKeys, []string{"regular"}) {
		t.Errorf("MapKindKeys = %v", got.MapKindKeys)
	}

	empty := parseWorkflowGamesListFilters(httptest.NewRequest("GET", "/", nil))
	if len(empty.PlayerKeys) != 0 || len(empty.MapNames) != 0 {
		t.Errorf("empty request should yield empty filters, got %+v", empty)
	}
}

func TestHasValidCenterBaseKind(t *testing.T) {
	for _, kind := range []string{"start", "starting", "natural", "expansion", "expa"} {
		if !hasValidCenterBaseKind(kind) {
			t.Errorf("expected %q to be a valid center base kind", kind)
		}
	}
	for _, kind := range []string{"", "mineral only", "unknown"} {
		if hasValidCenterBaseKind(kind) {
			t.Errorf("expected %q to be invalid", kind)
		}
	}
}
