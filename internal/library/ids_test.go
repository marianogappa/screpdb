package library

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestReplayIDFromChecksum(t *testing.T) {
	var zero [32]byte
	var maxSum [32]byte
	for i := range maxSum {
		maxSum[i] = 0xFF
	}
	real := sha256.Sum256([]byte("replay"))

	cases := []struct {
		name string
		sum  [32]byte
		want int64
	}{
		{"zero maps to one", zero, 1},
		{"all ones saturates at 48 bits", maxSum, MaxReplayID},
		{"sha256 takes the top 48 bits", real, int64(binary.BigEndian.Uint64(real[:8]) >> 16)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReplayIDFromChecksum(tc.sum)
			if got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
			if got <= 0 || got > MaxReplayID {
				t.Fatalf("id %d outside (0, 2^48)", got)
			}
			if PlayerID(got, 15) >= 1<<53 {
				t.Fatalf("player id %d exceeds JavaScript's exact-integer range", PlayerID(got, 15))
			}
		})
	}
}

func TestPlayerIDRoundTrip(t *testing.T) {
	for _, replayID := range []int64{1, 42, MaxReplayID} {
		for ordinal := uint8(0); ordinal < MaxPlayersPerReplay; ordinal++ {
			gotReplay, gotOrdinal := SplitPlayerID(PlayerID(replayID, ordinal))
			if gotReplay != replayID || gotOrdinal != ordinal {
				t.Fatalf("PlayerID(%d,%d) round-tripped to (%d,%d)", replayID, ordinal, gotReplay, gotOrdinal)
			}
		}
	}
}

func TestNextReplayIDWraps(t *testing.T) {
	if got := NextReplayID(5); got != 6 {
		t.Fatalf("NextReplayID(5) = %d", got)
	}
	if got := NextReplayID(MaxReplayID); got != 1 {
		t.Fatalf("NextReplayID(max) = %d, want 1", got)
	}
}

func TestChecksumHexRoundTrip(t *testing.T) {
	sum := sha256.Sum256([]byte("x"))
	decoded, err := ChecksumFromHex(ChecksumHex(sum))
	if err != nil || decoded != sum {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := ChecksumFromHex("abc"); err == nil {
		t.Fatal("short hex must fail")
	}
	if _, err := ChecksumFromHex("zz"); err == nil {
		t.Fatal("invalid hex must fail")
	}
}
