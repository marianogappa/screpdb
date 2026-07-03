package cmdenrich

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func bytep(b byte) *byte    { return &b }
func boolp(b bool) *bool    { return &b }
func u16p(v uint16) *uint16 { return &v }

func TestClassify_NilAndUnrecognized(t *testing.T) {
	if _, ok := Classify(nil); ok {
		t.Fatalf("nil command should not classify")
	}
	// Sync / Chat / anything with no recognizable ActionType or OrderName.
	if _, ok := Classify(&models.Command{ActionType: "Sync"}); ok {
		t.Fatalf("Sync command should not classify")
	}
	if _, ok := Classify(&models.Command{ActionType: "Chat", ChatMessage: strp("gg")}); ok {
		t.Fatalf("Chat command should not classify")
	}
}

func TestClassify_KindFromActionType(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		wantKind   Kind
		wantAggr   Aggression
	}{
		{"build", models.ActionTypeBuild, KindMakeBuilding, NonAggressive},
		{"train", models.ActionTypeTrain, KindMakeUnit, NonAggressive},
		{"unit morph", models.ActionTypeUnitMorph, KindMakeUnit, NonAggressive},
		{"tech", "Tech", KindTech, NonAggressive},
		{"upgrade", "Upgrade", KindUpgrade, NonAggressive},
		{"attack move", "Attack Move", KindAttackMove, Aggressive},
		{"attack", "Attack", KindAttackUnit, Aggressive},
		{"move", "Move", KindMove, Ambiguous},
		{"hold", "Hold Position", KindHold, NonAggressive},
		{"patrol", "Patrol", KindPatrol, Ambiguous},
		{"stop", "Stop", KindStop, NonAggressive},
		{"right click", "Right Click", KindRightClick, Ambiguous},
		{"unload all", "Unload All", KindUnloadAll, Aggressive},
		{"burrow", "Burrow", KindBurrow, NonAggressive},
		{"unburrow", "Unburrow", KindUnburrow, NonAggressive},
		{"siege", "Siege", KindSiege, Ambiguous},
		{"unsiege", "Unsiege", KindUnsiege, NonAggressive},
		{"load", "Load", KindLoad, NonAggressive},
		{"load bunker", "LoadBunker", KindLoadBunker, NonAggressive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fact, ok := Classify(&models.Command{ActionType: tt.actionType, PlayerID: 3})
			if !ok {
				t.Fatalf("expected %q to classify", tt.actionType)
			}
			if fact.Kind != tt.wantKind {
				t.Fatalf("kind = %d, want %d", fact.Kind, tt.wantKind)
			}
			if fact.Aggression != tt.wantAggr {
				t.Fatalf("aggression = %d, want %d", fact.Aggression, tt.wantAggr)
			}
			if fact.PlayerID != 3 {
				t.Fatalf("playerID = %d, want 3", fact.PlayerID)
			}
			if fact.Count != 1 {
				t.Fatalf("count = %d, want default 1", fact.Count)
			}
		})
	}
}

func TestClassify_HotkeyGroupAsSubject(t *testing.T) {
	fact, ok := Classify(&models.Command{ActionType: "Hotkey", HotkeyGroup: bytep(7)})
	if !ok {
		t.Fatalf("expected hotkey to classify")
	}
	if fact.Kind != KindHotkey {
		t.Fatalf("kind = %d, want KindHotkey", fact.Kind)
	}
	if fact.Subject != "7" {
		t.Fatalf("subject = %q, want \"7\"", fact.Subject)
	}
	// Missing HotkeyGroup must drop the command (can't surface a group number).
	if _, ok := Classify(&models.Command{ActionType: "Hotkey"}); ok {
		t.Fatalf("hotkey without group should not classify")
	}
}

func TestClassify_OrderNameFlattening(t *testing.T) {
	tests := []struct {
		name      string
		orderName string
		wantKind  Kind
		wantAggr  Aggression
	}{
		{"canonical attack move", models.UnitOrderAttackMove, KindAttackMove, Aggressive},
		{"canonical move", models.UnitOrderMove, KindMove, Ambiguous},
		{"canonical patrol", models.UnitOrderPatrol, KindPatrol, Ambiguous},
		{"canonical hold", models.UnitOrderHoldPosition, KindHold, NonAggressive},
		{"canonical stop", models.UnitOrderStop, KindStop, NonAggressive},
		{"attack unit", models.UnitOrderAttackUnit, KindAttackUnit, Aggressive},
		{"attack1", models.UnitOrderAttack1, KindAttackUnit, Aggressive},
		{"attack tile", models.UnitOrderAttackTile, KindAttackUnit, Aggressive},
		{"attack fixed range", models.UnitOrderAttackFixedRange, KindAttackUnit, Aggressive},
		// Space-stripped / lowercase forms used by fixtures resolve identically.
		{"spaced attack move", "Attack Move", KindAttackMove, Aggressive},
		{"lowercase move", "move", KindMove, Ambiguous},
		{"hold alias", "hold", KindHold, NonAggressive},
		{"bare attack alias", "attack", KindAttackUnit, Aggressive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &models.Command{ActionType: "Targeted Order", OrderName: strp(tt.orderName)}
			fact, ok := Classify(cmd)
			if !ok {
				t.Fatalf("expected %q to classify", tt.orderName)
			}
			if fact.Kind != tt.wantKind {
				t.Fatalf("kind = %d, want %d", fact.Kind, tt.wantKind)
			}
			if fact.Aggression != tt.wantAggr {
				t.Fatalf("aggression = %d, want %d", fact.Aggression, tt.wantAggr)
			}
			if fact.OrderName != tt.orderName {
				t.Fatalf("orderName = %q, want %q", fact.OrderName, tt.orderName)
			}
		})
	}
}

func TestClassify_RightClickFallback(t *testing.T) {
	// A Right Click with no recognized OrderName stays a contextual right-click.
	fact, ok := Classify(&models.Command{ActionType: "Right Click", OrderName: strp("SomethingElse")})
	if !ok {
		t.Fatalf("expected right click to classify")
	}
	if fact.Kind != KindRightClick {
		t.Fatalf("kind = %d, want KindRightClick", fact.Kind)
	}
	if fact.Aggression != Ambiguous {
		t.Fatalf("aggression = %d, want Ambiguous", fact.Aggression)
	}
}

func TestClassify_CastAndNuke(t *testing.T) {
	tests := []struct {
		name        string
		orderName   string
		wantSubject string
	}{
		{"psi storm strips Cast prefix", "CastPsionicStorm", "PsionicStorm"},
		{"irradiate strips Cast prefix", "CastIrradiate", "Irradiate"},
		{"recall strips Cast prefix", "CastRecall", "Recall"},
		{"nuke launch passes through", "NukeLaunch", "NukeLaunch"},
		{"nuclear strike passes through", "NuclearStrike", "NuclearStrike"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &models.Command{ActionType: "Targeted Order", OrderName: strp(tt.orderName)}
			fact, ok := Classify(cmd)
			if !ok {
				t.Fatalf("expected %q to classify", tt.orderName)
			}
			if fact.Kind != KindCast {
				t.Fatalf("kind = %d, want KindCast", fact.Kind)
			}
			if fact.Aggression != Aggressive {
				t.Fatalf("aggression = %d, want Aggressive", fact.Aggression)
			}
			if fact.Subject != tt.wantSubject {
				t.Fatalf("subject = %q, want %q", fact.Subject, tt.wantSubject)
			}
		})
	}
}

func TestClassify_UnloadVariantFromOrderName(t *testing.T) {
	// A TargetedOrder whose OrderName contains "unload" classifies as UnloadAll.
	fact, ok := Classify(&models.Command{ActionType: "Targeted Order", OrderName: strp("MoveUnload")})
	if !ok {
		t.Fatalf("expected MoveUnload to classify")
	}
	if fact.Kind != KindUnloadAll {
		t.Fatalf("kind = %d, want KindUnloadAll", fact.Kind)
	}
}

func TestClassify_TechUpgradeLoadSubjects(t *testing.T) {
	tests := []struct {
		name        string
		cmd         *models.Command
		wantKind    Kind
		wantSubject string
	}{
		{
			"tech subject from TechName",
			&models.Command{ActionType: "Tech", TechName: strp("Tank Siege Mode")},
			KindTech, "Tank Siege Mode",
		},
		{
			"upgrade subject from UpgradeName",
			&models.Command{ActionType: "Upgrade", UpgradeName: strp("Singularity Charge (Dragoon Range)")},
			KindUpgrade, "Singularity Charge (Dragoon Range)",
		},
		{
			"load subject from TargetUnitType",
			&models.Command{ActionType: "Load", TargetUnitType: strp("Dropship")},
			KindLoad, "Dropship",
		},
		{
			"load bunker subject from TargetUnitType",
			&models.Command{ActionType: "LoadBunker", TargetUnitType: strp("Bunker")},
			KindLoadBunker, "Bunker",
		},
		{
			"lay mine subject from OrderName",
			&models.Command{ActionType: "Targeted Order", OrderName: strp(models.UnitOrderPlaceMine)},
			KindLayMine, models.UnitOrderPlaceMine,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fact, ok := Classify(tt.cmd)
			if !ok {
				t.Fatalf("expected command to classify")
			}
			if fact.Kind != tt.wantKind {
				t.Fatalf("kind = %d, want %d", fact.Kind, tt.wantKind)
			}
			if fact.Subject != tt.wantSubject {
				t.Fatalf("subject = %q, want %q", fact.Subject, tt.wantSubject)
			}
		})
	}
}

func TestClassify_BuildCoordsConvertedToPixels(t *testing.T) {
	// Build carries TILE coords; Classify normalizes to pixels (×32+16).
	fact, ok := Classify(&models.Command{
		ActionType: models.ActionTypeBuild,
		UnitType:   strp(models.GeneralUnitPylon),
		X:          intp(10),
		Y:          intp(20),
	})
	if !ok {
		t.Fatalf("expected build to classify")
	}
	if fact.X == nil || *fact.X != 10*32+16 {
		t.Fatalf("X = %v, want %d", fact.X, 10*32+16)
	}
	if fact.Y == nil || *fact.Y != 20*32+16 {
		t.Fatalf("Y = %v, want %d", fact.Y, 20*32+16)
	}
	if fact.Subject != models.GeneralUnitPylon {
		t.Fatalf("subject = %q, want %q", fact.Subject, models.GeneralUnitPylon)
	}
}

func TestClassify_NonBuildCoordsUnchanged(t *testing.T) {
	// Move coords are already pixels; they pass through untouched.
	fact, ok := Classify(&models.Command{
		ActionType: "Move",
		X:          intp(500),
		Y:          intp(600),
	})
	if !ok {
		t.Fatalf("expected move to classify")
	}
	if fact.X == nil || *fact.X != 500 || fact.Y == nil || *fact.Y != 600 {
		t.Fatalf("coords = (%v,%v), want (500,600)", fact.X, fact.Y)
	}
}

func TestClassify_PlayerPointerOverridesPlayerID(t *testing.T) {
	// When Player is set, its PlayerID wins over the flat PlayerID field.
	fact, ok := Classify(&models.Command{
		ActionType: "Move",
		PlayerID:   0,
		Player:     &models.Player{PlayerID: 5},
	})
	if !ok {
		t.Fatalf("expected move to classify")
	}
	if fact.PlayerID != 5 {
		t.Fatalf("playerID = %d, want 5 (from Player pointer)", fact.PlayerID)
	}
}

func TestClassify_QueuedAndCount(t *testing.T) {
	fact, ok := Classify(&models.Command{
		ActionType:     models.ActionTypeUnitMorph,
		UnitType:       strp(models.GeneralUnitZergling),
		IsQueued:       boolp(true),
		MorphUnitCount: 3,
	})
	if !ok {
		t.Fatalf("expected morph to classify")
	}
	if !fact.Queued {
		t.Fatalf("Queued = false, want true")
	}
	if fact.Count != 3 {
		t.Fatalf("Count = %d, want 3", fact.Count)
	}
	// MorphUnitCount of 0 (not computed) defaults to 1.
	fact2, _ := Classify(&models.Command{ActionType: models.ActionTypeTrain, MorphUnitCount: 0})
	if fact2.Count != 1 {
		t.Fatalf("Count = %d, want 1 for uncomputed morph", fact2.Count)
	}
}

func TestClassify_TargetUnitTagPreserved(t *testing.T) {
	tag := u16p(42)
	fact, ok := Classify(&models.Command{
		ActionType:     "Load",
		TargetUnitTag:  tag,
		TargetUnitType: strp("Shuttle"),
	})
	if !ok {
		t.Fatalf("expected load to classify")
	}
	if fact.TargetUnitTag == nil || *fact.TargetUnitTag != 42 {
		t.Fatalf("TargetUnitTag = %v, want 42", fact.TargetUnitTag)
	}
}

func TestFromAction(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		wantKind   Kind
		wantAggr   Aggression
		wantOK     bool
	}{
		{"build", models.ActionTypeBuild, KindMakeBuilding, NonAggressive, true},
		{"train", models.ActionTypeTrain, KindMakeUnit, NonAggressive, true},
		{"attack", "Attack", KindAttackUnit, Aggressive, true},
		{"trimmed whitespace", "  Move  ", KindMove, Ambiguous, true},
		{"unrecognized", "Sync", KindUnknown, AggressionUnknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fact, ok := FromAction(tt.actionType, "  Pylon  ", 120, 2)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if fact.Kind != tt.wantKind {
				t.Fatalf("kind = %d, want %d", fact.Kind, tt.wantKind)
			}
			if fact.Aggression != tt.wantAggr {
				t.Fatalf("aggression = %d, want %d", fact.Aggression, tt.wantAggr)
			}
			if fact.Subject != "Pylon" {
				t.Fatalf("subject = %q, want trimmed \"Pylon\"", fact.Subject)
			}
			if fact.Second != 120 || fact.PlayerID != 2 {
				t.Fatalf("second/playerID = %d/%d, want 120/2", fact.Second, fact.PlayerID)
			}
		})
	}
}

func TestEconOf(t *testing.T) {
	pylon, ok := EconOf(models.GeneralUnitPylon)
	if !ok {
		t.Fatalf("expected Pylon in econ table")
	}
	if pylon.Minerals != 100 || pylon.SupplyDelta != 8 {
		t.Fatalf("Pylon econ = %+v, want Minerals 100 SupplyDelta 8", pylon)
	}
	if pylon.SupplyCost != 0 {
		t.Fatalf("Pylon SupplyCost = %d, want 0 (building)", pylon.SupplyCost)
	}
	zealot, _ := EconOf(models.GeneralUnitZealot)
	if zealot.SupplyCost != 2 {
		t.Fatalf("Zealot SupplyCost = %d, want 2", zealot.SupplyCost)
	}
	if _, ok := EconOf("NotAThing"); ok {
		t.Fatalf("unknown subject should return ok=false")
	}
}

func TestGatherRatePerMinute(t *testing.T) {
	if r := GatherRatePerMinute(models.GeneralUnitProbe); r != 68.1 {
		t.Fatalf("Probe rate = %v, want 68.1", r)
	}
	if r := GatherRatePerMinute(models.GeneralUnitSCV); r != 65.0 {
		t.Fatalf("SCV rate = %v, want 65.0", r)
	}
	if r := GatherRatePerMinute(models.GeneralUnitZealot); r != 0 {
		t.Fatalf("non-worker rate = %v, want 0", r)
	}
}

func TestProducerOf(t *testing.T) {
	if p, ok := ProducerOf(models.GeneralUnitZealot); !ok || p != models.GeneralUnitGateway {
		t.Fatalf("Zealot producer = %q (ok=%v), want Gateway", p, ok)
	}
	if p, ok := ProducerOf(models.GeneralUnitZergling); !ok || p != models.GeneralUnitSpawningPool {
		t.Fatalf("Zergling producer = %q (ok=%v), want Spawning Pool", p, ok)
	}
	if _, ok := ProducerOf("NotAUnit"); ok {
		t.Fatalf("unknown unit should return ok=false")
	}
}

func TestPrereqsOf(t *testing.T) {
	prereqs, ok := PrereqsOf(models.GeneralUnitPhotonCannon)
	if !ok {
		t.Fatalf("expected Photon Cannon prereqs")
	}
	if len(prereqs) != 2 {
		t.Fatalf("Photon Cannon prereqs = %v, want 2 entries", prereqs)
	}
	// Order is preserved: Pylon then Forge.
	if prereqs[0] != models.GeneralUnitPylon || prereqs[1] != models.GeneralUnitForge {
		t.Fatalf("Photon Cannon prereqs = %v, want [Pylon, Forge]", prereqs)
	}
	if _, ok := PrereqsOf(models.GeneralUnitPylon); ok {
		t.Fatalf("Pylon has no prereqs, want ok=false")
	}
}

func TestIsWorkerAndSupplyStructure(t *testing.T) {
	for _, w := range []string{models.GeneralUnitSCV, models.GeneralUnitProbe, models.GeneralUnitDrone} {
		if !IsWorker(w) {
			t.Fatalf("%q should be a worker", w)
		}
	}
	if IsWorker(models.GeneralUnitZealot) {
		t.Fatalf("Zealot should not be a worker")
	}
	for _, s := range []string{models.GeneralUnitPylon, models.GeneralUnitSupplyDepot, models.GeneralUnitOverlord} {
		if !IsSupplyStructure(s) {
			t.Fatalf("%q should be a supply structure", s)
		}
	}
	if IsSupplyStructure(models.GeneralUnitGateway) {
		t.Fatalf("Gateway should not be a supply structure")
	}
}

func TestAllEconSortedAndConsistent(t *testing.T) {
	entries := AllEcon()
	if len(entries) == 0 {
		t.Fatalf("AllEcon returned no entries")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Subject >= entries[i].Subject {
			t.Fatalf("AllEcon not sorted: %q >= %q", entries[i-1].Subject, entries[i].Subject)
		}
	}
	// Each entry round-trips through EconOf.
	for _, e := range entries {
		got, ok := EconOf(e.Subject)
		if !ok || got != e.Econ {
			t.Fatalf("EconOf(%q) = %+v (ok=%v), want %+v", e.Subject, got, ok, e.Econ)
		}
	}
}

func TestAllProducersAndPrereqsSorted(t *testing.T) {
	producers := AllProducers()
	for i := 1; i < len(producers); i++ {
		if producers[i-1].Unit >= producers[i].Unit {
			t.Fatalf("AllProducers not sorted at %d", i)
		}
	}
	prereqs := AllPrereqs()
	for i := 1; i < len(prereqs); i++ {
		if prereqs[i-1].Building >= prereqs[i].Building {
			t.Fatalf("AllPrereqs not sorted at %d", i)
		}
	}
	if len(AllGatherRates()) != 3 {
		t.Fatalf("expected 3 gather-rate entries, got %d", len(AllGatherRates()))
	}
}
