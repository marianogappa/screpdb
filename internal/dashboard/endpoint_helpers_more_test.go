package dashboard

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestOutlierIconKey(t *testing.T) {
	if got := outlierIconKey("Build", "Barracks"); got == "" {
		t.Error("Build Barracks should resolve to an icon key")
	}
	if got := outlierIconKey("Tech", "Psionic Storm"); got == "" {
		t.Error("Tech Psionic Storm should map through High Templar to an icon key")
	}
	if got := outlierIconKey("Upgrade", "Singularity Charge (Dragoon Range)"); got == "" {
		t.Error("Upgrade should map through Dragoon to an icon key")
	}
	if got := outlierIconKey("Tech", "Nonexistent Tech"); got != "" {
		t.Errorf("unknown tech should yield empty icon key, got %q", got)
	}
	if got := outlierIconKey("UnknownCategory", "x"); got != "" {
		t.Errorf("unknown category should yield empty icon key, got %q", got)
	}
}

func TestOutlierUpgradeToUnit_MapAndParen(t *testing.T) {
	if got := outlierUpgradeToUnit("Terran Infantry Weapons"); got != "Marine" {
		t.Errorf("mapped upgrade = %q, want Marine", got)
	}
	if got := outlierUpgradeToUnit("Grooved Spines (Hydralisk Range)"); got != "Hydralisk" {
		t.Errorf("pinned parenthetical = %q, want Hydralisk", got)
	}
	if got := outlierUpgradeToUnit("Made Up (Foobar Speed)"); got != "Foobar" {
		t.Errorf("generic parenthetical extraction = %q, want Foobar", got)
	}
	if got := outlierUpgradeToUnit("No Parens Unknown"); got != "" {
		t.Errorf("unknown with no parens should be empty, got %q", got)
	}
}

func TestWorkflowOutlierSpecsAndLookup(t *testing.T) {
	specs := workflowOutlierSpecs()
	if len(specs) != 6 {
		t.Fatalf("expected 6 outlier specs, got %d", len(specs))
	}
	spec, ok := lookupOutlierSpec("build")
	if !ok || spec.CategoryLabel != "Build" {
		t.Fatalf("case-insensitive lookup of build failed: %+v ok=%v", spec, ok)
	}
	if spec.NameColumn != "unit_type" {
		t.Fatalf("Build spec NameColumn = %q, want unit_type", spec.NameColumn)
	}
	orderSpec, ok := lookupOutlierSpec("Order")
	if !ok || !orderSpec.UseInstanceShare {
		t.Fatalf("Order spec should use instance share: %+v ok=%v", orderSpec, ok)
	}
	if _, ok := lookupOutlierSpec("nope"); ok {
		t.Error("unknown category should not resolve")
	}
}

func TestWorkflowCanonicalOutlierName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Attack Move", "attackmove"},
		{"  Cast: Psionic Storm  ", "castpsionicstorm"},
		{"Siege Tank (Tank Mode)", "siegetanktankmode"},
		{"", ""},
		{"!!!", ""},
	}
	for _, c := range cases {
		if got := workflowCanonicalOutlierName(c.in); got != c.want {
			t.Errorf("workflowCanonicalOutlierName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWorkflowSkipGenericTargetedOrder(t *testing.T) {
	for _, name := range []string{"Attack Move", "attack1", "Move", "Patrol", "Stop", "Hold Position"} {
		if !workflowSkipGenericTargetedOrder(name) {
			t.Errorf("expected %q to be skipped as generic", name)
		}
	}
	for _, name := range []string{"Cast Psionic Storm", "Build", ""} {
		if workflowSkipGenericTargetedOrder(name) {
			t.Errorf("expected %q not to be skipped", name)
		}
	}
}

func TestWorkflowItemAllowedForPrimaryRace(t *testing.T) {
	techSpec := workflowOutlierCategorySpec{CategoryLabel: "Tech"}
	orderSpec := workflowOutlierCategorySpec{CategoryLabel: "Order"}

	if !workflowItemAllowedForPrimaryRace("Terran", techSpec, "Stim Packs") {
		t.Error("non-Protoss race should allow everything")
	}
	if workflowItemAllowedForPrimaryRace("Protoss", techSpec, "Stim Packs") {
		t.Error("Protoss should reject a Terran tech")
	}
	if !workflowItemAllowedForPrimaryRace("Protoss", orderSpec, "Attack Move") {
		t.Error("Protoss should keep generic non-cast orders")
	}
	if workflowItemAllowedForPrimaryRace("Protoss", techSpec, "!!!") {
		t.Error("Protoss with an empty canonical name should be rejected")
	}
}

func TestPrimaryRaceFromBreakdown(t *testing.T) {
	if got := primaryRaceFromBreakdown(nil); got != "" {
		t.Errorf("empty breakdown = %q, want empty", got)
	}
	breakdown := []workflowPlayerRaceBreakdown{
		{Race: "Terran", GameCount: 3},
		{Race: "Protoss", GameCount: 10},
		{Race: "Zerg", GameCount: 5},
	}
	if got := primaryRaceFromBreakdown(breakdown); got != "Protoss" {
		t.Errorf("primary race = %q, want Protoss", got)
	}
}

func TestPlayerNameByID(t *testing.T) {
	players := []workflowGamePlayer{
		{PlayerID: 1, Name: "Bisu"},
		{PlayerID: 2, Name: "Flash"},
	}
	if got := playerNameByID(2, players); got != "Flash" {
		t.Errorf("playerNameByID(2) = %q, want Flash", got)
	}
	if got := playerNameByID(99, players); got != "" {
		t.Errorf("unknown id should be empty, got %q", got)
	}
}

func TestBuildComparativeMetric(t *testing.T) {
	values := map[string]float64{"bisu": 3.5}
	got := buildComparativeMetric("apm", "bisu", values)
	if got.Metric != "apm" || got.PlayerValue != 3.5 {
		t.Fatalf("buildComparativeMetric = %+v", got)
	}
	missing := buildComparativeMetric("apm", "nobody", values)
	if missing.PlayerValue != 0 {
		t.Fatalf("missing player value should be 0, got %v", missing.PlayerValue)
	}
}

func TestStringifyByteKeys(t *testing.T) {
	if got := stringifyByteKeys(nil); got != nil {
		t.Errorf("nil map should yield nil, got %v", got)
	}
	got := stringifyByteKeys(map[byte]int{0: 5, 3: 7})
	want := map[string]int{"0": 5, "3": 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stringifyByteKeys = %v, want %v", got, want)
	}
}

func TestStringifyIntKeysByte(t *testing.T) {
	if got := stringifyIntKeysByte(nil); got != nil {
		t.Errorf("nil map should yield nil, got %v", got)
	}
	got := stringifyIntKeysByte(map[int]byte{2: 4, 5: 9})
	want := map[string]int{"2": 4, "5": 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stringifyIntKeysByte = %v, want %v", got, want)
	}
}

func TestMeanAndStddevFloatSlice(t *testing.T) {
	if got := meanFloatSlice(nil); got != 0 {
		t.Errorf("mean of nil = %v, want 0", got)
	}
	values := []float64{2, 4, 6}
	mean := meanFloatSlice(values)
	if mean != 4 {
		t.Fatalf("mean = %v, want 4", mean)
	}
	if got := stddevFloatSlice(nil, 0); got != 0 {
		t.Errorf("stddev of nil = %v, want 0", got)
	}
	stddev := stddevFloatSlice(values, mean)
	want := math.Sqrt((4 + 0 + 4) / 3.0)
	if math.Abs(stddev-want) > 1e-9 {
		t.Fatalf("stddev = %v, want %v", stddev, want)
	}
}

func TestWorkflowViewportSwitchPopulationStats(t *testing.T) {
	if mean, std := workflowViewportSwitchPopulationStats(nil); mean != 0 || std != 0 {
		t.Fatalf("empty population = %v/%v, want 0/0", mean, std)
	}
	players := []workflowViewportMultitaskingAggregate{
		{averageViewportSwitchRate: 10},
		{averageViewportSwitchRate: 20},
	}
	mean, std := workflowViewportSwitchPopulationStats(players)
	if mean != 15 {
		t.Fatalf("mean = %v, want 15", mean)
	}
	if math.Abs(std-5) > 1e-9 {
		t.Fatalf("stddev = %v, want 5", std)
	}
}

func TestFindWorkflowViewportMultitaskingAggregate(t *testing.T) {
	all := []workflowViewportMultitaskingAggregate{
		{PlayerKey: "bisu"},
		{PlayerKey: "flash"},
	}
	got, ok := findWorkflowViewportMultitaskingAggregate(all, "flash")
	if !ok || got.PlayerKey != "flash" {
		t.Fatalf("expected to find flash, got %+v ok=%v", got, ok)
	}
	if _, ok := findWorkflowViewportMultitaskingAggregate(all, "nobody"); ok {
		t.Error("unknown key should not be found")
	}
}

func TestParseViewportSwitchRate(t *testing.T) {
	if v, ok := parseViewportSwitchRate(`{"switches_per_minute":12.5}`); !ok || v != 12.5 {
		t.Errorf("json payload = %v ok=%v, want 12.5", v, ok)
	}
	if v, ok := parseViewportSwitchRate("7.25"); !ok || v != 7.25 {
		t.Errorf("legacy float = %v ok=%v, want 7.25", v, ok)
	}
	if _, ok := parseViewportSwitchRate("   "); ok {
		t.Error("blank should be ok=false")
	}
	if _, ok := parseViewportSwitchRate("not-a-number"); ok {
		t.Error("garbage should be ok=false")
	}
}

func TestFormatQueryResults(t *testing.T) {
	if got := formatQueryResults(nil); got != "No results found." {
		t.Errorf("empty = %q, want \"No results found.\"", got)
	}
	out := formatQueryResults([]map[string]any{{"name": "Bisu"}})
	if !strings.Contains(out, "Query Results:") {
		t.Errorf("missing header: %q", out)
	}
	if !strings.Contains(out, "name") || !strings.Contains(out, "Bisu") {
		t.Errorf("missing column/value: %q", out)
	}
	if !strings.Contains(out, "Total rows: 1") {
		t.Errorf("missing row count: %q", out)
	}
}

func TestAllGameEventFeaturesAndFeaturingOrder(t *testing.T) {
	features := AllGameEventFeatures()
	if len(features) == 0 {
		t.Fatal("expected non-empty game event features")
	}
	for _, f := range features {
		if f.Key == "" {
			t.Fatalf("feature has empty key: %+v", f)
		}
	}

	order := FeaturingOrder()
	if len(order) == 0 {
		t.Fatal("expected non-empty featuring order")
	}
	order[0] = "__mutated__"
	if FeaturingOrder()[0] == "__mutated__" {
		t.Fatal("FeaturingOrder must return a defensive copy")
	}
}
