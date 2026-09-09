package library

import (
	"path/filepath"
	"sort"
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

// MergeChat returns dst's chat with src's lines folded in. Both records hold
// the same game, but the two files may disagree on player order, so src lines
// are re-attributed by player name; lines whose speaker is not in dst are
// dropped. The merge is idempotent: changed is false when dst already holds
// every src line.
func MergeChat(dst, src *Replay) (merged []ChatLine, changed bool) {
	ordinalByName := make(map[string]uint8, len(dst.Players))
	for i := range dst.Players {
		if _, ok := ordinalByName[dst.Players[i].Name]; !ok {
			ordinalByName[dst.Players[i].Name] = uint8(i)
		}
	}
	type chatKey struct {
		player uint8
		sec    uint16
		text   string
	}
	seen := make(map[chatKey]struct{}, len(dst.Chat))
	merged = append(merged, dst.Chat...)
	for _, line := range merged {
		seen[chatKey{line.Player, line.Sec, line.Text}] = struct{}{}
	}
	for _, line := range src.Chat {
		if int(line.Player) >= len(src.Players) {
			continue
		}
		ord, ok := ordinalByName[src.Players[line.Player].Name]
		if !ok {
			continue
		}
		key := chatKey{ord, line.Sec, line.Text}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, ChatLine{Player: ord, Sec: line.Sec, Text: line.Text})
		changed = true
	}
	if !changed {
		return dst.Chat, false
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Sec != merged[j].Sec {
			return merged[i].Sec < merged[j].Sec
		}
		return merged[i].Player < merged[j].Player
	})
	return merged, true
}
