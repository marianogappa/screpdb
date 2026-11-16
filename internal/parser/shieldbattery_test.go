package parser

import (
	"testing"

	scraprep "github.com/icza/screp/rep"
)

func TestShieldBatteryGameKeysAccountsByReplayPlayerID(t *testing.T) {
	r := &scraprep.Replay{
		Header: &scraprep.Header{Players: []*scraprep.Player{{ID: 0}, nil, {ID: 3}, {ID: 5}}},
		ShieldBattery: &scraprep.ShieldBattery{
			GameID:    "01a1da1c-8486-7801-827d-b56d61fe89e8",
			PidGameID: map[byte]uint32{0: 17185, 3: 18606},
		},
	}
	game := shieldBatteryGame(r)
	if game == nil || game.GameID != r.ShieldBattery.GameID {
		t.Fatalf("got %+v", game)
	}
	if len(game.UserIDs) != 2 || game.UserIDs[0] != 17185 || game.UserIDs[3] != 18606 {
		t.Fatalf("user ids %+v", game.UserIDs)
	}
}

func TestShieldBatteryGameAbsentOutsideShieldBattery(t *testing.T) {
	if game := shieldBatteryGame(&scraprep.Replay{Header: &scraprep.Header{}}); game != nil {
		t.Fatalf("got %+v", game)
	}
	if game := shieldBatteryGame(&scraprep.Replay{Header: &scraprep.Header{}, ShieldBattery: &scraprep.ShieldBattery{}}); game != nil {
		t.Fatalf("section without a game id gave %+v", game)
	}
}
