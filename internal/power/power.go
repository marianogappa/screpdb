// Package power answers the two questions a background job needs to be a good citizen: is the machine on
// battery, and how to run the current OS thread at low priority.
package power

import (
	"sync"
	"time"
)

const batteryRecheck = 30 * time.Second

var battery struct {
	mu      sync.Mutex
	checked time.Time
	on      bool
}

// OnBattery reports whether the machine runs on battery, checked at most every 30 s. It answers false
// when the platform cannot tell, so a desktop never pauses by mistake.
func OnBattery() bool {
	battery.mu.Lock()
	defer battery.mu.Unlock()
	if time.Since(battery.checked) >= batteryRecheck {
		battery.on = onBattery()
		battery.checked = time.Now()
	}
	return battery.on
}
