package spec

import "testing"

// TestEverySectionHasRows asserts every registered section renders at least one
// row. A golden table wired up with an empty enumerator (or none) fails here.
func TestEverySectionHasRows(t *testing.T) {
	secs := Sections()
	if len(secs) == 0 {
		t.Fatal("no sections registered")
	}
	for _, s := range secs {
		rows := s.Rows()
		if len(rows) == 0 {
			t.Errorf("section %q (%s) rendered no rows", s.Key, s.Title)
			continue
		}
		for i, row := range rows {
			if len(row) != len(s.Columns) {
				t.Errorf("section %q row %d has %d cells, want %d", s.Key, i, len(row), len(s.Columns))
			}
		}
	}
}

// TestEverySectionVerifies runs each section's import-based Verify assertion.
// This is the coverage guarantee: a section whose documented values are not
// backed by a real assertion (or whose values are wrong) fails here, so no
// golden table can be documented without being test-backed.
func TestEverySectionVerifies(t *testing.T) {
	for _, s := range Sections() {
		if s.Verify == nil {
			t.Errorf("section %q has no Verify assertion", s.Key)
			continue
		}
		if err := s.Verify(); err != nil {
			t.Errorf("section %q Verify failed: %v", s.Key, err)
		}
	}
}

// TestGenerateDeterministic asserts Generate is stable across calls (sorted keys
// and rows), so committed diffs stay minimal and meaningful.
func TestGenerateDeterministic(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("Generate produced different output across calls")
	}
}
