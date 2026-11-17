package library

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/botdetect"
)

func TestReplayHasBot(t *testing.T) {
	r := &Replay{Players: []Player{{Name: "Flash"}, {Name: "ComfyUI"}}}
	if r.HasBot() || r.Players[1].IsBot() {
		t.Fatal("no player tripped a signal, yet the game reads as having a bot")
	}
	r.Players[1].BotSignals = botdetect.SignalFarJumps
	if !r.HasBot() || !r.Players[1].IsBot() || r.Players[0].IsBot() {
		t.Fatal("the bot is not reported, or the human is")
	}
}
