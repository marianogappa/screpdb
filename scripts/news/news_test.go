package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSiteNewsIsValid(t *testing.T) {
	data, err := os.ReadFile("../../site/news.json")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseNews(data)
	if err != nil {
		t.Fatalf("site/news.json: %v", err)
	}
	for _, p := range validate(entries) {
		t.Error(p)
	}
}

func TestValidateCatchesStyleProblems(t *testing.T) {
	entries := []Entry{
		{Date: "2026-01-01", Kind: "release", Version: "1.0.0", Title: "Old", Items: []string{"ok"}},
		{Date: "2026-02-01", Kind: "release", Version: "1.0.0", Title: "Faster ingest — now", Items: []string{"Fixed earlyfilter"}},
		{Date: "2026-01-01", Kind: "video", Title: "No url"},
		{Date: "2026-01-01", Kind: "podcast", Title: "Unknown"},
	}
	got := strings.Join(validate(entries), "\n")
	for _, want := range []string{"newer than the entry above", "appears twice", "dash", `"ingest"`, `"earlyfilter"`, "https url", "podcast"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected a problem mentioning %s, got:\n%s", want, got)
		}
	}
}

func TestDraftCleansChangelogLines(t *testing.T) {
	changelog := `# Changelog

## [2.6.0](https://github.com/marianogappa/screpdb/compare/v2.5.0...v2.6.0) (2026-12-01)


### New

* **dashboard:** search games by map ([#550](https://github.com/marianogappa/screpdb/issues/550)) ([abc1234](https://github.com/marianogappa/screpdb/commit/abc1234))


### Fixed

* Bunker unloads no longer show up as drops ([#551](https://github.com/marianogappa/screpdb/issues/551)) ([def5678](https://github.com/marianogappa/screpdb/commit/def5678))

## [2.5.0](https://github.com/marianogappa/screpdb/compare/v2.4.0...v2.5.0) (2026-11-22)

### New

* older feature
`
	entry, err := draft(changelog, nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != "2.6.0" || entry.Date != "2026-12-01" {
		t.Fatalf("got version %s date %s", entry.Version, entry.Date)
	}
	want := []string{"Search games by map.", "Fixed: bunker unloads no longer show up as drops."}
	if strings.Join(entry.Items, "|") != strings.Join(want, "|") {
		t.Fatalf("items = %q, want %q", entry.Items, want)
	}
	if _, err := draft(changelog, []Entry{{Kind: "release", Version: "2.6.0"}}); err == nil {
		t.Fatal("expected an error when the release already has an entry")
	}
}

func TestSiteNewsMatchesReleases(t *testing.T) {
	data, err := os.ReadFile("../../site/news.json")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseNews(data)
	if err != nil {
		t.Fatalf("site/news.json: %v", err)
	}
	changelog, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile("../../.release-please-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, p := range checkSync(string(changelog), manifest["."], entries) {
		t.Error(p)
	}
}

func TestCheckSyncCatchesDrift(t *testing.T) {
	changelog := `## [2.6.0](x) (2026-12-01)
## [2.5.0](x) (2026-11-22)
`
	entries := []Entry{
		{Date: "2026-11-23", Kind: "release", Version: "2.5.0", Title: "TODO: the one thing"},
		{Date: "2026-11-22", Kind: "release", Version: "9.9.9", Title: "Ghost"},
		{Date: "2025-09-30", Kind: "release", Version: "0.1.0", Title: "Before the changelog"},
	}
	got := strings.Join(checkSync(changelog, "2.7.0", entries), "\n")
	for _, want := range []string{"manifest.json says 2.7.0", "2.6.0 has no entry", "dated 2026-11-23", "9.9.9 is not in CHANGELOG", "TODO"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected a problem mentioning %s, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "0.1.0") {
		t.Errorf("releases older than the changelog should be allowed, got:\n%s", got)
	}
	ok := []Entry{{Date: "2026-12-01", Kind: "release", Version: "2.6.0", Title: "Fine"}}
	if p := checkSync(changelog, "2.6.0", ok); len(p) != 0 {
		t.Errorf("expected no problems, got %q", p)
	}
}
