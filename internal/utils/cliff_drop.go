package utils

import (
	"regexp"
	"strings"
)

// bigGameHuntersMapPattern matches "Big Game Hunters" plus the (N) BGH /
// (N)BigGameHunters / Big-Game-Hunters / BGH-extra naming variants
// commonly seen in replay packs. Case-insensitive.
var bigGameHuntersMapPattern = regexp.MustCompile(
	`(?i)^(?:\(\d+\)\s*)?(?:big[\s._-]*game[\s._-]*hunters|bgh)(?:[\s._-].*)?$`,
)

// IsBigGameHuntersMap reports whether the map name matches the BGH
// naming family. Lives in utils so both internal/patterns/markers and
// internal/patterns/worldstate can share the predicate without inviting
// an import cycle.
func IsBigGameHuntersMap(name string) bool {
	return bigGameHuntersMapPattern.MatchString(strings.TrimSpace(name))
}

// Corner box dimensions (pixel space). A drop counts as a "cliff drop"
// when its position lands inside either the top-left or bottom-right
// rectangle of these dimensions, anchored to the corresponding corner
// of the map.
const (
	CliffDropCornerWidthPx  = 256
	CliffDropCornerHeightPx = 128
)

// IsCliffDropPosition reports whether (x,y) lies in the top-left or
// bottom-right corner box of a map of the given pixel dimensions. Pixel
// coordinates use BW conventions: (0,0) = top-left.
func IsCliffDropPosition(x, y, mapWidthPx, mapHeightPx int) bool {
	if mapWidthPx <= 0 || mapHeightPx <= 0 {
		return false
	}
	if x >= 0 && x < CliffDropCornerWidthPx && y >= 0 && y < CliffDropCornerHeightPx {
		return true
	}
	if x >= mapWidthPx-CliffDropCornerWidthPx && x <= mapWidthPx &&
		y >= mapHeightPx-CliffDropCornerHeightPx && y <= mapHeightPx {
		return true
	}
	return false
}
