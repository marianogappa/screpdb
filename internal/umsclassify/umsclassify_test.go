package umsclassify

import (
	"encoding/binary"
	"testing"
)

func makeSection(name [4]byte, data []byte) []byte {
	buf := make([]byte, 8+len(data))
	copy(buf[:4], name[:])
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(data)))
	copy(buf[8:], data)
	return buf
}

func makeTrigger(actions []trigAction) []byte {
	buf := make([]byte, triggerSize)
	for i, a := range actions {
		off := actionsOffset + i*actionSize
		buf[off+26] = a.actionType
		binary.LittleEndian.PutUint32(buf[off+20:off+24], a.number)
		if a.modifier != 0 {
			buf[off+27] = a.modifier
		}
	}
	return buf
}

type trigAction struct {
	actionType byte
	number     uint32
	modifier   byte
}

func makeUnit(owner byte, unitID uint16) []byte {
	buf := make([]byte, unitEntrySize)
	buf[unitOwnerOffset] = owner
	binary.LittleEndian.PutUint16(buf[unitIDOffset:unitIDOffset+2], unitID)
	return buf
}

func TestAllowlistedActionsPass(t *testing.T) {
	for _, at := range []byte{
		actionVictory, actionDefeat, actionPreserveTrigger,
		actionWait, actionPlayWAV, actionDisplayText,
		actionComment, actionSetAlliance,
	} {
		trig := makeTrigger([]trigAction{{actionType: at}})
		chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
		r := Classify(chk)
		if !r.MeleeLike {
			t.Errorf("action %d should be allowed, got reason=%s", at, r.Reason)
		}
	}
}

func TestForbiddenActionRejects(t *testing.T) {
	for _, at := range []byte{5, 6, 7, 10, 11, 12, 13, 14, 16, 20, 27, 28, 30, 45} {
		trig := makeTrigger([]trigAction{{actionType: at}})
		chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
		r := Classify(chk)
		if r.MeleeLike {
			t.Errorf("action %d should be forbidden", at)
		}
		if r.Reason != "forbidden_action" {
			t.Errorf("action %d: want reason forbidden_action, got %s", at, r.Reason)
		}
	}
}

func TestRunAIScriptSharedVisionPasses(t *testing.T) {
	var script [4]byte
	copy(script[:], "+Vi0")
	trig := makeTrigger([]trigAction{{actionType: actionRunAIScript, number: binary.LittleEndian.Uint32(script[:])}})
	chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("shared vision AI script should pass, got reason=%s", r.Reason)
	}
}

func TestRunAIScriptOtherRejects(t *testing.T) {
	var script [4]byte
	copy(script[:], "TMCu")
	trig := makeTrigger([]trigAction{{actionType: actionRunAIScript, number: binary.LittleEndian.Uint32(script[:])}})
	chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
	r := Classify(chk)
	if r.MeleeLike {
		t.Fatalf("non-vision AI script should reject")
	}
}

func TestSetResources50Passes(t *testing.T) {
	trig := makeTrigger([]trigAction{{actionType: actionSetResources, number: 50}})
	chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("SetResources 50 should pass, got reason=%s", r.Reason)
	}
}

func TestSetResourcesPhantomPasses(t *testing.T) {
	trig := makeTrigger([]trigAction{{actionType: actionSetResources, number: 50000}})
	chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("SetResources 50000 (Phantom) should pass, got reason=%s", r.Reason)
	}
}

func TestSetResourcesOtherRejects(t *testing.T) {
	for _, amount := range []uint32{100, 200, 500, 999} {
		trig := makeTrigger([]trigAction{{actionType: actionSetResources, number: amount}})
		chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
		r := Classify(chk)
		if r.MeleeLike {
			t.Errorf("SetResources %d should reject", amount)
		}
	}
}

func TestNoTriggersPass(t *testing.T) {
	chk := makeSection([4]byte{'U', 'N', 'I', 'T'}, nil)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("no triggers should pass, got reason=%s", r.Reason)
	}
}

func TestPlayerOwnedUnitRejects(t *testing.T) {
	trig := makeTrigger([]trigAction{{actionType: actionVictory}})
	units := makeUnit(0, 7) // player 0 owns a Marine (id 7)
	chk := append(
		makeSection([4]byte{'T', 'R', 'I', 'G'}, trig),
		makeSection([4]byte{'U', 'N', 'I', 'T'}, units)...,
	)
	r := Classify(chk)
	if r.MeleeLike {
		t.Fatalf("player-owned unit should reject")
	}
}

func TestStartLocationAllowed(t *testing.T) {
	units := makeUnit(0, startLocationUnitID)
	chk := makeSection([4]byte{'U', 'N', 'I', 'T'}, units)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("start location should be allowed, got reason=%s", r.Reason)
	}
}

func TestNeutralUnitsAllowed(t *testing.T) {
	units := makeUnit(11, 176) // neutral (owner >= 8) mineral field
	chk := makeSection([4]byte{'U', 'N', 'I', 'T'}, units)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("neutral unit should be allowed, got reason=%s", r.Reason)
	}
}

func TestObserverMapPasses(t *testing.T) {
	var sv [4]byte
	copy(sv[:], "+Vi0")
	trig := makeTrigger([]trigAction{
		{actionType: actionVictory},
		{actionType: actionDefeat},
		{actionType: actionSetResources, number: 50},
		{actionType: actionRunAIScript, number: binary.LittleEndian.Uint32(sv[:])},
		{actionType: actionDisplayText},
		{actionType: actionPlayWAV},
	})
	units := append(makeUnit(0, startLocationUnitID), makeUnit(11, 176)...)
	chk := append(
		makeSection([4]byte{'T', 'R', 'I', 'G'}, trig),
		makeSection([4]byte{'U', 'N', 'I', 'T'}, units)...,
	)
	r := Classify(chk)
	if !r.MeleeLike {
		t.Fatalf("observer map should pass, got reason=%s", r.Reason)
	}
}

func TestTrainerMapRejects(t *testing.T) {
	trig := makeTrigger([]trigAction{
		{actionType: actionVictory},
		{actionType: 44}, // CreateUnit
	})
	chk := makeSection([4]byte{'T', 'R', 'I', 'G'}, trig)
	r := Classify(chk)
	if r.MeleeLike {
		t.Fatalf("trainer map with CreateUnit should reject")
	}
}

func TestConcatenatedSections(t *testing.T) {
	trig1 := makeTrigger([]trigAction{{actionType: actionVictory}})
	trig2 := makeTrigger([]trigAction{{actionType: 44}}) // CreateUnit
	chk := append(
		makeSection([4]byte{'T', 'R', 'I', 'G'}, trig1),
		makeSection([4]byte{'T', 'R', 'I', 'G'}, trig2)...,
	)
	r := Classify(chk)
	if r.MeleeLike {
		t.Fatalf("concatenated TRIG with forbidden action should reject")
	}
}
