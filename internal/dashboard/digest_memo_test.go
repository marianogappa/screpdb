//go:build !js

package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
)

func TestARedoneGameIsReadAfreshNotFromTheOlderDigestKept(t *testing.T) {
	var m digestMemo
	checksum := [32]byte{1}
	loads := 0
	load := func(version int32) func() (*engine.Digest, bool) {
		return func() (*engine.Digest, bool) {
			loads++
			return &engine.Digest{EngineVersion: version}, true
		}
	}
	if d := m.get(checksum, 12, load(12)); d.EngineVersion != 12 {
		t.Fatalf("first read: %d", d.EngineVersion)
	}
	m.get(checksum, 12, load(12))
	if loads != 1 {
		t.Fatalf("a second view of one game reuses its digest: %d loads", loads)
	}
	if d := m.get(checksum, 13, load(13)); d.EngineVersion != 13 || loads != 2 {
		t.Fatalf("once the game is redone on version 13, its page reads the new digest: version %d, %d loads", d.EngineVersion, loads)
	}
	if len(m.order) != 1 {
		t.Fatalf("the game is kept once: %v", m.order)
	}
}
