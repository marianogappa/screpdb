package engine

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"slices"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

const snapshotMagic = "SNP1"

// SnapshotUnit is one unit or building as the engine had it, in map pixels.
type SnapshotUnit struct {
	Type      uint16
	Owner     int8
	X, Y      int32
	Completed bool
	Burrowed  bool
	Cloaked   bool
}

// Snapshot is the whole map at one second: every unit and building of every player, workers included.
type Snapshot struct {
	Second int
	Units  []SnapshotUnit
}

// Snapshots replays the game up to the last requested second and reads the engine's state at each one.
// It costs one engine pass up to that second; callers cache the result.
func Snapshots(ctx context.Context, replay []byte, data *Data, seconds []int) (map[int]Snapshot, error) {
	wanted := slices.Clone(seconds)
	slices.Sort(wanted)
	wanted = slices.Compact(wanted)
	g, err := screpsim.Open(replay, screpsim.Options{Engine: openbw.Options{Data: data, Kinds: openbw.NoKinds}})
	if err != nil {
		return nil, fmt.Errorf("engine snapshots: %w", err)
	}
	e := g.Engine()
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
			snap.Units = append(snap.Units, SnapshotUnit{
				Type: uint16(u.Type), Owner: u.Owner, X: u.X, Y: u.Y,
				Completed: u.Completed, Burrowed: u.Burrowed, Cloaked: u.Cloaked,
			})
		}
		out[sec] = snap
	}
	return out, nil
}

const (
	flagCompleted = 1 << iota
	flagBurrowed
	flagCloaked
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
			w.uint(uint64(u.Type))
			w.int(int64(u.Owner))
			w.int(int64(u.X))
			w.int(int64(u.Y))
			w.uint(flags)
		}
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
			u.Completed, u.Burrowed, u.Cloaked = flags&flagCompleted != 0, flags&flagBurrowed != 0, flags&flagCloaked != 0
		}
		out[sec] = snap
	}
	if r.err != nil || len(r.buf) != 0 {
		return nil, errCorrupt
	}
	return out, nil
}
