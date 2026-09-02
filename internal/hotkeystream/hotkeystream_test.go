package hotkeystream

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	events := []Event{
		{Sec: 0, Type: TypeAssignBuilding, Group: 5, Building: BuildingID("Hatchery"), TileX: 117, TileY: 52},
		{Sec: 3, Type: TypeSelect, Group: 5},
		{Sec: 3, Type: TypeAssignUnits, Group: 1, Count: 12},
		{Sec: 9, Type: TypeAdd, Group: 1},
		{Sec: 40, Type: TypeSelect, Group: 1},
		{Sec: 41, Type: TypeAssignBuilding, Group: 9, Building: BuildingID("Comsat Station"), TileX: TileUnknown, TileY: TileUnknown},
	}
	blob := Encode(append([]Event(nil), events...))
	decoded, err := Decode(blob)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded) != len(events) {
		t.Fatalf("got %d events, want %d", len(decoded), len(events))
	}
	for i, e := range events {
		if decoded[i] != e {
			t.Fatalf("event %d: got %+v, want %+v", i, decoded[i], e)
		}
	}
}

func TestEncodeEmptyReturnsNil(t *testing.T) {
	if got := Encode(nil); got != nil {
		t.Fatalf("Encode(nil) = %v, want nil", got)
	}
	events, err := Decode(nil)
	if err != nil || len(events) != 0 {
		t.Fatalf("Decode(nil) = %v, %v", events, err)
	}
}

func TestGroupEscapeRoundTrip(t *testing.T) {
	events := []Event{{Sec: 10, Type: TypeSelect, Group: 17}, {Sec: 11, Type: TypeSelect, Group: 15}}
	decoded, err := Decode(Encode(append([]Event(nil), events...)))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded[0].Group != 17 || decoded[1].Group != 15 {
		t.Fatalf("escaped groups mangled: %+v", decoded)
	}
}

func TestEncodeSortsOutOfOrderEvents(t *testing.T) {
	events := []Event{
		{Sec: 50, Type: TypeSelect, Group: 2},
		{Sec: 10, Type: TypeAssignUnits, Group: 2, Count: 4},
	}
	decoded, err := Decode(Encode(events))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded[0].Sec != 10 || decoded[1].Sec != 50 {
		t.Fatalf("events not time-ordered: %+v", decoded)
	}
}

func TestDecodeTruncatedBlobErrors(t *testing.T) {
	blob := Encode([]Event{{Sec: 1, Type: TypeAssignBuilding, Group: 3, Building: 40, TileX: 1, TileY: 2}})
	if _, err := Decode(blob[:len(blob)-1]); err == nil {
		t.Fatal("expected error for truncated building annotation")
	}
	blob = Encode([]Event{{Sec: 1, Type: TypeAssignUnits, Group: 3, Count: 5}})
	if _, err := Decode(blob[:len(blob)-1]); err == nil {
		t.Fatal("expected error for truncated count byte")
	}
}

func TestDecodeLegacyV1Blob(t *testing.T) {
	// Hand-encode a legacy v1 blob: frame deltas, no magic, no annotations.
	var blob []byte
	frames := []int32{0, 24, 240, 2400}
	types := []byte{1, 0, 2, 0} // legacy Assign, Select, Add, Select
	prev := int32(0)
	for i, f := range frames {
		blob = binary.AppendUvarint(blob, uint64(f-prev)<<6|uint64(types[i])<<4|uint64(3))
		prev = f
	}
	events, err := Decode(blob)
	if err != nil {
		t.Fatalf("Decode legacy: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	wantSecs := []int32{0, 1, 10, 100} // frame*42/1000
	for i, e := range events {
		if e.Sec != wantSecs[i] || e.Group != 3 || e.Type != types[i] {
			t.Fatalf("event %d: got %+v (want sec %d, type %d)", i, e, wantSecs[i], types[i])
		}
		if e.Count != 0 || e.Building != 0 {
			t.Fatalf("legacy event %d carries annotations: %+v", i, e)
		}
	}
}

func TestRandomRoundTripStaysCompact(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	events := make([]Event, 5000)
	sec := int32(0)
	// Type mix mirrors real streams: ~90% Select, ~9% Assign, ~1% Add.
	typeFor := func(roll int) byte {
		switch {
		case roll < 90:
			return TypeSelect
		case roll < 96:
			return TypeAssignUnits
		case roll < 99:
			return TypeAssignBuilding
		default:
			return TypeAdd
		}
	}
	for i := range events {
		sec += rng.Int31n(4)
		e := Event{Sec: sec, Type: typeFor(rng.Intn(100)), Group: byte(rng.Intn(10))}
		switch e.Type {
		case TypeAssignUnits:
			e.Count = byte(1 + rng.Intn(12))
		case TypeAssignBuilding:
			e.Building = 40
			e.TileX, e.TileY = byte(rng.Intn(255)), byte(rng.Intn(255))
		}
		events[i] = e
	}
	blob := Encode(append([]Event(nil), events...))
	if got := float64(len(blob)) / float64(len(events)); got > 2.0 {
		t.Fatalf("blob too large: %.2f bytes/event", got)
	}
	decoded, err := Decode(blob)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for i := range events {
		if decoded[i] != events[i] {
			t.Fatalf("event %d mismatch: got %+v want %+v", i, decoded[i], events[i])
		}
	}
}

func TestBuildingIDRoundTrip(t *testing.T) {
	for id, name := range buildingNames {
		if got := BuildingID(name); got != id {
			t.Fatalf("BuildingID(%q) = %d, want %d", name, got, id)
		}
	}
	if BuildingID("Not A Building") != 0 || BuildingName(0) != "" {
		t.Fatal("unknown building must map to 0 / empty")
	}
}
