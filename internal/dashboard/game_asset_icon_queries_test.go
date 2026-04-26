package dashboard

import "testing"

// Regression for the "Terran expansion overlay never renders" bug. The
// expansion overlay calls getUnitIcon('commandcenter') on the frontend,
// which fans out to /api/custom/game-assets/building?name=commandcenter
// → resolveGameAssetIconQuery. Pre-fix the key wasn't in
// gameAssetIconScmapQueries → handler returned 404 → frontend got null
// icon → overlay was suppressed for every Terran expansion event.
func TestResolveGameAssetIconQuery_Terran(t *testing.T) {
	cases := []struct {
		name              string
		input             string
		wantOK            bool
		wantCacheKey      string
		wantScmapContains string
	}{
		{name: "commandcenter exact", input: "commandcenter", wantOK: true, wantCacheKey: "commandcenter", wantScmapContains: "Command Center"},
		{name: "commandcenter race-prefixed", input: "terrancommandcenter", wantOK: true, wantCacheKey: "commandcenter", wantScmapContains: "Command Center"},
		{name: "nexus still works", input: "nexus", wantOK: true, wantCacheKey: "nexus", wantScmapContains: "Nexus"},
		{name: "hatchery still works", input: "hatchery", wantOK: true, wantCacheKey: "hatchery", wantScmapContains: "Hatchery"},
		{name: "unknown rejected", input: "totallymadeup", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheKey, scmapQuery, ok := resolveGameAssetIconQuery(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch for %q: got %v, want %v", tc.input, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if cacheKey != tc.wantCacheKey {
				t.Fatalf("cacheKey mismatch for %q: got %q, want %q", tc.input, cacheKey, tc.wantCacheKey)
			}
			if tc.wantScmapContains != "" && !contains(scmapQuery, tc.wantScmapContains) {
				t.Fatalf("scmapQuery for %q = %q, want substring %q", tc.input, scmapQuery, tc.wantScmapContains)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
