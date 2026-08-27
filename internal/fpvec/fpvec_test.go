package fpvec

import (
	"math"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	in := []float64{0, 1, -1, 0.25, math.Pi, math.MaxFloat64, math.SmallestNonzeroFloat64}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("dim %d: got %v, want %v", i, got[i], in[i])
		}
	}
}

func TestRoundTripEmpty(t *testing.T) {
	got, err := Decode(Encode(nil))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty vector, got %d dims", len(got))
	}
}

func TestDecodeRejectsMisalignedBlob(t *testing.T) {
	if _, err := Decode(make([]byte, 12)); err == nil {
		t.Fatal("expected error for blob length not a multiple of 8")
	}
}
