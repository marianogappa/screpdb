package dashboard

import (
	"context"
	"sort"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/propack"
	"github.com/marianogappa/screpdb/internal/skill"
)

// workflowPlayersSkill is the Players page Skill tab: where each of the library's players sits on the tiers,
// the median of their Skill averages, and the built-in progamers beside them.
type workflowPlayersSkill struct {
	Players  int                            `json:"players"`
	MinGames int                            `json:"min_games"`
	Combined *workflowSkillMedian           `json:"combined,omitempty"`
	Metrics  map[string]workflowSkillMedian `json:"metrics"`
	Library  []workflowLibrarySkill         `json:"library_players"`
	Featured []workflowFeaturedSkill        `json:"featured_players"`
	Model    *workflowSkillModel            `json:"model,omitempty"`
}

type workflowLibrarySkill struct {
	PlayerKey  string                     `json:"player_key"`
	PlayerName string                     `json:"player_name"`
	Games      int                        `json:"games"`
	Combined   skill.Placement            `json:"combined"`
	Metrics    map[string]skill.Placement `json:"metrics"`
}

type workflowSkillMedian struct {
	Position float64 `json:"position"`
	Value    float64 `json:"value"`
	Players  int     `json:"players"`
}

type workflowFeaturedSkill struct {
	PlayerKey   string                     `json:"player_key"`
	PlayerName  string                     `json:"player_name"`
	Race        string                     `json:"race,omitempty"`
	CountryCode string                     `json:"country_code,omitempty"`
	PhotoURL    string                     `json:"photo_url,omitempty"`
	Games       int                        `json:"games"`
	Combined    skill.Placement            `json:"combined"`
	Metrics     map[string]skill.Placement `json:"metrics"`
}

func (d *Dashboard) PlayersSkill(_ context.Context, _ apigen.PlayersSkillRequestObject) (any, error) {
	return d.playersSkill(), nil
}

func (d *Dashboard) playersSkill() workflowPlayersSkill {
	out := workflowPlayersSkill{MinGames: minSkillGames, Metrics: map[string]workflowSkillMedian{}, Library: []workflowLibrarySkill{}, Featured: []workflowFeaturedSkill{}}
	model, err := skill.Load()
	if err != nil {
		crashreport.Handle(err, nil, false)
		return out
	}
	out.Model = skillModelSummary(model)
	if d.library != nil {
		players := d.librarySkillPlayersMemo(model)
		out.Players = len(players)
		out.Library = players
		out.Combined, out.Metrics = skillMedians(players)
	}
	for _, pro := range d.featuredOverlayPros(func(p *propack.Pro) bool { return p.Skill != nil }) {
		out.Featured = append(out.Featured, workflowFeaturedSkill{
			PlayerKey: pro.Key(), PlayerName: pro.Label, Race: pro.MainRace, CountryCode: pro.Country, PhotoURL: proPhotoURL(pro),
			Games: pro.Skill.Games, Combined: proHeadline(model, pro), Metrics: pro.Skill.Metrics,
		})
	}
	return out
}

func (d *Dashboard) librarySkillPlayersMemo(model *skill.Model) []workflowLibrarySkill {
	view := d.library.lib.View()
	return view.Memo("players-skill", func() any { return librarySkillPlayers(model, view.Replays()) }).([]workflowLibrarySkill)
}

// skillByPlayer is each library player's Skill average for the list columns; nil while the engine is off,
// since Skill is an engine feature.
func (d *Dashboard) skillByPlayer(ctx context.Context) map[string]skill.Placement {
	if d.library == nil || !d.featureFlagEnabled(ctx, featureFlagEnginePass) {
		return nil
	}
	model, err := skill.Load()
	if err != nil {
		crashreport.Handle(err, nil, false)
		return nil
	}
	out := map[string]skill.Placement{}
	for _, p := range d.librarySkillPlayersMemo(model) {
		out[p.PlayerKey] = p.Combined
	}
	return out
}

// proHeadline recomputes a pro's tier from their metrics, so a pro pack baked before the headline followed
// the strips still agrees with them.
func proHeadline(model *skill.Model, pro *propack.Pro) skill.Placement {
	if h := model.Headline(model.Shares(proSkillRace(pro.MainRace)), pro.Skill.Metrics); h != nil {
		return *h
	}
	return pro.Skill.Combined
}

func proSkillRace(mainRace string) string {
	if mainRace == "" {
		return ""
	}
	return mainRace[:1]
}

// librarySkillPlayers is every player's Skill average, for the players with enough rated games.
func librarySkillPlayers(model *skill.Model, replays []*library.Replay) []workflowLibrarySkill {
	byPlayer := map[string][]ratedPlayer{}
	names := map[string]string{}
	for _, rec := range replays {
		for ordinal, r := range rateGame(model, rec).players {
			if r.result.Combined != nil {
				key := rec.Players[ordinal].Key
				byPlayer[key] = append(byPlayer[key], r)
				names[key] = rec.Players[ordinal].Name
			}
		}
	}
	keys := make([]string, 0, len(byPlayer))
	for k := range byPlayer {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []workflowLibrarySkill{}
	for _, k := range keys {
		if s := summarizeSkill(model, byPlayer[k]); s != nil {
			out = append(out, workflowLibrarySkill{PlayerKey: k, PlayerName: names[k], Games: s.Games, Combined: s.Combined, Metrics: s.Metrics})
		}
	}
	return out
}

func skillMedians(summaries []workflowLibrarySkill) (*workflowSkillMedian, map[string]workflowSkillMedian) {
	metrics := map[string]workflowSkillMedian{}
	if len(summaries) == 0 {
		return nil, metrics
	}
	combined := make([]skill.Placement, 0, len(summaries))
	byMetric := map[string][]skill.Placement{}
	for _, s := range summaries {
		combined = append(combined, s.Combined)
		for k, p := range s.Metrics {
			byMetric[k] = append(byMetric[k], p)
		}
	}
	c := medianPlacement(combined)
	for k, ps := range byMetric {
		metrics[k] = medianPlacement(ps)
	}
	return &c, metrics
}

func medianPlacement(ps []skill.Placement) workflowSkillMedian {
	positions := make([]float64, len(ps))
	values := make([]float64, len(ps))
	for i, p := range ps {
		positions[i], values[i] = p.Position, p.Value
	}
	return workflowSkillMedian{Position: medianFloat(positions), Value: medianFloat(values), Players: len(ps)}
}

func medianFloat(xs []float64) float64 {
	sort.Float64s(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}
