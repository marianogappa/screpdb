//go:build !js

package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
)

func TestEngineAssignsPairStreamAssignsInOrderWithinASecond(t *testing.T) {
	frame := func(sec int64) int32 { return int32(sec * 1000 / 42) }
	dg := &engine.Digest{Hotkeys: []engine.HotkeyAssign{
		{Frame: frame(100), Owner: 1, Group: 1, MainType: 37, Count: 12},
		{Frame: frame(100), Owner: 2, Group: 1, MainType: 0, Count: 3},
		{Frame: frame(300), Owner: 1, Group: 1, MainType: 43, Count: 9},
	}}
	a := newEngineAssigns(dg, 1)
	if m, ok := a.match(1, 100); !ok || m.MainType != 37 || m.Count != 12 {
		t.Fatalf("first assign: %+v %v", m, ok)
	}
	if m, ok := a.match(1, 100); !ok || m.MainType != 37 {
		t.Fatalf("a repeated ctrl+number in the same second is the same assign: %+v %v", m, ok)
	}
	if _, ok := a.match(1, 200); ok {
		t.Fatal("no engine assign near 200s")
	}
	if m, ok := a.match(1, 300); !ok || m.MainType != 43 {
		t.Fatalf("later assign: %+v %v", m, ok)
	}
	if _, ok := a.match(2, 100); ok {
		t.Fatal("another group never matches")
	}
}
