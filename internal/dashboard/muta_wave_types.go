package dashboard

// Pro references are medians (p25..p75) over 100 progamer ladder TvZs from 2026 in which the first
// Mutalisks reached a Terran base (scripts/zz-artosis-probe/mutawave).
const (
	proMutaTravelSeconds   = 44
	proTurretsAtArrival    = 5
	proTurretsAtArrivalP25 = 3
	proTurretsAtArrivalP75 = 8
	proTurretLeadSeconds   = 36
	proTurretLeadP25       = 18
	proTurretLeadP75       = 78
)

type workflowMutaWave struct {
	FirstHatchSecond     int64              `json:"first_hatch_second"`
	ArrivalSecond        int64              `json:"arrival_second"`
	Arrived              bool               `json:"arrived"`
	MutasAtArrival       int                `json:"mutas_at_arrival"`
	TurretsUp            int                `json:"turrets_up"`
	TurretsBuilding      int                `json:"turrets_building"`
	TurretReadySeconds   []int64            `json:"turret_ready_seconds"`
	Wave                 workflowMutaLedger `json:"wave"`
	Game                 workflowMutaLedger `json:"game"`
	Kills                []workflowMutaKill `json:"kills"`
	ProTravelSeconds     int64              `json:"pro_travel_seconds"`
	ProTurretsAtArrival  [3]int             `json:"pro_turrets_at_arrival"`
	ProTurretLeadSeconds [3]int             `json:"pro_turret_lead_seconds"`
}

type workflowMutaLedger struct {
	Workers            int   `json:"workers"`
	Turrets            int   `json:"turrets"`
	OtherBuildings     int   `json:"other_buildings"`
	Units              int   `json:"units"`
	KilledValue        int32 `json:"killed_value"`
	MutasLost          int   `json:"mutas_lost"`
	MutasLostToTurrets int   `json:"mutas_lost_to_turrets"`
	LostValue          int32 `json:"lost_value"`
}

type workflowMutaKill struct {
	Second int64  `json:"second"`
	Unit   string `json:"unit"`
	Muta   bool   `json:"muta"`
}
