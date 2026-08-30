package dashboard

import (
	"context"
	"log"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
)

// The monitor watches the bridge's *port* on a fast tick and only spends an
// HTTP probe when something actually changes. A loopback dial costs ~100µs,
// sends zero bytes and is unmetered, so watching at this cadence is free — it
// is the HTTP probes that make SC:R talk to Battle.net and that the #319
// budgets meter, and this scheme issues strictly fewer of them than the old
// unconditional 30s poll while reacting roughly an order of magnitude faster.
//
//	bnetPortPollInterval  how often we dial a port we already know about
//	bnetProbeInterval     slow HTTP re-probe, to catch auth-state changes
//	                      (logged out, went offline) that leave the port open
//	bnetDiscoveryInterval how often we re-enumerate every loopback port, the
//	                      one genuinely expensive operation (on macOS it spawns
//	                      lsof), used only when no candidate port is known
const (
	bnetPortPollInterval  = 2 * time.Second
	bnetProbeInterval     = 60 * time.Second
	bnetDiscoveryInterval = 20 * time.Second
)

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

// bnetMonitor holds the monitor loop's own state. Only the loop goroutine
// touches these, so they need no synchronisation; the state it publishes for
// readers goes through d.bnetState.
type bnetMonitor struct {
	// candidateAddr is the last address the bridge was ever seen on. It
	// outlives the connection: SC:R commonly rebinds the same port across
	// restarts, so re-dialling the remembered one costs microseconds and
	// usually catches a restart on the next tick instead of waiting for the
	// next full discovery sweep. When it is wrong we simply fall through to
	// discovery, so remembering it can only help.
	candidateAddr string
	portWasOpen   bool
	lastProbe     time.Time
	lastDiscovery time.Time
}

func (d *Dashboard) startBnetMonitor(ctx context.Context) {
	go func() {
		defer crashreport.GuardNonFatal(nil)
		if err := bnetfacade.EnableBudgetPersistence(); err != nil {
			log.Printf("bnet budget persistence disabled: %v", err)
		}
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})

		m := &bnetMonitor{}
		ticker := time.NewTicker(bnetPortPollInterval)
		defer ticker.Stop()
		m.tick(ctx, d)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.tick(ctx, d)
			}
		}
	}()
}

func (m *bnetMonitor) tick(ctx context.Context, d *Dashboard) {
	if d.bnetDisabled.Load() {
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning, Disabled: true})
		m.reset()
		return
	}

	if m.candidateAddr == "" {
		m.discover(ctx, d)
		return
	}

	portOpen := bnetfacade.BridgePortOpen(ctx, m.candidateAddr)

	// The port vanishing is proof the bridge is gone — publish it immediately
	// rather than paying an HTTP probe to be told the same thing.
	if !portOpen {
		if m.portWasOpen || d.currentBnetState() != bnetfacade.BridgeNotRunning {
			log.Printf("SC:R bridge port %s closed", m.candidateAddr)
		}
		m.portWasOpen = false
		d.bnetAddr.Store("")
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})
		if time.Since(m.lastDiscovery) >= bnetDiscoveryInterval {
			m.discover(ctx, d)
		}
		return
	}

	// The port is open. Spend an HTTP probe when the port just came back, or
	// on the slow interval to catch state that changes without the port moving
	// (the user logging out drops us to BridgeOffline while the port stays up).
	justOpened := !m.portWasOpen
	m.portWasOpen = true
	if justOpened || time.Since(m.lastProbe) >= bnetProbeInterval {
		m.probe(ctx, d, m.candidateAddr)
	}
}

// discover enumerates every loopback port and probes each for the bridge. It is
// the expensive path — on macOS it spawns lsof — so it is rate-limited to
// bnetDiscoveryInterval and only runs when no candidate address is known.
func (m *bnetMonitor) discover(ctx context.Context, d *Dashboard) {
	if time.Since(m.lastDiscovery) < bnetDiscoveryInterval && !m.lastDiscovery.IsZero() {
		return
	}
	m.lastDiscovery = time.Now()
	addr, err := bnetfacade.DiscoverBridgeAddr(ctx)
	if err != nil {
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})
		return
	}
	log.Printf("SC:R bridge discovered at %s", addr)
	m.candidateAddr = addr
	m.portWasOpen = true
	m.probe(ctx, d, addr)
}

func (m *bnetMonitor) probe(ctx context.Context, d *Dashboard, addr string) {
	m.lastProbe = time.Now()
	state, gateway := bnetfacade.ProbeGateway(ctx, addr)
	if gateway > 0 {
		d.bnetGateway.Store(int64(gateway))
	}
	if state == bnetfacade.BridgeNotRunning {
		// Something is listening but it is not the bridge — the remembered
		// address has been taken over by another process, so stop trusting it.
		m.candidateAddr = ""
		m.portWasOpen = false
		d.bnetAddr.Store("")
		d.bnetState.Store(bnetStatus{State: bnetfacade.BridgeNotRunning})
		return
	}
	d.bnetAddr.Store(addr)
	d.bnetState.Store(bnetStatus{State: state, Addr: addr})
}

func (m *bnetMonitor) reset() {
	m.candidateAddr = ""
	m.portWasOpen = false
	m.lastProbe = time.Time{}
	m.lastDiscovery = time.Time{}
}

func (d *Dashboard) currentBnetState() bnetfacade.BridgeState {
	if v := d.bnetState.Load(); v != nil {
		return v.(bnetStatus).State
	}
	return bnetfacade.BridgeNotRunning
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
