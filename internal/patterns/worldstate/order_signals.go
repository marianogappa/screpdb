package worldstate

import (
	"strings"

	"github.com/icza/screp/rep/repcmd"
)

// Order attack tiers for replay narrative. Targeted combat and posture orders (attack-move,
// patrol, hold, etc.) all count as strong opening pressure; only right-click is sustain-only.

type orderAttackClass int

const (
	orderAttackNone orderAttackClass = iota
	orderAttackWeak
	orderAttackStrong
)

func normalizeActionKey(s string) string {
	x := strings.ToLower(s)
	x = strings.ReplaceAll(x, " ", "")
	x = strings.ReplaceAll(x, "_", "")
	return x
}

// attackOpeningPressure is true for commands that count toward starting an attack/scout pressure
// range at an enemy base. Right-click alone never counts (use attackSustainAfterOpen for that).
func attackOpeningPressure(actionType string, orderID *byte, orderName string) bool {
	n := normalizeActionKey(actionType)
	switch n {
	case "rightclick":
		return false
	case "targetedorder":
		return classifyTargetedAttackOrder(orderID, orderName) != orderAttackNone
	default:
		return false
	}
}

// attackSustainAfterOpen extends an open attack range: opening-class commands or any right-click
// with coordinates (caller ensures polygon is enemy territory).
func attackSustainAfterOpen(actionType string, orderID *byte, orderName string) bool {
	if attackOpeningPressure(actionType, orderID, orderName) {
		return true
	}
	return normalizeActionKey(actionType) == "rightclick"
}

// enemyBasePressureForZergRush counts targeted aggression at an enemy base (not right-click).
func enemyBasePressureForZergRush(actionType string, orderID *byte, orderName string) bool {
	return attackOpeningPressure(actionType, orderID, orderName)
}

func classifyTargetedAttackOrder(orderID *byte, orderName string) orderAttackClass {
	if orderID != nil {
		if c := classifyAttackByOrderID(*orderID); c != orderAttackNone {
			return c
		}
	}
	return classifyAttackByOrderName(orderName)
}

func classifyAttackByOrderID(id byte) orderAttackClass {
	if repcmd.IsOrderIDKindAttack(id) {
		return orderAttackStrong
	}

	switch id {
	case repcmd.OrderIDHoldPosition,
		repcmd.OrderIDQueenHoldPosition,
		repcmd.OrderIDMedicHoldPosition,
		repcmd.OrderIDCarrierHoldPosition,
		repcmd.OrderIDReaverHoldPosition,
		0x98,                   // Patrol
		0x9e,                   // HarassMove
		0x9d,                   // AtkMoveEP
		0x36, 0x3c, 0x38, 0x3d: // CarrierMoveToAttack, ReaverMoveToAttack, CarrierFight, ReaverFight
		return orderAttackStrong
	}

	switch id {
	case repcmd.OrderIDAttackTile,
		0x62,       // Sieging
		0x71, 0x72, // FireYamatoGun, MoveToFireYamatoGun
		repcmd.OrderIDNukeLaunch,
		0x7e, 0x80, 0x7f, 0x81, // NukePaint, CastNuclearStrike, NukeUnit, NukeTrack
		0x41, 0x40, // ScarabAttack, InterceptorAttack
		0x13, 0x16, // TowerAttack, TurretAttack
		0x14,             // VultureMine
		0x86, 0x87, 0x88: // SuicideUnit, SuicideLocation, SuicideHoldPosition
		return orderAttackStrong
	}

	if id == 0xb7 { // DarkArchonMeld
		return orderAttackNone
	}

	switch id {
	case 0x1b, // CastInfestation
		0x73,                   // CastLockdown
		0x77, 0x78, 0x79, 0x7a, // CastDarkSwarm, Parasite, SpawnBroodlings, EMP
		0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, // PsionicStorm, Irradiate, Plague, Consume, Ensnare, StasisField
		0xb5, 0xb6, 0xb8, 0xb9, 0xba: // DisruptionWeb, MindControl, Feedback, OpticalFlare, Maelstrom
		return orderAttackStrong
	case repcmd.OrderIDCastScannerSweep,
		0x8d, 0xb4, 0x94, 0x95, // DefensiveMatrix, Restoration, Hallucination, Hallucination2
		0x83, // CloakNearbyUnits
		repcmd.OrderIDCastRecall:
		return orderAttackNone
	}

	return orderAttackNone
}

func classifyAttackByOrderName(orderName string) orderAttackClass {
	o := normalize(orderName)
	if o == "" {
		return orderAttackNone
	}
	if strings.Contains(o, "attackmove") || strings.Contains(o, "patrol") ||
		strings.Contains(o, "holdposition") || strings.Contains(o, "harassmove") {
		return orderAttackStrong
	}
	if strings.Contains(o, "attack") {
		return orderAttackStrong
	}
	if strings.Contains(o, "psionicstorm") || strings.Contains(o, "sieging") ||
		strings.Contains(o, "fireyamato") || strings.Contains(o, "nukelaunch") ||
		strings.Contains(o, "castnuclear") {
		return orderAttackStrong
	}
	if strings.HasPrefix(o, "cast") {
		if strings.Contains(o, "scanner") || strings.Contains(o, "defensivematrix") ||
			strings.Contains(o, "restoration") || strings.Contains(o, "hallucination") ||
			strings.Contains(o, "recall") {
			return orderAttackNone
		}
		return orderAttackStrong
	}
	return orderAttackNone
}
