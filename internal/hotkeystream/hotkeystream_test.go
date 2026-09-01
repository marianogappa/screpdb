package hotkeystream

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	events := []Event{
		{Frame: 0, Type: TypeAssign, Group: 1},
		{Frame: 24, Type: TypeSelect, Group: 1},
		{Frame: 24, Type: TypeAdd, Group: 2},
		{Frame: 1000, Type: TypeSelect, Group: 0},
		{Frame: 100000, Type: TypeSelect, Group: 9},
	}
	got, err := Decode(Encode(append([]Event(nil), events...)))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatalf("round trip mismatch:\n got %v\nwant %v", got, events)
	}
}

func TestEncodeEmptyReturnsNil(t *testing.T) {
	if got := Encode(nil); got != nil {
		t.Fatalf("Encode(nil) = %v, want nil", got)
	}
}

func TestGroupEscape(t *testing.T) {
	for _, group := range []byte{14, 15, 16, 100, 255} {
		events := []Event{{Frame: 10, Type: TypeSelect, Group: group}}
		got, err := Decode(Encode(append([]Event(nil), events...)))
		if err != nil {
			t.Fatalf("group %d: Decode: %v", group, err)
		}
		if !reflect.DeepEqual(got, events) {
			t.Fatalf("group %d: got %v want %v", group, got, events)
		}
	}
}

func TestEncodeSortsOutOfOrderFrames(t *testing.T) {
	events := []Event{
		{Frame: 50, Type: TypeSelect, Group: 3},
		{Frame: 10, Type: TypeAssign, Group: 3},
	}
	got, err := Decode(Encode(events))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := []Event{
		{Frame: 10, Type: TypeAssign, Group: 3},
		{Frame: 50, Type: TypeSelect, Group: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeTruncatedBlob(t *testing.T) {
	blob := Encode([]Event{{Frame: 10, Type: TypeSelect, Group: 15}})
	if _, err := Decode(blob[:len(blob)-1]); err == nil {
		t.Fatal("expected error for truncated blob")
	}
}

func TestRoundTripRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(357))
	events := make([]Event, 5000)
	frame := int32(0)
	for i := range events {
		frame += rng.Int31n(500)
		events[i] = Event{Frame: frame, Type: byte(rng.Intn(3)), Group: byte(rng.Intn(10))}
	}
	blob := Encode(append([]Event(nil), events...))
	got, err := Decode(blob)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatal("random round trip mismatch")
	}
	if avg := float64(len(blob)) / float64(len(events)); avg > 3 {
		t.Fatalf("encoding too large: %.2f bytes/event", avg)
	}
}

func TestTypeNames(t *testing.T) {
	for _, name := range []string{"Select", "Assign", "Add"} {
		typ, ok := TypeFromName(name)
		if !ok || TypeName(typ) != name {
			t.Fatalf("type name round trip failed for %q", name)
		}
	}
	if _, ok := TypeFromName("UNKNOWN"); ok {
		t.Fatal("TypeFromName should reject unknown names")
	}
}
