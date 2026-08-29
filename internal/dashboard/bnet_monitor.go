package dashboard

import (
	"context"
	"log"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
)

const bnetProbeInterval = 10 * time.Second

type bnetStatus struct {
	State    bnetfacade.BridgeState `json:"state"`
	Addr     string                 `json:"addr"`
	Disabled bool                   `json:"disabled"`
}

func (d *Dashboard) startBnetMonitor(ctx context.Context) {
	go func() {
		defer crashreport.GuardNonFatal(nil)
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})
		ticker := time.NewTicker(bnetProbeInterval)
		defer ticker.Stop()
		// Run one probe immediately, then on each tick.
		d.probeBnet(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.probeBnet(ctx)
			}
		}
	}()
}

func (d *Dashboard) probeBnet(ctx context.Context) {
	if d.bnetDisabled.Load() {
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning, Disabled: true})
		return
	}

	addr := d.bnetAddr.Load()
	addrStr, _ := addr.(string)

	if addrStr == "" {
		discovered, err := bnetfacade.DiscoverBridgeAddr(ctx)
		if err != nil {
			d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})
			return
		}
		addrStr = discovered
		d.bnetAddr.Store(addrStr)
		log.Printf("SC:R bridge discovered at %s", addrStr)
	}

	state := bnetfacade.ProbeBridge(ctx, addrStr)
	if state == bnetfacade.BridgeNotRunning {
		d.bnetAddr.Store("")
	}
	d.bnetState.Store(bnetStatus{State: state, Addr: addrStr})
}

func (d *Dashboard) getBnetStatus() bnetStatus {
	v := d.bnetState.Load()
	if v == nil {
		return bnetStatus{State: bnetfacade.BridgeNotRunning}
	}
	return v.(bnetStatus)
}

func (d *Dashboard) setBnetDisabled(disabled bool) {
	d.bnetDisabled.Store(disabled)
	if disabled {
		d.bnetAddr.Store("")
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning, Disabled: true})
	}
}

