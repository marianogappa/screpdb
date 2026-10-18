package casc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	blteMagic        = "BLTE"
	blteChunkInfoLen = 24
	blteTableFlags   = 0x0f
)

func decodeBLTE(b []byte) ([]byte, error) {
	if len(b) < 8 || string(b[:4]) != blteMagic {
		return nil, fmt.Errorf("casc: not BLTE")
	}
	headerLen := int(binary.BigEndian.Uint32(b[4:]))
	if headerLen == 0 {
		return decodeChunk(b[8:], -1)
	}
	if headerLen < 12 || headerLen > len(b) || b[8] != blteTableFlags {
		return nil, fmt.Errorf("casc: bad BLTE header")
	}
	count := int(b[9])<<16 | int(b[10])<<8 | int(b[11])
	if 12+count*blteChunkInfoLen > headerLen {
		return nil, fmt.Errorf("casc: bad BLTE chunk table")
	}
	var out []byte
	pos := headerLen
	for i := range count {
		info := b[12+i*blteChunkInfoLen:]
		compressed := int(binary.BigEndian.Uint32(info))
		decompressed := int(binary.BigEndian.Uint32(info[4:]))
		if compressed <= 0 || pos+compressed > len(b) {
			return nil, fmt.Errorf("casc: BLTE chunk %d overruns", i)
		}
		chunk, err := decodeChunk(b[pos:pos+compressed], decompressed)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		pos += compressed
	}
	return out, nil
}

// decodeChunk checks the decoded size when want is not negative.
func decodeChunk(c []byte, want int) ([]byte, error) {
	if len(c) == 0 {
		return nil, fmt.Errorf("casc: empty BLTE chunk")
	}
	var out []byte
	switch c[0] {
	case 'N':
		out = c[1:]
	case 'Z':
		r, err := zlib.NewReader(bytes.NewReader(c[1:]))
		if err != nil {
			return nil, fmt.Errorf("casc: zlib chunk: %w", err)
		}
		limit := int64(want)
		if want < 0 {
			limit = 1 << 30
		}
		out, err = io.ReadAll(io.LimitReader(r, limit+1))
		if err != nil {
			return nil, fmt.Errorf("casc: zlib chunk: %w", err)
		}
	case 'F':
		return decodeBLTE(c[1:])
	default:
		return nil, fmt.Errorf("casc: unsupported BLTE chunk mode %q", c[0])
	}
	if want >= 0 && len(out) != want {
		return nil, fmt.Errorf("casc: BLTE chunk decodes to %d bytes, want %d", len(out), want)
	}
	return out, nil
}
