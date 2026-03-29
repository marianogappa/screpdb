package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

func TestQuickFactoryPlayerDetector(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithCommand(1, 230, models.ActionTypeBuild, models.GeneralUnitFactory)
	replay, players := builder.Build()

	detector := NewQuickFactoryPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, command := range builder.GetCommands() {
		detector.ProcessCommand(command)
	}

	if !detector.ShouldSave() {
		t.Fatalf("expected quick factory detection")
	}
}

func TestMechPlayerDetector_UsesThirtyMinuteWindowForComposition(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1)

	for i := 0; i < 7; i++ {
		builder.WithCommand(1, 600+i, models.ActionTypeTrain, models.GeneralUnitVulture)
	}
	for i := 0; i < 3; i++ {
		builder.WithCommand(1, 700+i, models.ActionTypeTrain, models.GeneralUnitMarine)
	}
	// These units should not affect mech composition ratio due to >30m cutoff.
	for i := 0; i < 10; i++ {
		builder.WithCommand(1, 1900+i, models.ActionTypeTrain, models.GeneralUnitWraith)
	}

	replay, players := builder.Build()
	detector := NewMechPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, command := range builder.GetCommands() {
		detector.ProcessCommand(command)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected mech detection with 70%% ratio inside cutoff window")
	}
}

func TestBattlecruisersAndCarriersDetectors(t *testing.T) {
	terranBuilder := NewTestReplayBuilder().WithPlayer(1, "T", "Terran", 1)
	for i := 0; i < 10; i++ {
		terranBuilder.WithCommand(1, 100+i, models.ActionTypeTrain, models.GeneralUnitBattlecruiser)
	}
	replayT, playersT := terranBuilder.Build()
	bcDetector := NewBattlecruisersPlayerDetector()
	bcDetector.SetReplayPlayerID(1)
	bcDetector.Initialize(replayT, playersT)
	for _, command := range terranBuilder.GetCommands() {
		bcDetector.ProcessCommand(command)
	}
	if !bcDetector.ShouldSave() {
		t.Fatalf("expected battlecruisers detection")
	}

	protossBuilder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	for i := 0; i < 10; i++ {
		protossBuilder.WithCommand(1, 100+i, models.ActionTypeTrain, models.GeneralUnitCarrier)
	}
	replayP, playersP := protossBuilder.Build()
	carrierDetector := NewCarriersPlayerDetector()
	carrierDetector.SetReplayPlayerID(1)
	carrierDetector.Initialize(replayP, playersP)
	for _, command := range protossBuilder.GetCommands() {
		carrierDetector.ProcessCommand(command)
	}
	if !carrierDetector.ShouldSave() {
		t.Fatalf("expected carriers detection")
	}
}

func TestNeverUpgradedAndNeverResearchedDetectors(t *testing.T) {
	neverBuilder := NewTestReplayBuilder().WithPlayer(1, "P1", "Protoss", 1)
	replay1, players1 := neverBuilder.Build()
	upgradedNever := NewNeverUpgradedPlayerDetector()
	upgradedNever.SetReplayPlayerID(1)
	upgradedNever.Initialize(replay1, players1)
	upgradedNever.Finalize()
	if !upgradedNever.ShouldSave() {
		t.Fatalf("expected never upgraded detection")
	}

	researchedNever := NewNeverResearchedPlayerDetector()
	researchedNever.SetReplayPlayerID(1)
	researchedNever.Initialize(replay1, players1)
	researchedNever.Finalize()
	if !researchedNever.ShouldSave() {
		t.Fatalf("expected never researched detection")
	}

	usedBuilder := NewTestReplayBuilder().
		WithPlayer(1, "P1", "Protoss", 1).
		WithCommand(1, 120, "Upgrade", models.GeneralUnitForge).
		WithCommand(1, 140, "Tech", models.GeneralUnitGateway)
	replay2, players2 := usedBuilder.Build()
	for _, command := range usedBuilder.GetCommands() {
		command.UpgradeName = stringPtr("Some Upgrade")
		command.TechName = stringPtr("Some Tech")
	}

	upgradedUsed := NewNeverUpgradedPlayerDetector()
	upgradedUsed.SetReplayPlayerID(1)
	upgradedUsed.Initialize(replay2, players2)
	for _, command := range usedBuilder.GetCommands() {
		upgradedUsed.ProcessCommand(command)
	}
	upgradedUsed.Finalize()
	if upgradedUsed.ShouldSave() {
		t.Fatalf("did not expect never upgraded when upgrade command exists")
	}

	researchedUsed := NewNeverResearchedPlayerDetector()
	researchedUsed.SetReplayPlayerID(1)
	researchedUsed.Initialize(replay2, players2)
	for _, command := range usedBuilder.GetCommands() {
		researchedUsed.ProcessCommand(command)
	}
	researchedUsed.Finalize()
	if researchedUsed.ShouldSave() {
		t.Fatalf("did not expect never researched when tech command exists")
	}
}

func TestOrderAndBeforeDetectors(t *testing.T) {
	protossBuilder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithCommand(1, 120, models.ActionTypeBuild, models.GeneralUnitGateway).
		WithCommand(1, 180, models.ActionTypeBuild, models.GeneralUnitForge)
	replayP, playersP := protossBuilder.Build()
	gateThenForge := NewGateThenForgePlayerDetector()
	gateThenForge.SetReplayPlayerID(1)
	gateThenForge.Initialize(replayP, playersP)
	for _, command := range protossBuilder.GetCommands() {
		gateThenForge.ProcessCommand(command)
	}
	gateThenForge.Finalize()
	if !gateThenForge.ShouldSave() {
		t.Fatalf("expected gate then forge detection")
	}

	expaBeforeGate := NewExpaBeforeGatePlayerDetector()
	expaBeforeGate.SetReplayPlayerID(1)
	expaBeforeGate.Initialize(replayP, playersP)
	for _, command := range protossBuilder.GetCommands() {
		expaBeforeGate.ProcessCommand(command)
	}
	expaBeforeGate.Finalize()
	if expaBeforeGate.ShouldSave() {
		t.Fatalf("did not expect expa before gate when no nexus build exists")
	}

	zergBuilder := NewTestReplayBuilder().
		WithPlayer(1, "Z", "Zerg", 1).
		WithCommand(1, 100, models.ActionTypeBuild, models.GeneralUnitHatchery)
	replayZ, playersZ := zergBuilder.Build()
	hatchBeforePool := NewHatchBeforePoolPlayerDetector()
	hatchBeforePool.SetReplayPlayerID(1)
	hatchBeforePool.Initialize(replayZ, playersZ)
	for _, command := range zergBuilder.GetCommands() {
		hatchBeforePool.ProcessCommand(command)
	}
	hatchBeforePool.Finalize()
	if !hatchBeforePool.ShouldSave() {
		t.Fatalf("expected hatch before pool when pool never appears")
	}
}

func TestBecameRaceDetectors_FromProtossBuildingForeignStructures(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithCommand(1, 210, models.ActionTypeBuild, models.GeneralUnitBarracks).
		WithCommand(1, 330, models.ActionTypeBuild, models.GeneralUnitHatchery)
	replay, players := builder.Build()

	ws := worldstate.NewEngine(replay, players, nil)
	for _, command := range builder.GetCommands() {
		ws.ProcessCommand(command)
	}

	becameTerran := NewBecameTerranPlayerDetector()
	becameTerran.SetReplayPlayerID(1)
	becameTerran.SetWorldState(ws)
	becameTerran.Initialize(replay, players)
	becameTerran.Finalize()
	if !becameTerran.ShouldSave() {
		t.Fatalf("expected became terran detection")
	}
	if result := becameTerran.GetResult(); result == nil || result.ValueInt == nil || *result.ValueInt != 210 {
		t.Fatalf("expected became terran at 210s, got %+v", result)
	}

	becameZerg := NewBecameZergPlayerDetector()
	becameZerg.SetReplayPlayerID(1)
	becameZerg.SetWorldState(ws)
	becameZerg.Initialize(replay, players)
	becameZerg.Finalize()
	if !becameZerg.ShouldSave() {
		t.Fatalf("expected became zerg detection")
	}
	if result := becameZerg.GetResult(); result == nil || result.ValueInt == nil || *result.ValueInt != 330 {
		t.Fatalf("expected became zerg at 330s, got %+v", result)
	}
}
