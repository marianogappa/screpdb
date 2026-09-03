package compact

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
)

type layoutKey struct {
	name   string
	width  int
	height int
	hash   [32]byte
}

var layouts sync.Map

// internLayout returns one shared *MapLayout per (map name, size, base
// geometry) so every replay on the same map points at the same structure.
func internLayout(mapName string, l *models.MapContextLayout) *library.MapLayout {
	if l == nil {
		return nil
	}
	key := layoutKey{name: mapName, width: l.WidthTiles, height: l.HeightTiles, hash: hashBases(l.Bases)}
	if cached, ok := layouts.Load(key); ok {
		return cached.(*library.MapLayout)
	}
	built := &library.MapLayout{
		Name:        mapName,
		WidthTiles:  library.ClampU16(l.WidthTiles),
		HeightTiles: library.ClampU16(l.HeightTiles),
		Bases:       make([]library.MapBase, 0, len(l.Bases)),
	}
	for _, b := range l.Bases {
		base := library.MapBase{
			Name:             b.Name,
			Kind:             b.Kind,
			NaturalExpansion: b.NaturalExpansion,
			Clock:            library.ClampU8(b.Clock),
			MineralOnly:      b.MineralOnly,
			CenterX:          int32(b.Center.X),
			CenterY:          int32(b.Center.Y),
			Polygon:          make([]library.MapPoint, 0, len(b.Polygon)),
		}
		for _, pt := range b.Polygon {
			base.Polygon = append(base.Polygon, library.MapPoint{X: int32(pt.X), Y: int32(pt.Y)})
		}
		built.Bases = append(built.Bases, base)
	}
	shared, _ := layouts.LoadOrStore(key, built)
	return shared.(*library.MapLayout)
}

func hashBases(bases []models.MapContextBase) [32]byte {
	h := sha256.New()
	var buf [8]byte
	writeInt := func(v int) {
		binary.LittleEndian.PutUint64(buf[:], uint64(int64(v)))
		h.Write(buf[:])
	}
	writeString := func(s string) {
		writeInt(len(s))
		h.Write([]byte(s))
	}
	for _, b := range bases {
		writeString(b.Name)
		writeString(b.Kind)
		writeString(b.NaturalExpansion)
		writeInt(b.Clock)
		writeInt(b.Center.X)
		writeInt(b.Center.Y)
		if b.MineralOnly {
			writeInt(1)
		} else {
			writeInt(0)
		}
		writeInt(len(b.Polygon))
		for _, pt := range b.Polygon {
			writeInt(pt.X)
			writeInt(pt.Y)
		}
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
