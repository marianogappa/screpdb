package engine

import (
	"bytes"
	"compress/flate"
	"errors"
	"reflect"
	"testing"
)

func fullDigest() *Digest {
	d := sampleDigest()
	d.Research = append(d.Research, Research{Owner: 0, Upgrade: false, ID: 3, Level: 1, Producer: 12, Start: 6000, Complete: NoFrame, Cancel: 6500})
	d.Deaths = append(d.Deaths, Death{Frame: 7100, Unit: 9, Type: 41, Owner: 3, Class: 2, X: -5, Y: 4100, Reason: 2, KillerType: 0, KillerOwner: -1})
	d.OwnerChanges = append(d.OwnerChanges, OwnerChange{Frame: 8100, Unit: 4098, Type: 72, From: 3, To: 0, Reason: 2})
	d.Outcomes = append(d.Outcomes, Outcome{Frame: 19000, Owner: 5, Kind: OutcomeDropped}, Outcome{Frame: 19500, Owner: 6, Kind: OutcomeDefeated})
	d.Economy = append(d.Economy, EconomySample{Frame: 72, Owner: 7, Minerals: 1000, Gas: 500, GatheredMinerals: 4000, GatheredGas: 1200, SupplyUsed: 300, SupplyMax: 400, Workers: 60, Larva: 0})
	d.Army = append(d.Army, ArmySnapshot{Frame: 480, Units: []ArmyUnit{{Owner: 0, Type: 0, X: 1, Y: 2}, {Owner: 3, Type: 43, X: -1, Y: 255}}})
	d.Unloads = []Unload{
		{Frame: 3000, Transport: 501, TransportType: 11, Owner: 0, X: 640, Y: 320, Cargo: []uint16{0, 0, 32, 1}},
		{Frame: 3000, Transport: 502, TransportType: 72, Owner: 3, X: 100, Y: 100},
		{Frame: 4200, Transport: 501, TransportType: 11, Owner: 0, X: 700, Y: 330, Cargo: []uint16{103}},
	}
	d.Teleports = []Teleport{
		{Frame: 9000, Owner: 3, FromX: 100, FromY: 200, ToX: 3900, ToY: 3800, Units: []uint16{65, 65, 66}},
		{Frame: 9600, Owner: 0, FromX: 10, FromY: 20, ToX: 30, ToY: 40},
	}
	d.Casts = []Cast{
		{Frame: 5000, Owner: 3, Order: 0x89, X: 2000, Y: 2100},
		{Frame: 5000, Owner: 0, Order: 0x8e, X: 0, Y: 0},
		{Frame: 7777, Owner: 7, Order: 0x77, X: 4095, Y: 4095},
	}
	d.Hotkeys = []HotkeyAssign{
		{Frame: 100, Owner: 0, Group: 1, MainType: 7, Count: 12},
		{Frame: 100, Owner: 7, Group: 9, MainType: 43, Count: 255},
		{Frame: 2400, Owner: 3, Group: 0, MainType: 131, Count: 1},
	}
	d.Drift = Drift{EngineError: "", ErrorFrame: NoFrame, Selects: 4000, MissingSelects: 0, FirstMissing: NoFrame, DesyncFrame: 12000, UnlinkedCommands: 0, LinkedCommands: 5000}
	return d
}

func TestEncodeDecodeRoundTripEveryField(t *testing.T) {
	want := fullDigest()
	b, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip differs:\n got %+v\nwant %+v", got, want)
	}
	if got.Confidence() != ConfidenceLow || got.TrustedUntil() != 12000 {
		t.Fatalf("decoded drift lost: confidence %d trusted until %d", got.Confidence(), got.TrustedUntil())
	}
}

func TestEncodeDecodeEmptyDigest(t *testing.T) {
	b, err := Encode(&Digest{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &Digest{}) {
		t.Fatalf("empty digest decoded as %+v", got)
	}
}

func deflate(t *testing.T, raw []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteString(codecMagic)
	fw, err := flate.NewWriter(&out, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestDecodeRejectsTrailingBytes(t *testing.T) {
	var w writer
	encodeEmptyBody(&w)
	if _, err := Decode(deflate(t, w.buf)); err != nil {
		t.Fatalf("a well formed body failed: %v", err)
	}
	w.uint(42)
	if _, err := Decode(deflate(t, w.buf)); !errors.Is(err, errCorrupt) {
		t.Fatalf("trailing bytes: got %v, want errCorrupt", err)
	}
}

func encodeEmptyBody(w *writer) {
	for range 3 {
		w.int(0)
	}
	for range 13 {
		w.uint(0)
	}
	w.str("")
	for range 7 {
		w.int(0)
	}
}

func TestDecodeRejectsHugeCount(t *testing.T) {
	var w writer
	w.int(EngineVersion)
	w.int(1)
	w.int(1)
	w.uint(1 << 40)
	if _, err := Decode(deflate(t, w.buf)); !errors.Is(err, errCorrupt) {
		t.Fatalf("got %v, want errCorrupt", err)
	}
}

func TestDecodeRejectsBadFlateStream(t *testing.T) {
	_, err := Decode(append([]byte(codecMagic), 0xff, 0xff, 0xff))
	if err == nil || errors.Is(err, errCorrupt) {
		t.Fatalf("got %v, want a flate error", err)
	}
}

func TestReaderFailsOnTruncatedVarints(t *testing.T) {
	r := reader{buf: []byte{0x80}}
	if v := r.uint(); v != 0 || r.err == nil || r.buf != nil {
		t.Fatalf("uint on truncated varint: v %d err %v buf %v", v, r.err, r.buf)
	}
	r = reader{buf: []byte{0x80}}
	if v := r.int(); v != 0 || r.err == nil {
		t.Fatalf("int on truncated varint: v %d err %v", v, r.err)
	}
	r = reader{buf: []byte{5, 'a', 'b'}}
	if s := r.str(); s != "" || r.err == nil {
		t.Fatalf("str longer than the buffer: %q err %v", s, r.err)
	}
}

func TestReaderKeepsFirstError(t *testing.T) {
	first := errors.New("first")
	r := reader{buf: []byte{1}, err: first}
	r.fail()
	if r.err != first {
		t.Fatalf("fail replaced the first error with %v", r.err)
	}
}

func TestWriterReaderPrimitives(t *testing.T) {
	var w writer
	w.bool(true)
	w.bool(false)
	w.int(-300)
	w.uint(70000)
	w.str("héllo")
	writeTypes(&w, []uint16{1, 0xffff, 0})
	writeTypes(&w, nil)
	r := reader{buf: w.buf}
	if !r.bool() || r.bool() {
		t.Fatal("bools did not round trip")
	}
	if v := r.int(); v != -300 {
		t.Fatalf("int %d", v)
	}
	if v := r.uint(); v != 70000 {
		t.Fatalf("uint %d", v)
	}
	if s := r.str(); s != "héllo" {
		t.Fatalf("str %q", s)
	}
	if ts := readTypes(&r); !reflect.DeepEqual(ts, []uint16{1, 0xffff, 0}) {
		t.Fatalf("types %v", ts)
	}
	if ts := readTypes(&r); ts != nil {
		t.Fatalf("empty types decoded as %v", ts)
	}
	if r.err != nil || len(r.buf) != 0 {
		t.Fatalf("reader left err %v and %d bytes", r.err, len(r.buf))
	}
}

func TestMakeN(t *testing.T) {
	if s := makeN[[]int](0); s != nil {
		t.Fatalf("zero length is %v, want nil", s)
	}
	if s := makeN[[]int](3); len(s) != 3 {
		t.Fatalf("length %d, want 3", len(s))
	}
}

func TestConfidenceAndTrustedUntil(t *testing.T) {
	cases := []struct {
		name  string
		drift Drift
		conf  Confidence
		until int32
	}{
		{"engine error only", Drift{EngineError: "x", ErrorFrame: 300, DesyncFrame: NoFrame}, ConfidenceLow, 300},
		{"error before desync", Drift{EngineError: "x", ErrorFrame: 100, DesyncFrame: 200}, ConfidenceLow, 100},
		{"frames past the end", Drift{ErrorFrame: 5000, DesyncFrame: NoFrame}, ConfidenceHigh, 1001},
	}
	for _, c := range cases {
		d := &Digest{LastFrame: 1000, Drift: c.drift}
		if d.Confidence() != c.conf || d.TrustedUntil() != c.until {
			t.Errorf("%s: confidence %d until %d, want %d %d", c.name, d.Confidence(), d.TrustedUntil(), c.conf, c.until)
		}
	}
}
