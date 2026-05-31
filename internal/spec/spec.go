// Package spec generates SPECIFICATION.md — the human-readable, machine-decodable
// catalogue of every "golden value" screpdb relies on to make its derived claims
// (build-order names, expert timings, unit costs, build times, detection
// thresholds, …).
//
// The document is GENERATED from the real Go constants and tables (this package
// imports them and reads their live values), so it can never drift from the
// code. A guard test (spec_guard_test.go) regenerates the document and diffs it
// against the committed SPECIFICATION.md, failing CI if they differ. A coverage
// test (coverage_test.go) asserts every registered section is non-empty and
// carries an import-based Verify assertion. Together: if the spec is wrong, CI
// is red.
//
// Adding a new golden table is a small, local change: register a new Section
// (see sections_models.go / sections_detection.go) that reads the table's
// exported enumerator and provide a Verify that asserts its values.
package spec

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Section is one self-contained block of the specification: a prose intro
// followed by a single fixed-column table. Sections register themselves via
// Register (typically from an init function) and are emitted in Key order so
// the document is deterministic and diffs are minimal.
type Section struct {
	// Key controls ordering (sections are emitted sorted by Key) and must be
	// unique. Use a numeric prefix to group/order (e.g. "03-build-times").
	Key string
	// Title is the human-facing section heading.
	Title string
	// Intro is prose explaining what the table holds and why it matters to
	// what the user sees on screen.
	Intro string
	// Columns are the fixed table headers (machine-decodable contract).
	Columns []string
	// Rows returns the table body, pre-sorted for determinism. Each row must
	// have len(Columns) cells.
	Rows func() [][]string
	// Verify performs import-based assertions backing the section's values.
	// It returns a non-nil error if any value is wrong. The coverage test runs
	// every section's Verify; a section with no real assertion fails coverage.
	Verify func() error
}

var registry []Section

// Register adds a section to the document. Panics on duplicate or malformed
// Key so wiring mistakes surface immediately at init time.
func Register(s Section) {
	if s.Key == "" || s.Title == "" || s.Rows == nil || s.Verify == nil || len(s.Columns) == 0 {
		panic(fmt.Sprintf("spec.Register: incomplete section %q", s.Key))
	}
	for _, existing := range registry {
		if existing.Key == s.Key {
			panic(fmt.Sprintf("spec.Register: duplicate section key %q", s.Key))
		}
	}
	registry = append(registry, s)
}

// Sections returns all registered sections sorted by Key (a copy; safe to
// mutate). Used by Generate and the guard/coverage tests.
func Sections() []Section {
	out := make([]Section, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

const header = `# screpdb specification

> **Generated file — do not edit by hand.** Run ` + "`go generate ./...`" + ` to rebuild it,
> then commit. CI fails if it's stale or if any value isn't test-backed.

## Why this exists

screpdb makes a lot of derived claims — "this is a **9 Pool**", "your Spawning
Pool was 6s late", "a Zealot takes 25.2s". They all rest on **golden values**
baked into the code.

This document lets you audit them:

- Every value is read straight from the constants the app runs on (the doc is generated *from* them).
- Every value is checked by a test (` + "`go test ./...`" + `).

So the doc can't drift from the code, and the code can't be silently wrong.

Each section is a short intro plus a fixed-column table — readable by humans,
parseable by machines. Keys are sorted, so diffs stay small.
`

// Generate renders the full specification document. The output is deterministic:
// sections are emitted in Key order and each section's Rows are pre-sorted.
func Generate() ([]byte, error) {
	secs := Sections()

	var b bytes.Buffer
	b.WriteString(header)

	// Table of contents.
	b.WriteString("\n## Contents\n\n")
	for _, s := range secs {
		fmt.Fprintf(&b, "- [%s](#%s)\n", s.Title, anchor(s.Title))
	}

	for _, s := range secs {
		if err := writeSection(&b, s); err != nil {
			return nil, fmt.Errorf("section %q: %w", s.Key, err)
		}
	}

	return b.Bytes(), nil
}

func writeSection(b *bytes.Buffer, s Section) error {
	fmt.Fprintf(b, "\n## %s\n\n", s.Title)
	if intro := strings.TrimSpace(s.Intro); intro != "" {
		b.WriteString(intro)
		b.WriteString("\n\n")
	}

	rows := s.Rows()
	if len(rows) == 0 {
		return fmt.Errorf("no rows")
	}

	// Header.
	b.WriteString("| " + strings.Join(s.Columns, " | ") + " |\n")
	seps := make([]string, len(s.Columns))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |\n")

	// Body.
	for _, row := range rows {
		if len(row) != len(s.Columns) {
			return fmt.Errorf("row has %d cells, want %d: %v", len(row), len(s.Columns), row)
		}
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = cell(c)
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return nil
}

// cell sanitizes a table cell: pipes are escaped and newlines flattened so the
// markdown table stays valid and single-line per row. Empty cells render as an
// em dash for readability.
func cell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}

// anchor converts a section title into a GitHub-flavored-markdown heading anchor.
func anchor(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
