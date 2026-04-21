package worldstate

import "testing"

func TestIsCommitmentBuild_TownHallAndProduction(t *testing.T) {
	if !IsCommitmentBuild("Command Center") {
		t.Fatal("command center")
	}
	if !IsCommitmentBuild("Barracks") {
		t.Fatal("barracks")
	}
	if !IsCommitmentBuild("Cybernetics Core") {
		t.Fatal("cyber core")
	}
}

func TestIsAmbiguousAggressiveDefenseBuild(t *testing.T) {
	if !IsAmbiguousAggressiveDefenseBuild("Photon Cannon") {
		t.Fatal("cannon")
	}
	if !IsAmbiguousAggressiveDefenseBuild("Bunker") {
		t.Fatal("bunker")
	}
}
