// Package hotkeystream encodes a player's hotkey commands (Select / Assign /
// Add) as a compact delta-encoded varint blob, stored in players.hotkey_stream.
//
// Wire format v2 (issues #357, hotkey intel): the blob starts with the magic
// pair 0xFF 0x02, then one record per event:
//
//	uvarint(Δsec<<6 | type<<4 | group)   when group < 15
//	uvarint(Δsec<<6 | type<<4 | 15), uvarint(group)   when group >= 15
//
// Δsec is the in-game second delta from the previous event (events are
// time-ordered). The 2-bit type is 0=Select, 1=Assign of units, 2=Add (a
// group recall variant kept for legacy blobs; extraction folds shift+number
// group additions into assigns), 3=Assign of a proven building. Type 1 appends one byte: the selection size
// (unit count, capped at 255). Type 3 appends three bytes: the building's wire
// ID (see buildings.go) and its build-tile X and Y (255,255 when unknown).
//
// Blobs without the magic pair are legacy v1 streams (frame deltas, type
// 1=plain Assign, no annotations); Decode converts them transparently, with
// counts and buildings zeroed.
package hotkeystream

import (
	"encoding/binary"
	"fmt"
	"slices"
)

const (
	TypeSelect         byte = 0
	TypeAssignUnits    byte = 1
	TypeAdd            byte = 2
	TypeAssignBuilding byte = 3
)

const (
	groupEscape        = 15
	magicByte0    byte = 0xFF
	magicVersion2 byte = 0x02
	// TileUnknown marks a building assign whose placement could not be located.
	TileUnknown byte = 255
)

// framesPerSecond converts legacy v1 frame deltas: 1000ms/42ms per frame.
const frameMillis = 42

// Event is one hotkey command in a player's stream.
type Event struct {
	Sec   int32
	Type  byte
	Group byte
	// Count is the selection size for TypeAssignUnits events.
	Count byte
	// Building, TileX, TileY annotate TypeAssignBuilding events.
	Building byte
	TileX    byte
	TileY    byte
}

// TypeName maps a wire value back to a human-readable name.
func TypeName(t byte) string {
	switch t {
	case TypeSelect:
		return "Select"
	case TypeAssignUnits:
		return "Assign"
	case TypeAdd:
		return "Add"
	case TypeAssignBuilding:
		return "AssignBuilding"
	}
	return "UNKNOWN"
}

// Encode serializes events into the v2 blob format. It sorts events by second
// in place first, so delta encoding is safe even if the caller's stream
// carries out-of-order events. Returns nil for an empty stream (stored NULL).
func Encode(events []Event) []byte {
	if len(events) == 0 {
		return nil
	}
	slices.SortStableFunc(events, func(a, b Event) int { return int(a.Sec) - int(b.Sec) })
	buf := make([]byte, 0, len(events)*2+2)
	buf = append(buf, magicByte0, magicVersion2)
	prev := int32(0)
	for _, e := range events {
		delta := max(e.Sec-prev, 0)
		prev = e.Sec
		header := uint64(delta)<<6 | uint64(e.Type&0x3)<<4
		if e.Group >= groupEscape {
			buf = binary.AppendUvarint(buf, header|groupEscape)
			buf = binary.AppendUvarint(buf, uint64(e.Group))
		} else {
			buf = binary.AppendUvarint(buf, header|uint64(e.Group))
		}
		switch e.Type {
		case TypeAssignUnits:
			buf = append(buf, e.Count)
		case TypeAssignBuilding:
			buf = append(buf, e.Building, e.TileX, e.TileY)
		}
	}
	return buf
}

// Decode parses a blob back into time-ordered events. Legacy v1 blobs (no
// magic prefix) are converted: frames become seconds and assigns carry no
// count or building annotation.
func Decode(data []byte) ([]Event, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) >= 2 && data[0] == magicByte0 && data[1] == magicVersion2 {
		return decodeV2(data[2:])
	}
	return decodeV1(data)
}

func decodeV2(data []byte) ([]Event, error) {
	events := make([]Event, 0, len(data)/2)
	sec := int64(0)
	for i := 0; i < len(data); {
		v, n := binary.Uvarint(data[i:])
		if n <= 0 {
			return nil, fmt.Errorf("invalid uvarint at offset %d", i)
		}
		i += n
		sec += int64(v >> 6)
		e := Event{Sec: int32(sec), Type: byte(v>>4) & 0x3}
		group := v & 0xF
		if group == groupEscape {
			g, n := binary.Uvarint(data[i:])
			if n <= 0 {
				return nil, fmt.Errorf("invalid group uvarint at offset %d", i)
			}
			i += n
			if g > 255 {
				return nil, fmt.Errorf("group %d out of range at offset %d", g, i)
			}
			group = g
		}
		e.Group = byte(group)
		switch e.Type {
		case TypeAssignUnits:
			if i >= len(data) {
				return nil, fmt.Errorf("truncated count at offset %d", i)
			}
			e.Count = data[i]
			i++
		case TypeAssignBuilding:
			if i+3 > len(data) {
				return nil, fmt.Errorf("truncated building annotation at offset %d", i)
			}
			e.Building, e.TileX, e.TileY = data[i], data[i+1], data[i+2]
			i += 3
		}
		events = append(events, e)
	}
	return events, nil
}

func decodeV1(data []byte) ([]Event, error) {
	events := make([]Event, 0, len(data)/2)
	frame := int64(0)
	for i := 0; i < len(data); {
		v, n := binary.Uvarint(data[i:])
		if n <= 0 {
			return nil, fmt.Errorf("invalid uvarint at offset %d", i)
		}
		i += n
		frame += int64(v >> 6)
		e := Event{Sec: int32(frame * frameMillis / 1000), Type: byte(v>>4) & 0x3}
		group := v & 0xF
		if group == groupEscape {
			g, n := binary.Uvarint(data[i:])
			if n <= 0 {
				return nil, fmt.Errorf("invalid group uvarint at offset %d", i)
			}
			i += n
			if g > 255 {
				return nil, fmt.Errorf("group %d out of range at offset %d", g, i)
			}
			group = g
		}
		e.Group = byte(group)
		events = append(events, e)
	}
	return events, nil
}
