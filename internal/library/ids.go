package library

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// MaxReplayID bounds replay ids to 48 bits so that ids and the player ids
// derived from them (52 bits) survive JavaScript's 2^53 exact-integer range.
const MaxReplayID = int64(1)<<48 - 1

// MaxPlayersPerReplay is the number of ordinals a player id can encode.
const MaxPlayersPerReplay = 16

// ReplayIDFromChecksum derives a replay's stable id from its file checksum.
// Zero maps to 1 so that 0 stays free as the "no id" sentinel.
func ReplayIDFromChecksum(sum [32]byte) int64 {
	id := int64(binary.BigEndian.Uint64(sum[:8]) >> 16)
	if id == 0 {
		return 1
	}
	return id
}

// NextReplayID is the linear-probe step used to resolve id collisions
// between distinct checksums.
func NextReplayID(id int64) int64 {
	id++
	if id > MaxReplayID || id <= 0 {
		return 1
	}
	return id
}

// PlayerID encodes (replay, ordinal) into one id; ordinal is the player's
// index in Replay.Players.
func PlayerID(replayID int64, ordinal uint8) int64 {
	return replayID<<4 | int64(ordinal&0xF)
}

// SplitPlayerID recovers the replay id and ordinal from a player id.
func SplitPlayerID(playerID int64) (replayID int64, ordinal uint8) {
	return playerID >> 4, uint8(playerID & 0xF)
}

// ChecksumFromHex decodes the hex sha256 string produced by fileops.
func ChecksumFromHex(s string) ([32]byte, error) {
	var sum [32]byte
	raw, err := hex.DecodeString(s)
	if err != nil {
		return sum, fmt.Errorf("library: invalid checksum hex: %w", err)
	}
	if len(raw) != len(sum) {
		return sum, errors.New("library: checksum must be 32 bytes")
	}
	copy(sum[:], raw)
	return sum, nil
}

// ChecksumHex renders a checksum the way fileops reports it.
func ChecksumHex(sum [32]byte) string {
	return hex.EncodeToString(sum[:])
}
