// Package hotkeystream encodes a player's hotkey commands (Select / Assign /
// Add) as a compact delta-encoded varint blob, stored in players.hotkey_stream.
// Wire format per event (issue #357):
//
//	uvarint(Δframe<<6 | type<<4 | group)   when group < 15
//	uvarint(Δframe<<6 | type<<4 | 15), uvarint(group)   when group >= 15
//
// Δframe is the frame delta from the previous event (events are frame-ordered),
// type is 2 bits (0=Select, 1=Assign, 2=Add) and the low nibble holds the
// hotkey group, with 15 escaping to a follow-up uvarint for the rare replays
// carrying groups beyond 14.
package hotkeystream

import (
	"encoding/binary"
	"fmt"
	"slices"
)

const (
	TypeSelect byte = 0
	TypeAssign byte = 1
	TypeAdd    byte = 2
)

const groupEscape = 15

// Event is one hotkey command in a player's stream.
type Event struct {
	Frame int32
	Type  byte
	Group byte
}

// TypeFromName maps a screp hotkey type name to its wire value.
func TypeFromName(name string) (byte, bool) {
	switch name {
	case "Select":
		return TypeSelect, true
	case "Assign":
		return TypeAssign, true
	case "Add":
		return TypeAdd, true
	}
	return 0, false
}

// TypeName maps a wire value back to its screp hotkey type name.
func TypeName(t byte) string {
	switch t {
	case TypeSelect:
		return "Select"
	case TypeAssign:
		return "Assign"
	case TypeAdd:
		return "Add"
	}
	return "UNKNOWN"
}

// Encode serializes events into the blob format. It sorts events by frame in
// place first, so delta encoding is safe even if the caller's stream carries
// out-of-order frames. Returns nil for an empty stream (stored as NULL).
func Encode(events []Event) []byte {
	if len(events) == 0 {
		return nil
	}
	slices.SortStableFunc(events, func(a, b Event) int { return int(a.Frame) - int(b.Frame) })
	buf := make([]byte, 0, len(events)*2+binary.MaxVarintLen64)
	prev := int32(0)
	for _, e := range events {
		delta := max(e.Frame-prev, 0)
		prev = e.Frame
		header := uint64(delta)<<6 | uint64(e.Type&0x3)<<4
		if e.Group >= groupEscape {
			buf = binary.AppendUvarint(buf, header|groupEscape)
			buf = binary.AppendUvarint(buf, uint64(e.Group))
		} else {
			buf = binary.AppendUvarint(buf, header|uint64(e.Group))
		}
	}
	return buf
}

// Decode parses a blob back into frame-ordered events.
func Decode(data []byte) ([]Event, error) {
	events := make([]Event, 0, len(data)/2)
	frame := int64(0)
	for i := 0; i < len(data); {
		v, n := binary.Uvarint(data[i:])
		if n <= 0 {
			return nil, fmt.Errorf("invalid uvarint at offset %d", i)
		}
		i += n
		frame += int64(v >> 6)
		e := Event{Frame: int32(frame), Type: byte(v>>4) & 0x3}
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
