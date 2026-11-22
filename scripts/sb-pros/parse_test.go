package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func snapshot(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/post.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func byLabel(t *testing.T, entries []tlEntry) map[string]tlEntry {
	t.Helper()
	m := map[string]tlEntry{}
	for _, e := range entries {
		if _, dup := m[e.Label]; dup {
			t.Fatalf("duplicate label %q", e.Label)
		}
		m[e.Label] = e
	}
	return m
}

func TestParseSnapshot(t *testing.T) {
	entries, err := parsePage(snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	got := byLabel(t, entries)

	want := map[string][]tlAccount{
		"BishOp":   {{ID: 17185, Name: "K_BishOp"}},
		"Mong":     {{ID: 17745, Name: "Mong"}},
		"Rich":     {{ID: 19217, Name: "DolphinShorts", Formerly: []tlAccount{{ID: 17285, Name: "Rich12345"}, {ID: 16414, Name: "naughty"}}}},
		"SHinee":   {{ID: 17981, Name: "SHinee.", Formerly: []tlAccount{{ID: 17640, Name: "SHinee"}}}},
		"organ":    {{ID: 19870, Name: "Ever)T(PianO"}},
		"Larva":    {{ID: 18561, Name: "lhg719"}},
		"Tulbo":    {{ID: 17622, Name: "teolbo"}},
		"Barracks": {{ID: 18211, Name: "newchampion"}},
	}
	for label, accs := range want {
		e, ok := got[label]
		if !ok {
			t.Errorf("%s: missing", label)
			continue
		}
		if !reflect.DeepEqual(e.Accounts, accs) {
			t.Errorf("%s: got %+v, want %+v", label, e.Accounts, accs)
		}
	}

	races := map[string]string{"BishOp": "terran", "JD": "zerg", "Tulbo": "protoss"}
	for label, race := range races {
		if got[label].Race != race {
			t.Errorf("%s: race %q, want %q", label, got[label].Race, race)
		}
	}

	if len(entries) != 43 {
		t.Errorf("got %d entries, want 43", len(entries))
	}
	for _, shortlisted := range []string{"Bisu", "FlaSh", "UPDATE 1"} {
		if _, ok := got[shortlisted]; ok {
			t.Errorf("%s comes from the trailing sections and must be ignored", shortlisted)
		}
	}
	for _, a := range allAccounts(entries) {
		if a.ID == 18003 {
			t.Error("the potentially account 18003 must not be confirmed")
		}
	}
}

func TestParseGuards(t *testing.T) {
	post := snapshot(t)
	cases := map[string]string{
		"no forumPost":     strings.Replace(post, `class="forumPost"`, `class="other"`, 1),
		"no race headings": strings.NewReplacer("I. TERRAN", "Terran", "II. ZERG", "Zerg", "III. PROTOSS", "Protoss").Replace(post),
		"non-numeric id":   strings.Replace(post, "/users/17185/", "/users/abc/", 1),
		"too few accounts": truncateAfter(post, "Light:"),
	}
	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePage(page); !errors.Is(err, errFormatChanged) {
				t.Fatalf("got %v, want errFormatChanged", err)
			}
		})
	}
}

func truncateAfter(post, marker string) string {
	i := strings.Index(post, marker)
	return post[:i] + "</section></article>"
}

func TestVanishingGuard(t *testing.T) {
	entries, err := parsePage(snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	committed := &committedFile{parsed: output{Pros: map[string][]account{
		"bishop": {{ID: 17185}},
		"mong":   {{ID: 17745}},
		"rich":   {{ID: 19217}},
		"scan":   {{ID: 766}},
		"ghost":  {{ID: 1}},
	}}}
	if err := checkVanishing(committed, entries); err != nil {
		t.Fatalf("one of five vanishing must pass: %v", err)
	}
	committed.parsed.Pros["ghost2"] = []account{{ID: 2}}
	if err := checkVanishing(committed, entries); !errors.Is(err, errFormatChanged) {
		t.Fatalf("two of six vanishing must fail, got %v", err)
	}
	if err := checkVanishing(nil, entries); err != nil {
		t.Fatalf("no committed file must pass: %v", err)
	}
}

func TestBuildMapsLabels(t *testing.T) {
	entries, err := parsePage(snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	pack := []packPro{
		{ID: "bishop", Label: "BishOp"},
		{ID: "jaedong", Label: "Jaedong", Liquipedia: "https://liquipedia.net/starcraft/Jaedong"},
		{ID: "larva", Label: "Larva", Liquipedia: "https://liquipedia.net/starcraft/Larva_(Player)"},
		{ID: "effort", Label: "Effort"},
		{ID: "true", Label: "True", Liquipedia: "https://liquipedia.net/starcraft/BTS"},
	}
	out, unmatched := build(entries, pack, map[uint32]string{18561: "by.larva"})
	want := map[string][]account{
		"bishop":  {{ID: 17185, Name: "K_BishOp"}},
		"jaedong": {{ID: 17046, Name: "JaedongLee"}},
		"larva":   {{ID: 18561, Name: "by.larva"}},
		"effort":  {{ID: 18384, Name: "maesaeggi"}},
	}
	if !reflect.DeepEqual(out.Pros, want) {
		t.Fatalf("got %+v, want %+v", out.Pros, want)
	}
	if len(unmatched) != len(entries)-len(want) {
		t.Fatalf("got %d unmatched, want %d", len(unmatched), len(entries)-len(want))
	}

	a, _ := encode(out)
	b, _ := encode(out)
	if string(a) != string(b) {
		t.Fatal("encoding is not deterministic")
	}
}
