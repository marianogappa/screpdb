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

// chatEchoToleranceSec bounds how far apart two recordings of one game log the
// same chat line: each client records it on the frame it arrived, so the copies
// drift by a few frames and can straddle a second boundary.
const chatEchoToleranceSec = 2

// MergeChat returns dst's chat with src's lines folded in. Both records hold
// the same game, but the two files may disagree on player order, so src lines
// are re-attributed by player name; lines whose speaker is not in dst are
// dropped. A src line is the same message as an unmatched dst line from the
// same speaker with the same text within chatEchoToleranceSec, so repeated
// messages are kept once per repetition. The merge is idempotent: changed is
// false when dst already holds every src line.
func MergeChat(dst, src *Replay) (merged []ChatLine, changed bool) {
	ordinalByName := make(map[string]uint8, len(dst.Players))
	for i := range dst.Players {
		if _, ok := ordinalByName[dst.Players[i].Name]; !ok {
			ordinalByName[dst.Players[i].Name] = uint8(i)
		}
	}
	type chatKey struct {
		player uint8
		text   string
	}
	unmatched := make(map[chatKey][]int, len(dst.Chat))
	merged = append(merged, dst.Chat...)
	for i, line := range merged {
		key := chatKey{line.Player, line.Text}
		unmatched[key] = append(unmatched[key], i)
	}
	for _, line := range src.Chat {
		if int(line.Player) >= len(src.Players) {
			continue
		}
		ord, ok := ordinalByName[src.Players[line.Player].Name]
		if !ok {
			continue
		}
		key := chatKey{ord, line.Text}
		candidates := unmatched[key]
		best, bestDiff := -1, chatEchoToleranceSec+1
		for c, idx := range candidates {
			diff := int(merged[idx].Sec) - int(line.Sec)
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff {
				best, bestDiff = c, diff
			}
		}
		if best >= 0 {
			unmatched[key] = append(candidates[:best], candidates[best+1:]...)
			continue
		}
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
