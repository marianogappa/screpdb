package engine

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"slices"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpsim"
)

const snapshotMagic = "SNP3"

// ProgressDone is the Progress of a unit that is not being built or morphed.
const ProgressDone = 255

// SnapshotUnit is one unit or building as the engine had it, in map pixels.
type SnapshotUnit struct {
	Type      uint16
	Owner     int8
	X, Y      int32
	Completed bool
	Burrowed  bool
	Cloaked   bool
	Lifted    bool
	// Progress is how far a construction, warp-in or morph is, 0..254 of 255; ProgressDone otherwise.
	Progress uint8
}

// Snapshot is the whole map at one second: every unit and building of every player, workers included,
// and the map's creep, one bit per tile, row by row (CreepW tiles wide).
type Snapshot struct {
	Second int
	Units  []SnapshotUnit
	CreepW int
	Creep  []byte
	// Tileset is the map's tileset index, 0 (Badlands) to 7 (Twilight).
	Tileset int
}

func (s Snapshot) HasCreep(tx, ty int) bool {
	i := ty*s.CreepW + tx
	return s.CreepW > 0 && tx >= 0 && tx < s.CreepW && i >= 0 && i/8 < len(s.Creep) && s.Creep[i/8]&(1<<(i%8)) != 0
}

// Snapshots replays the game up to the last requested second and reads the engine's state at each one.
// It costs one engine pass up to that second; callers cache the result.
func Snapshots(ctx context.Context, replay []byte, data *Data, seconds []int) (map[int]Snapshot, error) {
	wanted := slices.Clone(seconds)
	slices.Sort(wanted)
	wanted = slices.Compact(wanted)
	g, err := screpsim.Open(replay, screpsim.Options{Engine: openbw.Options{Data: data, Kinds: openbw.NoKinds, Engine: &openbw.EngineOptions{}}})
	if err != nil {
		return nil, fmt.Errorf("engine snapshots: %w", err)
	}
	e := g.Engine()
	st := e.Engine().State()
	out := map[int]Snapshot{}
	for _, sec := range wanted {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		frame := int32(sec * EconomyEveryFrames)
		if frame > e.EndFrame() {
			break
		}
		if err := e.AdvanceTo(frame); err != nil {
			return nil, fmt.Errorf("engine snapshots: %w", err)
		}
		snap := Snapshot{Second: sec}
		for u := range e.Units() {
			if u.Owner > maxPlayerSlot || u.Dying || u.Loaded || u.Hallucination || u.Type == larvaType {
				continue
			}
			raw := st.UnitsContainer.TryGet(uint(u.ID.Index() - 1))
			if raw == nil || hidden(raw) {
				continue
			}
			snap.Units = append(snap.Units, SnapshotUnit{
				Type: uint16(u.Type), Owner: u.Owner, X: u.X, Y: u.Y,
				Completed: u.Completed, Burrowed: u.Burrowed, Cloaked: u.Cloaked,
				Lifted:   raw.UnitType.Flags&bwgame.UnitTypeTFlagBuilding != 0 && raw.StatusFlags&bwgame.UnitStatusFlagFlying != 0,
				Progress: progress(raw),
			})
		}
		snap.CreepW, snap.Creep = creepBits(st)
		snap.Tileset = int(st.Game.TilesetIndex)
		out[sec] = snap
	}
	return out, nil
}

// hidden is a unit the game does not draw: one still being trained inside its building, or a worker
// inside a Refinery.
func hidden(u *bwgame.Unit) bool {
	return u.Sprite == nil || u.Sprite.Flags&bwgame.SpriteFlagHidden != 0
}

// progress measures a construction against the type it becomes: the building itself, or for a morph
// (Egg, Cocoon, Hatchery to Lair, Creep Colony to Sunken) the head of its build queue.
func progress(u *bwgame.Unit) uint8 {
	morphing := u.OrderType != nil && (u.OrderType.ID == bwgame.OrdersIncompleteMorphing || u.OrderType.ID == bwgame.OrdersZergUnitMorph)
	if u.StatusFlags&bwgame.UnitStatusFlagCompleted != 0 && !morphing {
		return ProgressDone
	}
	total := u.UnitType.BuildTime
	if morphing && !u.BuildQueue.Empty() && u.BuildQueue.Front() != nil {
		total = u.BuildQueue.Front().BuildTime
	}
	if total <= 0 {
		return ProgressDone
	}
	done := float64(total-u.RemainingBuildTime) / float64(total)
	return uint8(max(0, min(254, int(done*254))))
}

func creepBits(st *bwgame.State) (int, []byte) {
	w, h := int(st.Game.MapTileWidth), int(st.Game.MapTileHeight)
	if w == 0 || h == 0 || len(st.Tiles) < w*h {
		return 0, nil
	}
	bits := make([]byte, (w*h+7)/8)
	found := false
	for i := range w * h {
		if st.Tiles[i].Flags&bwgame.TileFlagHasCreep != 0 {
			bits[i/8] |= 1 << (i % 8)
			found = true
		}
	}
	if !found {
		return w, nil
	}
	return w, bits
}

const (
	flagCompleted = 1 << iota
	flagBurrowed
	flagCloaked
	flagLifted
)

func EncodeSnapshots(snaps map[int]Snapshot) ([]byte, error) {
	var w writer
	secs := make([]int, 0, len(snaps))
	for s := range snaps {
		secs = append(secs, s)
	}
	slices.Sort(secs)
	w.uint(uint64(len(secs)))
	for _, s := range secs {
		w.uint(uint64(s))
		units := snaps[s].Units
		w.uint(uint64(len(units)))
		for _, u := range units {
			var flags uint64
			if u.Completed {
				flags |= flagCompleted
			}
			if u.Burrowed {
				flags |= flagBurrowed
			}
			if u.Cloaked {
				flags |= flagCloaked
			}
			if u.Lifted {
				flags |= flagLifted
			}
			w.uint(uint64(u.Type))
			w.int(int64(u.Owner))
			w.int(int64(u.X))
			w.int(int64(u.Y))
			w.uint(flags)
			w.uint(uint64(u.Progress))
		}
		w.uint(uint64(snaps[s].CreepW))
		w.str(string(snaps[s].Creep))
		w.uint(uint64(snaps[s].Tileset))
	}
	var out bytes.Buffer
	out.WriteString(snapshotMagic)
	fw, err := flate.NewWriter(&out, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(w.buf); err != nil {
		return nil, err
	}
	if err := fw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func DecodeSnapshots(b []byte) (map[int]Snapshot, error) {
	if !bytes.HasPrefix(b, []byte(snapshotMagic)) {
		return nil, errCorrupt
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(b[len(snapshotMagic):])))
	if err != nil {
		return nil, fmt.Errorf("engine snapshots: %w", err)
	}
	r := reader{buf: raw}
	out := map[int]Snapshot{}
	for range r.count() {
		sec := int(r.uint())
		snap := Snapshot{Second: sec, Units: makeN[[]SnapshotUnit](r.count())}
		for i := range snap.Units {
			u := &snap.Units[i]
			u.Type = uint16(r.uint())
			u.Owner = int8(r.int())
			u.X = int32(r.int())
			u.Y = int32(r.int())
			flags := r.uint()
			u.Completed, u.Burrowed, u.Cloaked, u.Lifted = flags&flagCompleted != 0, flags&flagBurrowed != 0, flags&flagCloaked != 0, flags&flagLifted != 0
			u.Progress = uint8(r.uint())
		}
		snap.CreepW = int(r.uint())
		if creep := r.str(); creep != "" {
			snap.Creep = []byte(creep)
		}
		snap.Tileset = int(r.uint())
		out[sec] = snap
	}
	if r.err != nil || len(r.buf) != 0 {
		return nil, errCorrupt
	}
	return out, nil
}
