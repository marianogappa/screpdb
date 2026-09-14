package bnetfacade

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// The discovery skiplist mutes a loopback port that answered a probe as "not
// the bridge", so a machine with no SC:R running does not re-GET
// /web-api/v1/gateway on every loopback port forever — which is how we ended up
// speaking HTTP to unrelated local daemons often enough to trip #384.
//
// The mute escalates per port rather than being one flat span, because the two
// things it has to tell apart look identical on the first probe (#424). SC:R
// binds its port at launch but starts serving /web-api/ appreciably later, and
// later still when nobody has logged in yet; probeBridgeURL maps that
// still-warming-up port to BridgeNotRunning, exactly like a postgres or a
// Spotify. A flat five-minute mute therefore silenced SC:R's own port for five
// minutes at startup, and the app reported not_running with the game open in
// front of the user.
//
// Escalating fixes that without giving up what the skiplist is for: a port that
// was merely warming up is retried inside probeSkipTTLInitial and costs one
// extra probe, while a genuine third-party daemon reaches probeSkipTTLMax after
// a few strikes and is then as quiet as it ever was. Sweep frequency is
// untouched, so this adds no lsof spawns and no extra traffic to other daemons.
//
// Note 401 is BridgeOffline, not BridgeNotRunning, and is never skiplisted at
// all: that is SC:R's bridge with nobody signed in, and muting it would mean
// missing the moment the user signs in.
const (
	probeSkipTTLInitial = 30 * time.Second
	probeSkipTTLMax     = 5 * time.Minute
)

// probeSkipBackoff is the mute a port has earned after this many consecutive
// not-the-bridge verdicts: 30s, 1m, 2m, 4m, then the 5m ceiling.
func probeSkipBackoff(strikes int) time.Duration {
	if strikes < 1 {
		strikes = 1
	}
	ttl := probeSkipTTLInitial
	for i := 1; i < strikes && ttl < probeSkipTTLMax; i++ {
		ttl *= 2
	}
	if ttl > probeSkipTTLMax {
		return probeSkipTTLMax
	}
	return ttl
}

// probeSkipEntry is one port's standing with the skiplist.
type probeSkipEntry struct {
	until time.Time
	// strikes counts the consecutive not-the-bridge verdicts this port has
	// given. It deliberately outlives the mute it earned: an entry that is
	// merely expired is still evidence, and forgetting it would reset every
	// stable daemon to the short mute each time its mute lapsed, so no port
	// would ever escalate and the skiplist would be a flat 30 seconds.
	strikes int
}

// probeSkiplist remembers loopback ports whose probe came back BridgeNotRunning.
type probeSkiplist struct {
	mu      sync.Mutex
	entries map[int]probeSkipEntry
}

func newProbeSkiplist() *probeSkiplist {
	return &probeSkiplist{entries: map[int]probeSkipEntry{}}
}

// retain drops every port that has stopped listening, and only those. Dropping
// ports that stopped listening is what makes the skiplist safe: the next
// process to bind one is probed fresh rather than inheriting the verdict earned
// by whatever held the port before it. Expiry alone is not grounds for
// forgetting — see probeSkipEntry.strikes.
func (s *probeSkiplist) retain(listening []int, _ time.Time) {
	live := make(map[int]bool, len(listening))
	for _, port := range listening {
		live[port] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for port := range s.entries {
		if !live[port] {
			delete(s.entries, port)
		}
	}
}

func (s *probeSkiplist) skips(port int, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[port]
	return ok && now.Before(entry.until)
}

func (s *probeSkiplist) remember(port int, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[port]
	entry.strikes++
	entry.until = now.Add(probeSkipBackoff(entry.strikes))
	s.entries[port] = entry
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
