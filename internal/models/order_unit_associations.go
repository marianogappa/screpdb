package models

const (
	RaceTerran  = "Terran"
	RaceZerg    = "Zerg"
	RaceProtoss = "Protoss"
)

// UnitOrigin pairs a unit name with its race. Used to attribute an
// order/action back to the unit that issued it so EnrichedCommands can be
// rolled up by unit composition (e.g. for analyzing attacks).
type UnitOrigin struct {
	Unit string
	Race string
}

// UnitOrderToUnit maps OrderName values to the unit that issues them.
//
// Only orders that are unambiguously tied to a single unit type are listed;
// generic orders (Move, Attack1, HoldPosition, …) belong to many units and
// are intentionally absent.
var UnitOrderToUnit = map[string]UnitOrigin{
	UnitOrderReaverStop: {Unit: GeneralUnitReaver, Race: RaceProtoss},

	UnitOrderVultureMine: {Unit: GeneralUnitVulture, Race: RaceTerran},
	UnitOrderPlaceMine:   {Unit: GeneralUnitVulture, Race: RaceTerran},

	UnitOrderDroneStartBuild: {Unit: GeneralUnitDrone, Race: RaceZerg},
	UnitOrderDroneBuild:      {Unit: GeneralUnitDrone, Race: RaceZerg},
	UnitOrderDroneLand:       {Unit: GeneralUnitDrone, Race: RaceZerg},
	UnitOrderDroneLiftOff:    {Unit: GeneralUnitDrone, Race: RaceZerg},

	UnitOrderCastInfestation:        {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderMoveToInfest:           {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderInfestingCommandCenter: {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderQueenHoldPosition:      {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderCastParasite:           {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderCastSpawnBroodlings:    {Unit: GeneralUnitQueen, Race: RaceZerg},
	UnitOrderCastEnsnare:            {Unit: GeneralUnitQueen, Race: RaceZerg},

	UnitOrderPlaceProtossBuilding:  {Unit: GeneralUnitProbe, Race: RaceProtoss},
	UnitOrderCreateProtossBuilding: {Unit: GeneralUnitProbe, Race: RaceProtoss},

	UnitOrderRepair:       {Unit: GeneralUnitSCV, Race: RaceTerran},
	UnitOrderMoveToRepair: {Unit: GeneralUnitSCV, Race: RaceTerran},

	UnitOrderCarrier:             {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierStop:         {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierAttack:       {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierMoveToAttack: {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierIgnore2:      {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierFight:        {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderCarrierHoldPosition: {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderInterceptorAttack:   {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	UnitOrderInterceptorReturn:   {Unit: GeneralUnitCarrier, Race: RaceProtoss},

	UnitOrderReaver:             {Unit: GeneralUnitReaver, Race: RaceProtoss},
	UnitOrderReaverAttack:       {Unit: GeneralUnitReaver, Race: RaceProtoss},
	UnitOrderReaverMoveToAttack: {Unit: GeneralUnitReaver, Race: RaceProtoss},
	UnitOrderReaverFight:        {Unit: GeneralUnitReaver, Race: RaceProtoss},
	UnitOrderReaverHoldPosition: {Unit: GeneralUnitReaver, Race: RaceProtoss},
	UnitOrderScarabAttack:       {Unit: GeneralUnitReaver, Race: RaceProtoss},

	UnitOrderSieging:   {Unit: GeneralUnitSiegeTankTankMode, Race: RaceTerran},
	UnitOrderUnsieging: {Unit: GeneralUnitTerranSiegeTankSiegeMode, Race: RaceTerran},

	UnitOrderGuardianAspect: {Unit: GeneralUnitMutalisk, Race: RaceZerg},

	UnitOrderArchonWarp:        {Unit: GeneralUnitHighTemplar, Race: RaceProtoss},
	UnitOrderCastPsionicStorm:  {Unit: GeneralUnitHighTemplar, Race: RaceProtoss},
	UnitOrderCastHallucination: {Unit: GeneralUnitHighTemplar, Race: RaceProtoss},
	UnitOrderHallucination2:    {Unit: GeneralUnitHighTemplar, Race: RaceProtoss},

	UnitOrderFireYamatoGun:       {Unit: GeneralUnitBattlecruiser, Race: RaceTerran},
	UnitOrderMoveToFireYamatoGun: {Unit: GeneralUnitBattlecruiser, Race: RaceTerran},

	UnitOrderCastLockdown:      {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukeWait:          {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukeTrain:         {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukeLaunch:        {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukePaint:         {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukeUnit:          {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderCastNuclearStrike: {Unit: GeneralUnitGhost, Race: RaceTerran},
	UnitOrderNukeTrack:         {Unit: GeneralUnitGhost, Race: RaceTerran},

	UnitOrderCastDarkSwarm: {Unit: GeneralUnitDefiler, Race: RaceZerg},
	UnitOrderCastPlague:    {Unit: GeneralUnitDefiler, Race: RaceZerg},
	UnitOrderCastConsume:   {Unit: GeneralUnitDefiler, Race: RaceZerg},

	UnitOrderCastEMPShockwave:    {Unit: GeneralUnitScienceVessel, Race: RaceTerran},
	UnitOrderCastDefensiveMatrix: {Unit: GeneralUnitScienceVessel, Race: RaceTerran},
	UnitOrderCastIrradiate:       {Unit: GeneralUnitScienceVessel, Race: RaceTerran},
	UnitOrderCastStasisField:     {Unit: GeneralUnitScienceVessel, Race: RaceTerran},

	UnitOrderInitializeArbiter: {Unit: GeneralUnitArbiter, Race: RaceProtoss},
	UnitOrderCloakNearbyUnits:  {Unit: GeneralUnitArbiter, Race: RaceProtoss},
	UnitOrderCastRecall:        {Unit: GeneralUnitArbiter, Race: RaceProtoss},

	UnitOrderMedic:             {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderMedicHeal:         {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderHealMove:          {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderMedicHoldPosition: {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderMedicHealToIdle:   {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderCastRestoration:   {Unit: GeneralUnitMedic, Race: RaceTerran},
	UnitOrderCastOpticalFlare:  {Unit: GeneralUnitMedic, Race: RaceTerran},

	UnitOrderCastDisruptionWeb: {Unit: GeneralUnitCorsair, Race: RaceProtoss},

	UnitOrderCastMindControl: {Unit: GeneralUnitDarkArchon, Race: RaceProtoss},
	UnitOrderDarkArchonMeld:  {Unit: GeneralUnitDarkArchon, Race: RaceProtoss},
	UnitOrderCastFeedback:    {Unit: GeneralUnitDarkArchon, Race: RaceProtoss},
	UnitOrderCastMaelstrom:   {Unit: GeneralUnitDarkArchon, Race: RaceProtoss},
}

// ActionTypeToUnit maps Command.ActionType values to the unit that issues
// them. ActionType lives in a different string namespace from OrderName, so
// it gets its own map even when the strings happen to overlap.
var ActionTypeToUnit = map[string]UnitOrigin{
	"CarrierStop": {Unit: GeneralUnitCarrier, Race: RaceProtoss},
	"ReaverStop":  {Unit: GeneralUnitReaver, Race: RaceProtoss},
	"Siege":       {Unit: GeneralUnitSiegeTankTankMode, Race: RaceTerran},
	"Unsiege":     {Unit: GeneralUnitTerranSiegeTankSiegeMode, Race: RaceTerran},
}
