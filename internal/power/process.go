//go:build darwin || windows

package power

import "strings"

func isStarCraftProcess(name string) bool {
	return strings.EqualFold(name, "StarCraft") || strings.EqualFold(name, "StarCraft.exe")
}
