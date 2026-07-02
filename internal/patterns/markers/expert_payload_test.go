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

func TestDecodePayloadLabel(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
		wantOK  bool
	}{
		{"n hatch tech", `{"label":"3 Hatch Muta"}`, "3 Hatch Muta", true},
		{"fuzzy opener", `{"label":"~9 Overpool"}`, "~9 Overpool", true},
		{"trims whitespace", `{"label":"  12 Hatch  "}`, "12 Hatch", true},
		{"empty label", `{"label":""}`, "", false},
		{"missing key", `{"expert_actuals":[]}`, "", false},
		{"empty payload", ``, "", false},
		{"invalid json", `{`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DecodePayloadLabel([]byte(tc.payload))
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("DecodePayloadLabel(%q) = (%q, %v), want (%q, %v)", tc.payload, got, ok, tc.want, tc.wantOK)
			}
		})
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
