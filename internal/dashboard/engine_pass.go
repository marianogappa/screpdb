//go:build !js

package dashboard

import (
	"path/filepath"

	"github.com/marianogappa/screpdb/internal/enginepass"
	"github.com/marianogappa/screpdb/internal/power"
)

const engineDigestCacheFile = "engine_digests.bin"

type enginePassRunner = *enginepass.Runner

// startEnginePass upgrades the library to engine data in the background. It does nothing more than
// log when there is no StarCraft: Remastered install: the fast pass alone keeps the dashboard working.
func (d *Dashboard) startEnginePass() {
	if !d.enginePassEnabled || d.library == nil || d.featureFlagEnabled(d.ctx, featureFlagDisableEnginePass) {
		return
	}
	d.enginePass = enginepass.New(enginepass.Options{
		Library:     d.library.lib,
		CachePath:   filepath.Join(d.library.root, engineDigestCacheFile),
		Paused:      d.enginePassPaused,
		LowPriority: true,
	})
	d.enginePass.Start()
}

func (d *Dashboard) enginePassPaused() bool {
	if d.featureFlagEnabled(d.ctx, featureFlagDisableEnginePass) {
		return true
	}
	return !d.featureFlagEnabled(d.ctx, featureFlagEnginePassOnBattery) && power.OnBattery()
}

func (d *Dashboard) focusEnginePass(replayID int64) {
	if d.enginePass != nil {
		d.enginePass.Focus(replayID)
	}
}

func (d *Dashboard) closeEnginePass() {
	if d.enginePass != nil {
		d.enginePass.Close()
	}
}
