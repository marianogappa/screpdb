package digestcache

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func key(seed byte, version int32) Key {
	return Key{Checksum: [32]byte{seed}, EngineVersion: version, DataHash: [32]byte{0xda}}
}

func allCurrent(Key) bool { return true }

func openAt(t *testing.T, path string, current func(Key) bool) *Cache {
	t.Helper()
	c, err := Open(path, current)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func mustGet(t *testing.T, c *Cache, k Key) Entry {
	t.Helper()
	e, ok, err := c.Get(k)
	if err != nil || !ok {
		t.Fatalf("Get %v: ok=%v err=%v", k.Checksum[0], ok, err)
	}
	return e
}

func TestPutGetAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	if _, ok, _ := c.Get(key(1, 1)); ok {
		t.Fatal("hit in an empty cache")
	}
	if err := c.Put(key(1, 1), Entry{Digest: []byte("one")}); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(key(2, 1), Entry{Failed: "engine error"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(key(1, 1), Entry{Digest: []byte("one again")}); err != nil {
		t.Fatal(err)
	}
	if e := mustGet(t, c, key(1, 1)); string(e.Digest) != "one again" {
		t.Fatalf("got %q", e.Digest)
	}
	c.Close()

	c = openAt(t, path, allCurrent)
	if c.Len() != 2 || !c.Has(key(2, 1)) {
		t.Fatalf("reopened with %d keys", c.Len())
	}
	if e := mustGet(t, c, key(1, 1)); string(e.Digest) != "one again" {
		t.Fatalf("after reopen got %q", e.Digest)
	}
	if e := mustGet(t, c, key(2, 1)); e.Failed != "engine error" || e.Digest != nil {
		t.Fatalf("failure entry %+v", e)
	}
}

func TestTornTailIsDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	c.Put(key(1, 1), Entry{Digest: []byte("kept")})
	c.Put(key(2, 1), Entry{Digest: bytes.Repeat([]byte("x"), 100)})
	c.Close()
	b, _ := os.ReadFile(path)
	os.WriteFile(path, b[:len(b)-20], 0o644)

	c = openAt(t, path, allCurrent)
	if c.Has(key(2, 1)) || string(mustGet(t, c, key(1, 1)).Digest) != "kept" {
		t.Fatal("torn record survived or a good one was lost")
	}
	if err := c.Put(key(3, 1), Entry{Digest: []byte("after")}); err != nil {
		t.Fatal(err)
	}
	c.Close()
	c = openAt(t, path, allCurrent)
	if string(mustGet(t, c, key(3, 1)).Digest) != "after" || string(mustGet(t, c, key(1, 1)).Digest) != "kept" {
		t.Fatal("append after a dropped tail is unreadable")
	}
}

func TestCorruptRecordStopsTheScan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	c.Put(key(1, 1), Entry{Digest: []byte("good")})
	c.Put(key(2, 1), Entry{Digest: []byte("flipped")})
	c.Close()
	b, _ := os.ReadFile(path)
	b[len(b)-1] ^= 0xff
	os.WriteFile(path, b, 0o644)

	c = openAt(t, path, allCurrent)
	if c.Has(key(2, 1)) || !c.Has(key(1, 1)) {
		t.Fatal("a record with a bad checksum was indexed")
	}
}

func TestCompactionDropsOldEngineVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	for i := range byte(10) {
		c.Put(key(i, 1), Entry{Digest: bytes.Repeat([]byte{i}, 1000)})
	}
	c.Put(key(0, 2), Entry{Digest: []byte("new")})
	c.Close()
	before, _ := os.Stat(path)

	onlyV2 := func(k Key) bool { return k.EngineVersion == 2 }
	c = openAt(t, path, onlyV2)
	after, _ := os.Stat(path)
	if after.Size() >= before.Size()/5 {
		t.Fatalf("not compacted: %d -> %d bytes", before.Size(), after.Size())
	}
	if c.Len() != 1 || string(mustGet(t, c, key(0, 2)).Digest) != "new" {
		t.Fatal("compaction lost the current record")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temporary file left behind")
	}
}

func TestFewStaleRecordsDoNotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	for i := range byte(10) {
		c.Put(key(i, 1), Entry{Digest: []byte("payload")})
	}
	c.Close()
	before, _ := os.Stat(path)
	c = openAt(t, path, func(k Key) bool { return k.Checksum[0] != 0 })
	after, _ := os.Stat(path)
	if after.Size() != before.Size() || c.Len() != 10 {
		t.Fatalf("rewrote for one stale record: %d -> %d bytes", before.Size(), after.Size())
	}
}

func TestNotACacheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	os.WriteFile(path, []byte("this is not a digest cache"), 0o644)
	c := openAt(t, path, allCurrent)
	if c.Len() != 0 {
		t.Fatal("indexed garbage")
	}
	if err := c.Put(key(1, 1), Entry{Digest: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	c.Close()
	if c = openAt(t, path, allCurrent); !c.Has(key(1, 1)) {
		t.Fatal("garbage was not replaced")
	}
}

func TestANewerVersionSupersedesTheSameReplaysOlderRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digests.bin")
	c := openAt(t, path, allCurrent)
	for _, k := range []Key{key(1, 7), key(2, 7), key(1, 9)} {
		if err := c.Put(k, Entry{Digest: []byte("d")}); err != nil {
			t.Fatal(err)
		}
	}
	if c.Has(key(1, 7)) || !c.Has(key(1, 9)) || !c.Has(key(2, 7)) {
		t.Fatal("the newer record should replace only its own replay's older one")
	}
	c.Close()
	reopened := openAt(t, path, allCurrent)
	if reopened.Has(key(1, 7)) || reopened.Len() != 2 {
		t.Fatalf("after reopening: %d records, older kept %v", reopened.Len(), reopened.Has(key(1, 7)))
	}
}
