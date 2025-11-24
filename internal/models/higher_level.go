package models

const (
	ActionTypeUnitMorph = "Unit Morph"
	ActionTypeTrain     = "Train"
	ActionTypeBuild     = "Build"

	UnitNameDrone         = "Drone"
	UnitNameProbe         = "Probe"
	UnitNameSCV           = "SCV"
	UnitNameOverlord      = "Overlord"
	UnitNameHatchery      = "Hatchery"
	UnitNameNexus         = "Nexus"
	UnitNameCommandCenter = "Command Center"
)

func (c *Command) IsUnitBuild() bool {
	return c.ActionType == ActionTypeUnitMorph || c.ActionType == ActionTypeTrain
}

func (c *Command) UnitBuildName() string {
	if !c.IsUnitBuild() || c.UnitType == nil {
		return ""
	}
	return *c.UnitType
}

func (c *Command) IsAttackingUnitBuild() bool {
	if !c.IsUnitBuild() {
		return false
	}
	name := c.UnitBuildName()
	return !isWorkerUnitBuild(name) && name != UnitNameOverlord
}

func isWorkerUnitBuild(name string) bool {
	return name == UnitNameDrone || name == UnitNameProbe || name == UnitNameSCV
}

func (c *Command) IsUpgrade() bool {
	return c.UpgradeName != nil
}

func (c *Command) GetUpgradeName() string {
	if !c.IsUpgrade() {
		return ""
	}
	return *c.UpgradeName
}

func (c *Command) IsBaseBuild() bool {
	return c.ActionType == ActionTypeBuild &&
		c.UnitType != nil && (*c.UnitType == UnitNameHatchery ||
		*c.UnitType == UnitNameNexus ||
		*c.UnitType == UnitNameCommandCenter)
}
