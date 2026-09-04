package db

import (
	"context"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/fpvec"
	"github.com/marianogappa/screpdb/internal/library"
)

const bnetGameSource = "AssumedBattleNet"

// playerMatchupOpponent resolves the single opposing human of a 1v1 game. The
// SQL self-join required exactly two non-observer human players; the flag is
// stamped at compaction for the same shape.
func playerMatchupOpponent(r *library.Replay, ordinal uint8) (uint8, bool) {
	if r == nil || !r.Flags.Has(library.FlagIsOneOnOne) {
		return 0, false
	}
	found, opponent := false, uint8(0)
	for i := range r.Players {
		if uint8(i) == ordinal || !humanNonObserver(&r.Players[i]) {
			continue
		}
		if found {
			return 0, false
		}
		found, opponent = true, uint8(i)
	}
	return opponent, found
}

func (s *LibStore) GetPlayerNameByKey(_ context.Context, playerKey string) (string, error) {
	name := ""
	for _, ref := range s.playerGames(playerKey) {
		p := ref.Player()
		if !humanNonObserver(p) {
			continue
		}
		if name == "" || p.Name < name {
			name = p.Name
		}
	}
	return strings.TrimSpace(name), nil
}

func (s *LibStore) GetPlayerOverviewSummary(_ context.Context, playerKey string) (*PlayerOverviewSummaryRow, error) {
	out := &PlayerOverviewSummaryRow{}
	name := ""
	var apmSum, eapmSum float64
	var apmGames, eapmGames int64
	for _, ref := range s.playerGames(playerKey) {
		p := ref.Player()
		if !humanNonObserver(p) {
			continue
		}
		if name == "" || p.Name < name {
			name = p.Name
		}
		out.GamesPlayed++
		if p.IsWinner() {
			out.Wins++
		}
		if p.APM > 0 {
			apmSum += float64(p.APM)
			apmGames++
		}
		if p.EAPM > 0 {
			eapmSum += float64(p.EAPM)
			eapmGames++
		}
	}
	out.PlayerName = name
	if apmGames > 0 {
		out.AverageAPM = apmSum / float64(apmGames)
	}
	if eapmGames > 0 {
		out.AverageEAPM = eapmSum / float64(eapmGames)
	}
	return out, nil
}

const playerRecentGamesLimit = 10

func (s *LibStore) ListPlayerRecentGames(_ context.Context, playerKey string) ([]PlayerRecentGameRow, error) {
	out := make([]PlayerRecentGameRow, 0, playerRecentGamesLimit)
	for _, ref := range s.playerGames(playerKey) {
		if !humanNonObserver(ref.Player()) {
			continue
		}
		r := ref.Replay
		out = append(out, PlayerRecentGameRow{
			ReplayID:           r.ID,
			ReplayDate:         r.Date.String(),
			FileName:           r.FileName(),
			MapName:            library.Strings.Name(r.Map),
			MapKind:            r.MapKind.String(),
			GameSource:         library.Strings.Name(r.GameSource),
			LobbyKind:          library.Strings.Name(r.LobbyKind),
			DurationSeconds:    int64(r.Duration),
			GameType:           library.Strings.Name(r.GameType),
			TeamFormat:         library.Strings.Name(r.TeamFormat),
			Matchup:            library.Strings.Name(r.Matchup),
			TeamStacking:       r.Flags.Has(library.FlagTeamStacking),
			TeamInfoIncomplete: r.Flags.Has(library.FlagTeamInfoIncomplete),
			PlayersLabel:       playersLabel(r),
			WinnersLabel:       winnersLabel(r),
		})
		if len(out) == playerRecentGamesLimit {
			break
		}
	}
	return out, nil
}

// playersLabel is the SQL's group_concat over non-observer humans ordered by
// team then slot order.
func playersLabel(r *library.Replay) string {
	ordinals := make([]uint8, 0, len(r.Players))
	for i := range r.Players {
		if humanNonObserver(&r.Players[i]) {
			ordinals = append(ordinals, uint8(i))
		}
	}
	sort.SliceStable(ordinals, func(i, j int) bool {
		return r.Players[ordinals[i]].Team < r.Players[ordinals[j]].Team
	})
	names := make([]string, 0, len(ordinals))
	for _, ordinal := range ordinals {
		names = append(names, r.Players[ordinal].Name)
	}
	return strings.Join(names, ", ")
}

func winnersLabel(r *library.Replay) string {
	names := make([]string, 0, len(r.Players))
	for i := range r.Players {
		p := &r.Players[i]
		if humanNonObserver(p) && p.IsWinner() {
			names = append(names, p.Name)
		}
	}
	return strings.Join(names, ", ")
}

func (s *LibStore) ListPlayerMatchups(_ context.Context, playerKey string) ([]PlayerMatchupRow, error) {
	type matchup struct {
		own string
		opp string
	}
	games := map[matchup]int64{}
	wins := map[matchup]int64{}
	replays := map[matchup]map[int64]struct{}{}
	for _, ref := range s.playerGames(playerKey) {
		self := ref.Player()
		if !humanNonObserver(self) {
			continue
		}
		opponent, ok := playerMatchupOpponent(ref.Replay, ref.Ordinal)
		if !ok {
			continue
		}
		key := matchup{
			own: strings.TrimSpace(self.Race.String()),
			opp: strings.TrimSpace(ref.Replay.Players[opponent].Race.String()),
		}
		if replays[key] == nil {
			replays[key] = map[int64]struct{}{}
		}
		replays[key][ref.Replay.ID] = struct{}{}
		games[key] = int64(len(replays[key]))
		if self.IsWinner() {
			wins[key]++
		}
	}
	out := make([]PlayerMatchupRow, 0, len(games))
	for key, count := range games {
		out = append(out, PlayerMatchupRow{OwnRace: key.own, OppRace: key.opp, Games: count, Wins: wins[key]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].OwnRace != out[j].OwnRace {
			return out[i].OwnRace < out[j].OwnRace
		}
		return out[i].OppRace < out[j].OppRace
	})
	return out, nil
}

func (s *LibStore) ListRaceSections(_ context.Context, playerKey string) ([]RaceSectionRow, error) {
	counts := map[string]*RaceSectionRow{}
	for _, ref := range s.playerGames(playerKey) {
		p := ref.Player()
		if !humanNonObserver(p) {
			continue
		}
		race := p.Race.String()
		row, ok := counts[race]
		if !ok {
			row = &RaceSectionRow{Race: race}
			counts[race] = row
		}
		row.GameCount++
		if p.IsWinner() {
			row.Wins++
		}
	}
	out := make([]RaceSectionRow, 0, len(counts))
	for _, row := range counts {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GameCount != out[j].GameCount {
			return out[i].GameCount > out[j].GameCount
		}
		return out[i].Race < out[j].Race
	})
	return out, nil
}

func (s *LibStore) ListRaceOrderRows(_ context.Context, playerKey string) ([]RaceOrderRow, error) {
	key := normalizeKey(playerKey)
	return memo(s.view(), "libstore.raceOrderRows:"+key, func() []RaceOrderRow {
		out := []RaceOrderRow{}
		for _, ref := range s.playerGames(key) {
			p := ref.Player()
			if p.IsObserver() {
				continue
			}
			race := p.Race.String()
			playerID := ref.ID()
			forEachOrder(ref.Replay, ref.Ordinal, func(actionType string, name string, sec uint16) {
				row := RaceOrderRow{PlayerID: playerID, Race: race, ActionType: actionType, Second: int64(sec)}
				if actionType == orderActionTech {
					row.TechName = &name
				} else {
					row.UpgradeName = &name
				}
				out = append(out, row)
			})
		}
		sortOrderRows(out, func(i int) (int64, int64) { return out[i].PlayerID, out[i].Second })
		return out
	}), nil
}

func (s *LibStore) ListMatchupOrderRows(_ context.Context, playerKey string) ([]MatchupOrderRow, error) {
	key := normalizeKey(playerKey)
	return memo(s.view(), "libstore.matchupOrderRows:"+key, func() []MatchupOrderRow {
		out := []MatchupOrderRow{}
		for _, ref := range s.playerGames(key) {
			self := ref.Player()
			if !humanNonObserver(self) {
				continue
			}
			opponent, ok := playerMatchupOpponent(ref.Replay, ref.Ordinal)
			if !ok {
				continue
			}
			ownRace := strings.TrimSpace(self.Race.String())
			oppRace := strings.TrimSpace(ref.Replay.Players[opponent].Race.String())
			playerID := ref.ID()
			forEachOrder(ref.Replay, ref.Ordinal, func(actionType string, name string, sec uint16) {
				row := MatchupOrderRow{
					PlayerID:   playerID,
					OwnRace:    ownRace,
					OppRace:    oppRace,
					ReplayID:   ref.Replay.ID,
					ActionType: actionType,
					Second:     int64(sec),
				}
				if actionType == orderActionTech {
					row.TechName = &name
				} else {
					row.UpgradeName = &name
				}
				out = append(out, row)
			})
		}
		sortOrderRows(out, func(i int) (int64, int64) { return out[i].PlayerID, out[i].Second })
		return out
	}), nil
}

const (
	orderActionTech    = "Tech"
	orderActionUpgrade = "Upgrade"
)

// forEachOrder visits one player's research and upgrade commands in second
// order, skipping the empty subjects the SQL's `<> ”` guard dropped.
func forEachOrder(r *library.Replay, ordinal uint8, fn func(actionType, name string, sec uint16)) {
	prod := &r.Prod
	for i := 0; i < prod.Len(); i++ {
		if prod.Player[i] != ordinal {
			continue
		}
		actionType := ""
		switch prod.Kind[i] {
		case library.ProdTech:
			actionType = orderActionTech
		case library.ProdUpgrade:
			actionType = orderActionUpgrade
		default:
			continue
		}
		name := prod.SubjectName(i)
		if name == "" {
			continue
		}
		fn(actionType, name, prod.Sec[i])
	}
}

func sortOrderRows[T any](rows []T, keys func(i int) (int64, int64)) {
	sort.SliceStable(rows, func(i, j int) bool {
		leftPlayer, leftSecond := keys(i)
		rightPlayer, rightSecond := keys(j)
		if leftPlayer != rightPlayer {
			return leftPlayer < rightPlayer
		}
		return leftSecond < rightSecond
	})
}

func (s *LibStore) ListPlayerChatRows(_ context.Context, playerKey string) ([]PlayerChatRow, error) {
	out := []PlayerChatRow{}
	for _, ref := range s.playerGames(playerKey) {
		if ref.Player().IsObserver() {
			continue
		}
		chat := ref.Replay.Chat
		mine := make([]PlayerChatRow, 0, len(chat))
		for i := range chat {
			if chat[i].Player != ref.Ordinal || strings.TrimSpace(chat[i].Text) == "" {
				continue
			}
			mine = append(mine, PlayerChatRow{ReplayID: ref.Replay.ID, Message: chat[i].Text})
		}
		// The SQL ordered seconds descending within a game; Chat is stored
		// ascending, so the per-game block is reversed rather than re-sorted.
		for i := len(mine) - 1; i >= 0; i-- {
			out = append(out, mine[i])
		}
	}
	return out, nil
}

func (s *LibStore) ListPlayerFirstExpansionTimings(_ context.Context, playerKey string) ([]PlayerFirstExpansionTimingRow, error) {
	out := []PlayerFirstExpansionTimingRow{}
	expansion, ok := library.EventTypes.Lookup("expansion")
	if !ok {
		return out, nil
	}
	type group struct {
		race     string
		mapKind  string
		replayID int64
	}
	first := map[group]int64{}
	order := []group{}
	for _, ref := range s.playerGames(playerKey) {
		p := ref.Player()
		if !humanNonObserver(p) || ref.Replay.MapKind == library.MapKindUseMapSettings {
			continue
		}
		for i := range ref.Replay.Events {
			e := &ref.Replay.Events[i]
			if e.Type != expansion || e.Source != ref.Ordinal {
				continue
			}
			key := group{
				race:     strings.TrimSpace(p.Race.String()),
				mapKind:  strings.TrimSpace(ref.Replay.MapKind.String()),
				replayID: ref.Replay.ID,
			}
			sec := int64(e.Sec)
			if existing, seen := first[key]; !seen {
				first[key] = sec
				order = append(order, key)
			} else if sec < existing {
				first[key] = sec
			}
		}
	}
	for _, key := range order {
		out = append(out, PlayerFirstExpansionTimingRow{
			Race:                 key.race,
			MapKind:              key.mapKind,
			ReplayID:             key.replayID,
			FirstExpansionSecond: first[key],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Race != out[j].Race {
			return out[i].Race < out[j].Race
		}
		if out[i].MapKind != out[j].MapKind {
			return out[i].MapKind < out[j].MapKind
		}
		return out[i].FirstExpansionSecond < out[j].FirstExpansionSecond
	})
	return out, nil
}

func (s *LibStore) GetPlayerFingerprintCoverage(_ context.Context, playerKey string, featureVersion int64) (int64, error) {
	replays := map[int64]struct{}{}
	for _, ref := range s.playerGames(playerKey) {
		fp := ref.Player().Fingerprint
		if fp == nil || int64(fp.FeatureVersion) != featureVersion {
			continue
		}
		replays[ref.Replay.ID] = struct{}{}
	}
	return int64(len(replays)), nil
}

func (s *LibStore) ListPlayerFingerprintVectors(_ context.Context, playerKey string, featureVersion int64) ([]PlayerFingerprintVectorRow, error) {
	refs := s.playerGames(playerKey)
	out := make([]PlayerFingerprintVectorRow, 0, len(refs))
	// playerGames is newest first; the SQL ordered by replay date ascending.
	for i := len(refs) - 1; i >= 0; i-- {
		ref := refs[i]
		fp := ref.Player().Fingerprint
		if fp == nil || int64(fp.FeatureVersion) != featureVersion {
			continue
		}
		r := ref.Replay
		if r.MapKind == library.MapKindMoney || !r.Flags.Has(library.FlagIsOneOnOne) {
			continue
		}
		out = append(out, PlayerFingerprintVectorRow{Vector: fpvec.Encode(fp.Vector), Race: fp.Race.String()})
	}
	return out, nil
}

func (s *LibStore) CountPlayerBnetGames(_ context.Context, playerKey string) (int64, error) {
	count := int64(0)
	for _, ref := range s.playerGames(playerKey) {
		if ref.Player().IsObserver() {
			continue
		}
		if library.Strings.Name(ref.Replay.GameSource) == bnetGameSource {
			count++
		}
	}
	return count, nil
}
