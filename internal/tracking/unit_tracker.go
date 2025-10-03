package tracking

import (
	"sync"
	"time"

	"github.com/icza/screp/rep/repcmd"
)

// UnitInfo represents information about a unit instance
type UnitInfo struct {
	UnitTag      uint16
	UnitType     string // "Marine", "Zergling", etc.
	UnitID       uint16 // The unit type ID
	PlayerID     int64
	CreatedAt    time.Time
	CreatedFrame int32
	X, Y         int
	IsAlive      bool
}

// UnitTracker maintains a mapping of UnitTag -> UnitInfo during replay parsing
type UnitTracker struct {
	units map[uint16]*UnitInfo
	mutex sync.RWMutex
}

// NewUnitTracker creates a new unit tracker
func NewUnitTracker() *UnitTracker {
	return &UnitTracker{
		units: make(map[uint16]*UnitInfo),
	}
}

// AddUnit adds a new unit to the tracker
func (ut *UnitTracker) AddUnit(unitTag uint16, unitType string, unitID uint16, playerID int64, frame int32, time time.Time, x, y int) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	ut.units[unitTag] = &UnitInfo{
		UnitTag:      unitTag,
		UnitType:     unitType,
		UnitID:       unitID,
		PlayerID:     playerID,
		CreatedAt:    time,
		CreatedFrame: frame,
		X:            x,
		Y:            y,
		IsAlive:      true,
	}
}

// RemoveUnit marks a unit as destroyed
func (ut *UnitTracker) RemoveUnit(unitTag uint16) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	if unit, exists := ut.units[unitTag]; exists {
		unit.IsAlive = false
	}
}

// GetUnitInfo retrieves unit information by tag
func (ut *UnitTracker) GetUnitInfo(unitTag uint16) (*UnitInfo, bool) {
	ut.mutex.RLock()
	defer ut.mutex.RUnlock()

	unit, exists := ut.units[unitTag]
	return unit, exists
}

// GetUnitTypesForTags returns a map of UnitTag -> UnitType for the given tags
func (ut *UnitTracker) GetUnitTypesForTags(unitTags []repcmd.UnitTag) map[uint16]string {
	ut.mutex.RLock()
	defer ut.mutex.RUnlock()

	result := make(map[uint16]string)
	for _, tag := range unitTags {
		if unit, exists := ut.units[uint16(tag)]; exists && unit.IsAlive {
			result[uint16(tag)] = unit.UnitType
		}
	}
	return result
}

// UpdateUnitPosition updates a unit's position
func (ut *UnitTracker) UpdateUnitPosition(unitTag uint16, x, y int) {
	ut.mutex.Lock()
	defer ut.mutex.Unlock()

	if unit, exists := ut.units[unitTag]; exists && unit.IsAlive {
		unit.X = x
		unit.Y = y
	}
}

// GetAllUnits returns all tracked units
func (ut *UnitTracker) GetAllUnits() map[uint16]*UnitInfo {
	ut.mutex.RLock()
	defer ut.mutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[uint16]*UnitInfo)
	for tag, unit := range ut.units {
		result[tag] = unit
	}
	return result
}

// ProcessCommand updates the unit tracker based on command type and returns resolved UnitInfos
func (ut *UnitTracker) ProcessCommand(cmd repcmd.Cmd, playerID int64, frame int32, time time.Time) []*UnitInfo {
	var resolvedUnits []*UnitInfo

	switch c := cmd.(type) {
	case *repcmd.RightClickCmd:
		// RightClickCmd has both UnitTag and Unit - this is a mapping we can store
		if c.UnitTag.Valid() && c.Unit != nil {
			// Resolve unit name from ID if Unit.Name is "None" or empty
			unitName := c.Unit.Name
			if unitName == "None" || unitName == "" {
				unitName = repcmd.UnitByID(c.Unit.ID).Name
			}

			ut.AddUnit(uint16(c.UnitTag), unitName, c.Unit.ID, playerID, frame, time, int(c.Pos.X), int(c.Pos.Y))
			if unitInfo, exists := ut.GetUnitInfo(uint16(c.UnitTag)); exists {
				resolvedUnits = append(resolvedUnits, unitInfo)
			}
		}

	case *repcmd.TargetedOrderCmd:
		// TargetedOrderCmd has both UnitTag and Unit - this is a mapping we can store
		if c.UnitTag.Valid() && c.Unit != nil {
			// Resolve unit name from ID if Unit.Name is "None" or empty
			unitName := c.Unit.Name
			if unitName == "None" || unitName == "" {
				unitName = repcmd.UnitByID(c.Unit.ID).Name
			}

			ut.AddUnit(uint16(c.UnitTag), unitName, c.Unit.ID, playerID, frame, time, 0, 0) // Position not available
			if unitInfo, exists := ut.GetUnitInfo(uint16(c.UnitTag)); exists {
				resolvedUnits = append(resolvedUnits, unitInfo)
			}
		}

	case *repcmd.SelectCmd:
		// SelectCmd has UnitTags but no Unit - try to resolve them
		for _, tag := range c.UnitTags {
			if unitInfo, exists := ut.GetUnitInfo(uint16(tag)); exists && unitInfo.IsAlive {
				resolvedUnits = append(resolvedUnits, unitInfo)
			}
		}

	case *repcmd.CancelTrainCmd:
		// CancelTrainCmd has UnitTag but no Unit - try to resolve it
		if c.UnitTag.Valid() {
			if unitInfo, exists := ut.GetUnitInfo(uint16(c.UnitTag)); exists {
				resolvedUnits = append(resolvedUnits, unitInfo)
				ut.RemoveUnit(uint16(c.UnitTag))
			}
		}

	case *repcmd.UnloadCmd:
		// UnloadCmd has UnitTag but no Unit - try to resolve it
		if c.UnitTag.Valid() {
			if unitInfo, exists := ut.GetUnitInfo(uint16(c.UnitTag)); exists {
				resolvedUnits = append(resolvedUnits, unitInfo)
			}
		}

	case *repcmd.TrainCmd:
		// TrainCmd has Unit but no UnitTag - return the unit information directly
		if c.Unit != nil {
			resolvedUnits = append(resolvedUnits, &UnitInfo{
				UnitTag:      0, // No UnitTag available
				UnitType:     c.Unit.Name,
				UnitID:       c.Unit.ID,
				PlayerID:     playerID,
				CreatedAt:    time,
				CreatedFrame: frame,
				X:            0, // Position not available
				Y:            0,
				IsAlive:      true,
			})
		}

	case *repcmd.BuildCmd:
		// BuildCmd has Unit but no UnitTag - return the unit information directly
		if c.Unit != nil {
			resolvedUnits = append(resolvedUnits, &UnitInfo{
				UnitTag:      0, // No UnitTag available
				UnitType:     c.Unit.Name,
				UnitID:       c.Unit.ID,
				PlayerID:     playerID,
				CreatedAt:    time,
				CreatedFrame: frame,
				X:            int(c.Pos.X), // Position available for buildings
				Y:            int(c.Pos.Y),
				IsAlive:      true,
			})
		}

	case *repcmd.BuildingMorphCmd:
		// BuildingMorphCmd has Unit but no UnitTag - return the unit information directly
		if c.Unit != nil {
			resolvedUnits = append(resolvedUnits, &UnitInfo{
				UnitTag:      0, // No UnitTag available
				UnitType:     c.Unit.Name,
				UnitID:       c.Unit.ID,
				PlayerID:     playerID,
				CreatedAt:    time,
				CreatedFrame: frame,
				X:            0, // Position not available
				Y:            0,
				IsAlive:      true,
			})
		}
	}

	return resolvedUnits
}

// ExtractUnitTagFromCommand attempts to extract a UnitTag from various command types
func ExtractUnitTagFromCommand(cmd repcmd.Cmd) (uint16, bool) {
	switch c := cmd.(type) {
	case *repcmd.RightClickCmd:
		if c.UnitTag.Valid() {
			return uint16(c.UnitTag), true
		}
	case *repcmd.CancelTrainCmd:
		if c.UnitTag.Valid() {
			return uint16(c.UnitTag), true
		}
	case *repcmd.UnloadCmd:
		if c.UnitTag.Valid() {
			return uint16(c.UnitTag), true
		}
	}
	return 0, false
}
