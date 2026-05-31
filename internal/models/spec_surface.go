package models

import "sort"

// This file exposes deterministic, exported "spec surface" enumerators over the
// private golden-value tables in this package (techTable, upgradeTable,
// workerUnits, flyingUnits). They exist so the SPECIFICATION.md generator and
// the cross-consistency tests can read the real values without the tables
// having to be exported (the maps stay the single source of truth).

// TechSpecEntry is one row of the tech-research table: the Tech* name plus its
// static metadata.
type TechSpecEntry struct {
	Name string
	Meta TechMeta
}

// AllTechMeta returns every researched tech with its metadata, sorted by name.
func AllTechMeta() []TechSpecEntry {
	out := make([]TechSpecEntry, 0, len(techTable))
	for name, meta := range techTable {
		out = append(out, TechSpecEntry{Name: name, Meta: meta})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// UpgradeSpecEntry is one row of the upgrade table: the Upgrade* name plus its
// static metadata (including per-level costs/durations).
type UpgradeSpecEntry struct {
	Name string
	Meta UpgradeMeta
}

// AllUpgradeMeta returns every upgrade with its metadata, sorted by name.
func AllUpgradeMeta() []UpgradeSpecEntry {
	out := make([]UpgradeSpecEntry, 0, len(upgradeTable))
	for name, meta := range upgradeTable {
		out = append(out, UpgradeSpecEntry{Name: name, Meta: meta})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// WorkerUnitNames returns the canonical worker unit names (SCV/Probe/Drone),
// sorted.
func WorkerUnitNames() []string {
	out := make([]string, 0, len(workerUnits))
	for name := range workerUnits {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// FlyingUnitNames returns the canonical flying (non-transportable) unit names,
// sorted.
func FlyingUnitNames() []string {
	out := make([]string, 0, len(flyingUnits))
	for name := range flyingUnits {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
