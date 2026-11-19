//go:build js

package dashboard

// The browser demo has no StarCraft install to read game data from, so it runs on the fast pass only.
type enginePassHolder struct{}

func (d *Dashboard) startEnginePass() {}

func (d *Dashboard) focusEnginePass(int64) {}

func (d *Dashboard) enginePending(int64) bool { return false }

func (d *Dashboard) engineUnavailable(int64) bool { return false }

func (d *Dashboard) engineUpgrading(int64) bool { return false }

func (d *Dashboard) completeFallback(int64) (int64, bool) { return 0, false }

func (d *Dashboard) closeEnginePass() {}
