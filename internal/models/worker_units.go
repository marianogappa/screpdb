package models

var workerUnits = map[string]struct{}{
	GeneralUnitSCV:   {},
	GeneralUnitProbe: {},
	GeneralUnitDrone: {},
}

// IsWorker reports whether a unit type is a worker (SCV/Probe/Drone). The
// drop classifier excludes workers because workers trained inside the drop
// window are typically unrelated to the unload — and including them dilutes
// the reaver_drop subtype routing.
func IsWorker(name string) bool {
	_, ok := workerUnits[name]
	return ok
}
