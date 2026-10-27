package dashboard

import (
	"reflect"
	"testing"
)

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
