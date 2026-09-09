package winsandbox

import "testing"

func TestValidSeeReplayDestName(t *testing.T) {
	for _, name := range []string{"", SeeReplayDefaultName, SeeReplayCompleteName} {
		if !validSeeReplayDestName(name) {
			t.Errorf("%q rejected", name)
		}
	}
	for _, name := range []string{"other.rep", "../watch_me.rep", `..\watch_me.rep`, "watch_me.rep.exe"} {
		if validSeeReplayDestName(name) {
			t.Errorf("%q accepted", name)
		}
	}
}

func TestValidPlaceReplayDestName(t *testing.T) {
	if !validPlaceReplayDestName("bgh-complete.rep") {
		t.Error("valid name rejected")
	}
	for _, name := range []string{
		"",
		"-complete.rep",
		"bgh.rep",
		"sub/bgh-complete.rep",
		`sub\bgh-complete.rep`,
		"..-complete.rep",
		"../bgh-complete.rep",
	} {
		if validPlaceReplayDestName(name) {
			t.Errorf("%q accepted", name)
		}
	}
}
