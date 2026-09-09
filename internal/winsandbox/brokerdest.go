package winsandbox

import "strings"

// See-replay destination filenames. The pair lets the user stage their own
// copy and the downloaded complete copy of one game side by side (issue #341).
const (
	SeeReplayDefaultName  = "watch_me.rep"
	SeeReplayCompleteName = "watch_me_complete.rep"
)

// validSeeReplayDestName keeps the see-replay destination fixed: a compromised
// worker can only ever name one of the two known staging files.
func validSeeReplayDestName(name string) bool {
	return name == "" || name == SeeReplayDefaultName || name == SeeReplayCompleteName
}

// validPlaceReplayDestName constrains what the place-replay broker request may
// create in the user's replay folder: a bare basename of a downloaded complete
// copy, nothing else.
func validPlaceReplayDestName(name string) bool {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false
	}
	return strings.HasSuffix(name, "-complete.rep") && len(name) > len("-complete.rep")
}
