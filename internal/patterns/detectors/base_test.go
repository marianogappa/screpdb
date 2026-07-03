package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func cmd(playerID byte, actionType, unitType string, seconds int) *models.Command {
	return &models.Command{
		SecondsFromGameStart: seconds,
		ActionType:           actionType,
		UnitType:             stringPtr(unitType),
		Player:               &models.Player{PlayerID: playerID},
	}
}

func TestMatchers(t *testing.T) {
	build := cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 10)
	train := cmd(1, models.ActionTypeTrain, "Zealot", 20)

	tests := []struct {
		name    string
		matcher CommandMatcher
		command *models.Command
		want    bool
	}{
		{"ActionType hit", MatchActionType(models.ActionTypeBuild), build, true},
		{"ActionType miss", MatchActionType(models.ActionTypeTrain), build, false},
		{"UnitType hit", MatchUnitType(models.GeneralUnitGateway), build, true},
		{"UnitType miss", MatchUnitType("Zealot"), build, false},
		{"ActionAndUnit hit", MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitGateway), build, true},
		{"ActionAndUnit action miss", MatchActionAndUnit(models.ActionTypeTrain, models.GeneralUnitGateway), build, false},
		{"ActionAndUnit unit miss", MatchActionAndUnit(models.ActionTypeBuild, "Zealot"), build, false},
		{"Any one hits", MatchAny(MatchUnitType("Zealot"), MatchActionType(models.ActionTypeBuild)), build, true},
		{"Any none hit", MatchAny(MatchUnitType("Zealot"), MatchActionType(models.ActionTypeTrain)), build, false},
		{"All hit", MatchAll(MatchActionType(models.ActionTypeTrain), MatchUnitType("Zealot")), train, true},
		{"All one miss", MatchAll(MatchActionType(models.ActionTypeTrain), MatchUnitType("Dragoon")), train, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.matcher(tc.command); got != tc.want {
				t.Fatalf("matcher = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchUnitType_NilUnitType(t *testing.T) {
	c := &models.Command{ActionType: models.ActionTypeBuild}
	if MatchUnitType(models.GeneralUnitGateway)(c) {
		t.Fatalf("expected no match when UnitType is nil")
	}
	if MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitGateway)(c) {
		t.Fatalf("expected no match when UnitType is nil")
	}
}

func TestFirstOccurrenceDetector(t *testing.T) {
	f := &FirstOccurrenceDetector{}
	matcher := MatchActionType(models.ActionTypeBuild)

	if f.IsMatched() {
		t.Fatalf("expected not matched initially")
	}
	if f.GetSeconds() != nil {
		t.Fatalf("expected nil seconds initially")
	}
	if f.ProcessFirstOccurrence(cmd(1, models.ActionTypeTrain, "Zealot", 5), matcher) {
		t.Fatalf("non-matching command should not register first occurrence")
	}
	if !f.ProcessFirstOccurrence(cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 42), matcher) {
		t.Fatalf("expected first matching command to register")
	}
	if !f.IsMatched() {
		t.Fatalf("expected matched after first occurrence")
	}
	if f.GetSeconds() == nil || *f.GetSeconds() != 42 {
		t.Fatalf("expected recorded second 42, got %v", f.GetSeconds())
	}
	// Subsequent matches must not overwrite the first.
	if f.ProcessFirstOccurrence(cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 99), matcher) {
		t.Fatalf("expected false on subsequent match")
	}
	if *f.GetSeconds() != 42 {
		t.Fatalf("expected first second (42) preserved, got %d", *f.GetSeconds())
	}
}

func TestCountDetector(t *testing.T) {
	c := NewCountDetector()
	matcher := MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitGateway)

	// Command with nil Player is ignored.
	if c.ProcessCount(&models.Command{ActionType: models.ActionTypeBuild, UnitType: stringPtr(models.GeneralUnitGateway)}, matcher, 1) {
		t.Fatalf("nil-player command must not count")
	}
	// Non-matching command does not count.
	if c.ProcessCount(cmd(1, models.ActionTypeTrain, "Zealot", 5), matcher, 2) {
		t.Fatalf("non-matching command must not count")
	}
	if c.ProcessCount(cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 10), matcher, 2) {
		t.Fatalf("first match below threshold must not trip")
	}
	if !c.ProcessCount(cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 20), matcher, 2) {
		t.Fatalf("second match must reach threshold 2")
	}
	if got := c.GetCounts()[1]; got != 2 {
		t.Fatalf("expected count 2 for player 1, got %d", got)
	}
	if !c.HasAnyCountAbove(2) {
		t.Fatalf("expected HasAnyCountAbove(2) true")
	}
	if c.HasAnyCountAbove(3) {
		t.Fatalf("expected HasAnyCountAbove(3) false")
	}
}

func TestCountDetector_PerPlayer(t *testing.T) {
	c := NewCountDetector()
	matcher := MatchActionType(models.ActionTypeBuild)
	c.ProcessCount(cmd(1, models.ActionTypeBuild, "X", 1), matcher, 99)
	c.ProcessCount(cmd(2, models.ActionTypeBuild, "X", 2), matcher, 99)
	c.ProcessCount(cmd(2, models.ActionTypeBuild, "X", 3), matcher, 99)

	counts := c.GetCounts()
	if counts[1] != 1 || counts[2] != 2 {
		t.Fatalf("expected {1:1, 2:2}, got %v", counts)
	}
	if !c.HasAnyCountAbove(2) {
		t.Fatalf("player 2 has 2, expected HasAnyCountAbove(2) true")
	}
}

func TestGetOpponentInOneVOne(t *testing.T) {
	own := &models.Player{PlayerID: 1, Team: 1, Race: "Protoss"}
	opp := &models.Player{PlayerID: 2, Team: 2, Race: "Zerg"}
	obs := &models.Player{PlayerID: 3, Team: 3, Race: "Terran", IsObserver: true}

	t.Run("single opponent", func(t *testing.T) {
		got := getOpponentInOneVOne([]*models.Player{own, opp, obs}, own)
		if got != opp {
			t.Fatalf("expected opp, got %v", got)
		}
	})
	t.Run("observers ignored, none left", func(t *testing.T) {
		got := getOpponentInOneVOne([]*models.Player{own, obs}, own)
		if got != nil {
			t.Fatalf("expected nil (only observer besides own), got %v", got)
		}
	})
	t.Run("same team ignored", func(t *testing.T) {
		ally := &models.Player{PlayerID: 4, Team: 1, Race: "Protoss"}
		got := getOpponentInOneVOne([]*models.Player{own, ally}, own)
		if got != nil {
			t.Fatalf("expected nil (only same-team player), got %v", got)
		}
	})
	t.Run("more than one opponent returns nil", func(t *testing.T) {
		opp2 := &models.Player{PlayerID: 5, Team: 2, Race: "Terran"}
		got := getOpponentInOneVOne([]*models.Player{own, opp, opp2}, own)
		if got != nil {
			t.Fatalf("expected nil when two opponents present, got %v", got)
		}
	})
	t.Run("nil entry skipped", func(t *testing.T) {
		got := getOpponentInOneVOne([]*models.Player{own, nil, opp}, own)
		if got != opp {
			t.Fatalf("expected opp with a nil entry in the slice, got %v", got)
		}
	})
}

func TestGetPlayerByReplayPlayerID(t *testing.T) {
	p1 := &models.Player{PlayerID: 1}
	p2 := &models.Player{PlayerID: 2}
	players := []*models.Player{nil, p1, p2}
	if got := getPlayerByReplayPlayerID(players, 2); got != p2 {
		t.Fatalf("expected p2, got %v", got)
	}
	if got := getPlayerByReplayPlayerID(players, 9); got != nil {
		t.Fatalf("expected nil for missing id, got %v", got)
	}
}

func TestIsPlayerRace(t *testing.T) {
	players := []*models.Player{{PlayerID: 1, Race: "Protoss"}}
	if !isPlayerRace(players, 1, "protoss") {
		t.Fatalf("expected case-insensitive race match")
	}
	if isPlayerRace(players, 1, "Zerg") {
		t.Fatalf("expected no match for wrong race")
	}
	if isPlayerRace(players, 2, "Protoss") {
		t.Fatalf("expected no match for missing player")
	}
}

func TestIsBuildOfAndIsUnitProductionOf(t *testing.T) {
	build := cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 1)
	morph := cmd(1, models.ActionTypeUnitMorph, models.GeneralUnitZergling, 1)
	train := cmd(1, models.ActionTypeTrain, "Zealot", 1)
	nilUnit := &models.Command{ActionType: models.ActionTypeBuild}

	if !isBuildOf(build, models.GeneralUnitGateway) {
		t.Fatalf("expected isBuildOf true for matching build")
	}
	if isBuildOf(build, "Pylon") {
		t.Fatalf("expected isBuildOf false for wrong unit")
	}
	if isBuildOf(train, "Zealot") {
		t.Fatalf("expected isBuildOf false for Train action")
	}
	if isBuildOf(nilUnit, models.GeneralUnitGateway) {
		t.Fatalf("expected isBuildOf false when UnitType nil")
	}
	if !isUnitProductionOf(morph, models.GeneralUnitZergling) {
		t.Fatalf("expected isUnitProductionOf true for morph")
	}
	if !isUnitProductionOf(train, "Zealot") {
		t.Fatalf("expected isUnitProductionOf true for train")
	}
	if isUnitProductionOf(build, models.GeneralUnitGateway) {
		t.Fatalf("expected isUnitProductionOf false for Build action")
	}
}

func TestPlayerUnitCounter(t *testing.T) {
	c := NewPlayerUnitCounter(1)
	c.ProcessCommand(cmd(1, models.ActionTypeTrain, "Zealot", 10), nil)
	c.ProcessCommand(cmd(1, models.ActionTypeTrain, "Zealot", 20), nil)
	c.ProcessCommand(cmd(1, models.ActionTypeTrain, "Dragoon", 30), nil)
	// Other player ignored.
	c.ProcessCommand(cmd(2, models.ActionTypeTrain, "Zealot", 40), nil)
	// Build (not unit production) ignored.
	c.ProcessCommand(cmd(1, models.ActionTypeBuild, models.GeneralUnitGateway, 50), nil)
	// nil command / nil player ignored.
	c.ProcessCommand(nil, nil)
	c.ProcessCommand(&models.Command{ActionType: models.ActionTypeTrain, UnitType: stringPtr("Zealot")}, nil)

	if got := c.Count("Zealot"); got != 2 {
		t.Fatalf("expected 2 Zealots, got %d", got)
	}
	if got := c.Count("Dragoon"); got != 1 {
		t.Fatalf("expected 1 Dragoon, got %d", got)
	}
	if got := c.TotalExcluding("Zealot"); got != 1 {
		t.Fatalf("expected total excluding Zealot = 1 (Dragoon), got %d", got)
	}
	if got := c.TotalExcluding(); got != 3 {
		t.Fatalf("expected total = 3, got %d", got)
	}
}

func TestPlayerUnitCounter_MaxSecondInclusive(t *testing.T) {
	c := NewPlayerUnitCounter(1)
	cap60 := 60
	c.ProcessCommand(cmd(1, models.ActionTypeTrain, "Zealot", 60), &cap60) // at cap → counted
	c.ProcessCommand(cmd(1, models.ActionTypeTrain, "Zealot", 61), &cap60) // past cap → dropped
	if got := c.Count("Zealot"); got != 1 {
		t.Fatalf("expected 1 Zealot within cap, got %d", got)
	}
}

func TestPlayerUnitCounter_NilUnitType(t *testing.T) {
	c := NewPlayerUnitCounter(1)
	c.ProcessCommand(&models.Command{
		ActionType: models.ActionTypeTrain,
		Player:     &models.Player{PlayerID: 1},
	}, nil)
	if got := c.TotalExcluding(); got != 0 {
		t.Fatalf("expected 0 with nil UnitType, got %d", got)
	}
}

func TestHasReplayDurationAtLeast(t *testing.T) {
	d := &BaseDetector{}
	if d.HasReplayDurationAtLeast(1) {
		t.Fatalf("expected false with nil replay")
	}
	d.Initialize(&models.Replay{DurationSeconds: 300}, nil)
	if !d.HasReplayDurationAtLeast(300) {
		t.Fatalf("expected true at exactly the threshold")
	}
	if d.HasReplayDurationAtLeast(301) {
		t.Fatalf("expected false above duration")
	}
}

func TestBaseDetectorFinalize(t *testing.T) {
	d := &BaseDetector{}
	if d.IsFinished() {
		t.Fatalf("expected not finished initially")
	}
	d.Finalize()
	if !d.IsFinished() {
		t.Fatalf("expected finished after Finalize")
	}
}

func TestDetectorLevels(t *testing.T) {
	var p BasePlayerDetector
	if p.Level() != core.LevelPlayer {
		t.Fatalf("expected LevelPlayer, got %v", p.Level())
	}
	var r BaseReplayDetector
	if r.Level() != core.LevelReplay {
		t.Fatalf("expected LevelReplay, got %v", r.Level())
	}
}

func TestBaseReplayDetector_ShouldProcessCommand(t *testing.T) {
	var r BaseReplayDetector
	if !r.ShouldProcessCommand(cmd(1, models.ActionTypeBuild, "X", 1)) {
		t.Fatalf("expected true for command with a player")
	}
	if r.ShouldProcessCommand(&models.Command{ActionType: models.ActionTypeBuild}) {
		t.Fatalf("expected false for command without a player")
	}
}

func TestBuildResultRequiresFinished(t *testing.T) {
	var p BasePlayerDetector
	p.SetReplayPlayerID(1)
	p.Initialize(&models.Replay{ID: 7}, nil)
	if p.BuildPlayerResult("pat", 10, nil) != nil {
		t.Fatalf("expected nil player result before finished")
	}
	p.SetFinished(true)
	res := p.BuildPlayerResult("pat", 10, nil)
	if res == nil {
		t.Fatalf("expected non-nil player result after finished")
	}
	if res.Level != core.LevelPlayer || res.ReplayID != 7 || res.DetectedAtSecond != 10 {
		t.Fatalf("unexpected player result: %+v", res)
	}
	if res.ReplayPlayerID == nil || *res.ReplayPlayerID != 1 {
		t.Fatalf("expected ReplayPlayerID 1, got %v", res.ReplayPlayerID)
	}

	var r BaseReplayDetector
	r.Initialize(&models.Replay{ID: 9}, nil)
	if r.BuildReplayResult("pat", 5, nil) != nil {
		t.Fatalf("expected nil replay result before finished")
	}
	r.SetFinished(true)
	rres := r.BuildReplayResult("pat", 5, nil)
	if rres == nil {
		t.Fatalf("expected non-nil replay result after finished")
	}
	if rres.Level != core.LevelReplay || rres.ReplayID != 9 || rres.DetectedAtSecond != 5 {
		t.Fatalf("unexpected replay result: %+v", rres)
	}
	if rres.ReplayPlayerID != nil {
		t.Fatalf("expected nil ReplayPlayerID on replay result, got %v", rres.ReplayPlayerID)
	}
}
