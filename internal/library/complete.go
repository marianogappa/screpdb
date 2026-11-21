package library

import (
	"path/filepath"
	"slices"
	"strings"
)

// CompleteSuffix marks a downloaded co-player copy of a game the user's own
// recording did not capture to the end (issue #341). "<stem>-complete<ext>"
// pairs with "<stem><ext>" in the same directory: the complete copy carries
// the whole game, the user's own file stays on disk as provenance and as the
// source of their chat.
const CompleteSuffix = "-complete"

// IsCompletePath reports whether path names a complete copy.
func IsCompletePath(path string) bool {
	_, ok := CompleteBasePath(path)
	return ok
}

// CompleteBasePath returns the path of the user's own copy a complete copy
// pairs with.
func CompleteBasePath(path string) (string, bool) {
	ext := filepath.Ext(path)
	stem := path[:len(path)-len(ext)]
	if !strings.HasSuffix(stem, CompleteSuffix) {
		return "", false
	}
	base := stem[:len(stem)-len(CompleteSuffix)]
	if filepath.Base(base+ext) == ext || base == "" {
		return "", false
	}
	return base + ext, true
}

// CompletePathFor returns the complete-copy path pairing with the user's own
// copy at path.
func CompletePathFor(path string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)] + CompleteSuffix + ext
}

// OwnChat returns the own copy's chat re-attributed to the complete record's
// player order, which may differ, by player name. The complete copy was
// recorded by someone else, so its own chat is never shown; lines whose speaker
// is not in complete are dropped. changed is false when complete already holds
// exactly that chat.
func OwnChat(complete, own *Replay) (chat []ChatLine, changed bool) {
	ordinalByName := make(map[string]uint8, len(complete.Players))
	for i := range complete.Players {
		if _, ok := ordinalByName[complete.Players[i].Name]; !ok {
			ordinalByName[complete.Players[i].Name] = uint8(i)
		}
	}
	chat = make([]ChatLine, 0, len(own.Chat))
	for _, line := range own.Chat {
		if int(line.Player) >= len(own.Players) {
			continue
		}
		ord, ok := ordinalByName[own.Players[line.Player].Name]
		if !ok {
			continue
		}
		chat = append(chat, ChatLine{Player: ord, Sec: line.Sec, Text: line.Text})
	}
	if slices.Equal(chat, complete.Chat) {
		return complete.Chat, false
	}
	return chat, true
}
