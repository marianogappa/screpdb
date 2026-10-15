package casc

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func installDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("SCREPDB_STARCRAFT_DIR")
	if dir == "" {
		t.Skip("SCREPDB_STARCRAFT_DIR is not set; these tests read a real StarCraft: Remastered install")
	}
	return dir
}

func TestReadFromInstall(t *testing.T) {
	s, err := Open(installDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.BuildName == "" {
		t.Fatal("no build name")
	}
	for _, name := range []string{"arr\\units.dat", "arr/images.tbl", "scripts/iscript.bin", "triggers\\Melee.trg", "TileSet\\badlands.cv5", "unit\\terran\\marine.grp"} {
		b, err := s.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s is empty", name)
		}
		if ref := os.Getenv("OPENBW_DATA"); ref != "" {
			want, err := os.ReadFile(filepath.Join(ref, normalizeName(name)))
			if err == nil && !bytes.Equal(b, want) {
				t.Fatalf("%s differs from %s", name, ref)
			}
		}
	}
	if _, err := s.ReadFile("no/such/file"); err == nil {
		t.Fatal("read a missing file")
	}
}
