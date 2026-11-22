package main

import (
	"fmt"
	"strings"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

const (
	// openingSeconds and openingBinds bound the opening sequence: the first binds of distinct keys.
	openingSeconds = 60
	openingBinds   = 3
)

// notTech are the buildings the hotkey stream files under tech that a player would not call tech: supply,
// gas, static defence and creep.
var notTech = map[string]bool{
	"Pylon": true, "Supply Depot": true, "Assimilator": true, "Refinery": true, "Extractor": true,
	"Photon Cannon": true, "Shield Battery": true, "Bunker": true, "Missile Turret": true,
	"Creep Colony": true, "Sunken Colony": true, "Spore Colony": true,
}

type gameTraits struct {
	Measures map[string]float64 `json:"measures"`
	// Opening is the game's first binds, e.g. "5:Nexus,1:units,2:units".
	Opening string `json:"opening"`
}

// extractTraits measures one game. cmds is the player's raw command stream, needed for Shift+number, which
// the hotkey stream folds into plain assigns.
func extractTraits(events []hotkeystream.Event, cmds []repcmd.Cmd, pid byte, secs int) gameTraits {
	rates, _ := hotkeystream.GameRates(events, secs)
	t := gameTraits{Measures: map[string]float64{
		"rebinds_min": rates.RebindsPerMinute,
		"presses_min": rates.PressesPerMinute,
	}}
	adds, assigns := 0, 0
	hotkeyCmds, allCmds, deselects := 0, 0, 0
	for _, c := range cmds {
		b := c.BaseCmd()
		if b.PlayerID != pid {
			continue
		}
		if b.Type.ID == repcmd.TypeIDSelectRemove || b.Type.ID == repcmd.TypeIDSelectRemove121 {
			deselects++
		}
		if b.Frame.Seconds() > hotkeystream.RateWindowSeconds {
			continue
		}
		allCmds++
		if h, ok := c.(*repcmd.HotkeyCmd); ok && h.Group <= 9 {
			hotkeyCmds++
		}
		if h, ok := c.(*repcmd.HotkeyCmd); ok && h.Group <= 9 {
			switch h.HotkeyType.ID {
			case repcmd.HotkeyTypeIDAdd:
				adds++
			case repcmd.HotkeyTypeIDAssign:
				assigns++
			}
		}
	}
	if adds+assigns > 0 {
		t.Measures["shift_add_share"] = float64(adds) / float64(adds+assigns)
	}
	if allCmds > 0 {
		t.Measures["hotkey_share"] = float64(hotkeyCmds) / float64(allCmds)
	}
	if secs > 0 {
		t.Measures["deselects_min"] = float64(deselects) / (float64(secs) / 60)
	}
	hallBinds, techBinds := 0, 0
	for _, e := range events {
		if e.Group > 9 || e.Type != hotkeystream.TypeAssignBuilding || e.Sec > hotkeystream.RateWindowSeconds {
			continue
		}
		name := hotkeystream.BuildingName(e.Building)
		switch cat := hotkeystream.CategoryOf(name); {
		case cat == hotkeystream.CategoryHall:
			hallBinds++
		case cat == hotkeystream.CategoryTech && !notTech[name]:
			techBinds++
		}
	}
	t.Measures["hall_binds"] = float64(hallBinds)
	t.Measures["tech_binds"] = float64(techBinds)
	var binds []string
	seen := map[byte]bool{}
	for _, e := range events {
		if e.Sec > openingSeconds || len(binds) == openingBinds {
			break
		}
		if e.Group > 9 || seen[e.Group] || (e.Type != hotkeystream.TypeAssignUnits && e.Type != hotkeystream.TypeAssignBuilding) {
			continue
		}
		seen[e.Group] = true
		what := "units"
		if e.Type == hotkeystream.TypeAssignBuilding {
			what = hotkeystream.BuildingName(e.Building)
		}
		binds = append(binds, fmt.Sprintf("%d:%s", e.Group, what))
	}
	t.Opening = strings.Join(binds, ",")
	return t
}
