// Package compact turns a parsed models.ReplayData into the library's compact
// immutable record, keeping only what the dashboard reads.
package compact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/gamerules"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// FileMeta describes the file a replay was parsed from.
type FileMeta struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Checksum [32]byte
}

const (
	featureZergFuzzyOpener   = "bo_z_fuzzy"
	featureViewportMultitask = "viewport_multitasking"
	leaveReasonDropped       = "Dropped"
	actionTrain              = "Train"
	actionUnitMorph          = "Unit Morph"
	actionBuildingMorph      = "Building Morph"
	actionBuild              = "Build"
	actionTech               = "Tech"
	actionUpgrade            = "Upgrade"
	actionTargetedOrder      = "Targeted Order"
	actionChat               = "Chat"
	actionLeaveGame          = "Leave Game"
)

// FromReplayData compacts one parsed replay. It consumes the pattern
// orchestrator's results (GetResults must not have been called before) and
// never retains a *models.Command. Malformed input yields an error, never a
// panic.
func FromReplayData(data *models.ReplayData, file FileMeta) (*library.Replay, error) {
	if data == nil || data.Replay == nil {
		return nil, errors.New("compact: nil replay data")
	}
	if strings.TrimSpace(file.Path) == "" {
		return nil, errors.New("compact: file path is required")
	}
	if len(data.Players) > library.MaxPlayersPerReplay {
		return nil, fmt.Errorf("compact: %d players exceed the %d supported", len(data.Players), library.MaxPlayersPerReplay)
	}
	for i, p := range data.Players {
		if p == nil {
			return nil, fmt.Errorf("compact: player %d is nil", i)
		}
	}

	rep := data.Replay
	r := &library.Replay{
		ID:            library.ReplayIDFromChecksum(file.Checksum),
		Checksum:      file.Checksum,
		Paths:         []library.FileRef{{Path: file.Path, Size: file.Size, ModTime: file.ModTime}},
		Date:          rep.ReplayDate,
		Duration:      library.ClampU16(rep.DurationSeconds),
		Frames:        uint32(max(rep.FrameCount, 0)),
		MapKind:       library.ParseMapKind(rep.MapKind),
		HomeTeamSize:  library.ClampU8(int(rep.HomeTeamSize)),
		AvailSlots:    rep.AvailSlotsCount,
		MapWidth:      rep.MapWidth,
		MapHeight:     rep.MapHeight,
		Title:         rep.Title,
		Host:          library.Strings.Intern(rep.Host),
		Map:           library.Strings.Intern(rep.MapName),
		GameType:      library.Strings.Intern(rep.GameType),
		GameSpeed:     library.Strings.Intern(rep.GameSpeed),
		Engine:        library.Strings.Intern(rep.Engine),
		EngineVersion: library.Strings.Intern(rep.EngineVersion),
		GameSource:    library.Strings.Intern(rep.GameSource),
		LobbyKind:     library.Strings.Intern(rep.LobbyKind),
		TeamFormat:    library.Strings.Intern(rep.TeamFormat),
		Matchup:       library.Strings.Intern(rep.Matchup),
	}
	if rep.TeamStacking {
		r.Flags |= library.FlagTeamStacking
	}
	if rep.TeamInfoIncomplete {
		r.Flags |= library.FlagTeamInfoIncomplete
	}
	if library.IsAutosavePath(file.Path) {
		r.Flags |= library.FlagIsAutosave
	}

	ords := newOrdinals(data.Players)
	r.Players = compactPlayers(data, ords)
	compactCommands(r, data.Commands, ords)

	if orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator); ok && orch != nil {
		r.Markers = compactMarkers(r, orch.GetResults(), ords)
		r.Events = compactEvents(orch.ReplayEvents(), ords)
	}
	r.Alliance = compactAlliance(data.AllianceSnapshots, ords)
	if data.MapContext != nil {
		r.Layout = internLayout(rep.MapName, data.MapContext.Layout)
	}

	for i := range r.Players {
		p := &r.Players[i]
		if p.IsHuman() && !p.IsObserver() {
			p.Cadence = cadenceFor(r, uint8(i))
		}
	}
	if r.HasNonObserverComputer() {
		r.Flags |= library.FlagHasComputer
	}
	if r.OneOnOne() {
		r.Flags |= library.FlagIsOneOnOne
	}
	trimPlayerBuffers(r)
	return r, nil
}

// trimPlayerBuffers gives back what the encoders over-allocated and drops the
// fingerprint vectors of games no fingerprint is ever computed for. Both
// buffers arrive grown by append, which on a hotkey stream is most of its
// size, and the corpus holds every replay for the life of the process, so a
// copy into a right-sized array pays for itself immediately. Clipping would
// not: it lowers the capacity but keeps the same oversized allocation.
func trimPlayerBuffers(r *library.Replay) {
	keepVectors := r.FingerprintEligible()
	for i := range r.Players {
		p := &r.Players[i]
		p.HotkeyStream = shrink(p.HotkeyStream)
		if p.Fingerprint == nil {
			continue
		}
		if !keepVectors {
			p.Fingerprint.Vector = nil
			continue
		}
		p.Fingerprint.Vector = shrink(p.Fingerprint.Vector)
	}
}

// shrink copies s into an exactly sized array when append left a wasteful tail
// behind, and leaves it alone when the waste is not worth a copy.
func shrink[T any](s []T) []T {
	if len(s) == 0 || cap(s) <= len(s)+len(s)/16 {
		return s
	}
	return slices.Clone(s)
}

// ordinals maps replay-local player identities to indexes in Replay.Players.
// Computers all share replay player id 255, so the id table resolves to the
// first such slot; commands carry a *models.Player and resolve exactly.
type ordinals struct {
	byPID [256]int16
	byPtr map[*models.Player]uint8
}

func newOrdinals(players []*models.Player) *ordinals {
	o := &ordinals{byPtr: make(map[*models.Player]uint8, len(players))}
	for i := range o.byPID {
		o.byPID[i] = -1
	}
	for i, p := range players {
		o.byPtr[p] = uint8(i)
		if o.byPID[p.PlayerID] < 0 {
			o.byPID[p.PlayerID] = int16(i)
		}
	}
	return o
}

func (o *ordinals) fromPID(pid *byte) (uint8, bool) {
	if pid == nil {
		return library.NoPlayer, false
	}
	idx := o.byPID[*pid]
	if idx < 0 {
		return library.NoPlayer, false
	}
	return uint8(idx), true
}

func (o *ordinals) fromPlayer(p *models.Player) (uint8, bool) {
	if p == nil {
		return library.NoPlayer, false
	}
	if idx, ok := o.byPtr[p]; ok {
		return idx, true
	}
	return o.fromPID(&p.PlayerID)
}

func compactPlayers(data *models.ReplayData, ords *ordinals) []library.Player {
	players := make([]library.Player, len(data.Players))
	streamClaimed := map[byte]bool{}
	for i, p := range data.Players {
		lp := library.Player{
			Name:           p.Name,
			Key:            library.PlayerKey(p.Name),
			Race:           library.ParseRace(p.Race),
			Type:           library.ParsePlayerType(p.Type),
			Team:           p.Team,
			Slot:           library.ClampU8(int(p.SlotID)),
			ReplayPlayerID: p.PlayerID,
			Color:          library.Strings.Intern(p.Color),
			APM:            library.ClampU16(p.APM),
			EAPM:           library.ClampU16(p.EAPM),
		}
		if p.IsObserver {
			lp.Flags |= library.PlayerObserver
		}
		if p.IsWinner {
			lp.Flags |= library.PlayerWinner
		}
		if p.StartLocationX != nil && p.StartLocationY != nil {
			lp.Flags |= library.PlayerHasStartLocation
			lp.StartX = library.ClampU16(*p.StartLocationX)
			lp.StartY = library.ClampU16(*p.StartLocationY)
			if p.StartLocationOclock != nil {
				lp.StartOclock = library.ClampU8(*p.StartLocationOclock)
			}
		}
		if blob, ok := data.HotkeyStreams[p.PlayerID]; ok && len(blob) > 0 && !streamClaimed[p.PlayerID] {
			streamClaimed[p.PlayerID] = true
			lp.HotkeyStream = blob
		}
		players[i] = lp
	}
	for _, fp := range data.FingerprintVectors {
		start, ok := ords.fromPID(&fp.PlayerID)
		if !ok {
			continue
		}
		for i := int(start); i < len(players); i++ {
			if players[i].ReplayPlayerID != fp.PlayerID || players[i].Fingerprint != nil {
				continue
			}
			players[i].Fingerprint = &library.Fingerprint{
				Race:           library.ParseRace(fp.Race),
				FeatureVersion: library.ClampU16(fp.FeatureVersion),
				ModelTag:       library.Strings.Intern(fp.ModelTag),
				Frames:         int32(fp.Frames),
				CmdCount:       int32(fp.CmdCount),
				Vector:         fp.Vector,
			}
			break
		}
	}
	return players
}

func compactCommands(r *library.Replay, commands []*models.Command, ords *ordinals) {
	for _, cmd := range commands {
		if cmd == nil {
			continue
		}
		ord, ok := ords.fromPlayer(cmd.Player)
		if !ok {
			continue
		}
		sec := library.ClampU16(cmd.SecondsFromGameStart)
		switch cmd.ActionType {
		case actionTrain:
			appendProd(r, sec, ord, library.ProdTrain, cmd.UnitType, cmd.MorphUnitCount)
		case actionUnitMorph:
			appendProd(r, sec, ord, library.ProdUnitMorph, cmd.UnitType, cmd.MorphUnitCount)
		case actionBuildingMorph:
			appendProd(r, sec, ord, library.ProdBuildingMorph, cmd.UnitType, cmd.MorphUnitCount)
		case actionBuild:
			appendProd(r, sec, ord, library.ProdBuild, cmd.UnitType, cmd.MorphUnitCount)
		case actionTech:
			appendProd(r, sec, ord, library.ProdTech, cmd.TechName, 0)
		case actionUpgrade:
			appendProd(r, sec, ord, library.ProdUpgrade, cmd.UpgradeName, 0)
		case actionTargetedOrder:
			if cmd.OrderName != nil && gamerules.IsCompositionSpellOrder(*cmd.OrderName) {
				appendProd(r, sec, ord, library.ProdCast, cmd.OrderName, 0)
			}
		case actionChat:
			if cmd.ChatMessage != nil {
				r.Chat = append(r.Chat, library.ChatLine{Player: ord, Sec: sec, Text: strings.Clone(*cmd.ChatMessage)})
			}
		case actionLeaveGame:
			p := &r.Players[ord]
			if p.Flags.Has(library.PlayerLeft) {
				continue
			}
			p.Flags |= library.PlayerLeft
			p.LeaveSec = sec
			if cmd.LeaveReason != nil {
				reason := strings.TrimSpace(*cmd.LeaveReason)
				p.LeaveReason = library.Strings.Intern(reason)
				if reason == leaveReasonDropped {
					p.Flags |= library.PlayerDropped
				}
			}
		}
	}
	r.Prod.SortBySecond()
	r.Prod.Clip()
}

func appendProd(r *library.Replay, sec uint16, ord uint8, kind library.ProdKind, subject *string, count int) {
	if subject == nil {
		return
	}
	name := strings.TrimSpace(*subject)
	if name == "" {
		return
	}
	if count <= 0 {
		count = 1
	}
	r.Prod.Append(sec, ord, kind, kind.Table().Intern(name), library.ClampU8(count))
}

func compactMarkers(r *library.Replay, results []*core.PatternResult, ords *ordinals) []library.Marker {
	out := make([]library.Marker, 0, len(results))
	for _, res := range results {
		if res == nil {
			continue
		}
		def := markers.ByPatternName(res.PatternName)
		if def == nil {
			continue
		}
		player := library.NoPlayer
		if res.ReplayPlayerID != nil {
			ord, ok := ords.fromPID(res.ReplayPlayerID)
			if !ok {
				continue
			}
			player = ord
		}
		m := library.Marker{
			Feature: library.Features.Intern(def.FeatureKey),
			Player:  player,
			Sec:     library.ClampU16(res.DetectedAtSecond),
		}
		if len(res.Payload) > 0 {
			m.Payload = bytes.Clone(res.Payload)
		}
		switch def.FeatureKey {
		case featureZergFuzzyOpener:
			var payload struct {
				Label string `json:"label"`
			}
			if json.Unmarshal(res.Payload, &payload) == nil && payload.Label != "" {
				m.Label = library.Strings.Intern(payload.Label)
			}
		case featureViewportMultitask:
			var payload struct {
				SwitchesPerMinute float64 `json:"switches_per_minute"`
			}
			if player != library.NoPlayer && json.Unmarshal(res.Payload, &payload) == nil {
				r.Players[player].Viewport = float32(payload.SwitchesPerMinute)
				r.Players[player].Flags |= library.PlayerHasViewport
			}
		}
		out = append(out, m)
	}
	return out
}

func compactEvents(events []worldstate.ReplayEvent, ords *ordinals) []library.GameEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]library.GameEvent, 0, len(events))
	for _, ev := range events {
		ge := library.GameEvent{
			Type:   library.EventTypes.Intern(ev.EventType),
			Sec:    library.ClampU16(ev.Second),
			Source: library.NoPlayer,
			Target: library.NoPlayer,
		}
		if ord, ok := ords.fromPID(ev.SourceReplayPlayerID); ok {
			ge.Source = ord
		}
		if ord, ok := ords.fromPID(ev.TargetReplayPlayerID); ok {
			ge.Target = ord
		}
		if ev.LocationBaseType != nil {
			if trimmed := strings.TrimSpace(*ev.LocationBaseType); trimmed != "" {
				ge.LocationBaseType = library.Strings.Intern(trimmed)
			}
		}
		if ev.LocationBaseOclock != nil {
			ge.LocationOclock = library.ClampU8(*ev.LocationBaseOclock)
		}
		if ev.LocationNaturalOfClock != nil {
			ge.LocationNaturalOclock = library.ClampU8(*ev.LocationNaturalOfClock)
		}
		if ev.LocationMineralOnly != nil && *ev.LocationMineralOnly {
			ge.LocationMineralOnly = true
		}
		ge.Detail = eventDetail(ev)
		out = append(out, ge)
	}
	return out
}

func eventDetail(ev worldstate.ReplayEvent) *library.EventDetail {
	var d library.EventDetail
	for _, unit := range ev.AttackUnitTypes {
		if unit = strings.TrimSpace(unit); unit != "" {
			d.AttackUnits = append(d.AttackUnits, library.Units.Intern(unit))
		}
	}
	if len(ev.AttackCastCounts) > 0 {
		names := make([]string, 0, len(ev.AttackCastCounts))
		for name := range ev.AttackCastCounts {
			names = append(names, name)
		}
		sort.Strings(names)
		d.CastCounts = make([]library.CastCount, 0, len(names))
		for _, name := range names {
			d.CastCounts = append(d.CastCounts, library.CastCount{
				Order: library.Orders.Intern(name),
				Count: library.ClampU16(ev.AttackCastCounts[name]),
			})
		}
	}
	if ev.Payload != nil && *ev.Payload != "" {
		d.Payload = []byte(*ev.Payload)
	}
	if d.AttackUnits == nil && d.CastCounts == nil && d.Payload == nil {
		return nil
	}
	return &d
}

func compactAlliance(snapshots []models.AllianceSnapshot, ords *ordinals) *library.AllianceTimeline {
	if len(snapshots) == 0 {
		return nil
	}
	tl := &library.AllianceTimeline{Snapshots: make([]library.AllianceSnapshot, 0, len(snapshots))}
	for _, s := range snapshots {
		snap := library.AllianceSnapshot{Sec: library.ClampU16(s.Sec), Stacking: s.Stacking}
		for _, team := range s.Teams {
			members := make([]uint8, 0, len(team))
			for i := range team {
				if ord, ok := ords.fromPID(&team[i]); ok {
					members = append(members, ord)
				}
			}
			if len(members) > 0 {
				snap.Teams = append(snap.Teams, members)
			}
		}
		tl.Snapshots = append(tl.Snapshots, snap)
	}
	return tl
}
