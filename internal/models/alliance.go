package models

// AllianceSnapshot is one observed team topology, valid from Sec until the
// next snapshot's Sec (or game end for the last one). Teams hold replay
// player IDs sorted ascending; teams are ordered by their smallest ID.
type AllianceSnapshot struct {
	Sec      int
	Teams    [][]byte
	Stacking bool
}
