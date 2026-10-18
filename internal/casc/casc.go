// Package casc reads files from a local Blizzard CASC storage, the format of a StarCraft: Remastered
// install. It supports what SC:R uses: v7 .idx indexes, BLTE with raw and zlib chunks, the encoding
// table and StarCraft's text root ("path|ckey" lines). It only reads.
package casc

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

type key = [16]byte

var ErrNotFound = errors.New("casc: file not found")

type Storage struct {
	dataDir   string
	BuildName string
	BuildKey  string
	index     map[[eKeyIndexLen]byte]location
	encoding  map[key]key
	root      map[string]key

	mu    sync.Mutex
	files map[int]*os.File
}

type location struct {
	archive int
	offset  int64
	size    uint32
}

func Open(installDir string) (*Storage, error) {
	buildKey, err := activeBuildKey(filepath.Join(installDir, ".build.info"))
	if err != nil {
		return nil, err
	}
	dataDir := filepath.Join(installDir, "Data")
	config, err := readBuildConfig(dataDir, buildKey)
	if err != nil {
		return nil, err
	}
	s := &Storage{dataDir: dataDir, BuildName: config["build-name"], BuildKey: buildKey, files: map[int]*os.File{}}
	if s.index, err = loadIndexes(filepath.Join(dataDir, "data")); err != nil {
		return nil, err
	}
	if err := s.loadEncoding(config["encoding"]); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.loadRoot(config["root"]); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for i, f := range s.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
		delete(s.files, i)
	}
	return first
}

// ReadFile reads a file by its root path, ignoring case and slash direction ("arr\\units.dat").
func (s *Storage) ReadFile(name string) ([]byte, error) {
	ckey, ok := s.root[normalizeName(name)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return s.readByContentKey(ckey)
}

func (s *Storage) Exists(name string) bool {
	_, ok := s.root[normalizeName(name)]
	return ok
}

func normalizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
}

func (s *Storage) readByContentKey(ckey key) ([]byte, error) {
	ekey, ok := s.encoding[ckey]
	if !ok {
		return nil, fmt.Errorf("casc: no encoding entry for %x", ckey)
	}
	data, err := s.readByEncodedKey(ekey)
	if err != nil {
		return nil, err
	}
	if md5.Sum(data) != ckey {
		return nil, fmt.Errorf("casc: content of %x fails its checksum", ckey)
	}
	return data, nil
}

func (s *Storage) readByEncodedKey(ekey key) ([]byte, error) {
	var k [eKeyIndexLen]byte
	copy(k[:], ekey[:])
	loc, ok := s.index[k]
	if !ok {
		return nil, fmt.Errorf("casc: %x is not in the local storage", ekey)
	}
	if loc.size < dataHeaderLen {
		return nil, fmt.Errorf("casc: entry %x is too small", ekey)
	}
	f, err := s.archive(loc.archive)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, loc.size)
	if _, err := f.ReadAt(raw, loc.offset); err != nil {
		return nil, fmt.Errorf("casc: read data.%03d: %w", loc.archive, err)
	}
	if !bytes.Equal(reversed(raw[:16])[:eKeyIndexLen], ekey[:eKeyIndexLen]) {
		return nil, fmt.Errorf("casc: data.%03d at %d holds another entry than %x", loc.archive, loc.offset, ekey)
	}
	return decodeBLTE(raw[dataHeaderLen:])
}

func reversed(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[len(b)-1-i] = c
	}
	return out
}

func (s *Storage) archive(n int) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.files[n]; ok {
		return f, nil
	}
	f, err := iofacade.Open(filepath.Join(s.dataDir, "data", fmt.Sprintf("data.%03d", n)))
	if err != nil {
		return nil, fmt.Errorf("casc: %w", err)
	}
	s.files[n] = f
	return f, nil
}

func parseKey(s string) (key, error) {
	var k key
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(k) {
		return k, fmt.Errorf("casc: bad key %q", s)
	}
	copy(k[:], b)
	return k, nil
}
