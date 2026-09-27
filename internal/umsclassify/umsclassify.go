package umsclassify

import "encoding/binary"

const (
	triggerSize   = 2400
	conditionSize = 20
	conditionsN   = 16
	actionSize    = 32
	actionsN      = 64
	actionsOffset = conditionsN * conditionSize // 320

	unitEntrySize       = 36
	unitIDOffset        = 8
	unitOwnerOffset     = 16
	startLocationUnitID = 214
)

const (
	actionVictory         = 1
	actionDefeat          = 2
	actionPreserveTrigger = 3
	actionWait            = 4
	actionPlayWAV         = 8
	actionDisplayText     = 9
	actionRunAIScript     = 15
	actionSetResources    = 26
	actionComment         = 47
	actionSetAlliance     = 57
)

var sharedVisionScripts = map[[4]byte]bool{
	{'+', 'V', 'i', '0'}: true, {'+', 'V', 'i', '1'}: true,
	{'+', 'V', 'i', '2'}: true, {'+', 'V', 'i', '3'}: true,
	{'+', 'V', 'i', '4'}: true, {'+', 'V', 'i', '5'}: true,
	{'+', 'V', 'i', '6'}: true, {'+', 'V', 'i', '7'}: true,
	{'-', 'V', 'i', '0'}: true, {'-', 'V', 'i', '1'}: true,
	{'-', 'V', 'i', '2'}: true, {'-', 'V', 'i', '3'}: true,
	{'-', 'V', 'i', '4'}: true, {'-', 'V', 'i', '5'}: true,
	{'-', 'V', 'i', '6'}: true, {'-', 'V', 'i', '7'}: true,
}

type Result struct {
	MeleeLike bool
	Reason    string
}

func Classify(chk []byte) Result {
	trig := extractSection(chk, [4]byte{'T', 'R', 'I', 'G'})
	if trig == nil {
		return Result{true, "no_triggers"}
	}
	if r := checkTriggers(trig); !r.MeleeLike {
		return r
	}

	units := extractSection(chk, [4]byte{'U', 'N', 'I', 'T'})
	if r := checkUnits(units); !r.MeleeLike {
		return r
	}

	return Result{true, "melee_like"}
}

func checkTriggers(trig []byte) Result {
	n := len(trig) / triggerSize
	for i := range n {
		base := i * triggerSize
		for j := range actionsN {
			off := base + actionsOffset + j*actionSize
			if off+actionSize > len(trig) {
				break
			}
			action := trig[off : off+actionSize]
			actionType := action[26]
			if actionType == 0 {
				continue
			}

			switch actionType {
			case actionVictory, actionDefeat, actionPreserveTrigger,
				actionWait, actionPlayWAV, actionDisplayText,
				actionComment, actionSetAlliance:
				continue

			case actionRunAIScript:
				var script [4]byte
				copy(script[:], action[20:24])
				if !sharedVisionScripts[script] {
					return Result{false, "forbidden_ai_script"}
				}

			case actionSetResources:
				amount := binary.LittleEndian.Uint32(action[20:24])
				if amount != 50 && amount < 1000 {
					return Result{false, "forbidden_set_resources"}
				}

			default:
				return Result{false, "forbidden_action"}
			}
		}
	}
	return Result{true, ""}
}

func checkUnits(units []byte) Result {
	if units == nil {
		return Result{true, "no_units"}
	}
	n := len(units) / unitEntrySize
	for i := range n {
		off := i * unitEntrySize
		if off+unitEntrySize > len(units) {
			break
		}
		entry := units[off : off+unitEntrySize]
		owner := entry[unitOwnerOffset]
		unitID := binary.LittleEndian.Uint16(entry[unitIDOffset : unitIDOffset+2])
		if owner < 8 && unitID != startLocationUnitID {
			return Result{false, "player_owned_unit"}
		}
	}
	return Result{true, ""}
}

func extractSection(chk []byte, name [4]byte) []byte {
	var out []byte
	i := 0
	for i+8 <= len(chk) {
		sName := chk[i : i+4]
		sSize := int(int32(binary.LittleEndian.Uint32(chk[i+4 : i+8])))
		i += 8
		if sSize < 0 || i+sSize > len(chk) {
			break
		}
		if sName[0] == name[0] && sName[1] == name[1] && sName[2] == name[2] && sName[3] == name[3] {
			out = append(out, chk[i:i+sSize]...)
		}
		i += sSize
	}
	return out
}
