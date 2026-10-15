//go:build !darwin && !linux && !windows

package power

func onBattery() bool { return false }

func LowerThreadPriority() {}
