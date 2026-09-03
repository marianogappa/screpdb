// Package librarytest builds library.Replay fixtures for tests.
package librarytest

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
)

// BaseDate is the date of the first fixture built in a process; each later
// fixture is one minute older, so creation order is newest-first order.
var BaseDate = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

var seq atomic.Int64

type Option func(*library.Replay)

// Replay builds a committed-shape record: a checksum-derived id, one file
// path, a Melee game on a Regular map lasting 15 minutes, unless options say
// otherwise. Player keys and production order are finalised after options run.
func Replay(opts ...Option) *library.Replay {
	n := seq.Add(1)
	r := &library.Replay{
		Date:     BaseDate.Add(-time.Duration(n) * time.Minute),
		Duration: 900,
		MapKind:  library.MapKindRegular,
		Map:      library.Strings.Intern("Fighting Spirit"),
		GameType: library.Strings.Intern("Melee"),
	}
	for _, opt := range opts {
		opt(r)
	}
	var zero [32]byte
	if r.Checksum == zero {
		r.Checksum = sha256.Sum256([]byte(fmt.Sprintf("fixture-%d", n)))
	}
	if r.ID == 0 {
		r.ID = library.ReplayIDFromChecksum(r.Checksum)
	}
	if len(r.Paths) == 0 {
		r.Paths = []library.FileRef{{Path: fmt.Sprintf("/replays/fixture-%d.rep", n), Size: 100_000, ModTime: r.Date}}
	}
	library.SortPaths(r.Paths)
	for i := range r.Players {
		r.Players[i].Key = library.PlayerKey(r.Players[i].Name)
	}
	r.Prod.SortBySecond()
	r.Prod.Clip()
	r.Markers = slices.Clone(r.Markers)
	r.Events = slices.Clone(r.Events)
	r.Chat = slices.Clone(r.Chat)
	return r
}

func WithID(id int64) Option { return func(r *library.Replay) { r.ID = id } }

// WithChecksum sets the checksum to sha256(seed).
func WithChecksum(seed string) Option {
	return func(r *library.Replay) { r.Checksum = sha256.Sum256([]byte(seed)) }
}

// WithRawChecksum sets the checksum bytes directly.
func WithRawChecksum(sum [32]byte) Option {
	return func(r *library.Replay) { r.Checksum = sum }
}

// WithPath appends a file holding this replay.
func WithPath(path string, modTime time.Time) Option {
	return func(r *library.Replay) {
		r.Paths = append(r.Paths, library.FileRef{Path: path, Size: 100_000, ModTime: modTime})
	}
}

func WithDate(t time.Time) Option { return func(r *library.Replay) { r.Date = t } }
func WithDuration(sec int) Option {
	return func(r *library.Replay) { r.Duration = library.ClampU16(sec) }
}
func WithTitle(title string) Option { return func(r *library.Replay) { r.Title = title } }

func WithMap(name string) Option {
	return func(r *library.Replay) { r.Map = library.Strings.Intern(name) }
}

func WithGameType(name string) Option {
	return func(r *library.Replay) { r.GameType = library.Strings.Intern(name) }
}

func WithMatchup(matchup string) Option {
	return func(r *library.Replay) { r.Matchup = library.Strings.Intern(matchup) }
}

func WithMapKind(kind library.MapKind) Option { return func(r *library.Replay) { r.MapKind = kind } }
func WithFlags(flags library.Flags) Option    { return func(r *library.Replay) { r.Flags |= flags } }

type PlayerOption func(*library.Player)

// WithPlayer appends a human, non-observer player on team = ordinal+1.
func WithPlayer(name string, opts ...PlayerOption) Option {
	return func(r *library.Replay) {
		p := library.Player{
			Name:           name,
			Race:           library.RaceTerran,
			Type:           library.PlayerTypeHuman,
			Team:           uint8(len(r.Players) + 1),
			Slot:           uint8(len(r.Players)),
			ReplayPlayerID: uint8(len(r.Players)),
			APM:            120,
			EAPM:           90,
		}
		for _, opt := range opts {
			opt(&p)
		}
		r.Players = append(r.Players, p)
	}
}

func Race(race library.Race) PlayerOption { return func(p *library.Player) { p.Race = race } }
func Team(team uint8) PlayerOption        { return func(p *library.Player) { p.Team = team } }
func Observer() PlayerOption              { return func(p *library.Player) { p.Flags |= library.PlayerObserver } }
func Winner() PlayerOption                { return func(p *library.Player) { p.Flags |= library.PlayerWinner } }
func Computer() PlayerOption              { return func(p *library.Player) { p.Type = library.PlayerTypeComputer } }
func Type(t library.PlayerType) PlayerOption {
	return func(p *library.Player) { p.Type = t }
}
func APM(apm, eapm int) PlayerOption {
	return func(p *library.Player) { p.APM, p.EAPM = library.ClampU16(apm), library.ClampU16(eapm) }
}

// WithProd appends one production event; the stream is sorted when Replay returns.
func WithProd(ordinal uint8, sec int, kind library.ProdKind, subject string) Option {
	return WithProdCount(ordinal, sec, kind, subject, 1)
}

func WithProdCount(ordinal uint8, sec int, kind library.ProdKind, subject string, count int) Option {
	return func(r *library.Replay) {
		r.Prod.Append(library.ClampU16(sec), ordinal, kind, kind.Table().Intern(subject), library.ClampU8(count))
	}
}

// WithMarker appends a marker; use library.NoPlayer for replay-level markers.
func WithMarker(feature string, ordinal uint8, sec int, payload string) Option {
	return func(r *library.Replay) {
		m := library.Marker{Feature: library.Features.Intern(feature), Player: ordinal, Sec: library.ClampU16(sec)}
		if payload != "" {
			m.Payload = []byte(payload)
		}
		r.Markers = append(r.Markers, m)
	}
}

// WithFuzzyLabel appends a bo_z_fuzzy marker carrying an interned label.
func WithFuzzyLabel(ordinal uint8, sec int, label string) Option {
	return func(r *library.Replay) {
		r.Markers = append(r.Markers, library.Marker{
			Feature: library.Features.Intern("bo_z_fuzzy"),
			Label:   library.Strings.Intern(label),
			Player:  ordinal,
			Sec:     library.ClampU16(sec),
			Payload: []byte(fmt.Sprintf(`{"label":%q}`, label)),
		})
	}
}

// WithEvent appends a game event; use library.NoPlayer for absent source/target.
func WithEvent(eventType string, sec int, source, target uint8) Option {
	return func(r *library.Replay) {
		r.Events = append(r.Events, library.GameEvent{
			Type:   library.EventTypes.Intern(eventType),
			Sec:    library.ClampU16(sec),
			Source: source,
			Target: target,
		})
	}
}

func WithChat(ordinal uint8, sec int, text string) Option {
	return func(r *library.Replay) {
		r.Chat = append(r.Chat, library.ChatLine{Player: ordinal, Sec: library.ClampU16(sec), Text: text})
	}
}

func WithHotkeyStream(ordinal uint8, blob []byte) Option {
	return func(r *library.Replay) {
		if int(ordinal) < len(r.Players) {
			r.Players[ordinal].HotkeyStream = blob
		}
	}
}

func WithFingerprint(ordinal uint8, vector []float64) Option {
	return func(r *library.Replay) {
		if int(ordinal) < len(r.Players) {
			r.Players[ordinal].Fingerprint = &library.Fingerprint{
				Race:           r.Players[ordinal].Race,
				FeatureVersion: 1,
				ModelTag:       library.Strings.Intern("fixture"),
				Vector:         vector,
			}
		}
	}
}

func WithCadence(ordinal uint8, cadence library.Cadence) Option {
	return func(r *library.Replay) {
		if int(ordinal) < len(r.Players) {
			c := cadence
			r.Players[ordinal].Cadence = &c
		}
	}
}

func WithLayout(layout *library.MapLayout) Option {
	return func(r *library.Replay) { r.Layout = layout }
}
