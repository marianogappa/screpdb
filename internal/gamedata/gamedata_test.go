package gamedata

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/marianogappa/screpdb/internal/casc/casctest"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestCandidates(t *testing.T) {
	env := map[string]string{"ProgramFiles(x86)": `C:\Program Files (x86)`, "ProgramFiles": `C:\Program Files`}
	got := candidates("windows", func(k string) string { return env[k] }, "")
	want := []string{filepath.Join(`C:\Program Files (x86)`, "StarCraft"), filepath.Join(`C:\Program Files`, "StarCraft")}
	if !slices.Equal(got, want) {
		t.Fatalf("windows: got %q, want %q", got, want)
	}
	env = map[string]string{InstallDirEnv: "/games/sc"}
	got = candidates("linux", func(k string) string { return env[k] }, "")
	if !slices.Equal(got, []string{"/games/sc"}) {
		t.Fatalf("override: got %q", got)
	}
}

func TestLinuxCandidatesUseWinePrefixes(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(home, "Games", "battlenet")
	var db []byte
	db = append(db, protoString(2, `C:\Games\StarCraft`)...)
	db = append(db, protoString(3, `D:\SC`)...)
	productDB := filepath.Join(prefix, "drive_c", "ProgramData", "Battle.net", "Agent", "product.db")
	if err := os.MkdirAll(filepath.Dir(productDB), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(productDB, db, 0o644); err != nil {
		t.Fatal(err)
	}
	got := candidates("linux", func(string) string { return "" }, home)
	for _, want := range []string{
		filepath.Join(prefix, "drive_c", "Games", "StarCraft"),
		filepath.Join(prefix, "dosdevices", "d:", "SC"),
		filepath.Join(home, ".wine", "drive_c", "Program Files (x86)", "StarCraft"),
		filepath.Join(home, ".local", "share", "bottles", "bottles", "Battle.net", "drive_c", "Program Files", "StarCraft"),
	} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %q", want, got)
		}
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

func fakeImagesTable(entries ...string) []byte {
	b := binary.LittleEndian.AppendUint16(nil, uint16(len(entries)))
	off := 2 + 2*len(entries)
	var strs []byte
	for _, e := range entries {
		b = binary.LittleEndian.AppendUint16(b, uint16(off+len(strs)))
		strs = append(append(strs, e...), 0)
	}
	return append(b, strs...)
}

func fakeGameFiles() map[string][]byte {
	files := map[string][]byte{
		"arr/images.tbl":         fakeImagesTable(`terran\marine.grp`, `Zerg\zavBirth.grp`, `terran\marine.grp`, ""),
		"scripts/iscript.bin":    []byte("iscript"),
		"triggers/Melee.trg":     []byte("melee"),
		"unit/terran/marine.grp": []byte("marine"),
		"unit/zerg/zavbirth.grp": []byte("birth"),
	}
	for _, d := range datFiles {
		files["arr/"+d+".dat"] = []byte(d)
	}
	for _, t := range tilesets {
		files["TileSet/"+t+".cv5"] = []byte(t + "cv5")
		files["TileSet/"+t+".vf4"] = []byte(t + "vf4")
	}
	return files
}

func TestEnsureFromFakeInstall(t *testing.T) {
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)
	install := casctest.BuildInstall(t, fakeGameFiles())
	t.Setenv(InstallDirEnv, install)
	found, err := FindInstall()
	if err != nil || found != install {
		t.Fatalf("FindInstall: %q %v", found, err)
	}
	iofacade.Reset()

	out := filepath.Join(t.TempDir(), DirName)
	m, err := Ensure(install, out)
	if err != nil {
		t.Fatal(err)
	}
	want := len(datFiles) + 3 + 2*len(tilesets) + 2
	if m.Files != want || m.BuildName != "9.9.9-test" {
		t.Fatalf("manifest %+v, want %d files", m, want)
	}
	if b, err := os.ReadFile(filepath.Join(out, "unit", "zerg", "zavbirth.grp")); err != nil || string(b) != "birth" {
		t.Fatalf("zavbirth.grp: %q %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(out, "triggers", "melee.trg")); err != nil || string(b) != "melee" {
		t.Fatalf("melee.trg: %q %v", b, err)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary folder left behind: %v", err)
	}

	stale := filepath.Join(out, "stale")
	os.WriteFile(stale, nil, 0o644)
	if _, err := Ensure(install, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("Ensure re-extracted although the build did not change")
	}
	os.WriteFile(filepath.Join(out, manifestName), []byte(`{"build_key":"older","files":3}`), 0o644)
	if _, err := Ensure(install, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("Ensure kept the data of an older build")
	}
}

func TestEnsureFailsCleanlyOnMissingFile(t *testing.T) {
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)
	files := fakeGameFiles()
	delete(files, "scripts/iscript.bin")
	install := casctest.BuildInstall(t, files)
	out := filepath.Join(t.TempDir(), DirName)
	if _, err := Ensure(install, out); err == nil {
		t.Fatal("Ensure succeeded without iscript.bin")
	}
	for _, dir := range []string{out, out + ".tmp"} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s exists after a failed extract", dir)
		}
	}
}
