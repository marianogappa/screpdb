package fileops

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateReplayDir(t *testing.T) {
	populated := t.TempDir()
	if err := os.WriteFile(filepath.Join(populated, "g.rep"), []byte("g"), 0o644); err != nil {
		t.Fatalf("write g.rep: %v", err)
	}

	emptyDir := t.TempDir()

	onlyLast := t.TempDir()
	if err := os.WriteFile(filepath.Join(onlyLast, "LastReplay.rep"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write LastReplay.rep: %v", err)
	}

	nonRep := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonRep, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	fileNotDir := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(fileNotDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write afile: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{name: "valid populated dir", dir: populated, wantErr: ""},
		{name: "empty string", dir: "   ", wantErr: "replay folder is required"},
		{name: "missing dir", dir: filepath.Join(emptyDir, "nope"), wantErr: "replay folder does not exist"},
		{name: "path is a file", dir: fileNotDir, wantErr: "replay folder is not a directory"},
		{name: "no rep files", dir: emptyDir, wantErr: "does not contain any .rep files"},
		{name: "only non-rep files", dir: nonRep, wantErr: "does not contain any .rep files"},
		{name: "only ignored LastReplay", dir: onlyLast, wantErr: "does not contain any .rep files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReplayDir(tt.dir)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestWalkReplayFiles_RecursesAndFilters(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writes := map[string]string{
		filepath.Join(root, "top.rep"):               "1",
		filepath.Join(root, "UPPER.REP"):             "2",
		filepath.Join(nested, "buried.rep"):          "3",
		filepath.Join(root, "sub", "LastReplay.rep"): "4",
		filepath.Join(root, "readme.txt"):            "5",
		filepath.Join(root, "noext"):                 "6",
	}
	for p, body := range writes {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	files, err := WalkReplayFiles(root)
	if err != nil {
		t.Fatalf("WalkReplayFiles: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f.Name] = true
	}
	want := map[string]bool{"top.rep": true, "UPPER.REP": true, "buried.rep": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked names = %v, want %v", got, want)
	}
}

func TestWalkReplayFiles_MissingRootErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := WalkReplayFiles(missing); err == nil {
		t.Fatalf("expected error walking missing root, got nil")
	}
}

func TestHasReplayFiles(t *testing.T) {
	withRep := t.TempDir()
	if err := os.WriteFile(filepath.Join(withRep, "a.rep"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.rep: %v", err)
	}

	nested := t.TempDir()
	deep := filepath.Join(nested, "x", "y")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deep, "b.rep"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b.rep: %v", err)
	}

	empty := t.TempDir()

	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{name: "blank dir returns false", dir: "  ", want: false},
		{name: "rep at top level", dir: withRep, want: true},
		{name: "rep nested deep", dir: nested, want: true},
		{name: "no rep files", dir: empty, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasReplayFiles(tt.dir)
			if err != nil {
				t.Fatalf("HasReplayFiles: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortFilesByModTime(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	files := []FileInfo{
		{Name: "old", ModTime: base},
		{Name: "newest", ModTime: base.Add(48 * time.Hour)},
		{Name: "mid", ModTime: base.Add(24 * time.Hour)},
	}
	SortFilesByModTime(files)
	gotOrder := []string{files[0].Name, files[1].Name, files[2].Name}
	wantOrder := []string{"newest", "mid", "old"}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("order = %v, want %v", gotOrder, wantOrder)
	}
}

func TestFilterFilesByDate(t *testing.T) {
	now := time.Now()
	recent := FileInfo{Name: "recent", ModTime: now.Add(-1 * time.Hour)}
	future := FileInfo{Name: "future", ModTime: now.Add(72 * time.Hour)}
	old := FileInfo{Name: "old", ModTime: now.AddDate(0, -6, 0)}
	files := []FileInfo{recent, future, old}

	names := func(fs []FileInfo) []string {
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, f.Name)
		}
		return out
	}

	t.Run("no constraints returns all", func(t *testing.T) {
		got := FilterFilesByDate(files, nil, nil)
		if !reflect.DeepEqual(names(got), []string{"recent", "future", "old"}) {
			t.Fatalf("got %v", names(got))
		}
	})

	t.Run("upToDate excludes files after cutoff", func(t *testing.T) {
		cutoff := now
		got := FilterFilesByDate(files, &cutoff, nil)
		if !reflect.DeepEqual(names(got), []string{"recent", "old"}) {
			t.Fatalf("got %v, want [recent old]", names(got))
		}
	})

	t.Run("upToMonths excludes files older than window", func(t *testing.T) {
		months := 3
		got := FilterFilesByDate(files, nil, &months)
		if !reflect.DeepEqual(names(got), []string{"recent", "future"}) {
			t.Fatalf("got %v, want [recent future]", names(got))
		}
	})

	t.Run("both constraints intersect", func(t *testing.T) {
		cutoff := now
		months := 3
		got := FilterFilesByDate(files, &cutoff, &months)
		if !reflect.DeepEqual(names(got), []string{"recent"}) {
			t.Fatalf("got %v, want [recent]", names(got))
		}
	})
}

func TestLimitFiles(t *testing.T) {
	mk := func(n int) []FileInfo {
		fs := make([]FileInfo, n)
		for i := range fs {
			fs[i] = FileInfo{Name: fmt.Sprintf("f%d", i)}
		}
		return fs
	}

	tests := []struct {
		name  string
		count int
		limit int
		want  int
	}{
		{name: "limit smaller than len truncates", count: 5, limit: 2, want: 2},
		{name: "limit equal to len keeps all", count: 3, limit: 3, want: 3},
		{name: "limit larger than len keeps all", count: 2, limit: 10, want: 2},
		{name: "zero limit keeps all", count: 4, limit: 0, want: 4},
		{name: "negative limit keeps all", count: 4, limit: -1, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LimitFiles(mk(tt.count), tt.limit)
			if len(got) != tt.want {
				t.Fatalf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestNewFileInfoFromPath(t *testing.T) {
	root := t.TempDir()
	body := []byte("replay-bytes")
	path := filepath.Join(root, "One.rep")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fi, err := NewFileInfoFromPath(path)
	if err != nil {
		t.Fatalf("NewFileInfoFromPath: %v", err)
	}
	if fi.Path != path {
		t.Fatalf("Path = %q, want %q", fi.Path, path)
	}
	if fi.Name != "One.rep" {
		t.Fatalf("Name = %q, want One.rep", fi.Name)
	}
	if fi.Size != int64(len(body)) {
		t.Fatalf("Size = %d, want %d", fi.Size, len(body))
	}
	wantSum := fmt.Sprintf("%x", sha256.Sum256(body))
	if fi.Checksum != wantSum {
		t.Fatalf("Checksum = %q, want %q", fi.Checksum, wantSum)
	}
	if fi.ModTime.IsZero() {
		t.Fatalf("ModTime should be populated")
	}
}

func TestNewFileInfoFromPath_MissingErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.rep")
	if _, err := NewFileInfoFromPath(missing); err == nil {
		t.Fatalf("expected error for missing path, got nil")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}
