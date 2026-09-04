package db

import (
	"strings"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// view is the filtered corpus every read but the unfiltered ones works from.
func (s *LibStore) view() *library.View { return s.lib.View() }

// snapshot is the unfiltered corpus, for the few reads that must ignore the
// global filter (corpus totals, settings, bnet).
func (s *LibStore) snapshot() *library.Snapshot { return s.lib.Snapshot() }

func (s *LibStore) replay(replayID int64) (*library.Replay, error) {
	r, ok := s.view().ByID(replayID)
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

// replaysByIDs resolves ids against the filtered view, preserving the caller's
// order and skipping ids the filter or the corpus does not have.
func (s *LibStore) replaysByIDs(ids []int64) []*library.Replay {
	v := s.view()
	out := make([]*library.Replay, 0, len(ids))
	for _, id := range ids {
		if r, ok := v.ByID(id); ok {
			out = append(out, r)
		}
	}
	return out
}

// playerGames returns the filtered slots a player key occupies, newest game
// first.
func (s *LibStore) playerGames(playerKey string) []library.PlayerRef {
	return s.view().PlayerGames(normalizeKey(playerKey))
}

// resolvePlayer recovers the replay and player ordinal behind a player id.
func (s *LibStore) resolvePlayer(playerID int64) (*library.Replay, uint8, bool) {
	replayID, ordinal := library.SplitPlayerID(playerID)
	r, ok := s.view().ByID(replayID)
	if !ok || int(ordinal) >= len(r.Players) {
		return nil, 0, false
	}
	return r, ordinal, true
}

func rowPlayerID(r *library.Replay, ordinal uint8) int64 {
	return library.PlayerID(r.ID, ordinal)
}

func ordinalForPlayerID(r *library.Replay, playerID int64) (uint8, bool) {
	replayID, ordinal := library.SplitPlayerID(playerID)
	if r == nil || replayID != r.ID || int(ordinal) >= len(r.Players) {
		return 0, false
	}
	return ordinal, true
}

// memo is the typed form of View.Memo: one build per key for the lifetime of
// the view, which is the lifetime of the (snapshot, filter) pair.
func memo[T any](v *library.View, key string, build func() T) T {
	value := v.Memo(key, func() any { return build() })
	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero
	}
	return typed
}

// normalizeKey matches the dashboard's normalizePlayerKey, the corpus-wide
// identity of a player name.
func normalizeKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// humanNonObserver is the "counts as a player" test every population read
// applies.
func humanNonObserver(p *library.Player) bool {
	return p != nil && p.IsHuman() && !p.IsObserver()
}

// forEachProd visits the production stream of one replay in second order,
// keeping only events of the given kind. Pass library.NoPlayer as the ordinal
// to visit every player.
func forEachProd(r *library.Replay, kind library.ProdKind, ordinal uint8, fn func(sec uint16, player uint8, subject uint16, count uint8)) {
	if r == nil {
		return
	}
	prod := &r.Prod
	for i := 0; i < prod.Len(); i++ {
		if prod.Kind[i] != kind {
			continue
		}
		if ordinal != library.NoPlayer && prod.Player[i] != ordinal {
			continue
		}
		fn(prod.Sec[i], prod.Player[i], prod.Subject[i], prod.Count[i])
	}
}

// markerLabel is a marker's resolved display value ("3 Hatch Muta") rather
// than its static placeholder. Compaction interns the label for the markers
// that carry one; the payload is decoded for the rest.
func markerLabel(m *library.Marker) (string, bool) {
	if m == nil {
		return "", false
	}
	if label := library.Strings.Name(m.Label); label != "" {
		return label, true
	}
	return markers.DecodePayloadLabel(m.Payload)
}
