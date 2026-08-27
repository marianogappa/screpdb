// Package fpvec encodes fingerprint feature vectors as compact BLOBs for
// SQLite storage: a little-endian IEEE-754 float64 array with no header.
// The row's feature_version column, not the blob, identifies the schema.
package fpvec

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Encode serializes a feature vector as little-endian float64 bytes.
func Encode(vector []float64) []byte {
	out := make([]byte, 8*len(vector))
	for i, v := range vector {
		binary.LittleEndian.PutUint64(out[i*8:], math.Float64bits(v))
	}
	return out
}

// Decode deserializes a blob produced by Encode.
func Decode(blob []byte) ([]float64, error) {
	if len(blob)%8 != 0 {
		return nil, fmt.Errorf("fpvec: blob length %d is not a multiple of 8", len(blob))
	}
	out := make([]float64, len(blob)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(blob[i*8:]))
	}
	return out, nil
}
