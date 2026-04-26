package markers

import "testing"

func TestEncodeDecodeExpertActuals_RoundTrip(t *testing.T) {
	resolutions := []ExpertResolution{
		{Found: true, ActualSecond: 70},
		{Found: true, ActualSecond: 86},
		{Found: false},
	}
	encoded, err := EncodeExpertActuals(resolutions)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	actuals := DecodeExpertActuals(encoded)
	if len(actuals) != 3 {
		t.Fatalf("expected 3 actuals, got %d", len(actuals))
	}
	if !actuals[0].Found || actuals[0].Second != 70 {
		t.Fatalf("entry 0 wrong: %+v", actuals[0])
	}
	if !actuals[1].Found || actuals[1].Second != 86 {
		t.Fatalf("entry 1 wrong: %+v", actuals[1])
	}
	if actuals[2].Found {
		t.Fatalf("entry 2 should be missing, got %+v", actuals[2])
	}
	if actuals[2].Second != 0 {
		t.Fatalf("missing entry should have zero second, got %d", actuals[2].Second)
	}
}

func TestDecodeExpertActuals_EmptyAndInvalid(t *testing.T) {
	if got := DecodeExpertActuals(nil); got != nil {
		t.Fatalf("nil payload should decode to nil, got %v", got)
	}
	if got := DecodeExpertActuals([]byte("{")); got != nil {
		t.Fatalf("invalid JSON should decode to nil, got %v", got)
	}
	if got := DecodeExpertActuals([]byte("{}")); got != nil {
		t.Fatalf("missing key should decode to nil, got %v", got)
	}
}
