package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

const (
	secondsFourMinutes  = 4 * 60
	secondsFiveMinutes  = 5 * 60
	secondsSevenMinutes = 7 * 60
	secondsTenMinutes   = 10 * 60
	secondsThirtyMinute = 30 * 60
)

type QuickFactoryPlayerDetector struct {
	BasePlayerDetector
	firstFactorySecond *int
}

func NewQuickFactoryPlayerDetector() *QuickFactoryPlayerDetector {
	return &QuickFactoryPlayerDetector{}
}

func (d *QuickFactoryPlayerDetector) Name() string { return "Quick factory" }

func (d *QuickFactoryPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") {
		return false
	}
	if d.firstFactorySecond != nil {
		return false
	}
	if !isBuildOf(command, models.GeneralUnitFactory) {
		return false
	}
	sec := command.SecondsFromGameStart
	d.firstFactorySecond = &sec
	d.SetFinished(true)
	return true
}

func (d *QuickFactoryPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *QuickFactoryPlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		d.firstFactorySecond != nil &&
		*d.firstFactorySecond < secondsFourMinutes &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran")
}

type MechPlayerDetector struct {
	BasePlayerDetector
	barracksBuildCount int
	unitCounts30m      *PlayerUnitCounter
	marineCount        int
	medicCount         int
	firebatCount       int
}

func NewMechPlayerDetector() *MechPlayerDetector {
	return &MechPlayerDetector{}
}

func (d *MechPlayerDetector) Name() string { return "Mech" }

func (d *MechPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.BasePlayerDetector.Initialize(replay, players)
	d.unitCounts30m = NewPlayerUnitCounter(d.GetReplayPlayerID())
}

func (d *MechPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") {
		return false
	}
	if isBuildOf(command, models.GeneralUnitBarracks) {
		d.barracksBuildCount++
	}
	if isUnitProductionOf(command, models.GeneralUnitMarine) {
		d.marineCount++
	}
	if isUnitProductionOf(command, models.GeneralUnitMedic) {
		d.medicCount++
	}
	if isUnitProductionOf(command, models.GeneralUnitFirebat) {
		d.firebatCount++
	}
	maxSecond := secondsThirtyMinute
	d.unitCounts30m.ProcessCommand(command, &maxSecond)
	return false
}

func (d *MechPlayerDetector) Finalize() {
	d.SetFinished(true)
}

func (d *MechPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *MechPlayerDetector) ShouldSave() bool {
	if !d.IsFinished() || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") {
		return false
	}
	if d.unitCounts30m == nil {
		return false
	}
	if d.marineCount > 10 || d.medicCount > 0 || d.firebatCount > 0 || d.barracksBuildCount > 1 {
		return false
	}
	totalNonSCV := d.unitCounts30m.TotalExcluding(models.GeneralUnitSCV)
	if totalNonSCV == 0 {
		return false
	}
	mechCount := d.unitCounts30m.Count(models.GeneralUnitVulture) +
		d.unitCounts30m.Count(models.GeneralUnitGoliath) +
		d.unitCounts30m.Count(models.GeneralUnitSiegeTankTankMode) +
		d.unitCounts30m.Count(models.GeneralUnitTerranSiegeTankSiegeMode)
	return (mechCount*100)/totalNonSCV >= 70
}

type BattlecruisersPlayerDetector struct {
	BasePlayerDetector
	count int
}

func NewBattlecruisersPlayerDetector() *BattlecruisersPlayerDetector {
	return &BattlecruisersPlayerDetector{}
}

func (d *BattlecruisersPlayerDetector) Name() string { return "Battlecruisers" }

func (d *BattlecruisersPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") {
		return false
	}
	if !isUnitProductionOf(command, models.GeneralUnitBattlecruiser) {
		return false
	}
	d.count++
	d.SetFinished(true)
	return true
}

func (d *BattlecruisersPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *BattlecruisersPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.count >= 1 && isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran")
}

type CarriersPlayerDetector struct {
	BasePlayerDetector
	count int
}

func NewCarriersPlayerDetector() *CarriersPlayerDetector {
	return &CarriersPlayerDetector{}
}

func (d *CarriersPlayerDetector) Name() string { return "Carriers" }

func (d *CarriersPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") {
		return false
	}
	if !isUnitProductionOf(command, models.GeneralUnitCarrier) {
		return false
	}
	d.count++
	d.SetFinished(true)
	return true
}

func (d *CarriersPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *CarriersPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.count >= 1 && isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss")
}

type playerFirstEventByTypeDetector struct {
	BasePlayerDetector
	name      string
	eventType string
	second    *int
}

func (d *playerFirstEventByTypeDetector) ProcessCommand(command *models.Command) bool {
	_ = command
	return false
}

func (d *playerFirstEventByTypeDetector) Finalize() {
	d.SetFinished(true)
	ws := d.GetWorldState()
	if ws == nil {
		return
	}
	d.second = ws.FirstEventSecondForPlayer(d.GetReplayPlayerID(), d.eventType)
}

func (d *playerFirstEventByTypeDetector) Name() string { return d.name }

func (d *playerFirstEventByTypeDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	return d.BuildPlayerResult(d.Name(), nil, d.second, nil, nil)
}

func (d *playerFirstEventByTypeDetector) ShouldSave() bool {
	return d.IsFinished() && d.second != nil
}

func NewMadeDropsPlayerDetector() *playerFirstEventByTypeDetector {
	return &playerFirstEventByTypeDetector{name: "Made drops", eventType: "drop"}
}

func NewMadeRecallsPlayerDetector() *playerFirstEventByTypeDetector {
	return &playerFirstEventByTypeDetector{name: "Made recalls", eventType: "recall"}
}

func NewThrewNukesPlayerDetector() *playerFirstEventByTypeDetector {
	return &playerFirstEventByTypeDetector{name: "Threw Nukes", eventType: "nuke"}
}

func NewBecameTerranPlayerDetector() *playerFirstEventByTypeDetector {
	return &playerFirstEventByTypeDetector{name: "Became Terran", eventType: "became_terran"}
}

func NewBecameZergPlayerDetector() *playerFirstEventByTypeDetector {
	return &playerFirstEventByTypeDetector{name: "Became Zerg", eventType: "became_zerg"}
}

type FastExpaPlayerDetector struct {
	BasePlayerDetector
	second *int
	where  *string
}

func NewFastExpaPlayerDetector() *FastExpaPlayerDetector {
	return &FastExpaPlayerDetector{}
}

func (d *FastExpaPlayerDetector) Name() string { return "Fast expa" }

func (d *FastExpaPlayerDetector) ProcessCommand(command *models.Command) bool {
	_ = command
	return false
}

func (d *FastExpaPlayerDetector) Finalize() {
	d.SetFinished(true)
	ws := d.GetWorldState()
	if ws == nil {
		return
	}
	sec, where := ws.FirstExpansionForPlayer(d.GetReplayPlayerID())
	if sec == nil || where == nil {
		return
	}
	if *sec < secondsFiveMinutes {
		d.second = sec
		d.where = where
	}
}

func (d *FastExpaPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	return d.BuildPlayerResult(d.Name(), nil, d.second, d.where, nil)
}

func (d *FastExpaPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.second != nil && d.where != nil
}

type GateThenForgePlayerDetector struct {
	BasePlayerDetector
	firstGate  *int
	firstForge *int
}

func NewGateThenForgePlayerDetector() *GateThenForgePlayerDetector {
	return &GateThenForgePlayerDetector{}
}

func (d *GateThenForgePlayerDetector) Name() string { return "Gate then Forge" }

func (d *GateThenForgePlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") {
		return false
	}
	if d.firstGate == nil && isBuildOf(command, models.GeneralUnitGateway) {
		sec := command.SecondsFromGameStart
		d.firstGate = &sec
	}
	if d.firstForge == nil && isBuildOf(command, models.GeneralUnitForge) {
		sec := command.SecondsFromGameStart
		d.firstForge = &sec
	}
	return false
}

func (d *GateThenForgePlayerDetector) Finalize() { d.SetFinished(true) }

func (d *GateThenForgePlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *GateThenForgePlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") &&
		d.firstGate != nil &&
		d.firstForge != nil &&
		*d.firstGate < secondsFiveMinutes &&
		*d.firstForge < secondsFiveMinutes &&
		*d.firstGate < *d.firstForge
}

type ForgeThenGatePlayerDetector struct {
	BasePlayerDetector
	firstGate  *int
	firstForge *int
}

func NewForgeThenGatePlayerDetector() *ForgeThenGatePlayerDetector {
	return &ForgeThenGatePlayerDetector{}
}

func (d *ForgeThenGatePlayerDetector) Name() string { return "Forge then Gate" }

func (d *ForgeThenGatePlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") {
		return false
	}
	if d.firstGate == nil && isBuildOf(command, models.GeneralUnitGateway) {
		sec := command.SecondsFromGameStart
		d.firstGate = &sec
	}
	if d.firstForge == nil && isBuildOf(command, models.GeneralUnitForge) {
		sec := command.SecondsFromGameStart
		d.firstForge = &sec
	}
	return false
}

func (d *ForgeThenGatePlayerDetector) Finalize() { d.SetFinished(true) }

func (d *ForgeThenGatePlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *ForgeThenGatePlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") &&
		d.firstGate != nil &&
		d.firstForge != nil &&
		*d.firstGate < secondsFiveMinutes &&
		*d.firstForge < secondsFiveMinutes &&
		*d.firstForge < *d.firstGate
}

type NeverUpgradedPlayerDetector struct {
	BasePlayerDetector
	didUpgrade bool
}

func NewNeverUpgradedPlayerDetector() *NeverUpgradedPlayerDetector {
	return &NeverUpgradedPlayerDetector{}
}

func (d *NeverUpgradedPlayerDetector) Name() string { return "Never upgraded" }

func (d *NeverUpgradedPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}
	if command.UpgradeName != nil || command.ActionType == "Upgrade" {
		d.didUpgrade = true
	}
	return false
}

func (d *NeverUpgradedPlayerDetector) Finalize() { d.SetFinished(true) }

func (d *NeverUpgradedPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *NeverUpgradedPlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		d.HasReplayDurationAtLeast(secondsTenMinutes) &&
		!d.didUpgrade
}

type NeverResearchedPlayerDetector struct {
	BasePlayerDetector
	didResearch bool
}

func NewNeverResearchedPlayerDetector() *NeverResearchedPlayerDetector {
	return &NeverResearchedPlayerDetector{}
}

func (d *NeverResearchedPlayerDetector) Name() string { return "Never researched" }

func (d *NeverResearchedPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}
	if command.TechName != nil || command.ActionType == "Tech" {
		d.didResearch = true
	}
	return false
}

func (d *NeverResearchedPlayerDetector) Finalize() { d.SetFinished(true) }

func (d *NeverResearchedPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *NeverResearchedPlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		d.HasReplayDurationAtLeast(secondsTenMinutes) &&
		!d.didResearch
}

type HatchBeforePoolPlayerDetector struct {
	BasePlayerDetector
	firstHatch *int
	firstPool  *int
}

func NewHatchBeforePoolPlayerDetector() *HatchBeforePoolPlayerDetector {
	return &HatchBeforePoolPlayerDetector{}
}

func (d *HatchBeforePoolPlayerDetector) Name() string { return "Hatch before Pool" }

func (d *HatchBeforePoolPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Zerg") {
		return false
	}
	if d.firstHatch == nil && isBuildOf(command, models.GeneralUnitHatchery) {
		sec := command.SecondsFromGameStart
		d.firstHatch = &sec
	}
	if d.firstPool == nil && isBuildOf(command, models.GeneralUnitSpawningPool) {
		sec := command.SecondsFromGameStart
		d.firstPool = &sec
	}
	return false
}

func (d *HatchBeforePoolPlayerDetector) Finalize() { d.SetFinished(true) }

func (d *HatchBeforePoolPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *HatchBeforePoolPlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Zerg") &&
		d.firstHatch != nil &&
		(d.firstPool == nil || *d.firstHatch < *d.firstPool)
}

type ExpaBeforeGatePlayerDetector struct {
	BasePlayerDetector
	firstNexus *int
	firstGate  *int
}

func NewExpaBeforeGatePlayerDetector() *ExpaBeforeGatePlayerDetector {
	return &ExpaBeforeGatePlayerDetector{}
}

func (d *ExpaBeforeGatePlayerDetector) Name() string { return "Expa before Gate" }

func (d *ExpaBeforeGatePlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") {
		return false
	}
	if d.firstNexus == nil && isBuildOf(command, models.GeneralUnitNexus) {
		sec := command.SecondsFromGameStart
		d.firstNexus = &sec
	}
	if d.firstGate == nil && isBuildOf(command, models.GeneralUnitGateway) {
		sec := command.SecondsFromGameStart
		d.firstGate = &sec
	}
	return false
}

func (d *ExpaBeforeGatePlayerDetector) Finalize() { d.SetFinished(true) }

func (d *ExpaBeforeGatePlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *ExpaBeforeGatePlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Protoss") &&
		d.firstNexus != nil &&
		(d.firstGate == nil || *d.firstNexus < *d.firstGate)
}

type ExpaBeforeBarracksPlayerDetector struct {
	BasePlayerDetector
	firstCommandCenter *int
	firstBarracks      *int
}

func NewExpaBeforeBarracksPlayerDetector() *ExpaBeforeBarracksPlayerDetector {
	return &ExpaBeforeBarracksPlayerDetector{}
}

func (d *ExpaBeforeBarracksPlayerDetector) Name() string { return "Expa before Barracks" }

func (d *ExpaBeforeBarracksPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) || !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") {
		return false
	}
	if d.firstCommandCenter == nil && isBuildOf(command, models.GeneralUnitCommandCenter) {
		sec := command.SecondsFromGameStart
		d.firstCommandCenter = &sec
	}
	if d.firstBarracks == nil && isBuildOf(command, models.GeneralUnitBarracks) {
		sec := command.SecondsFromGameStart
		d.firstBarracks = &sec
	}
	return false
}

func (d *ExpaBeforeBarracksPlayerDetector) Finalize() { d.SetFinished(true) }

func (d *ExpaBeforeBarracksPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	valueBool := true
	return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
}

func (d *ExpaBeforeBarracksPlayerDetector) ShouldSave() bool {
	return d.IsFinished() &&
		isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), "Terran") &&
		d.firstCommandCenter != nil &&
		(d.firstBarracks == nil || *d.firstCommandCenter < *d.firstBarracks)
}
