package engine

import (
	"reflect"
	"testing"
)

func sampleDigest() *Digest {
	return &Digest{
		EngineVersion: EngineVersion,
		EndFrame:      20000,
		LastFrame:     19998,
		Players:       []Player{{Slot: 0, PlayerID: 0, Race: 1}, {Slot: 3, PlayerID: 2, Race: 0}},
		Productions: []Production{
			{Owner: 0, Type: 7, From: 228, Unit: 2049, Producer: 1, X: 1200, Y: 300, Start: 10, Complete: 310, Cancel: NoFrame, Cmd: 4},
			{Owner: 3, Type: 131, From: 41, Unit: 3, Producer: 3, X: 64, Y: 4000, Start: 900, Complete: NoFrame, Cancel: 1000, Cmd: NoCmd},
		},
		Research:     []Research{{Owner: 3, Upgrade: true, ID: 10, Level: 2, Producer: 77, Start: 5000, Complete: 9000, Cancel: NoFrame}},
		Deaths:       []Death{{Frame: 7000, Unit: 2049, Type: 7, Owner: 0, X: 1300, Y: 310, Reason: 1, KillerType: 37, KillerOwner: 3}},
		OwnerChanges: []OwnerChange{{Frame: 8000, Unit: 4097, Type: 0, From: 0, To: 3, Reason: 1}},
		Outcomes:     []Outcome{{Frame: 19990, Owner: 0, Kind: OutcomeLeft}, {Frame: 19998, Owner: 3, Kind: OutcomeWon}},
		Economy: []EconomySample{
			{Frame: 24, Owner: 0, Minerals: 50, SupplyUsed: 8, SupplyMax: 20, Workers: 4},
			{Frame: 24, Owner: 3, Minerals: 50, SupplyUsed: 8, SupplyMax: 18, Workers: 4, Larva: 3},
			{Frame: 48, Owner: 0, Minerals: 10, GatheredMinerals: 40, SupplyUsed: 10, SupplyMax: 20, Workers: 5},
			{Frame: 48, Owner: 3, Minerals: 2, Gas: 0, GatheredMinerals: 40, SupplyUsed: 10, SupplyMax: 18, Workers: 4, Larva: 2},
		},
		Army:  []ArmySnapshot{{Frame: 0}, {Frame: 240, Units: []ArmyUnit{{Owner: 3, Type: 37, X: 10, Y: 200}}}},
		Drift: Drift{EngineError: "boom", ErrorFrame: 19998, MissingSelects: 3, FirstMissing: 400, DesyncFrame: NoFrame, UnlinkedCommands: 1, LinkedCommands: 900},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := sampleDigest()
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
}

func TestEncodeIsDeterministic(t *testing.T) {
	a, _ := Encode(sampleDigest())
	b, _ := Encode(sampleDigest())
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two encodings of one digest differ")
	}
}

func TestDecodeRejectsCorruptInput(t *testing.T) {
	good, err := Encode(sampleDigest())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":     nil,
		"bad magic": append([]byte("XXXX"), good[4:]...),
		"truncated": good[:len(good)/2],
	}
	for name, b := range cases {
		if _, err := Decode(b); err == nil {
			t.Errorf("%s: Decode succeeded", name)
		}
	}
	for i := len(codecMagic); i < len(good); i++ {
		b := append([]byte(nil), good...)
		b[i] ^= 0xff
		_, _ = Decode(b)
	}
}
