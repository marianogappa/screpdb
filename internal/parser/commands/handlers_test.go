package commands

import (
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
)

func baseCmd(id byte, typeName string, playerID byte, frame repcore.Frame) *repcmd.Base {
	return &repcmd.Base{
		Frame:    frame,
		PlayerID: playerID,
		Type:     &repcmd.Type{Enum: repcore.Enum{Name: typeName}, ID: id},
	}
}

func TestBuildHandlerParsesPositionUnitAndOrder(t *testing.T) {
	base := baseCmd(repcmd.TypeIDBuild, "Build", 3, 480)
	cmd := &repcmd.BuildCmd{
		Base:  base,
		Pos:   repcore.Point{X: 42, Y: 99},
		Unit:  &repcmd.Unit{Enum: repcore.Enum{Name: "Barracks"}, ID: 106},
		Order: &repcmd.Order{Enum: repcore.Enum{Name: "BuildingLand"}, ID: 71},
	}

	got := NewBuildCommandHandler().Handle(cmd, base)

	if got == nil {
		t.Fatal("Handle returned nil")
	}
	if got.ActionType != "Build" {
		t.Errorf("ActionType: want Build, got %q", got.ActionType)
	}
	if got.PlayerID != 3 {
		t.Errorf("PlayerID: want 3, got %d", got.PlayerID)
	}
	if got.X == nil || *got.X != 42 || got.Y == nil || *got.Y != 99 {
		t.Errorf("Pos: want (42,99), got %v,%v", got.X, got.Y)
	}
	if got.UnitType == nil || *got.UnitType != "Barracks" {
		t.Errorf("UnitType: want Barracks, got %v", got.UnitType)
	}
	if got.UnitID == nil || *got.UnitID != byte(106) {
		t.Errorf("UnitID: want 106, got %v", got.UnitID)
	}
	if got.OrderName == nil || *got.OrderName != "BuildingLand" {
		t.Errorf("OrderName: want BuildingLand, got %v", got.OrderName)
	}
	if got.OrderID == nil || *got.OrderID != 71 {
		t.Errorf("OrderID: want 71, got %v", got.OrderID)
	}
}

func TestBuildHandlerNilUnitAndOrder(t *testing.T) {
	base := baseCmd(repcmd.TypeIDBuild, "Build", 1, 0)
	cmd := &repcmd.BuildCmd{Base: base, Pos: repcore.Point{X: 1, Y: 2}}

	got := NewBuildCommandHandler().Handle(cmd, base)

	if got.UnitType != nil || got.UnitID != nil {
		t.Errorf("nil Unit should leave unit fields nil, got %v/%v", got.UnitType, got.UnitID)
	}
	if got.OrderName != nil || got.OrderID != nil {
		t.Errorf("nil Order should leave order fields nil, got %v/%v", got.OrderName, got.OrderID)
	}
	if got.X == nil || *got.X != 1 {
		t.Errorf("X should always be set, got %v", got.X)
	}
}

func TestLandHandlerParsesAllFields(t *testing.T) {
	base := baseCmd(repcmd.VirtualTypeIDLand, "Land", 2, 96)
	cmd := &repcmd.LandCmd{
		Base:  base,
		Pos:   repcore.Point{X: 7, Y: 8},
		Unit:  &repcmd.Unit{Enum: repcore.Enum{Name: "Command Center"}, ID: 106},
		Order: &repcmd.Order{Enum: repcore.Enum{Name: "BuildingLand"}, ID: 71},
	}

	got := NewLandCommandHandler().Handle(cmd, base)

	if got.ActionType != "Land" {
		t.Errorf("ActionType: want Land, got %q", got.ActionType)
	}
	if got.X == nil || *got.X != 7 || got.Y == nil || *got.Y != 8 {
		t.Errorf("Pos: want (7,8), got %v,%v", got.X, got.Y)
	}
	if got.UnitType == nil || *got.UnitType != "Command Center" {
		t.Errorf("UnitType: want Command Center, got %v", got.UnitType)
	}
	if got.OrderName == nil || *got.OrderName != "BuildingLand" {
		t.Errorf("OrderName: want BuildingLand, got %v", got.OrderName)
	}
}

func TestRightClickHandlerWithTargetUnit(t *testing.T) {
	base := baseCmd(repcmd.TypeIDRightClick, "Right Click", 1, 24)
	cmd := &repcmd.RightClickCmd{
		Base:    base,
		Pos:     repcore.Point{X: 500, Y: 600},
		UnitTag: repcmd.UnitTag(0x1234),
		Unit:    &repcmd.Unit{Enum: repcore.Enum{Name: "Dropship"}, ID: 11},
		Queued:  true,
	}

	got := NewRightClickCommandHandler("RightClick", repcmd.TypeIDRightClick).Handle(cmd, base)

	if got.X == nil || *got.X != 500 || got.Y == nil || *got.Y != 600 {
		t.Errorf("Pos: want (500,600), got %v,%v", got.X, got.Y)
	}
	if got.IsQueued == nil || !*got.IsQueued {
		t.Errorf("IsQueued: want true, got %v", got.IsQueued)
	}
	if got.TargetUnitType == nil || *got.TargetUnitType != "Dropship" {
		t.Errorf("TargetUnitType: want Dropship, got %v", got.TargetUnitType)
	}
	if got.TargetUnitTag == nil || *got.TargetUnitTag != uint16(0x1234) {
		t.Errorf("TargetUnitTag: want 0x1234, got %v", got.TargetUnitTag)
	}
}

func TestRightClickHandlerNoTargetUnit(t *testing.T) {
	base := baseCmd(repcmd.TypeIDRightClick, "Right Click", 1, 24)
	cmd := &repcmd.RightClickCmd{
		Base:   base,
		Pos:    repcore.Point{X: 10, Y: 20},
		Queued: false,
	}

	got := NewRightClickCommandHandler("RightClick", repcmd.TypeIDRightClick).Handle(cmd, base)

	if got.IsQueued == nil || *got.IsQueued {
		t.Errorf("IsQueued: want false, got %v", got.IsQueued)
	}
	if got.TargetUnitType != nil || got.TargetUnitTag != nil {
		t.Errorf("nil Unit should leave target fields nil, got %v/%v", got.TargetUnitType, got.TargetUnitTag)
	}
}

func TestTargetedOrderHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTargetedOrder, "Targeted Order", 4, 48)
	cmd := &repcmd.TargetedOrderCmd{
		Base:    base,
		Pos:     repcore.Point{X: 3, Y: 4},
		UnitTag: repcmd.UnitTag(77),
		Unit:    &repcmd.Unit{Enum: repcore.Enum{Name: "Overlord"}, ID: 42},
		Order:   &repcmd.Order{Enum: repcore.Enum{Name: "Move"}, ID: 6},
		Queued:  true,
	}

	got := NewTargetedOrderCommandHandler("TargetedOrder", repcmd.TypeIDTargetedOrder).Handle(cmd, base)

	if got.X == nil || *got.X != 3 || got.Y == nil || *got.Y != 4 {
		t.Errorf("Pos: want (3,4), got %v,%v", got.X, got.Y)
	}
	if got.OrderName == nil || *got.OrderName != "Move" {
		t.Errorf("OrderName: want Move, got %v", got.OrderName)
	}
	if got.OrderID == nil || *got.OrderID != 6 {
		t.Errorf("OrderID: want 6, got %v", got.OrderID)
	}
	if got.IsQueued == nil || !*got.IsQueued {
		t.Errorf("IsQueued: want true, got %v", got.IsQueued)
	}
	if got.TargetUnitType == nil || *got.TargetUnitType != "Overlord" {
		t.Errorf("TargetUnitType: want Overlord, got %v", got.TargetUnitType)
	}
	if got.TargetUnitTag == nil || *got.TargetUnitTag != uint16(77) {
		t.Errorf("TargetUnitTag: want 77, got %v", got.TargetUnitTag)
	}
}

func TestTrainHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTrain, "Train", 1, 0)
	cmd := &repcmd.TrainCmd{Base: base, Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Marine"}, ID: 0}}

	got := NewTrainCommandHandler().Handle(cmd, base)

	if got.UnitType == nil || *got.UnitType != "Marine" {
		t.Errorf("UnitType: want Marine, got %v", got.UnitType)
	}
	if got.UnitID == nil || *got.UnitID != 0 {
		t.Errorf("UnitID: want 0, got %v", got.UnitID)
	}
}

func TestUnitMorphHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDUnitMorph, "Unit Morph", 1, 0)
	cmd := &repcmd.TrainCmd{Base: base, Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Mutalisk"}, ID: 43}}

	got := NewUnitMorphCommandHandler().Handle(cmd, base)

	if got.ActionType != "Unit Morph" {
		t.Errorf("ActionType from base: want Unit Morph, got %q", got.ActionType)
	}
	if got.UnitType == nil || *got.UnitType != "Mutalisk" {
		t.Errorf("UnitType: want Mutalisk, got %v", got.UnitType)
	}
}

func TestBuildingMorphHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDBuildingMorph, "Building Morph", 1, 0)
	cmd := &repcmd.BuildingMorphCmd{Base: base, Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Lair"}, ID: 149}}

	got := NewBuildingMorphCommandHandler().Handle(cmd, base)

	if got.UnitType == nil || *got.UnitType != "Lair" {
		t.Errorf("UnitType: want Lair, got %v", got.UnitType)
	}
	if got.UnitID == nil || *got.UnitID != byte(149) {
		t.Errorf("UnitID: want 149, got %v", got.UnitID)
	}
}

func TestTrainFighterHandlerWithTrainCmd(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTrainFighter, "Train Fighter", 1, 0)
	cmd := &repcmd.TrainCmd{Base: base, Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Interceptor"}, ID: 73}}

	got := NewTrainFighterCommandHandler().Handle(cmd, base)

	if got.UnitType == nil || *got.UnitType != "Interceptor" {
		t.Errorf("UnitType: want Interceptor, got %v", got.UnitType)
	}
}

func TestTrainFighterHandlerWithNonTrainCmd(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTrainFighter, "Train Fighter", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base}

	got := NewTrainFighterCommandHandler().Handle(cmd, base)

	if got.UnitType != nil || got.UnitID != nil {
		t.Errorf("non-TrainCmd should leave unit fields nil, got %v/%v", got.UnitType, got.UnitID)
	}
}

func TestTechHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTech, "Tech", 1, 0)
	cmd := &repcmd.TechCmd{Base: base, Tech: &repcmd.Tech{Enum: repcore.Enum{Name: "Stim Packs"}}}

	got := NewTechCommandHandler().Handle(cmd, base)

	if got.TechName == nil || *got.TechName != "Stim Packs" {
		t.Errorf("TechName: want Stim Packs, got %v", got.TechName)
	}
}

func TestTechHandlerNilTech(t *testing.T) {
	base := baseCmd(repcmd.TypeIDTech, "Tech", 1, 0)
	cmd := &repcmd.TechCmd{Base: base}

	got := NewTechCommandHandler().Handle(cmd, base)

	if got.TechName != nil {
		t.Errorf("nil Tech should leave TechName nil, got %v", got.TechName)
	}
}

func TestUpgradeHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDUpgrade, "Upgrade", 1, 0)
	cmd := &repcmd.UpgradeCmd{Base: base, Upgrade: &repcmd.Upgrade{Enum: repcore.Enum{Name: "Terran Infantry Weapons"}}}

	got := NewUpgradeCommandHandler().Handle(cmd, base)

	if got.UpgradeName == nil || *got.UpgradeName != "Terran Infantry Weapons" {
		t.Errorf("UpgradeName: want Terran Infantry Weapons, got %v", got.UpgradeName)
	}
}

func TestHotkeyHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDHotkey, "Hotkey", 1, 0)
	cmd := &repcmd.HotkeyCmd{
		Base:       base,
		HotkeyType: &repcmd.HotkeyType{Enum: repcore.Enum{Name: "Assign"}, ID: 0},
		Group:      5,
	}

	got := NewHotkeyCommandHandler().Handle(cmd, base)

	if got.HotkeyType == nil || *got.HotkeyType != "Assign" {
		t.Errorf("HotkeyType: want Assign, got %v", got.HotkeyType)
	}
	if got.HotkeyGroup == nil || *got.HotkeyGroup != 5 {
		t.Errorf("HotkeyGroup: want 5, got %v", got.HotkeyGroup)
	}
}

func TestHotkeyHandlerNilTypeStillSetsGroup(t *testing.T) {
	base := baseCmd(repcmd.TypeIDHotkey, "Hotkey", 1, 0)
	cmd := &repcmd.HotkeyCmd{Base: base, Group: 9}

	got := NewHotkeyCommandHandler().Handle(cmd, base)

	if got.HotkeyType != nil {
		t.Errorf("nil HotkeyType should leave field nil, got %v", got.HotkeyType)
	}
	if got.HotkeyGroup == nil || *got.HotkeyGroup != 9 {
		t.Errorf("HotkeyGroup: want 9, got %v", got.HotkeyGroup)
	}
}

func TestChatHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDChat, "Chat", 1, 0)
	cmd := &repcmd.ChatCmd{Base: base, Message: "gl hf"}

	got := NewChatCommandHandler().Handle(cmd, base)

	if got.ChatMessage == nil || *got.ChatMessage != "gl hf" {
		t.Errorf("ChatMessage: want gl hf, got %v", got.ChatMessage)
	}
}

func TestChatHandlerEmptyMessage(t *testing.T) {
	base := baseCmd(repcmd.TypeIDChat, "Chat", 1, 0)
	cmd := &repcmd.ChatCmd{Base: base, Message: ""}

	got := NewChatCommandHandler().Handle(cmd, base)

	if got.ChatMessage != nil {
		t.Errorf("empty message should map to nil ChatMessage, got %v", *got.ChatMessage)
	}
}

func TestGameSpeedHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDGameSpeed, "Game Speed", 1, 0)
	cmd := &repcmd.GameSpeedCmd{Base: base, Speed: &repcore.Speed{Enum: repcore.Enum{Name: "Fastest"}, ID: 6}}

	got := NewGameSpeedCommandHandler().Handle(cmd, base)

	if got.GameSpeed == nil || *got.GameSpeed != "Fastest" {
		t.Errorf("GameSpeed: want Fastest, got %v", got.GameSpeed)
	}
}

func TestVisionHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDVision, "Vision", 1, 0)
	cmd := &repcmd.VisionCmd{Base: base, SlotIDs: repcmd.Bytes{2, 4, 7}}

	got := NewVisionCommandHandler().Handle(cmd, base)

	if got.VisionPlayerIDs == nil {
		t.Fatal("VisionPlayerIDs is nil")
	}
	want := []int64{2, 4, 7}
	ids := *got.VisionPlayerIDs
	if len(ids) != len(want) {
		t.Fatalf("VisionPlayerIDs len: want %d, got %d", len(want), len(ids))
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("VisionPlayerIDs[%d]: want %d, got %d", i, want[i], ids[i])
		}
	}
}

func TestAllianceHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDAlliance, "Alliance", 1, 0)
	cmd := &repcmd.AllianceCmd{Base: base, SlotIDs: repcmd.Bytes{1, 3}, AlliedVictory: true}

	got := NewAllianceCommandHandler().Handle(cmd, base)

	if got.AlliancePlayerIDs == nil {
		t.Fatal("AlliancePlayerIDs is nil")
	}
	if got.IsAlliedVictory == nil || !*got.IsAlliedVictory {
		t.Errorf("IsAlliedVictory: want true, got %v", got.IsAlliedVictory)
	}
	if len(*got.AlliancePlayerIDs) != 2 {
		t.Errorf("AlliancePlayerIDs len: want 2, got %d", len(*got.AlliancePlayerIDs))
	}
}

func TestLeaveGameHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDLeaveGame, "Leave Game", 1, 0)
	cmd := &repcmd.LeaveGameCmd{Base: base, Reason: &repcmd.LeaveReason{Enum: repcore.Enum{Name: "Quit"}, ID: 6}}

	got := NewLeaveGameCommandHandler().Handle(cmd, base)

	if got.LeaveReason == nil || *got.LeaveReason != "Quit" {
		t.Errorf("LeaveReason: want Quit, got %v", got.LeaveReason)
	}
}

func TestLeaveGameHandlerNilReason(t *testing.T) {
	base := baseCmd(repcmd.TypeIDLeaveGame, "Leave Game", 1, 0)
	cmd := &repcmd.LeaveGameCmd{Base: base}

	got := NewLeaveGameCommandHandler().Handle(cmd, base)

	if got.LeaveReason != nil {
		t.Errorf("nil Reason yields empty string, which stringPtr maps to nil; got %v", got.LeaveReason)
	}
}

func TestLiftOffHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDLiftOff, "Lift Off", 1, 0)
	cmd := &repcmd.LiftOffCmd{Base: base, Pos: repcore.Point{X: 55, Y: 66}}

	got := NewLiftOffCommandHandler().Handle(cmd, base)

	if got.X == nil || *got.X != 55 || got.Y == nil || *got.Y != 66 {
		t.Errorf("Pos: want (55,66), got %v,%v", got.X, got.Y)
	}
}

func TestMinimapPingHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDMinimapPing, "Minimap Ping", 1, 0)
	cmd := &repcmd.MinimapPingCmd{Base: base, Pos: repcore.Point{X: 1000, Y: 2000}}

	got := NewMinimapPingCommandHandler().Handle(cmd, base)

	if got.X == nil || *got.X != 1000 || got.Y == nil || *got.Y != 2000 {
		t.Errorf("Pos: want (1000,2000), got %v,%v", got.X, got.Y)
	}
}

func TestCancelTrainHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDCancelTrain, "Cancel Train", 2, 0)
	cmd := &repcmd.GeneralCmd{Base: base}

	got := NewCancelTrainCommandHandler().Handle(cmd, base)

	if got.ActionType != "Cancel Train" {
		t.Errorf("ActionType: want Cancel Train, got %q", got.ActionType)
	}
	if got.PlayerID != 2 {
		t.Errorf("PlayerID: want 2, got %d", got.PlayerID)
	}
}

func TestUnloadHandler(t *testing.T) {
	base := baseCmd(repcmd.TypeIDUnload, "Unload", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base}

	got := NewUnloadCommandHandler("Unload", repcmd.TypeIDUnload).Handle(cmd, base)

	if got.ActionType != "Unload" {
		t.Errorf("ActionType: want Unload, got %q", got.ActionType)
	}
}

func TestQueueableHandlerWithQueueableCmd(t *testing.T) {
	base := baseCmd(repcmd.TypeIDSiege, "Siege", 1, 0)
	cmd := &repcmd.QueueableCmd{Base: base, Queued: true}

	got := NewQueueableCommandHandler("Siege", repcmd.TypeIDSiege).Handle(cmd, base)

	if got.IsQueued == nil || !*got.IsQueued {
		t.Errorf("IsQueued: want true, got %v", got.IsQueued)
	}
}

func TestQueueableHandlerWithNonQueueableCmd(t *testing.T) {
	base := baseCmd(repcmd.TypeIDStop, "Stop", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base}

	got := NewQueueableCommandHandler("Stop", repcmd.TypeIDStop).Handle(cmd, base)

	if got.IsQueued != nil {
		t.Errorf("non-QueueableCmd should leave IsQueued nil, got %v", *got.IsQueued)
	}
}

func TestGeneralHandlerWithData(t *testing.T) {
	base := baseCmd(0xFF, "Unknown", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}

	got := NewGeneralCommandHandler("Unknown", 0xFF).Handle(cmd, base)

	if got.GeneralData == nil || *got.GeneralData != "deadbeef" {
		t.Errorf("GeneralData: want deadbeef, got %v", got.GeneralData)
	}
}

func TestGeneralHandlerEmptyData(t *testing.T) {
	base := baseCmd(0xFF, "Unknown", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base, Data: nil}

	got := NewGeneralCommandHandler("Unknown", 0xFF).Handle(cmd, base)

	if got.GeneralData != nil {
		t.Errorf("empty data should map to nil GeneralData, got %v", *got.GeneralData)
	}
}

func TestGeneralHandlerNonGeneralCmd(t *testing.T) {
	base := baseCmd(0xFF, "Unknown", 1, 0)
	cmd := &repcmd.QueueableCmd{Base: base}

	got := NewGeneralCommandHandler("Unknown", 0xFF).Handle(cmd, base)

	if got.GeneralData != nil {
		t.Errorf("non-GeneralCmd should leave GeneralData nil, got %v", *got.GeneralData)
	}
}

func TestBaseCommandHandlerAccessors(t *testing.T) {
	h := NewBuildCommandHandler()
	if h.GetActionType() != "Build" {
		t.Errorf("GetActionType: want Build, got %q", h.GetActionType())
	}
	if h.GetActionID() != repcmd.TypeIDBuild {
		t.Errorf("GetActionID: want %d, got %d", repcmd.TypeIDBuild, h.GetActionID())
	}
}
