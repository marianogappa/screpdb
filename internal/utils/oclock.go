package utils

import (
	"math"
)

// CalculateStartLocationOclock calculates the clock position for a start location
// based on the map dimensions and start location coordinates.
//
// The map is divided into 8 equal slices with clock positions: 11, 12, 1, 3, 5, 6, 7, 9
// where 11, 1, 5, 7 correspond to the rectangle's edges and 12, 3, 6, 9 correspond
// to the middle of each rectangle's lines.
//
// Parameters:
//   - tileX, tileY: Map dimensions in tiles
//   - startLocationX, startLocationY: Start location coordinates in pixels
//
// Returns one of: 11, 12, 1, 3, 5, 6, 7, 9
func CalculateStartLocationOclock(tileX, tileY, startLocationX, startLocationY int) int {
	// Convert tile dimensions to pixel dimensions
	mapWidth := tileX * 32
	mapHeight := tileY * 32

	// Calculate the center of the map
	centerX := float64(mapWidth) / 2.0
	centerY := float64(mapHeight) / 2.0

	// Convert start location to relative coordinates from center
	relX := float64(startLocationX) - centerX
	relY := float64(startLocationY) - centerY

	// Calculate angle in radians (0 to 2π, with 0 being right/east)
	// Note: Y axis is flipped in StarCraft (0,0 is top-left), so we use relY directly
	angle := math.Atan2(relY, relX)

	// Convert to degrees and normalize to 0-360 range
	angleDegrees := angle * 180.0 / math.Pi
	if angleDegrees < 0 {
		angleDegrees += 360
	}

	// Map the angle to clock positions
	// The 8 slices are: 11, 12, 1, 3, 5, 6, 7, 9
	// Each slice covers 45 degrees (360/8 = 45)
	//
	// Corrected clock positions and their angle ranges (adjusted for StarCraft coordinate system):
	// 3:  337.5° - 22.5° (east, 0°)
	// 5:  22.5° - 67.5°  (southeast, 45°)
	// 6:  67.5° - 112.5° (south, 90°)
	// 7:  112.5° - 157.5° (southwest, 135°)
	// 9:  157.5° - 202.5° (west, 180°)
	// 11: 202.5° - 247.5° (northwest, 225°)
	// 12: 247.5° - 292.5° (north, 270°)
	// 1:  292.5° - 337.5° (northeast, 315°)

	switch {
	case angleDegrees >= 337.5 || angleDegrees < 22.5:
		return 3 // East (0°)
	case angleDegrees >= 22.5 && angleDegrees < 67.5:
		return 5 // Southeast (45°)
	case angleDegrees >= 67.5 && angleDegrees < 112.5:
		return 6 // South (90°)
	case angleDegrees >= 112.5 && angleDegrees < 157.5:
		return 7 // Southwest (135°)
	case angleDegrees >= 157.5 && angleDegrees < 202.5:
		return 9 // West (180°)
	case angleDegrees >= 202.5 && angleDegrees < 247.5:
		return 11 // Northwest (225°)
	case angleDegrees >= 247.5 && angleDegrees < 292.5:
		return 12 // North (270°)
	case angleDegrees >= 292.5 && angleDegrees < 337.5:
		return 1 // Northeast (315°)
	default:
		return 3 // Default fallback (should never reach here)
	}
}
