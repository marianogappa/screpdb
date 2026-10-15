package engine

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const codecMagic = "SDG1"

var errCorrupt = errors.New("engine digest: corrupt")

func Encode(d *Digest) ([]byte, error) {
	var w writer
	w.int(int64(d.EngineVersion))
	w.int(int64(d.EndFrame))
	w.int(int64(d.LastFrame))
	w.uint(uint64(len(d.Players)))
	for _, p := range d.Players {
		w.int(int64(p.Slot))
		w.int(int64(p.PlayerID))
		w.uint(uint64(p.Race))
	}
	w.uint(uint64(len(d.Productions)))
	var prev int32
	for _, p := range d.Productions {
		w.int(int64(p.Start - prev))
		prev = p.Start
		w.int(int64(p.Owner))
		w.uint(uint64(p.Type))
		w.uint(uint64(p.From))
		w.uint(uint64(p.Unit))
		w.uint(uint64(p.Producer))
		w.int(int64(p.X))
		w.int(int64(p.Y))
		w.int(int64(p.Complete - p.Start))
		w.int(int64(p.Cancel - p.Start))
		w.int(int64(p.Cmd))
	}
	w.uint(uint64(len(d.Research)))
	prev = 0
	for _, r := range d.Research {
		w.int(int64(r.Start - prev))
		prev = r.Start
		w.int(int64(r.Owner))
		w.bool(r.Upgrade)
		w.uint(uint64(r.ID))
		w.int(int64(r.Level))
		w.uint(uint64(r.Producer))
		w.int(int64(r.Complete - r.Start))
		w.int(int64(r.Cancel - r.Start))
	}
	w.uint(uint64(len(d.Deaths)))
	prev = 0
	for _, x := range d.Deaths {
		w.int(int64(x.Frame - prev))
		prev = x.Frame
		w.uint(uint64(x.Unit))
		w.uint(uint64(x.Type))
		w.int(int64(x.Owner))
		w.uint(uint64(x.Class))
		w.int(int64(x.X))
		w.int(int64(x.Y))
		w.uint(uint64(x.Reason))
		w.uint(uint64(x.KillerType))
		w.int(int64(x.KillerOwner))
	}
	w.uint(uint64(len(d.OwnerChanges)))
	prev = 0
	for _, c := range d.OwnerChanges {
		w.int(int64(c.Frame - prev))
		prev = c.Frame
		w.uint(uint64(c.Unit))
		w.uint(uint64(c.Type))
		w.int(int64(c.From))
		w.int(int64(c.To))
		w.uint(uint64(c.Reason))
	}
	w.uint(uint64(len(d.Outcomes)))
	for _, o := range d.Outcomes {
		w.int(int64(o.Frame))
		w.int(int64(o.Owner))
		w.uint(uint64(o.Kind))
	}
	writeEconomy(&w, d.Economy)
	writeArmy(&w, d.Army)
	w.str(d.Drift.EngineError)
	w.int(int64(d.Drift.ErrorFrame))
	w.int(int64(d.Drift.Selects))
	w.int(int64(d.Drift.MissingSelects))
	w.int(int64(d.Drift.FirstMissing))
	w.int(int64(d.Drift.DesyncFrame))
	w.int(int64(d.Drift.UnlinkedCommands))
	w.int(int64(d.Drift.LinkedCommands))

	var out bytes.Buffer
	out.WriteString(codecMagic)
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

// Economy is written per owner so each value is a delta from the same owner's previous second.
func writeEconomy(w *writer, samples []EconomySample) {
	w.uint(uint64(len(samples)))
	var prev [maxPlayerSlot + 1]EconomySample
	var prevFrame int32
	for _, s := range samples {
		w.int(int64(s.Frame - prevFrame))
		prevFrame = s.Frame
		w.int(int64(s.Owner))
		p := &prev[s.Owner&maxPlayerSlot]
		w.int(int64(s.Minerals - p.Minerals))
		w.int(int64(s.Gas - p.Gas))
		w.int(int64(s.GatheredMinerals - p.GatheredMinerals))
		w.int(int64(s.GatheredGas - p.GatheredGas))
		w.int(int64(s.SupplyUsed - p.SupplyUsed))
		w.int(int64(s.SupplyMax - p.SupplyMax))
		w.int(int64(s.Workers - p.Workers))
		w.int(int64(s.Larva - p.Larva))
		*p = s
	}
}

func writeArmy(w *writer, snaps []ArmySnapshot) {
	w.uint(uint64(len(snaps)))
	var prevFrame int32
	for _, s := range snaps {
		w.int(int64(s.Frame - prevFrame))
		prevFrame = s.Frame
		w.uint(uint64(len(s.Units)))
		for _, u := range s.Units {
			w.int(int64(u.Owner))
			w.uint(uint64(u.Type))
			w.int(int64(u.X))
			w.int(int64(u.Y))
		}
	}
}

func Decode(b []byte) (*Digest, error) {
	if !bytes.HasPrefix(b, []byte(codecMagic)) {
		return nil, errCorrupt
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(b[len(codecMagic):])))
	if err != nil {
		return nil, fmt.Errorf("engine digest: %w", err)
	}
	r := reader{buf: raw}
	d := &Digest{}
	d.EngineVersion = int32(r.int())
	d.EndFrame = int32(r.int())
	d.LastFrame = int32(r.int())
	d.Players = makeN[[]Player](r.count())
	for i := range d.Players {
		d.Players[i] = Player{Slot: int8(r.int()), PlayerID: int32(r.int()), Race: uint8(r.uint())}
	}
	d.Productions = makeN[[]Production](r.count())
	var prev int32
	for i := range d.Productions {
		p := &d.Productions[i]
		p.Start = prev + int32(r.int())
		prev = p.Start
		p.Owner = int8(r.int())
		p.Type = uint16(r.uint())
		p.From = uint16(r.uint())
		p.Unit = uint32(r.uint())
		p.Producer = uint32(r.uint())
		p.X = int32(r.int())
		p.Y = int32(r.int())
		p.Complete = p.Start + int32(r.int())
		p.Cancel = p.Start + int32(r.int())
		p.Cmd = int32(r.int())
	}
	d.Research = makeN[[]Research](r.count())
	prev = 0
	for i := range d.Research {
		x := &d.Research[i]
		x.Start = prev + int32(r.int())
		prev = x.Start
		x.Owner = int8(r.int())
		x.Upgrade = r.bool()
		x.ID = uint8(r.uint())
		x.Level = int32(r.int())
		x.Producer = uint32(r.uint())
		x.Complete = x.Start + int32(r.int())
		x.Cancel = x.Start + int32(r.int())
	}
	d.Deaths = makeN[[]Death](r.count())
	prev = 0
	for i := range d.Deaths {
		x := &d.Deaths[i]
		x.Frame = prev + int32(r.int())
		prev = x.Frame
		x.Unit = uint32(r.uint())
		x.Type = uint16(r.uint())
		x.Owner = int8(r.int())
		x.Class = uint8(r.uint())
		x.X = int32(r.int())
		x.Y = int32(r.int())
		x.Reason = uint8(r.uint())
		x.KillerType = uint16(r.uint())
		x.KillerOwner = int8(r.int())
	}
	d.OwnerChanges = makeN[[]OwnerChange](r.count())
	prev = 0
	for i := range d.OwnerChanges {
		c := &d.OwnerChanges[i]
		c.Frame = prev + int32(r.int())
		prev = c.Frame
		c.Unit = uint32(r.uint())
		c.Type = uint16(r.uint())
		c.From = int8(r.int())
		c.To = int8(r.int())
		c.Reason = uint8(r.uint())
	}
	d.Outcomes = makeN[[]Outcome](r.count())
	for i := range d.Outcomes {
		d.Outcomes[i] = Outcome{Frame: int32(r.int()), Owner: int8(r.int()), Kind: OutcomeKind(r.uint())}
	}
	d.Economy = readEconomy(&r)
	d.Army = readArmy(&r)
	d.Drift.EngineError = r.str()
	d.Drift.ErrorFrame = int32(r.int())
	d.Drift.Selects = int32(r.int())
	d.Drift.MissingSelects = int32(r.int())
	d.Drift.FirstMissing = int32(r.int())
	d.Drift.DesyncFrame = int32(r.int())
	d.Drift.UnlinkedCommands = int32(r.int())
	d.Drift.LinkedCommands = int32(r.int())
	if r.err != nil || len(r.buf) != 0 {
		return nil, errCorrupt
	}
	return d, nil
}

func readEconomy(r *reader) []EconomySample {
	samples := makeN[[]EconomySample](r.count())
	var prev [maxPlayerSlot + 1]EconomySample
	var prevFrame int32
	for i := range samples {
		s := &samples[i]
		s.Frame = prevFrame + int32(r.int())
		prevFrame = s.Frame
		s.Owner = int8(r.int())
		p := &prev[s.Owner&maxPlayerSlot]
		s.Minerals = p.Minerals + int32(r.int())
		s.Gas = p.Gas + int32(r.int())
		s.GatheredMinerals = p.GatheredMinerals + int32(r.int())
		s.GatheredGas = p.GatheredGas + int32(r.int())
		s.SupplyUsed = p.SupplyUsed + int32(r.int())
		s.SupplyMax = p.SupplyMax + int32(r.int())
		s.Workers = p.Workers + int32(r.int())
		s.Larva = p.Larva + int32(r.int())
		*p = *s
	}
	return samples
}

func readArmy(r *reader) []ArmySnapshot {
	snaps := makeN[[]ArmySnapshot](r.count())
	var prevFrame int32
	for i := range snaps {
		s := &snaps[i]
		s.Frame = prevFrame + int32(r.int())
		prevFrame = s.Frame
		s.Units = makeN[[]ArmyUnit](r.count())
		for j := range s.Units {
			s.Units[j] = ArmyUnit{Owner: int8(r.int()), Type: uint16(r.uint()), X: int32(r.int()), Y: int32(r.int())}
		}
	}
	return snaps
}

func makeN[S ~[]E, E any](n int) S {
	if n == 0 {
		return nil
	}
	return make(S, n)
}

type writer struct{ buf []byte }

func (w *writer) int(v int64)   { w.buf = binary.AppendVarint(w.buf, v) }
func (w *writer) uint(v uint64) { w.buf = binary.AppendUvarint(w.buf, v) }
func (w *writer) str(s string)  { w.uint(uint64(len(s))); w.buf = append(w.buf, s...) }
func (w *writer) bool(b bool) {
	if b {
		w.uint(1)
	} else {
		w.uint(0)
	}
}

type reader struct {
	buf []byte
	err error
}

func (r *reader) uint() uint64 {
	v, n := binary.Uvarint(r.buf)
	if n <= 0 {
		r.fail()
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *reader) int() int64 {
	v, n := binary.Varint(r.buf)
	if n <= 0 {
		r.fail()
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *reader) bool() bool { return r.uint() != 0 }

func (r *reader) str() string {
	n := r.count()
	s := string(r.buf[:n])
	r.buf = r.buf[n:]
	return s
}

// count bounds a length by the bytes left so a corrupt record cannot force a huge allocation.
func (r *reader) count() int {
	n := r.uint()
	if n > uint64(len(r.buf)) {
		r.fail()
		return 0
	}
	return int(n)
}

func (r *reader) fail() {
	if r.err == nil {
		r.err = errCorrupt
	}
	r.buf = nil
}
