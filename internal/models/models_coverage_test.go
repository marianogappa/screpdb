package models

import (
	"sort"
	"testing"
)

func strptr(s string) *string { return &s }

func TestBuildTimeOf(t *testing.T) {
	cases := []struct {
		name string
		want float64
	}{
		{GeneralUnitSpawningPool, 50},
		{GeneralUnitHatchery, 75},
		{GeneralUnitDrone, 12.6},
		{GeneralUnitZealot, 25.2},
		{GeneralUnitCommandCenter, 75},
		{GeneralUnitMarine, 15},
	}
	for _, c := range cases {
		got, ok := BuildTimeOf(c.name)
		if !ok {
			t.Errorf("BuildTimeOf(%q): want ok, got not found", c.name)
			continue
		}
		if got != c.want {
			t.Errorf("BuildTimeOf(%q) = %v, want %v", c.name, got, c.want)
		}
	}

	if got, ok := BuildTimeOf("Not A Unit"); ok || got != 0 {
		t.Errorf("BuildTimeOf(unknown) = (%v, %v), want (0, false)", got, ok)
	}
}

func TestAllBuildTimes(t *testing.T) {
	all := AllBuildTimes()
	if len(all) == 0 {
		t.Fatal("AllBuildTimes returned empty")
	}
	for _, e := range all {
		if e.Name == "" {
			t.Errorf("AllBuildTimes: empty name in entry %+v", e)
		}
		if e.Seconds <= 0 {
			t.Errorf("AllBuildTimes: non-positive duration for %q: %v", e.Name, e.Seconds)
		}
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Error("AllBuildTimes not sorted by Name")
	}
}

func TestIsFlyingUnit(t *testing.T) {
	fliers := []string{
		GeneralUnitWraith, GeneralUnitMutalisk, GeneralUnitOverlord,
		GeneralUnitCarrier, GeneralUnitObserver, GeneralUnitScout,
	}
	for _, u := range fliers {
		if !IsFlyingUnit(u) {
			t.Errorf("IsFlyingUnit(%q) = false, want true", u)
		}
	}
	ground := []string{GeneralUnitMarine, GeneralUnitZealot, GeneralUnitDrone, "Nonexistent"}
	for _, u := range ground {
		if IsFlyingUnit(u) {
			t.Errorf("IsFlyingUnit(%q) = true, want false", u)
		}
	}
}

func TestIsWorker(t *testing.T) {
	for _, u := range []string{GeneralUnitSCV, GeneralUnitProbe, GeneralUnitDrone} {
		if !IsWorker(u) {
			t.Errorf("IsWorker(%q) = false, want true", u)
		}
	}
	for _, u := range []string{GeneralUnitMarine, GeneralUnitOverlord, "Whatever"} {
		if IsWorker(u) {
			t.Errorf("IsWorker(%q) = true, want false", u)
		}
	}
}

func TestCommandIsUnitBuildAndName(t *testing.T) {
	marine := UnitNameSCV
	train := &Command{ActionType: ActionTypeTrain, UnitType: &marine}
	if !train.IsUnitBuild() {
		t.Error("Train command: IsUnitBuild = false, want true")
	}
	if got := train.UnitBuildName(); got != UnitNameSCV {
		t.Errorf("UnitBuildName = %q, want %q", got, UnitNameSCV)
	}

	morph := &Command{ActionType: ActionTypeUnitMorph, UnitType: strptr("Mutalisk")}
	if !morph.IsUnitBuild() {
		t.Error("Unit Morph command: IsUnitBuild = false, want true")
	}
	if got := morph.UnitBuildName(); got != "Mutalisk" {
		t.Errorf("UnitBuildName = %q, want Mutalisk", got)
	}

	build := &Command{ActionType: ActionTypeBuild, UnitType: strptr("Barracks")}
	if build.IsUnitBuild() {
		t.Error("Build command: IsUnitBuild = true, want false")
	}
	if got := build.UnitBuildName(); got != "" {
		t.Errorf("UnitBuildName on non-unit-build = %q, want empty", got)
	}

	nilType := &Command{ActionType: ActionTypeTrain, UnitType: nil}
	if got := nilType.UnitBuildName(); got != "" {
		t.Errorf("UnitBuildName with nil UnitType = %q, want empty", got)
	}
}

func TestCommandIsAttackingUnitBuild(t *testing.T) {
	attacker := &Command{ActionType: ActionTypeTrain, UnitType: strptr("Marine")}
	if !attacker.IsAttackingUnitBuild() {
		t.Error("Marine train: IsAttackingUnitBuild = false, want true")
	}

	for _, name := range []string{UnitNameSCV, UnitNameProbe, UnitNameDrone, UnitNameOverlord} {
		c := &Command{ActionType: ActionTypeTrain, UnitType: strptr(name)}
		if c.IsAttackingUnitBuild() {
			t.Errorf("%s train: IsAttackingUnitBuild = true, want false", name)
		}
	}

	notBuild := &Command{ActionType: ActionTypeBuild, UnitType: strptr("Marine")}
	if notBuild.IsAttackingUnitBuild() {
		t.Error("Build action: IsAttackingUnitBuild = true, want false")
	}
}

func TestCommandIsUpgradeAndName(t *testing.T) {
	up := &Command{UpgradeName: strptr(UpgradeSingularityChargeDragoonRange)}
	if !up.IsUpgrade() {
		t.Error("IsUpgrade = false, want true")
	}
	if got := up.GetUpgradeName(); got != UpgradeSingularityChargeDragoonRange {
		t.Errorf("GetUpgradeName = %q, want %q", got, UpgradeSingularityChargeDragoonRange)
	}

	notUp := &Command{ActionType: ActionTypeTrain, UpgradeName: nil}
	if notUp.IsUpgrade() {
		t.Error("nil UpgradeName: IsUpgrade = true, want false")
	}
	if got := notUp.GetUpgradeName(); got != "" {
		t.Errorf("GetUpgradeName with nil = %q, want empty", got)
	}
}

func TestCommandIsBaseBuild(t *testing.T) {
	for _, name := range []string{UnitNameHatchery, UnitNameNexus, UnitNameCommandCenter} {
		c := &Command{ActionType: ActionTypeBuild, UnitType: strptr(name)}
		if !c.IsBaseBuild() {
			t.Errorf("Build %s: IsBaseBuild = false, want true", name)
		}
	}

	notBase := &Command{ActionType: ActionTypeBuild, UnitType: strptr("Barracks")}
	if notBase.IsBaseBuild() {
		t.Error("Build Barracks: IsBaseBuild = true, want false")
	}

	trainCC := &Command{ActionType: ActionTypeTrain, UnitType: strptr(UnitNameCommandCenter)}
	if trainCC.IsBaseBuild() {
		t.Error("Train Command Center: IsBaseBuild = true, want false")
	}

	nilType := &Command{ActionType: ActionTypeBuild, UnitType: nil}
	if nilType.IsBaseBuild() {
		t.Error("Build with nil UnitType: IsBaseBuild = true, want false")
	}
}

func TestPlayerIsHumanAndNonObserver(t *testing.T) {
	human := &Player{Type: PlayerTypeHuman, IsObserver: false}
	if !human.IsHuman() {
		t.Error("Human player: IsHuman = false, want true")
	}
	if !human.IsNonObserverHuman() {
		t.Error("Human non-observer: IsNonObserverHuman = false, want true")
	}

	obs := &Player{Type: PlayerTypeHuman, IsObserver: true}
	if !obs.IsHuman() {
		t.Error("Human observer: IsHuman = false, want true")
	}
	if obs.IsNonObserverHuman() {
		t.Error("Human observer: IsNonObserverHuman = true, want false")
	}

	comp := &Player{Type: "Computer"}
	if comp.IsHuman() {
		t.Error("Computer: IsHuman = true, want false")
	}
	if comp.IsNonObserverHuman() {
		t.Error("Computer: IsNonObserverHuman = true, want false")
	}
}

func TestAllTechMeta(t *testing.T) {
	all := AllTechMeta()
	if len(all) == 0 {
		t.Fatal("AllTechMeta returned empty")
	}
	for _, e := range all {
		if e.Name == "" {
			t.Errorf("AllTechMeta: empty name in %+v", e)
		}
		if e.Meta.Race == "" {
			t.Errorf("AllTechMeta: empty race for %q", e.Name)
		}
		if e.Meta.BuildingSubject == "" {
			t.Errorf("AllTechMeta: empty building subject for %q", e.Name)
		}
		if e.Meta.DurationS <= 0 {
			t.Errorf("AllTechMeta: non-positive duration for %q: %v", e.Name, e.Meta.DurationS)
		}
		if e.Meta.Minerals < 0 || e.Meta.Gas < 0 {
			t.Errorf("AllTechMeta: negative cost for %q", e.Name)
		}
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Error("AllTechMeta not sorted by Name")
	}
}

func TestAllUpgradeMeta(t *testing.T) {
	all := AllUpgradeMeta()
	if len(all) == 0 {
		t.Fatal("AllUpgradeMeta returned empty")
	}
	for _, e := range all {
		if e.Name == "" {
			t.Errorf("AllUpgradeMeta: empty name in %+v", e)
		}
		if e.Meta.MaxLevel != 1 && e.Meta.MaxLevel != 3 {
			t.Errorf("AllUpgradeMeta: %q MaxLevel = %d, want 1 or 3", e.Name, e.Meta.MaxLevel)
		}
		for lvl := 0; lvl < e.Meta.MaxLevel; lvl++ {
			if e.Meta.Levels[lvl].DurationS <= 0 {
				t.Errorf("AllUpgradeMeta: %q level %d non-positive duration %v", e.Name, lvl, e.Meta.Levels[lvl].DurationS)
			}
		}
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Error("AllUpgradeMeta not sorted by Name")
	}
}

func TestWorkerUnitNames(t *testing.T) {
	names := WorkerUnitNames()
	want := []string{GeneralUnitDrone, GeneralUnitProbe, GeneralUnitSCV}
	sort.Strings(want)
	if len(names) != len(want) {
		t.Fatalf("WorkerUnitNames len = %d, want %d (%v)", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("WorkerUnitNames[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestFlyingUnitNames(t *testing.T) {
	names := FlyingUnitNames()
	if len(names) == 0 {
		t.Fatal("FlyingUnitNames returned empty")
	}
	if !sort.StringsAreSorted(names) {
		t.Error("FlyingUnitNames not sorted")
	}
	seen := map[string]bool{}
	for _, n := range names {
		if n == "" {
			t.Error("FlyingUnitNames: empty name")
		}
		seen[n] = true
	}
	if !seen[GeneralUnitMutalisk] || !seen[GeneralUnitOverlord] {
		t.Error("FlyingUnitNames missing an expected flyer (Mutalisk/Overlord)")
	}
}

func TestLookupTech(t *testing.T) {
	m, ok := LookupTech(TechStimPacks)
	if !ok {
		t.Fatal("LookupTech(Stim Packs): want ok")
	}
	if m.Race != RaceTerran {
		t.Errorf("Stim Packs race = %q, want %q", m.Race, RaceTerran)
	}
	if m.BuildingSubject != GeneralUnitAcademy {
		t.Errorf("Stim Packs building = %q, want %q", m.BuildingSubject, GeneralUnitAcademy)
	}
	if m.Minerals != 100 || m.Gas != 100 || m.DurationS != 50.4 {
		t.Errorf("Stim Packs meta = %+v, want minerals/gas/dur 100/100/50.4", m)
	}

	l, ok := LookupTech(TechLurkerAspect)
	if !ok {
		t.Fatal("LookupTech(Lurker Aspect): want ok")
	}
	if l.Race != RaceZerg || l.BuildingSubject != GeneralUnitHydraliskDen {
		t.Errorf("Lurker Aspect meta = %+v", l)
	}

	if _, ok := LookupTech("Feedback"); ok {
		t.Error("LookupTech(Feedback): want not found (default ability)")
	}
	if _, ok := LookupTech("Made-Up Tech"); ok {
		t.Error("LookupTech(unknown): want not found")
	}
}

func TestLookupUpgrade(t *testing.T) {
	m, ok := LookupUpgrade(UpgradeVentralSacsOverlordTransport)
	if !ok {
		t.Fatal("LookupUpgrade(Ventral Sacs): want ok")
	}
	if m.Race != RaceZerg || m.MaxLevel != 1 {
		t.Errorf("Ventral Sacs meta = %+v, want Zerg one-shot", m)
	}
	if m.Levels[0].DurationS != 100.8 {
		t.Errorf("Ventral Sacs L1 duration = %v, want 100.8", m.Levels[0].DurationS)
	}

	tiered, ok := LookupUpgrade(UpgradeProtossGroundWeapons)
	if !ok {
		t.Fatal("LookupUpgrade(Protoss Ground Weapons): want ok")
	}
	if tiered.MaxLevel != 3 {
		t.Errorf("Protoss Ground Weapons MaxLevel = %d, want 3", tiered.MaxLevel)
	}
	if tiered.Levels[0].DurationS >= tiered.Levels[1].DurationS ||
		tiered.Levels[1].DurationS >= tiered.Levels[2].DurationS {
		t.Errorf("tiered durations not strictly increasing: %+v", tiered.Levels)
	}

	if _, ok := LookupUpgrade("Made-Up Upgrade"); ok {
		t.Error("LookupUpgrade(unknown): want not found")
	}
}

func TestIsHPUpgrade(t *testing.T) {
	tiered := []string{
		UpgradeTerranInfantryArmor, UpgradeZergCarapace,
		UpgradeProtossGroundWeapons, UpgradeProtossPlasmaShields,
	}
	for _, u := range tiered {
		if !IsHPUpgrade(u) {
			t.Errorf("IsHPUpgrade(%q) = false, want true (tiered)", u)
		}
	}
	oneShots := []string{
		UpgradeMetabolicBoostZerglingSpeed, UpgradeSingularityChargeDragoonRange,
		UpgradeU238ShellsMarineRange,
	}
	for _, u := range oneShots {
		if IsHPUpgrade(u) {
			t.Errorf("IsHPUpgrade(%q) = true, want false (one-shot)", u)
		}
	}
	if IsHPUpgrade("Made-Up Upgrade") {
		t.Error("IsHPUpgrade(unknown) = true, want false")
	}
}

func TestAllUnitGeometry(t *testing.T) {
	all := AllUnitGeometry()
	if len(all) != len(unitGeometry) {
		t.Fatalf("AllUnitGeometry len = %d, want %d", len(all), len(unitGeometry))
	}
	for _, u := range all {
		if u.Name == "" {
			t.Errorf("AllUnitGeometry: empty name in %+v", u)
		}
		if u.WidthPixels <= 0 || u.HeightPixels <= 0 {
			t.Errorf("AllUnitGeometry: non-positive box for %q: %dx%d", u.Name, u.WidthPixels, u.HeightPixels)
		}
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Error("AllUnitGeometry not sorted by Name")
	}
}

func TestAllBuildingGeometry(t *testing.T) {
	all := AllBuildingGeometry()
	if len(all) != len(buildingGeometry) {
		t.Fatalf("AllBuildingGeometry len = %d, want %d", len(all), len(buildingGeometry))
	}
	for _, b := range all {
		if b.Name == "" {
			t.Errorf("AllBuildingGeometry: empty name in %+v", b)
		}
		if b.BoxWidthPixels <= 0 || b.BoxHeightPixels <= 0 {
			t.Errorf("AllBuildingGeometry: non-positive box for %q: %dx%d", b.Name, b.BoxWidthPixels, b.BoxHeightPixels)
		}
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		t.Error("AllBuildingGeometry not sorted by Name")
	}
}
