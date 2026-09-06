package bnetfacade

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// probeSkipTTL bounds how long a loopback port that already answered a
// discovery probe as "not the bridge" is skipped on later sweeps.
//
// Without it, a machine with no SC:R running re-GETs /web-api/v1/gateway on
// every loopback port every bnetDiscoveryInterval, forever, which is how we end
// up talking HTTP to unrelated local daemons often enough to trip #384. The TTL
// only has to cover the one case port liveness cannot: a listener that stays up
// and *becomes* the bridge in place, which SC:R never does — it binds its own
// port when it launches, and a port that was not listening is never in the
// skiplist to begin with.
const probeSkipTTL = 5 * time.Minute

// probeSkiplist remembers loopback ports whose probe came back BridgeNotRunning.
// Only that verdict is remembered: BridgeOffline is SC:R's bridge with nobody
// logged in, and skipping it would mean missing the moment the user signs in.
type probeSkiplist struct {
	mu    sync.Mutex
	until map[int]time.Time
}

func newProbeSkiplist() *probeSkiplist {
	return &probeSkiplist{until: map[int]time.Time{}}
}

// retain drops every remembered port that has expired or stopped listening.
// Dropping ports that stopped listening is what makes the skiplist safe: the
// next process to bind one is probed fresh rather than inheriting the verdict
// earned by whatever held the port before it.
func (s *probeSkiplist) retain(listening []int, now time.Time) {
	live := make(map[int]bool, len(listening))
	for _, port := range listening {
		live[port] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for port, expiry := range s.until {
		if !live[port] || !now.Before(expiry) {
			delete(s.until, port)
		}
	}
}

func (s *probeSkiplist) skips(port int, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiry, ok := s.until[port]
	return ok && now.Before(expiry)
}

func (s *probeSkiplist) remember(port int, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.until[port] = now.Add(probeSkipTTL)
}

var notBridgePorts = newProbeSkiplist()

// DiscoverBridgeAddr enumerates loopback listening ports using platform-specific
// methods and probes each for the SC:R web-api bridge. Only a fully
// authenticated response (200 with JSON) confirms a port as SC:R's bridge —
// a 401 during discovery is ignored because other loopback services (e.g. the
// Battle.net Agent) may also return 401 on arbitrary paths. Once a bridge addr
// is discovered, the caller should use ProbeBridge to track state transitions
// including 401 (offline / not logged in).
//
// Ports that answered a recent sweep as definitely-not-the-bridge are skipped;
// see probeSkiplist.
func DiscoverBridgeAddr(ctx context.Context) (string, error) {
	ports, err := loopbackListeningPorts()
	if err != nil {
		return "", fmt.Errorf("bnetfacade: port discovery: %w", err)
	}
	return discoverBridgeAddr(ctx, ports, notBridgePorts, time.Now())
}

func discoverBridgeAddr(ctx context.Context, ports []int, skip *probeSkiplist, now time.Time) (string, error) {
	sort.Ints(ports)
	skip.retain(ports, now)
	for _, port := range ports {
		if skip.skips(port, now) {
			continue
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		state := ProbeBridge(ctx, addr)
		if state == BridgeConnected {
			return addr, nil
		}
		// A cancelled sweep fails every remaining probe for reasons that say
		// nothing about the peer, so it must not poison the skiplist.
		if state == BridgeNotRunning && ctx.Err() == nil {
			skip.remember(port, now)
		}
	}
	return "", fmt.Errorf("bnetfacade: no SC:R bridge found among %d loopback ports", len(ports))
}
