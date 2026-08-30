package dashboard

import (
	"context"
	"log"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
)

const bnetProbeInterval = 30 * time.Second

type bnetStatus struct {
	State          bnetfacade.BridgeState `json:"state"`
	Addr           string                 `json:"addr"`
	Disabled       bool                   `json:"disabled"`
	RequestsToday  int                    `json:"requests_today"`
	DailyCap       int                    `json:"daily_cap"`
	DownloadsToday int                    `json:"downloads_today"`
	CooldownUntil  string                 `json:"cooldown_until,omitempty"`
	Gateway        int                    `json:"gateway,omitempty"`
	GatewayName    string                 `json:"gateway_name,omitempty"`
}

func (d *Dashboard) startBnetMonitor(ctx context.Context) {
	go func() {
		defer crashreport.GuardNonFatal(nil)
		if err := bnetfacade.EnableBudgetPersistence(); err != nil {
			log.Printf("bnet budget persistence disabled: %v", err)
		}
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

	state, gateway := bnetfacade.ProbeGateway(ctx, addrStr)
	if gateway > 0 {
		d.bnetGateway.Store(int64(gateway))
	}
	if state == bnetfacade.BridgeNotRunning {
		d.bnetAddr.Store("")
	}
	d.bnetState.Store(bnetStatus{State: state, Addr: addrStr})
}

func (d *Dashboard) getBnetStatus() bnetStatus {
	s := bnetStatus{State: bnetfacade.BridgeNotRunning}
	if v := d.bnetState.Load(); v != nil {
		s = v.(bnetStatus)
	}
	budget := bnetfacade.BudgetSnapshot()
	s.RequestsToday = budget.BridgeUsedToday
	s.DailyCap = budget.BridgeDailyCap
	s.DownloadsToday = budget.DownloadsUsedToday
	if !budget.CooldownUntil.IsZero() {
		s.CooldownUntil = budget.CooldownUntil.Format(time.RFC3339)
	}
	if gw := int(d.bnetGateway.Load()); gw > 0 {
		s.Gateway = gw
		s.GatewayName = bnetfacade.GatewayNames[gw]
	}
	return s
}

func (d *Dashboard) setBnetDisabled(disabled bool) {
	d.bnetDisabled.Store(disabled)
	if disabled {
		d.bnetAddr.Store("")
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning, Disabled: true})
	}
}
