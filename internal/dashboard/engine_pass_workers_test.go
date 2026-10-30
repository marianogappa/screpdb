//go:build !js

package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/enginepass"
)

func TestEnginePassWorkersFollowTheSettingAndStarCraft(t *testing.T) {
	d := newTestDashboard(t)
	scRunning := false
	d.enginePass.starCraftRunning = func() bool { return scRunning }

	if got := d.enginePassWorkers(); got != enginepass.DefaultWorkers() {
		t.Fatalf("setting off: %d workers, want %d", got, enginepass.DefaultWorkers())
	}
	if _, err := d.setFeatureFlag(d.ctx, featureFlagEnginePassAllCores, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = d.setFeatureFlag(d.ctx, featureFlagEnginePassAllCores, false) })
	if got := d.enginePassWorkers(); got != enginepass.AllWorkers() {
		t.Fatalf("setting on, StarCraft closed: %d workers, want %d", got, enginepass.AllWorkers())
	}
	scRunning = true
	if got := d.enginePassWorkers(); got != enginepass.DefaultWorkers() {
		t.Fatalf("setting on, StarCraft running: %d workers, want %d", got, enginepass.DefaultWorkers())
	}
}

func TestTheBatterySettingIsGone(t *testing.T) {
	d := newTestDashboard(t)
	if _, err := d.setFeatureFlag(d.ctx, "engine_pass_on_battery", true); err == nil {
		t.Fatal("the retired battery flag is still settable")
	}
}
