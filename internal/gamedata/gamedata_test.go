package gamedata

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestCandidates(t *testing.T) {
	env := map[string]string{"ProgramFiles(x86)": `C:\Program Files (x86)`, "ProgramFiles": `C:\Program Files`}
	got := candidates("windows", func(k string) string { return env[k] })
	want := []string{filepath.Join(`C:\Program Files (x86)`, "StarCraft"), filepath.Join(`C:\Program Files`, "StarCraft")}
	if !slices.Equal(got, want) {
		t.Fatalf("windows: got %q, want %q", got, want)
	}
	env = map[string]string{InstallDirEnv: "/games/sc"}
	got = candidates("linux", func(k string) string { return env[k] })
	if !slices.Equal(got, []string{"/games/sc"}) {
		t.Fatalf("override: got %q", got)
	}
}

func protoString(field byte, s string) []byte {
	return append([]byte{field<<3 | 2, byte(len(s))}, s...)
}

func TestInstallPathsIn(t *testing.T) {
	var db []byte
	db = append(db, protoString(1, "s1")...)
	db = append(db, protoString(2, "/Applications/StarCraft")...)
	db = append(db, protoString(3, `D:\Games\StarCraft`)...)
	db = append(db, protoString(4, "enUS")...)
	db = append(db, 0x2a, 0x2f)
	got := installPathsIn(db)
	for _, want := range []string{"/Applications/StarCraft", `D:\Games\StarCraft`} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	for _, p := range got {
		if p != "/Applications/StarCraft" && p != `D:\Games\StarCraft` && !isAbsolutePath(p) {
			t.Fatalf("not a path: %q", p)
		}
	}
}

func TestParseTable(t *testing.T) {
	b := []byte{2, 0, 6, 0, 10, 0, 'a', '\\', 'b', 0, 'c', 0}
	got, err := parseTable(b)
	if err != nil || !slices.Equal(got, []string{`a\b`, "c"}) {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := parseTable([]byte{9, 0}); err == nil {
		t.Fatal("accepted a truncated table")
	}
}

func TestFindInstallWithoutStarCraft(t *testing.T) {
	t.Cleanup(iofacade.Reset)
	t.Setenv(InstallDirEnv, t.TempDir())
	if dir, err := FindInstall(); err == nil && dir == os.Getenv(InstallDirEnv) {
		t.Fatal("an empty folder passed as an install")
	}
}

// Extracts from a real install and compares with a reference extract made by openbw-go's bwdata-export.
func TestEnsureFromInstall(t *testing.T) {
	install := os.Getenv(InstallDirEnv)
	if install == "" {
		t.Skip(InstallDirEnv + " is not set; this test reads a real StarCraft: Remastered install")
	}
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)
	out := filepath.Join(t.TempDir(), DirName)
	m, err := Ensure(install, out)
	if err != nil {
		t.Fatal(err)
	}
	if m.Files < 900 {
		t.Fatalf("extracted %d files", m.Files)
	}
	again, err := Ensure(install, out)
	if err != nil || again != m {
		t.Fatalf("second Ensure: %+v %v", again, err)
	}
	ref := os.Getenv("OPENBW_DATA")
	if ref == "" {
		return
	}
	compared := 0
	err = filepath.Walk(ref, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() == "SHA256SUMS" {
			return err
		}
		rel, _ := filepath.Rel(ref, path)
		got, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			return err
		}
		want, _ := os.ReadFile(path)
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the reference", rel)
		}
		compared++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d files, %d compared byte for byte", m.BuildName, m.Files, compared)
}
