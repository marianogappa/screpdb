package parser

import (
	"testing"

	scraprep "github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcore"
	"github.com/icza/screp/repparser/repdecoder"
)

func human() *scraprep.Player {
	return &scraprep.Player{Type: repcore.PlayerTypeHuman}
}

func computer() *scraprep.Player {
	return &scraprep.Player{Type: repcore.PlayerTypeComputer}
}

func TestDeriveGameSource(t *testing.T) {
	tests := []struct {
		name string
		rep  *scraprep.Replay
		want string
	}{
		{
			name: "ShieldBattery",
			rep: &scraprep.Replay{
				Header:        &scraprep.Header{Players: []*scraprep.Player{human(), human()}},
				ShieldBattery: &scraprep.ShieldBattery{GameID: "abc"},
			},
			want: "ShieldBattery",
		},
		{
			name: "SinglePlayer_one_human",
			rep: &scraprep.Replay{
				Header: &scraprep.Header{Players: []*scraprep.Player{human(), computer()}},
			},
			want: "SinglePlayer",
		},
		{
			name: "PreSCR_legacy_format",
			rep: &scraprep.Replay{
				Header:    &scraprep.Header{Players: []*scraprep.Player{human(), human()}},
				RepFormat: repdecoder.RepFormatLegacy,
			},
			want: "PreSCR",
		},
		{
			name: "AssumedBattleNet_modern_two_humans",
			rep: &scraprep.Replay{
				Header:    &scraprep.Header{Players: []*scraprep.Player{human(), human()}},
				RepFormat: repdecoder.RepFormatModern121,
			},
			want: "AssumedBattleNet",
		},
		{
			name: "SinglePlayer_beats_PreSCR",
			rep: &scraprep.Replay{
				Header:    &scraprep.Header{Players: []*scraprep.Player{human()}},
				RepFormat: repdecoder.RepFormatLegacy,
			},
			want: "SinglePlayer",
		},
		{
			name: "ShieldBattery_beats_SinglePlayer",
			rep: &scraprep.Replay{
				Header:        &scraprep.Header{Players: []*scraprep.Player{human()}},
				ShieldBattery: &scraprep.ShieldBattery{},
			},
			want: "ShieldBattery",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveGameSource(tt.rep)
			if got != tt.want {
				t.Errorf("deriveGameSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveLobbyKind(t *testing.T) {
	tests := []struct {
		name       string
		gameSource string
		title      string
		want       string
	}{
		{"PreSCR_unknown", "PreSCR", "anything", "Unknown"},
		{"SinglePlayer_unknown", "SinglePlayer", "anything", "Unknown"},
		{"ShieldBattery_unknown", "ShieldBattery", "ShieldBattery Lobby", "Unknown"},
		{"matchmaking_12char", "AssumedBattleNet", "zsqYHmOErMoO", "Matchmaking"},
		{"matchmaking_bracket_chars", "AssumedBattleNet", "x_b[GOHXBBNO", "Matchmaking"},
		{"custom_human_title", "AssumedBattleNet", "2v2v2v2 BGH Noobs only", "Custom"},
		{"custom_short_title", "AssumedBattleNet", "aca", "Custom"},
		{"custom_12char_with_space", "AssumedBattleNet", "hello world!", "Custom"},
		{"custom_12char_with_digit", "AssumedBattleNet", "aaaa1aaaaaaa", "Custom"},
		{"custom_empty_title", "AssumedBattleNet", "", "Custom"},
		{"matchmaking_all_lowercase", "AssumedBattleNet", "abcdefghijkl", "Matchmaking"},
		{"custom_11char", "AssumedBattleNet", "abcdefghijk", "Custom"},
		{"custom_13char", "AssumedBattleNet", "abcdefghijklm", "Custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveLobbyKind(tt.gameSource, tt.title)
			if got != tt.want {
				t.Errorf("deriveLobbyKind(%q, %q) = %q, want %q", tt.gameSource, tt.title, got, tt.want)
			}
		})
	}
}

func TestIsMatchmakingTitle(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"zsqYHmOErMoO", true},
		{"x_b[GOHXBBNO", true},
		{"u\\FJgKLgKey_", true},
		{"AAAAAAAAAAAA", true},
		{"zzzzzzzzzzzz", true},
		{"hello world!", false},
		{"aaaa1aaaaaaa", false},
		{"AAAA AAAAAAA", false},
		{"aaaa{aaaaaaa", false},
		{"aaaa@aaaaaaa", false},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := isMatchmakingTitle(tt.title)
			if got != tt.want {
				t.Errorf("isMatchmakingTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}
