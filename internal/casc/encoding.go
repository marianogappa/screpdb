package casc

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	encodingHeaderLen = 22
	encodingKeyLen    = 16
	pageTableEntryLen = 2 * encodingKeyLen
)

// loadEncoding reads the encoding table, which maps a file's content key to its encoded (storage) key.
// configValue is "<ckey> <ekey>" from the build config.
func (s *Storage) loadEncoding(configValue string) error {
	fields := strings.Fields(configValue)
	if len(fields) < 2 {
		return fmt.Errorf("casc: build config encoding %q needs a content and an encoded key", configValue)
	}
	ckey, err := parseKey(fields[0])
	if err != nil {
		return err
	}
	ekey, err := parseKey(fields[1])
	if err != nil {
		return err
	}
	b, err := s.readByEncodedKey(ekey)
	if err != nil {
		return fmt.Errorf("casc: encoding table: %w", err)
	}
	if md5.Sum(b) != ckey {
		return fmt.Errorf("casc: encoding table fails its checksum")
	}
	s.encoding, err = parseEncoding(b)
	return err
}

func parseEncoding(b []byte) (map[key]key, error) {
	if len(b) < encodingHeaderLen || string(b[:2]) != "EN" {
		return nil, fmt.Errorf("casc: not an encoding table")
	}
	if b[3] != encodingKeyLen || b[4] != encodingKeyLen {
		return nil, fmt.Errorf("casc: encoding keys of %d/%d bytes are not supported", b[3], b[4])
	}
	pageSize := int(binary.BigEndian.Uint16(b[5:])) * 1024
	pageCount := int(binary.BigEndian.Uint32(b[9:]))
	specBlockLen := int(binary.BigEndian.Uint32(b[18:]))
	pages := encodingHeaderLen + specBlockLen + pageCount*pageTableEntryLen
	if pageSize == 0 || pages+pageCount*pageSize > len(b) {
		return nil, fmt.Errorf("casc: encoding table is truncated")
	}
	out := map[key]key{}
	for i := range pageCount {
		page := b[pages+i*pageSize : pages+(i+1)*pageSize]
		for p := 0; p+6+encodingKeyLen <= len(page); {
			keyCount := int(page[p])
			if keyCount == 0 {
				break
			}
			entryLen := 6 + encodingKeyLen*(1+keyCount)
			if p+entryLen > len(page) {
				return nil, fmt.Errorf("casc: encoding page %d overruns", i)
			}
			var ck, ek key
			copy(ck[:], page[p+6:])
			copy(ek[:], page[p+6+encodingKeyLen:])
			out[ck] = ek
			p += entryLen
		}
	}
	return out, nil
}

// loadRoot reads StarCraft's root: text lines "path|ckey".
func (s *Storage) loadRoot(configValue string) error {
	ckey, err := parseKey(strings.Fields(configValue)[0])
	if err != nil {
		return err
	}
	b, err := s.readByContentKey(ckey)
	if err != nil {
		return fmt.Errorf("casc: root: %w", err)
	}
	s.root = map[string]key{}
	for _, line := range strings.Split(string(b), "\n") {
		name, hexKey, ok := strings.Cut(strings.TrimRight(line, "\r"), "|")
		if !ok {
			continue
		}
		raw, err := hex.DecodeString(hexKey)
		if err != nil || len(raw) != encodingKeyLen {
			continue
		}
		var k key
		copy(k[:], raw)
		s.root[normalizeName(name)] = k
	}
	if len(s.root) == 0 {
		return fmt.Errorf("casc: root lists no files; not a StarCraft storage")
	}
	return nil
}
