// Package digestcache stores engine digests in one append-only file in the app-data dir. One file, not
// one per replay, because Windows antivirus scans each new file and that slows a cold start badly.
//
// Each record is a 12-byte header (magic, payload length, CRC-32 of the payload) and a payload: the
// replay checksum, the EngineVersion, the game data hash, a status byte and the body (an encoded digest,
// or the error of a failed pass, which is cached so a bad replay is not retried on every launch).
package digestcache

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	recordMagic   = "EDR1"
	headerLen     = 12
	keyLen        = 32 + 4 + 32
	statusLen     = 1
	maxPayloadLen = 64 << 20

	statusDigest byte = 0
	statusFailed byte = 1

	// Compaction rewrites the file once stale records (another EngineVersion or game data) are this share
	// of it, so an EngineVersion bump frees the space within a launch or two without rewriting on every one.
	compactStaleShare = 0.5
)

type Key struct {
	Checksum      [32]byte
	EngineVersion int32
	DataHash      [32]byte
}

type Entry struct {
	Digest []byte
	Failed string
}

type location struct {
	offset int64
	length int
}

type Cache struct {
	path string

	mu      sync.Mutex
	file    *os.File
	index   map[Key]location
	size    int64
	stale   int64
	current func(Key) bool
}

// Open reads the record index of the file at path, creating nothing until the first Put. current says
// which keys are still worth keeping; records for other keys are dropped by compaction. A torn or corrupt
// tail, from a crash during an append, is dropped by rewriting the file without it.
func Open(path string, current func(Key) bool) (*Cache, error) {
	c := &Cache{path: path, index: map[Key]location{}, current: current}
	b, err := iofacade.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("digest cache: %w", err)
	}
	valid := c.scan(b)
	if valid < int64(len(b)) || c.needsCompaction() {
		if err := c.rewrite(b); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Cache) scan(b []byte) (valid int64) {
	for off := int64(0); off < int64(len(b)); {
		payload, ok := recordAt(b, off)
		if !ok {
			return off
		}
		var k Key
		copy(k.Checksum[:], payload)
		k.EngineVersion = int32(binary.LittleEndian.Uint32(payload[32:]))
		copy(k.DataHash[:], payload[36:])
		recLen := int64(headerLen + len(payload))
		if old, dup := c.index[k]; dup {
			c.stale += int64(headerLen + old.length)
		}
		c.index[k] = location{offset: off + headerLen, length: len(payload)}
		c.size = off + recLen
		off += recLen
	}
	for k := range c.index {
		c.dropOlder(k)
	}
	for k, loc := range c.index {
		if !c.current(k) {
			c.stale += int64(headerLen + loc.length)
		}
	}
	return int64(len(b))
}

// dropOlder forgets the same replay's records from earlier engine versions: a newer digest supersedes
// them, so compaction can reclaim their space.
func (c *Cache) dropOlder(k Key) {
	older := k
	for v := int32(0); v < k.EngineVersion; v++ {
		older.EngineVersion = v
		if loc, ok := c.index[older]; ok {
			c.stale += int64(headerLen + loc.length)
			delete(c.index, older)
		}
	}
}

func recordAt(b []byte, off int64) ([]byte, bool) {
	if off+headerLen > int64(len(b)) || string(b[off:off+4]) != recordMagic {
		return nil, false
	}
	n := int64(binary.LittleEndian.Uint32(b[off+4:]))
	if n < keyLen+statusLen || n > maxPayloadLen || off+headerLen+n > int64(len(b)) {
		return nil, false
	}
	payload := b[off+headerLen : off+headerLen+n]
	if crc32.ChecksumIEEE(payload) != binary.LittleEndian.Uint32(b[off+8:]) {
		return nil, false
	}
	return payload, true
}

func (c *Cache) needsCompaction() bool {
	return c.size > 0 && float64(c.stale) >= compactStaleShare*float64(c.size)
}

// rewrite writes only the current, latest record of each key to a temporary file and renames it over
// the cache. The read handle is closed first: Windows cannot rename over an open file.
func (c *Cache) rewrite(old []byte) error {
	c.closeFile()
	var out bytes.Buffer
	index := map[Key]location{}
	for k, loc := range c.index {
		if !c.current(k) {
			continue
		}
		payload := old[loc.offset : loc.offset+int64(loc.length)]
		index[k] = location{offset: int64(out.Len()) + headerLen, length: len(payload)}
		out.Write(frame(payload))
	}
	tmp := c.path + ".tmp"
	if err := iofacade.WriteFile(tmp, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("digest cache: compact: %w", err)
	}
	if err := iofacade.Rename(tmp, c.path); err != nil {
		return fmt.Errorf("digest cache: compact: %w", err)
	}
	c.index, c.size, c.stale = index, int64(out.Len()), 0
	return nil
}

func frame(payload []byte) []byte {
	rec := make([]byte, headerLen, headerLen+len(payload))
	copy(rec, recordMagic)
	binary.LittleEndian.PutUint32(rec[4:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(rec[8:], crc32.ChecksumIEEE(payload))
	return append(rec, payload...)
}

func (c *Cache) Get(k Key) (Entry, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	loc, ok := c.index[k]
	if !ok {
		return Entry{}, false, nil
	}
	if c.file == nil {
		f, err := iofacade.Open(c.path)
		if err != nil {
			return Entry{}, false, fmt.Errorf("digest cache: %w", err)
		}
		c.file = f
	}
	payload := make([]byte, loc.length)
	if _, err := c.file.ReadAt(payload, loc.offset); err != nil && !errors.Is(err, io.EOF) {
		return Entry{}, false, fmt.Errorf("digest cache: %w", err)
	}
	body := payload[keyLen+statusLen:]
	if payload[keyLen] == statusFailed {
		return Entry{Failed: string(body)}, true, nil
	}
	return Entry{Digest: body}, true, nil
}

func (c *Cache) Has(k Key) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.index[k]
	return ok
}

func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.index)
}

// Put appends a record in one write, so a crash tears at most this record, which the next Open drops.
func (c *Cache) Put(k Key, e Entry) error {
	payload := make([]byte, 0, keyLen+statusLen+len(e.Digest)+len(e.Failed))
	payload = append(payload, k.Checksum[:]...)
	payload = binary.LittleEndian.AppendUint32(payload, uint32(k.EngineVersion))
	payload = append(payload, k.DataHash[:]...)
	if e.Failed != "" {
		payload = append(append(payload, statusFailed), e.Failed...)
	} else {
		payload = append(append(payload, statusDigest), e.Digest...)
	}
	if len(payload) > maxPayloadLen {
		return fmt.Errorf("digest cache: record of %d bytes is too large", len(payload))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := iofacade.AppendFile(c.path, frame(payload), 0o644); err != nil {
		return fmt.Errorf("digest cache: %w", err)
	}
	if old, dup := c.index[k]; dup {
		c.stale += int64(headerLen + old.length)
	}
	c.index[k] = location{offset: c.size + headerLen, length: len(payload)}
	c.size += int64(headerLen + len(payload))
	c.dropOlder(k)
	return nil
}

func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeFile()
}

func (c *Cache) closeFile() error {
	if c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	return err
}
