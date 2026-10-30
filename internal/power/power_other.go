//go:build !darwin && !linux && !windows

package power

func starCraftRunning() bool { return false }

func LowerThreadPriority() {}
