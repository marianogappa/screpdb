package casc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/casc/casctest"
)

func TestSyntheticStorage(t *testing.T) {
	files := map[string][]byte{
		"arr/units.dat":          bytes.Repeat([]byte{1, 2, 3}, 400),
		"TileSet/Badlands.cv5":   []byte("tiles"),
		"unit/terran/marine.grp": bytes.Repeat([]byte("marine"), 50),
	}
	dir := casctest.BuildInstall(t, files)
	info, err := ReadBuildInfo(dir)
	if err != nil || info.Product != "StarCraft1" || info.Name != "9.9.9-test" {
		t.Fatalf("build info %+v %v", info, err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for name, want := range files {
		for _, spelling := range []string{name, strings.ToUpper(strings.ReplaceAll(name, "/", "\\"))} {
			got, err := s.ReadFile(spelling)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("%s: %q %v", spelling, got, err)
			}
		}
	}
	if !s.Exists("ARR\\UNITS.DAT") || s.Exists("nope") {
		t.Fatal("Exists is wrong")
	}
	if _, err := s.ReadFile("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestOpenRejectsBrokenInstalls(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("opened an empty folder")
	}
	dir := casctest.BuildInstall(t, map[string][]byte{"a": []byte("a")})
	os.WriteFile(filepath.Join(dir, ".build.info"), []byte("Branch!STRING:0|Active!DEC:1|Build Key!HEX:16\nus|0|x\n"), 0o644)
	if _, err := Open(dir); err == nil {
		t.Fatal("opened an install without an active build")
	}
}

func TestDecodeBLTEErrors(t *testing.T) {
	for name, b := range map[string][]byte{
		"no magic":     []byte("XXXX\x00\x00\x00\x00N"),
		"empty chunk":  []byte("BLTE\x00\x00\x00\x00"),
		"encrypted":    []byte("BLTE\x00\x00\x00\x00E"),
		"bad table":    []byte("BLTE\x00\x00\x00\x10\x0f\x00\x00\x05"),
		"bad zlib":     []byte("BLTE\x00\x00\x00\x00Zgarbage"),
		"short header": []byte("BLT"),
	} {
		if _, err := decodeBLTE(b); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	content := bytes.Repeat([]byte("x"), 100)
	b := casctest.BLTEChunked(content)
	binary.BigEndian.PutUint32(b[16:], 999)
	if _, err := decodeBLTE(b); err == nil {
		t.Error("accepted a chunk whose size disagrees with the table")
	}
}

func TestParseIndexErrors(t *testing.T) {
	good := make([]byte, indexEntriesAt)
	binary.LittleEndian.PutUint16(good[8:], indexVersion)
	good[12], good[13], good[14] = encodedSizeLen, storageOffsetLen, eKeyIndexLen
	cases := map[string]func([]byte){
		"wrong bucket":  func(b []byte) { b[10] = 3 },
		"wrong version": func(b []byte) { b[8] = 5 },
		"wrong layout":  func(b []byte) { b[14] = 16 },
		"overrun":       func(b []byte) { binary.LittleEndian.PutUint32(b[0x20:], indexEntryLen) },
	}
	for name, mutate := range cases {
		b := append([]byte(nil), good...)
		mutate(b)
		if err := parseIndex(b, 0, map[[eKeyIndexLen]byte]location{}); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	if err := parseIndex(good[:10], 0, nil); err == nil {
		t.Error("parsed a truncated index")
	}
}

func TestParseEncodingErrors(t *testing.T) {
	if _, err := parseEncoding([]byte("XX")); err == nil {
		t.Error("parsed a non-encoding table")
	}
	table := casctest.EncodingTable(map[key]key{{1}: {2}})
	if _, err := parseEncoding(table[:len(table)-10]); err == nil {
		t.Error("parsed a truncated table")
	}
	table[3] = 9
	if _, err := parseEncoding(table); err == nil {
		t.Error("parsed 9-byte keys")
	}
}
