package casc

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	indexBuckets     = 16
	indexVersion     = 7
	eKeyIndexLen     = 9
	storageOffsetLen = 5
	encodedSizeLen   = 4
	indexEntryLen    = eKeyIndexLen + storageOffsetLen + encodedSizeLen
	indexEntriesAt   = 0x28
	dataHeaderLen    = 0x1e
)

// loadIndexes reads the newest .idx of each bucket ("BBVVVVVVVV.idx": bucket, then version, in hex).
func loadIndexes(dir string) (map[[eKeyIndexLen]byte]location, error) {
	entries, err := iofacade.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("casc: %w", err)
	}
	var newest [indexBuckets]string
	var newestVersion [indexBuckets]uint64
	for _, e := range entries {
		name := e.Name()
		if len(name) != 14 || !strings.HasSuffix(name, ".idx") {
			continue
		}
		bucket, err1 := strconv.ParseUint(name[:2], 16, 8)
		version, err2 := strconv.ParseUint(name[2:10], 16, 32)
		if err1 != nil || err2 != nil || bucket >= indexBuckets {
			continue
		}
		if newest[bucket] == "" || version > newestVersion[bucket] {
			newest[bucket], newestVersion[bucket] = name, version
		}
	}
	index := map[[eKeyIndexLen]byte]location{}
	for bucket, name := range newest {
		if name == "" {
			return nil, fmt.Errorf("casc: no index for bucket %x in %s", bucket, dir)
		}
		b, err := iofacade.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("casc: %w", err)
		}
		if err := parseIndex(b, bucket, index); err != nil {
			return nil, fmt.Errorf("casc: %s: %w", name, err)
		}
	}
	return index, nil
}

func parseIndex(b []byte, bucket int, into map[[eKeyIndexLen]byte]location) error {
	if len(b) < indexEntriesAt {
		return fmt.Errorf("too short")
	}
	h := b[8:]
	if binary.LittleEndian.Uint16(h) != indexVersion || int(h[2]) != bucket {
		return fmt.Errorf("unsupported header (version %d, bucket %d)", binary.LittleEndian.Uint16(h), h[2])
	}
	if h[4] != encodedSizeLen || h[5] != storageOffsetLen || h[6] != eKeyIndexLen {
		return fmt.Errorf("unsupported entry layout %d/%d/%d", h[4], h[5], h[6])
	}
	offsetBits := uint(h[7])
	entriesLen := int(binary.LittleEndian.Uint32(b[0x20:]))
	if entriesLen%indexEntryLen != 0 || indexEntriesAt+entriesLen > len(b) {
		return fmt.Errorf("bad entries block of %d bytes", entriesLen)
	}
	for p := indexEntriesAt; p < indexEntriesAt+entriesLen; p += indexEntryLen {
		var k [eKeyIndexLen]byte
		copy(k[:], b[p:])
		var storage uint64
		for _, c := range b[p+eKeyIndexLen : p+eKeyIndexLen+storageOffsetLen] {
			storage = storage<<8 | uint64(c)
		}
		into[k] = location{
			archive: int(storage >> offsetBits),
			offset:  int64(storage & (1<<offsetBits - 1)),
			size:    binary.LittleEndian.Uint32(b[p+eKeyIndexLen+storageOffsetLen:]),
		}
	}
	return nil
}
