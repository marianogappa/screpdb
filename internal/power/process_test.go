//go:build darwin || windows

package power

import "testing"

func TestIsStarCraftProcess(t *testing.T) {
	for name, want := range map[string]bool{
		"StarCraft":        true,
		"StarCraft.exe":    true,
		"starcraft.EXE":    true,
		"StarCraft II":     false,
		"SC2_x64.exe":      false,
		"Battle.net.exe":   false,
		"StarCraft Helper": false,
		"":                 false,
	} {
		if got := isStarCraftProcess(name); got != want {
			t.Errorf("isStarCraftProcess(%q) = %v, want %v", name, got, want)
		}
	}
}
