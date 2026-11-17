package db

import "context"

// BotGamesByPlayerKey counts, per player key, the games in which that player
// tripped a bot signal. Players with none are absent.
func (s *LibStore) BotGamesByPlayerKey(context.Context) (map[string]int64, error) {
	v := s.view()
	return memo(v, "players:bot_games", func() map[string]int64 {
		out := map[string]int64{}
		for _, r := range v.Replays() {
			for i := range r.Players {
				if p := &r.Players[i]; p.IsBot() {
					out[p.Key]++
				}
			}
		}
		return out
	}), nil
}
