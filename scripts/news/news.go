package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Localized struct {
	Title   string   `json:"title,omitempty"`
	Items   []string `json:"items,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

type Entry struct {
	Date    string     `json:"date"`
	Kind    string     `json:"kind"`
	Version string     `json:"version,omitempty"`
	Title   string     `json:"title"`
	Items   []string   `json:"items,omitempty"`
	Summary string     `json:"summary,omitempty"`
	URL     string     `json:"url,omitempty"`
	Ko      *Localized `json:"ko,omitempty"`
}

var (
	versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	dashRe    = regexp.MustCompile(`[—–]| - `)
	jargonRe  = regexp.MustCompile(`(?i)\b(earlyfilter|builddedup|worldstate|facades?|ingest\w*|digests?|corpus|refactor\w*|sqlite|wasm|mcp|toons?|markers?|pills?)\b`)
)

func parseNews(data []byte) ([]Entry, error) {
	var entries []Entry
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func validate(entries []Entry) []string {
	var problems []string
	add := func(i int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("entry %d (%s): %s", i, entries[i].Title, fmt.Sprintf(format, args...)))
	}
	seen := map[string]bool{}
	var prev time.Time
	for i, e := range entries {
		date, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			add(i, "date %q is not YYYY-MM-DD", e.Date)
		} else if i > 0 && date.After(prev) {
			add(i, "date %s is newer than the entry above it; keep newest first", e.Date)
		}
		prev = date
		if strings.TrimSpace(e.Title) == "" {
			add(i, "missing title")
		}
		switch e.Kind {
		case "release":
			if !versionRe.MatchString(e.Version) {
				add(i, "release version %q is not X.Y.Z", e.Version)
			}
			if seen[e.Version] {
				add(i, "version %s appears twice", e.Version)
			}
			seen[e.Version] = true
			if len(e.Items) == 0 {
				add(i, "release has no items")
			}
		case "video", "article":
			if !strings.HasPrefix(e.URL, "https://") {
				add(i, "%s needs an https url", e.Kind)
			}
		default:
			add(i, "kind %q is not release, video or article", e.Kind)
		}
		for _, text := range texts(e) {
			if dashRe.MatchString(text) {
				add(i, "uses a dash as punctuation: %q", text)
			}
			if m := jargonRe.FindString(text); m != "" {
				add(i, "uses the internal word %q: %q", m, text)
			}
		}
	}
	return problems
}

func texts(e Entry) []string {
	out := append([]string{e.Title, e.Summary}, e.Items...)
	if e.Ko != nil {
		out = append(out, e.Ko.Title, e.Ko.Summary)
		out = append(out, e.Ko.Items...)
	}
	return out
}

type release struct {
	Version string
	Date    string
	New     []string
	Fixed   []string
}

var (
	releaseHeadRe = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\].*\((\d{4}-\d{2}-\d{2})\)\s*$`)
	scopeRe       = regexp.MustCompile(`^\*\*[^*]+:\*\*\s*`)
	linkRe        = regexp.MustCompile(`\s*\(\[[^\]]*\]\([^)]*\)\)|,?\s*closes \[#\d+\]\([^)]*\)|\[#\d+\]\([^)]*\)`)
)

func latestRelease(changelog string) (release, bool) {
	var r release
	section := ""
	for line := range strings.SplitSeq(changelog, "\n") {
		if m := releaseHeadRe.FindStringSubmatch(line); m != nil {
			if r.Version != "" {
				break
			}
			r.Version, r.Date = m[1], m[2]
			continue
		}
		if r.Version == "" {
			continue
		}
		if name, ok := strings.CutPrefix(line, "### "); ok {
			section = name
			continue
		}
		raw, ok := strings.CutPrefix(line, "* ")
		if !ok {
			continue
		}
		item := cleanItem(raw)
		switch section {
		case "Features", "New", "Performance Improvements", "Faster":
			r.New = append(r.New, item)
		case "Bug Fixes", "Fixed":
			r.Fixed = append(r.Fixed, item)
		}
	}
	return r, r.Version != ""
}

func cleanItem(s string) string {
	s = scopeRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(linkRe.ReplaceAllString(s, ""))
	if s == "" {
		return s
	}
	s = strings.ToUpper(s[:1]) + s[1:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

func draft(changelog string, entries []Entry) (Entry, error) {
	r, ok := latestRelease(changelog)
	if !ok {
		return Entry{}, fmt.Errorf("no release found in CHANGELOG.md")
	}
	for _, e := range entries {
		if e.Kind == "release" && e.Version == r.Version {
			return Entry{}, fmt.Errorf("%s already has a news entry", r.Version)
		}
	}
	items := append([]string{}, r.New...)
	for _, f := range r.Fixed {
		items = append(items, "Fixed: "+strings.ToLower(f[:1])+f[1:])
	}
	return Entry{Date: r.Date, Kind: "release", Version: r.Version, Title: "TODO: the one thing worth telling a friend", Items: items}, nil
}

func releaseDates(changelog string) (dates map[string]string, latest, oldestDate string) {
	dates = map[string]string{}
	for line := range strings.SplitSeq(changelog, "\n") {
		if m := releaseHeadRe.FindStringSubmatch(line); m != nil {
			dates[m[1]] = m[2]
			if latest == "" {
				latest = m[1]
			}
			oldestDate = m[2]
		}
	}
	return dates, latest, oldestDate
}

func checkSync(changelog, manifestVersion string, entries []Entry) []string {
	var problems []string
	dates, latest, oldestDate := releaseDates(changelog)
	if manifestVersion != "" && manifestVersion != latest {
		problems = append(problems, fmt.Sprintf(".release-please-manifest.json says %s but the newest CHANGELOG.md release is %s", manifestVersion, latest))
	}
	hasLatest := false
	for _, e := range entries {
		if strings.Contains(e.Title, "TODO") {
			problems = append(problems, fmt.Sprintf("%q still has the draft TODO title", e.Title))
		}
		if e.Kind != "release" {
			continue
		}
		if e.Version == latest {
			hasLatest = true
		}
		date, ok := dates[e.Version]
		switch {
		case !ok && e.Date >= oldestDate:
			problems = append(problems, fmt.Sprintf("release %s is not in CHANGELOG.md", e.Version))
		case ok && date != e.Date:
			problems = append(problems, fmt.Sprintf("release %s is dated %s but CHANGELOG.md says %s", e.Version, e.Date, date))
		}
	}
	if latest != "" && !hasLatest {
		problems = append(problems, fmt.Sprintf("release %s has no entry in site/news.json; run `go run ./scripts/news draft` and rewrite it for players (docs/WRITING_FOR_PLAYERS.md)", latest))
	}
	return problems
}
