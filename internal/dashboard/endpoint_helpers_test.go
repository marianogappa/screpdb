package dashboard

import (
	"bytes"
	"database/sql"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func aliasRow(canonical, source, updatedAt string) dashboarddb.PlayerAliasRow {
	return dashboarddb.PlayerAliasRow{CanonicalAlias: canonical, Source: source, UpdatedAt: updatedAt}
}

func TestFormatClockFromSeconds(t *testing.T) {
	cases := []struct {
		second int64
		want   string
	}{
		{0, "0:00"},
		{5, "0:05"},
		{59, "0:59"},
		{60, "1:00"},
		{125, "2:05"},
		{3600, "60:00"},
		{-30, "0:00"},
	}
	for _, c := range cases {
		if got := formatClockFromSeconds(c.second); got != c.want {
			t.Errorf("formatClockFromSeconds(%d) = %q, want %q", c.second, got, c.want)
		}
	}
}

func TestFormatWorkflowSliceLabel(t *testing.T) {
	cases := []struct {
		start, endExclusive int64
		want                string
	}{
		{240, 300, "4:00-4:59"},
		{300, 360, "5:00-5:59"},
		{600, 600, "10:00-10:00"},
		{600, 500, "10:00-10:00"},
	}
	for _, c := range cases {
		if got := formatWorkflowSliceLabel(c.start, c.endExclusive); got != c.want {
			t.Errorf("formatWorkflowSliceLabel(%d,%d) = %q, want %q", c.start, c.endExclusive, got, c.want)
		}
	}
}

func TestWorkflowSliceBoundaries(t *testing.T) {
	if got := workflowSliceBoundaries(100); len(got) != 0 {
		t.Fatalf("short game should yield no boundaries, got %v", got)
	}
	got := workflowSliceBoundaries(700)
	want := []int64{240, 300, 360, 420, 600}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workflowSliceBoundaries(700) = %v, want %v", got, want)
	}
	long := workflowSliceBoundaries(5000)
	if long[len(long)-1] != 4800 {
		t.Fatalf("expected trailing 600s step to reach 4800, got %v", long)
	}
	for i := 1; i < len(long); i++ {
		if long[i] <= long[i-1] {
			t.Fatalf("boundaries not strictly increasing: %v", long)
		}
	}
}

func TestSliceStartForSecond(t *testing.T) {
	boundaries := []int64{240, 300, 360, 420, 600}
	cases := []struct {
		second int64
		want   int64
	}{
		{100, 240},
		{240, 240},
		{299, 240},
		{300, 300},
		{450, 420},
		{9999, 600},
	}
	for _, c := range cases {
		if got := sliceStartForSecond(c.second, boundaries); got != c.want {
			t.Errorf("sliceStartForSecond(%d) = %d, want %d", c.second, got, c.want)
		}
	}
	if got := sliceStartForSecond(500, nil); got != 0 {
		t.Errorf("sliceStartForSecond with no boundaries = %d, want 0", got)
	}
}

func TestOrdinalSuffix(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "st"}, {2, "nd"}, {3, "rd"}, {4, "th"},
		{11, "th"}, {12, "th"}, {13, "th"},
		{21, "st"}, {22, "nd"}, {23, "rd"},
		{111, "th"}, {112, "th"}, {113, "th"}, {101, "st"},
	}
	for _, c := range cases {
		if got := ordinalSuffix(c.n); got != c.want {
			t.Errorf("ordinalSuffix(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestNullableString(t *testing.T) {
	if got := nullableString(nil); got != "" {
		t.Errorf("nullableString(nil) = %q, want empty", got)
	}
	v := "x"
	if got := nullableString(&v); got != "x" {
		t.Errorf("nullableString(&\"x\") = %q, want \"x\"", got)
	}
}

func TestBaseLabel(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	s := func(v string) *string { return &v }
	cases := []struct {
		name                  string
		baseType              *string
		baseOclock, naturalOf *int64
		want                  string
	}{
		{"nil type", nil, i(3), nil, ""},
		{"starting center", s("starting"), i(0), nil, "center base"},
		{"starting oclock", s("starting"), i(9), nil, "9"},
		{"starting no oclock", s("starting"), nil, nil, "starting base"},
		{"natural of", s("natural"), i(12), i(12), "12's natural"},
		{"natural near", s("natural"), i(6), i(12), "12's natural near 6"},
		{"natural center of center", s("natural"), i(0), i(0), "center base"},
		{"natural center of known", s("natural"), i(0), i(12), "12's natural (center base)"},
		{"expansion near", s("mineral only"), i(3), nil, "an expansion near 3"},
		{"expansion center", s("mineral only"), i(0), nil, "center base"},
		{"expansion no oclock", s("mineral only"), nil, nil, "expansion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := baseLabel(c.baseType, c.baseOclock, c.naturalOf); got != c.want {
				t.Fatalf("baseLabel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseAttackRelativeLocation(t *testing.T) {
	loc, x, y, ok := parseAttackRelativeLocation(`{"loc":"near 3 o'clock","x":12.5,"y":7}`)
	if !ok || loc != "near 3 o'clock" || x != 12.5 || y != 7 {
		t.Fatalf("unexpected parse: loc=%q x=%v y=%v ok=%v", loc, x, y, ok)
	}
	for _, raw := range []string{"", "   ", "not json", `{"loc":""}`, `{"x":1}`} {
		if _, _, _, ok := parseAttackRelativeLocation(raw); ok {
			t.Errorf("expected ok=false for %q", raw)
		}
	}
}

func TestParseAttackUnitTypes(t *testing.T) {
	if got := parseAttackUnitTypes(nil); got != nil {
		t.Errorf("nil input should yield nil, got %v", got)
	}
	raw := `["Marine"," Marine ","Firebat",""]`
	got := parseAttackUnitTypes(&raw)
	want := []string{"Marine", "Firebat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseAttackUnitTypes dedup/trim = %v, want %v", got, want)
	}
	empty := `[]`
	if got := parseAttackUnitTypes(&empty); got != nil {
		t.Errorf("empty array should yield nil, got %v", got)
	}
	bad := `{`
	if got := parseAttackUnitTypes(&bad); got != nil {
		t.Errorf("invalid json should yield nil, got %v", got)
	}
}

func TestParseAttackCastCounts(t *testing.T) {
	if got := parseAttackCastCounts(nil); got != nil {
		t.Errorf("nil input should yield nil, got %v", got)
	}
	raw := `{"Psionic Storm":3,"Lockdown":1}`
	got := parseAttackCastCounts(&raw)
	if got["Psionic Storm"] != 3 || got["Lockdown"] != 1 {
		t.Fatalf("parseAttackCastCounts = %v", got)
	}
	empty := `{}`
	if got := parseAttackCastCounts(&empty); got != nil {
		t.Errorf("empty object should yield nil, got %v", got)
	}
}

func TestBuildWorkflowPatternValue(t *testing.T) {
	pv := buildWorkflowPatternValue("bo_9_pool", "", 150, `{"pool":9}`)
	if pv.EventType != "bo_9_pool" || pv.DetectedSecond != 150 {
		t.Fatalf("unexpected pattern value: %+v", pv)
	}
	if string(pv.Payload) != `{"pool":9}` {
		t.Fatalf("expected payload preserved, got %q", string(pv.Payload))
	}

	bare := buildWorkflowPatternValue("k", "", 10, "true")
	if bare.Payload != nil {
		t.Fatalf(`"true" payload should be dropped, got %q`, string(bare.Payload))
	}
	empty := buildWorkflowPatternValue("k", "", 10, "")
	if empty.Payload != nil {
		t.Fatalf("empty payload should be dropped, got %q", string(empty.Payload))
	}
}

func TestPerformancePercentileFromSortedValues_EdgeCases(t *testing.T) {
	if got := performancePercentileFromSortedValues(nil, 5, false); got != 0 {
		t.Errorf("empty slice should be 0, got %v", got)
	}
	if got := performancePercentileFromSortedValues([]float64{5}, 5, false); got != 100 {
		t.Errorf("single value should be 100, got %v", got)
	}
	sorted := []float64{10, 20, 30, 40, 50}
	if got := performancePercentileFromSortedValues(sorted, 50, false); got != 100 {
		t.Errorf("max value higher-is-better should be 100, got %v", got)
	}
	if got := performancePercentileFromSortedValues(sorted, 10, false); got != 0 {
		t.Errorf("min value higher-is-better should be 0, got %v", got)
	}
	if got := performancePercentileFromSortedValues(sorted, 10, true); got != 100 {
		t.Errorf("min value lower-is-better should be 100, got %v", got)
	}
	if got := performancePercentileFromSortedValues(sorted, 30, false); got != 50 {
		t.Errorf("median higher-is-better should be 50, got %v", got)
	}
}

func TestMatchupConfidenceForGames(t *testing.T) {
	cases := []struct {
		games int64
		want  string
	}{
		{0, "low"}, {4, "low"}, {5, "medium"}, {14, "medium"}, {15, "high"}, {100, "high"},
	}
	for _, c := range cases {
		if got := matchupConfidenceForGames(c.games); got != c.want {
			t.Errorf("matchupConfidenceForGames(%d) = %q, want %q", c.games, got, c.want)
		}
	}
}

func TestNormalizeUnitName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Siege Tank (Tank Mode)", "siegetanktankmode"},
		{"  Marine  ", "marine"},
		{"", ""},
		{"Zerg Zergling", "zergzergling"},
	}
	for _, c := range cases {
		if got := normalizeUnitName(c.in); got != c.want {
			t.Errorf("normalizeUnitName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUnitNameAliases(t *testing.T) {
	got := unitNameAliases("Terran Marine")
	set := map[string]struct{}{}
	for _, a := range got {
		set[a] = struct{}{}
	}
	if _, ok := set["terranmarine"]; !ok {
		t.Fatalf("expected full normalized alias, got %v", got)
	}
	if _, ok := set["marine"]; !ok {
		t.Fatalf("expected race-stripped alias, got %v", got)
	}
	if got := unitNameAliases(""); got != nil {
		t.Fatalf("empty name should yield nil, got %v", got)
	}
}

func TestParseCommandUnitNames(t *testing.T) {
	one := nullStringValid("Marine")
	list := nullStringValid(`["Marine","Firebat"," Marine "]`)
	got := parseCommandUnitNames(one, list)
	want := []string{"Marine", "Firebat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCommandUnitNames = %v, want %v (dedup by normalized name)", got, want)
	}
}

func TestSplitSequence(t *testing.T) {
	if got := splitSequence(""); len(got) != 0 {
		t.Fatalf("empty sequence should yield empty slice, got %v", got)
	}
	got := splitSequence("Gateway -> Cybernetics Core -> Gateway")
	want := []string{"Gateway", "Cybernetics Core", "Gateway"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitSequence = %v, want %v", got, want)
	}
}

func TestBestSequence(t *testing.T) {
	if got := bestSequence(map[string]int64{}); got != "" {
		t.Fatalf("empty map should yield empty, got %q", got)
	}
	got := bestSequence(map[string]int64{"a": 3, "b": 3, "c": 1})
	if got != "a" {
		t.Fatalf("ties broken lexicographically, want a, got %q", got)
	}
	if got := bestSequence(map[string]int64{"z": 5, "a": 1}); got != "z" {
		t.Fatalf("highest count should win, want z, got %q", got)
	}
}

func TestPrettySplitUppercase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"CyberneticsCore", "Cybernetics Core"},
		{"", ""},
		{"  Gateway  ", "Gateway"},
		{"nineOverPool", "nine Over Pool"},
	}
	for _, c := range cases {
		if got := prettySplitUppercase(c.in); got != c.want {
			t.Errorf("prettySplitUppercase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSummarizeChatTokens(t *testing.T) {
	got := summarizeChatTokens("gg wp the Zerg is strong gg")
	want := []string{"gg", "zerg", "strong", "gg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summarizeChatTokens = %v, want %v (stopwords + <3 dropped, gg kept)", got, want)
	}
}

func TestSummarizeChatCounts(t *testing.T) {
	counts := map[string]int64{"zerg": 5, "rush": 5, "gg": 10, "zero": 0}
	got := summarizeChatCounts(counts, 2)
	if len(got) != 2 {
		t.Fatalf("expected cap of 2, got %d: %v", len(got), got)
	}
	if got[0].Term != "gg" || got[0].Count != 10 {
		t.Fatalf("expected gg first, got %+v", got[0])
	}
	if got[1].Term != "rush" {
		t.Fatalf("tie should sort by term asc, got %+v", got[1])
	}
}

func TestSummarizeChatExamples(t *testing.T) {
	if got := summarizeChatExamples(nil, 5); len(got) != 0 {
		t.Fatalf("nil should yield empty slice, got %v", got)
	}
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	got := summarizeChatExamples([]string{"  hi   there  ", "", long}, 5)
	if got[0] != "hi there" {
		t.Fatalf("expected whitespace collapse, got %q", got[0])
	}
	if len(got[1]) != 160 || got[1][157:] != "..." {
		t.Fatalf("expected truncation to 160 with ellipsis, got len=%d", len(got[1]))
	}
}

func TestBuildPlayerNarrativeHints(t *testing.T) {
	base := workflowPlayerOverview{PlayerName: "Bisu", GamesPlayed: 10, WinRate: 0.6}
	hints := buildPlayerNarrativeHints(base)
	if len(hints) != 1 {
		t.Fatalf("expected only the base hint, got %v", hints)
	}
	full := workflowPlayerOverview{PlayerName: "Bisu", GamesPlayed: 10, WinRate: 0.6, HotkeyUsageRate: 0.9, CarrierCommandCount: 3}
	hints = buildPlayerNarrativeHints(full)
	if len(hints) != 3 {
		t.Fatalf("expected 3 hints when hotkeys+carrier present, got %v", hints)
	}
}

func TestNeverEligibilityHelpers(t *testing.T) {
	if !neverAlliedMultiTeamEligible(2, 0) {
		t.Error("expected eligible with team games and no alliance commands")
	}
	if neverAlliedMultiTeamEligible(2, 1) {
		t.Error("alliance commands should disqualify")
	}
	if neverAlliedMultiTeamEligible(0, 0) {
		t.Error("no multi-team games should disqualify")
	}
	if !neverHotkeysEligible(1, 0) {
		t.Error("expected eligible with games and zero hotkey rate")
	}
	if neverHotkeysEligible(1, 0.1) {
		t.Error("non-zero hotkey rate should disqualify")
	}
	if neverHotkeysEligible(0, 0) {
		t.Error("no games should disqualify")
	}
}

func TestTeamFormatToClass(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1v1", ""}, {"", ""}, {"2v2", "2v2"}, {"3v3", "3v3"},
		{"2v2v2", "multi-team"}, {"4v4", ""},
	}
	for _, c := range cases {
		if got := teamFormatToClass(c.in); got != c.want {
			t.Errorf("teamFormatToClass(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOutlierPillLabel(t *testing.T) {
	if got := outlierPillLabel("Carrier", "Money"); got != "Carrier "+outlierMoneyBag {
		t.Errorf("money map should append bag, got %q", got)
	}
	if got := outlierPillLabel("Carrier", "Regular"); got != "Carrier" {
		t.Errorf("regular map should not append bag, got %q", got)
	}
}

func TestOutlierTechAndUpgradeToUnit(t *testing.T) {
	if got := outlierTechToUnit("Psionic Storm"); got != "High Templar" {
		t.Errorf("Psionic Storm -> %q, want High Templar", got)
	}
	if got := outlierTechToUnit("Nonexistent Tech"); got != "" {
		t.Errorf("unknown tech should be empty, got %q", got)
	}
	if got := outlierUpgradeToUnit("Singularity Charge (Dragoon Range)"); got != "Dragoon" {
		t.Errorf("parenthetical extraction failed, got %q", got)
	}
}

func TestFormatWorkflowPlayersLabelFromList(t *testing.T) {
	if got := formatWorkflowPlayersLabelFromList(nil); got != "" {
		t.Fatalf("empty players should yield empty label, got %q", got)
	}
	oneTeam := []workflowGameListPlayer{
		{Name: "A", Team: 1},
		{Name: "B", Team: 1},
	}
	if got := formatWorkflowPlayersLabelFromList(oneTeam); got != "A, B" {
		t.Fatalf("single team = %q, want \"A, B\"", got)
	}
	twoTeams := []workflowGameListPlayer{
		{Name: "A", Team: 1},
		{Name: "B", Team: 2},
	}
	if got := formatWorkflowPlayersLabelFromList(twoTeams); got != "A vs B" {
		t.Fatalf("1v1 = %q, want \"A vs B\"", got)
	}
	teamGame := []workflowGameListPlayer{
		{Name: "A", Team: 1},
		{Name: "B", Team: 1},
		{Name: "C", Team: 2},
		{Name: "D", Team: 2},
	}
	if got := formatWorkflowPlayersLabelFromList(teamGame); got != "A, B vs C, D" {
		t.Fatalf("2v2 = %q, want \"A, B vs C, D\"", got)
	}
}

func TestParseWorkflowUnitCadenceFilterMode(t *testing.T) {
	strict, err := parseWorkflowUnitCadenceFilterMode("")
	if err != nil || strict != workflowUnitCadenceFilterStrict {
		t.Fatalf("empty should default to strict, got %v err=%v", strict, err)
	}
	broad, err := parseWorkflowUnitCadenceFilterMode("BROAD")
	if err != nil || broad != workflowUnitCadenceFilterBroad {
		t.Fatalf("case-insensitive broad failed, got %v err=%v", broad, err)
	}
	if _, err := parseWorkflowUnitCadenceFilterMode("garbage"); err == nil {
		t.Fatal("expected error for invalid filter mode")
	}
}

func TestPrettyWorkflowRaceLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"protoss", "Protoss"}, {"TERRAN", "Terran"}, {" zerg ", "Zerg"},
		{"random", "Random"}, {"", "Random"},
	}
	for _, c := range cases {
		if got := prettyWorkflowRaceLabel(c.in); got != c.want {
			t.Errorf("prettyWorkflowRaceLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseCSVQueryValues(t *testing.T) {
	got := parseCSVQueryValues([]string{"a,B", "b", " c ,a"}, true)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCSVQueryValues lower+dedup = %v, want %v", got, want)
	}
	got = parseCSVQueryValues([]string{"Foo,Bar"}, false)
	want = []string{"Foo", "Bar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCSVQueryValues preserve-case = %v, want %v", got, want)
	}
	if got := parseCSVQueryValues(nil, true); len(got) != 0 {
		t.Fatalf("nil should yield empty, got %v", got)
	}
}

func TestBuildInClausePlaceholders(t *testing.T) {
	if got := buildInClausePlaceholders(0); got != "" {
		t.Errorf("size 0 = %q, want empty", got)
	}
	if got := buildInClausePlaceholders(-1); got != "" {
		t.Errorf("negative = %q, want empty", got)
	}
	if got := buildInClausePlaceholders(1); got != "?" {
		t.Errorf("size 1 = %q, want ?", got)
	}
	if got := buildInClausePlaceholders(3); got != "?, ?, ?" {
		t.Errorf("size 3 = %q, want \"?, ?, ?\"", got)
	}
}

func TestParseWorkflowPlayersListSort(t *testing.T) {
	cases := []struct {
		sortBy, sortDir string
		wantCol         string
		wantDesc        bool
	}{
		{"name", "asc", "player_name", false},
		{"apm", "desc", "average_apm", true},
		{"", "", "games_played", true},
		{"unknown", "asc", "games_played", false},
		{"last_played", "", "last_played_days_ago", true},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/?sort_by="+c.sortBy+"&sort_dir="+c.sortDir, nil)
		got := parseWorkflowPlayersListSort(req)
		if got.Column != c.wantCol || got.Desc != c.wantDesc {
			t.Errorf("sort_by=%q sort_dir=%q => %+v, want col=%q desc=%v", c.sortBy, c.sortDir, got, c.wantCol, c.wantDesc)
		}
	}
}

func TestParseWorkflowPlayersListFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/?name=+Bisu+&only_5_plus=true&last_played=1m,3m", nil)
	got := parseWorkflowPlayersListFilters(req)
	if got.NameContains != "Bisu" {
		t.Errorf("expected trimmed name, got %q", got.NameContains)
	}
	if !got.OnlyFivePlus {
		t.Error("expected only_5_plus true")
	}
	if !reflect.DeepEqual(got.LastPlayedBuckets, []string{"1m", "3m"}) {
		t.Errorf("last played buckets = %v", got.LastPlayedBuckets)
	}

	req = httptest.NewRequest("GET", "/?only_5_plus=0", nil)
	if parseWorkflowPlayersListFilters(req).OnlyFivePlus {
		t.Error("only_5_plus=0 should be false")
	}
}

func TestParseOptionalQueryHelpers(t *testing.T) {
	req := httptest.NewRequest("GET", "/?n=42&f=3.5&bad=x", nil)
	if v, ok := parseOptionalInt64Query(req, "n"); !ok || v != 42 {
		t.Errorf("int64 n = %d ok=%v", v, ok)
	}
	if _, ok := parseOptionalInt64Query(req, "bad"); ok {
		t.Error("non-numeric should be ok=false")
	}
	if _, ok := parseOptionalInt64Query(req, "missing"); ok {
		t.Error("missing key should be ok=false")
	}
	if v, ok := parseOptionalFloatQuery(req, "f"); !ok || v != 3.5 {
		t.Errorf("float f = %v ok=%v", v, ok)
	}
	if _, ok := parseOptionalFloatQuery(req, "bad"); ok {
		t.Error("non-numeric float should be ok=false")
	}
}

func TestParseReplayID(t *testing.T) {
	if _, err := parseReplayID(""); err == nil {
		t.Error("empty should error")
	}
	if _, err := parseReplayID("abc"); err == nil {
		t.Error("non-numeric should error")
	}
	id, err := parseReplayID("123")
	if err != nil || id != 123 {
		t.Errorf("parseReplayID(123) = %d err=%v", id, err)
	}
}

func TestParsePagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=10&offset=5", nil)
	if l, o := parsePagination(req, 25, 100); l != 10 || o != 5 {
		t.Errorf("got limit=%d offset=%d, want 10/5", l, o)
	}
	req = httptest.NewRequest("GET", "/?limit=500", nil)
	if l, _ := parsePagination(req, 25, 100); l != 100 {
		t.Errorf("limit should cap at max, got %d", l)
	}
	req = httptest.NewRequest("GET", "/?limit=-1&offset=-1", nil)
	if l, o := parsePagination(req, 25, 100); l != 25 || o != 0 {
		t.Errorf("invalid values should fall back to defaults, got limit=%d offset=%d", l, o)
	}
}

func TestNormalizePlayerKey(t *testing.T) {
	if got := normalizePlayerKey("  BiSu  "); got != "bisu" {
		t.Errorf("normalizePlayerKey = %q, want bisu", got)
	}
}

func TestDecodeAskQuestion(t *testing.T) {
	req := httptest.NewRequest("POST", "/", jsonBody(`{"question":"  what build?  "}`))
	q, err := decodeAskQuestion(req)
	if err != nil || q != "what build?" {
		t.Fatalf("decodeAskQuestion = %q err=%v", q, err)
	}
	req = httptest.NewRequest("POST", "/", jsonBody(`{"question":"  "}`))
	if _, err := decodeAskQuestion(req); err == nil {
		t.Error("blank question should error")
	}
	req = httptest.NewRequest("POST", "/", jsonBody(`not json`))
	if _, err := decodeAskQuestion(req); err == nil {
		t.Error("invalid body should error")
	}
}

func TestFormatDisplayNameWithAlias(t *testing.T) {
	cases := []struct {
		name, alias, want string
	}{
		{"Bisu", "you", "Bisu (you)"},
		{"Bisu", "", "Bisu"},
		{"", "you", ""},
		{"you", "you", "you"},
		{"Bisu (you)", "you", "Bisu (you)"},
	}
	for _, c := range cases {
		if got := formatDisplayNameWithAlias(c.name, c.alias); got != c.want {
			t.Errorf("formatDisplayNameWithAlias(%q,%q) = %q, want %q", c.name, c.alias, got, c.want)
		}
	}
}

func TestAliasSourcePriority(t *testing.T) {
	if aliasSourcePriority(aliasSourceYou) <= aliasSourcePriority(aliasSourceManual) {
		t.Error("you should outrank manual")
	}
	if aliasSourcePriority(aliasSourceManual) <= aliasSourcePriority(aliasSourceImported) {
		t.Error("manual should outrank imported")
	}
	if aliasSourcePriority("garbage") != 0 {
		t.Error("unknown source should be 0")
	}
}

func TestChooseBetterAliasTieBreakByUpdatedThenName(t *testing.T) {
	current := aliasRow("Zeta", aliasSourceManual, "2020-01-01")
	newer := aliasRow("Alpha", aliasSourceManual, "2021-01-01")
	if !chooseBetterAlias(&current, newer) {
		t.Error("newer updated_at should win at equal priority")
	}
	sameTime := aliasRow("Alpha", aliasSourceManual, "2020-01-01")
	if !chooseBetterAlias(&current, sameTime) {
		t.Error("lexicographically smaller canonical should win on full tie")
	}
	if chooseBetterAlias(nil, sameTime) != true {
		t.Error("nil current should always be beaten")
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("/a/b/../b/c", "/a/b/c") {
		t.Error("cleaned equivalent paths should match")
	}
	if samePath("/a/b", "/a/c") {
		t.Error("different paths should not match")
	}
	if !samePath("  /x  ", "/x") {
		t.Error("whitespace should be trimmed before comparison")
	}
}

func TestResolveWorkflowMapVisual(t *testing.T) {
	d := &Dashboard{}
	if v := d.resolveWorkflowMapVisual(1, "", "/path.rep", "ck"); v.Available || v.ResolutionNote == "" {
		t.Fatalf("empty map name should be unavailable with note, got %+v", v)
	}
	if v := d.resolveWorkflowMapVisual(0, "Fighting Spirit", "/path.rep", "ck"); v.Available {
		t.Fatalf("missing replay id should be unavailable, got %+v", v)
	}
	if v := d.resolveWorkflowMapVisual(5, "Fighting Spirit", "", "ck"); v.Available {
		t.Fatalf("missing file path should be unavailable, got %+v", v)
	}
	v := d.resolveWorkflowMapVisual(5, "Fighting Spirit", "/path.rep", "abc")
	if !v.Available || v.URL == "" || v.MatchedImage != "rendered" {
		t.Fatalf("expected available rendered visual, got %+v", v)
	}
	if v.URL != v.ThumbnailURL {
		t.Fatalf("url and thumbnail should match, got %q vs %q", v.URL, v.ThumbnailURL)
	}
}

func TestLegacyGlobalReplayFilterGameTypeValue(t *testing.T) {
	single := globalReplayFilterConfig{GameTypes: []string{globalReplayFilterGameTypeMelee}}
	if got := legacyGlobalReplayFilterGameTypeValue(single); got != globalReplayFilterGameTypeMelee {
		t.Errorf("single game type = %q, want melee", got)
	}
	multi := globalReplayFilterConfig{GameTypes: []string{globalReplayFilterGameTypeMelee, globalReplayFilterGameTypeOneOnOne}}
	if got := legacyGlobalReplayFilterGameTypeValue(multi); got != "all" {
		t.Errorf("multi game type = %q, want all", got)
	}
	if got := legacyGlobalReplayFilterGameTypeValue(globalReplayFilterConfig{}); got != "all" {
		t.Errorf("empty = %q, want all", got)
	}
}

func TestNormalizeGlobalReplayFilterConfigRejectsInvalid(t *testing.T) {
	if _, err := normalizeGlobalReplayFilterConfig(globalReplayFilterConfig{GameTypes: []string{"bogus"}}); err == nil {
		t.Error("invalid game type should error")
	}
	if _, err := normalizeGlobalReplayFilterConfig(globalReplayFilterConfig{MapKinds: []string{"lava"}}); err == nil {
		t.Error("invalid map kind should error")
	}
	got, err := normalizeGlobalReplayFilterConfig(globalReplayFilterConfig{
		GameTypes: []string{"MELEE", " melee ", "one_on_one"},
	})
	if err != nil {
		t.Fatalf("valid config errored: %v", err)
	}
	if !reflect.DeepEqual(got.GameTypes, []string{"melee", "one_on_one"}) {
		t.Fatalf("expected dedup+lower+sort, got %v", got.GameTypes)
	}
	if got.CompiledReplaysFilterSQL == nil || *got.CompiledReplaysFilterSQL == "" {
		t.Fatal("expected compiled SQL to be populated")
	}
}

func TestUnmarshalStringSlice(t *testing.T) {
	if got, err := unmarshalStringSlice(""); err != nil || len(got) != 0 {
		t.Fatalf("empty = %v err=%v", got, err)
	}
	if got, err := unmarshalStringSlice("null"); err != nil || len(got) != 0 {
		t.Fatalf("null should yield empty slice, got %v err=%v", got, err)
	}
	got, err := unmarshalStringSlice(`["a","b"]`)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unmarshal = %v err=%v", got, err)
	}
	if _, err := unmarshalStringSlice("not json"); err == nil {
		t.Error("invalid json should error")
	}
}

func TestComposeReplayFilterSQL(t *testing.T) {
	if got := composeReplayFilterSQL(nil, nil); got != nil {
		t.Fatalf("both nil should yield nil, got %v", *got)
	}
	g := "g_sql"
	if got := composeReplayFilterSQL(&g, nil); got == nil || *got == "" {
		t.Fatalf("global-only should compose non-empty, got %v", got)
	}
}

func nullStringValid(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

func jsonBody(s string) io.Reader {
	return bytes.NewReader([]byte(s))
}
