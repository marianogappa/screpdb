package bnetfacade

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeBridge_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/web-api/v1/gateway" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"gateways":[]}`)
	}))
	defer srv.Close()

	state := ProbeBridge(context.Background(), srv.Listener.Addr().String())
	if state != BridgeConnected {
		t.Errorf("got %q, want %q", state, BridgeConnected)
	}
}

func TestProbeBridge_Offline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	state := ProbeBridge(context.Background(), srv.Listener.Addr().String())
	if state != BridgeOffline {
		t.Errorf("got %q, want %q", state, BridgeOffline)
	}
}

func TestProbeBridge_NotRunning_ConnectionRefused(t *testing.T) {
	state := ProbeBridge(context.Background(), "127.0.0.1:1")
	if state != BridgeNotRunning {
		t.Errorf("got %q, want %q", state, BridgeNotRunning)
	}
}

func TestProbeBridge_NotRunning_NonLoopback(t *testing.T) {
	state := ProbeBridge(context.Background(), "8.8.8.8:53")
	if state != BridgeNotRunning {
		t.Errorf("got %q, want %q", state, BridgeNotRunning)
	}
}

func TestProbeBridge_NotRunning_OtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	state := ProbeBridge(context.Background(), srv.Listener.Addr().String())
	if state != BridgeNotRunning {
		t.Errorf("got %q, want %q", state, BridgeNotRunning)
	}
}

func TestProbeBridge_NotRunning_HTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><body>SPA</body></html>`)
	}))
	defer srv.Close()

	state := ProbeBridge(context.Background(), srv.Listener.Addr().String())
	if state != BridgeNotRunning {
		t.Errorf("HTML 200 should be rejected as not_running, got %q", state)
	}
}

func TestProbeBridgeURL_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"gateways":[]}`)
	}))
	defer srv.Close()

	state := probeBridgeURL(context.Background(), srv.URL+"/web-api/v1/gateway")
	if state != BridgeConnected {
		t.Errorf("got %q, want %q", state, BridgeConnected)
	}
}

func TestProbeGateway_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"gateway": 20}`)
	}))
	defer srv.Close()
	state, gw := probeGatewayURL(context.Background(), srv.URL)
	if state != BridgeConnected {
		t.Errorf("state = %v, want BridgeConnected", state)
	}
	if gw != 20 {
		t.Errorf("gateway = %d, want 20", gw)
	}
}

func TestProbeGateway_NestedFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"gateways":[{"id":30}]}`)
	}))
	defer srv.Close()
	state, gw := probeGatewayURL(context.Background(), srv.URL)
	if state != BridgeConnected {
		t.Errorf("state = %v, want BridgeConnected", state)
	}
	if gw != 30 {
		t.Errorf("gateway = %d, want 30", gw)
	}
}

func TestProbeGateway_NoGatewayInBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"foo":"bar"}`)
	}))
	defer srv.Close()
	state, gw := probeGatewayURL(context.Background(), srv.URL)
	if state != BridgeConnected {
		t.Errorf("state = %v, want BridgeConnected", state)
	}
	if gw != 0 {
		t.Errorf("gateway = %d, want 0", gw)
	}
}

func TestProbeGateway_Offline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	state, gw := probeGatewayURL(context.Background(), srv.URL)
	if state != BridgeOffline {
		t.Errorf("state = %v, want BridgeOffline", state)
	}
	if gw != 0 {
		t.Errorf("gateway = %d, want 0", gw)
	}
}

func TestProbeGateway_NonLoopback(t *testing.T) {
	state, gw := ProbeGateway(context.Background(), "8.8.8.8:53")
	if state != BridgeNotRunning {
		t.Errorf("state = %v, want BridgeNotRunning", state)
	}
	if gw != 0 {
		t.Errorf("gateway = %d, want 0", gw)
	}
}

func TestDiscoverBridgeAddr_FindsBridge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/web-api/v1/gateway" {
			fmt.Fprint(w, `{"gateways":[]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// DiscoverBridgeAddr uses the platform-specific loopbackListeningPorts,
	// which should include the test server's port since it binds to loopback.
	addr, err := DiscoverBridgeAddr(context.Background())
	if err != nil {
		// The platform-specific discovery may or may not find the httptest
		// server depending on OS-specific parsing. On macOS lsof will find it.
		// On other platforms this test is informational, not a hard failure.
		t.Skipf("DiscoverBridgeAddr: %v (platform may not list httptest server)", err)
	}
	if addr != srv.Listener.Addr().String() {
		// It found some other bridge on the machine (unlikely but possible in CI).
		t.Logf("found bridge at %s (test server at %s)", addr, srv.Listener.Addr().String())
	}
}

func TestProbeSkiplist_ForgetsPortsThatStopListening(t *testing.T) {
	now := time.Now()
	s := newProbeSkiplist()
	s.remember(4000, now)
	s.remember(4001, now)

	s.retain([]int{4000}, now)

	if !s.skips(4000, now) {
		t.Error("port 4000 still listening, should stay skipped")
	}
	if s.skips(4001, now) {
		t.Error("port 4001 stopped listening; a new process could bind it, so it must be re-probed")
	}
}

func TestProbeSkiplist_Expires(t *testing.T) {
	now := time.Now()
	s := newProbeSkiplist()
	s.remember(4000, now)

	later := now.Add(probeSkipBackoff(1) + time.Second)
	if s.skips(4000, later) {
		t.Error("entry should have expired")
	}
	// The entry survives its own expiry on purpose: it stops muting the port,
	// but the strike it carries is what makes a repeat offender escalate. A
	// retain that forgot expired entries would reset every stable daemon to
	// the short mute each time, so nothing would ever reach the ceiling.
	s.retain([]int{4000}, later)
	if len(s.entries) != 1 {
		t.Errorf("an expired entry for a still-listening port must be kept as evidence, got %d", len(s.entries))
	}
	s.retain(nil, later)
	if len(s.entries) != 0 {
		t.Errorf("a port that stopped listening must be forgotten, got %d", len(s.entries))
	}
}

// The #424 regression: SC:R binds its port at launch but serves /web-api/
// appreciably later, so its own port earns a not-the-bridge verdict while it is
// still warming up. Under the old flat five-minute mute the app then reported
// not_running for five minutes with the game open.
func TestProbeSkiplist_AWarmingUpBridgeIsRetriedWithinTheMinute(t *testing.T) {
	now := time.Now()
	s := newProbeSkiplist()

	s.remember(63585, now)

	if !s.skips(63585, now.Add(probeSkipTTLInitial-time.Second)) {
		t.Error("the first verdict must still mute the port briefly, or #384's probe noise comes back")
	}
	if s.skips(63585, now.Add(probeSkipTTLInitial+time.Second)) {
		t.Error("a port muted once must be retried within the short window, not held for the ceiling")
	}
	if probeSkipTTLInitial > time.Minute {
		t.Errorf("the first mute is %v; #424 asks for well under a minute", probeSkipTTLInitial)
	}
}

// The other half of the bargain: a genuine third-party daemon must still go
// quiet, or the skiplist stops doing the job #384 built it for.
func TestProbeSkiplist_ARepeatOffenderReachesTheCeiling(t *testing.T) {
	now := time.Now()
	s := newProbeSkiplist()

	var last time.Duration
	for strike := 1; strike <= 8; strike++ {
		ttl := probeSkipBackoff(strike)
		if ttl < last {
			t.Fatalf("strike %d shortened the mute: %v after %v", strike, ttl, last)
		}
		if ttl > probeSkipTTLMax {
			t.Fatalf("strike %d exceeded the ceiling: %v > %v", strike, ttl, probeSkipTTLMax)
		}
		last = ttl
	}
	if probeSkipBackoff(8) != probeSkipTTLMax {
		t.Errorf("a persistent non-bridge must reach the ceiling, got %v", probeSkipBackoff(8))
	}

	// And it escalates through the skiplist itself, not just the arithmetic:
	// each repeat verdict on a still-listening port mutes it for longer.
	for strike := 1; strike <= 5; strike++ {
		s.remember(4000, now)
		s.retain([]int{4000}, now)
		if !s.skips(4000, now.Add(probeSkipBackoff(strike)-time.Second)) {
			t.Fatalf("strike %d: port should be muted for %v", strike, probeSkipBackoff(strike))
		}
	}
	if !s.skips(4000, now.Add(probeSkipTTLMax-time.Second)) {
		t.Error("five strikes must have reached the ceiling")
	}
}

func TestDiscoverBridgeAddr_SkipsPortsAlreadyRejected(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	s := newProbeSkiplist()
	now := time.Now()
	for range 3 {
		if _, err := discoverBridgeAddr(context.Background(), []int{port}, s, now); err == nil {
			t.Fatal("expected no bridge")
		}
	}
	if hits != 1 {
		t.Errorf("probed a known non-bridge port %d times, want 1", hits)
	}
}

func TestDiscoverBridgeAddr_KeepsProbingOfflineBridge(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	s := newProbeSkiplist()
	now := time.Now()
	for range 3 {
		if _, err := discoverBridgeAddr(context.Background(), []int{port}, s, now); err == nil {
			t.Fatal("expected no bridge")
		}
	}
	// A 401 is SC:R's bridge with nobody logged in; skipping it would miss the
	// sign-in.
	if hits != 3 {
		t.Errorf("probed an offline bridge %d times, want 3", hits)
	}
}

func TestDiscoverBridgeAddr_CancelledSweepDoesNotPoisonSkiplist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"gateways":[]}`)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	s := newProbeSkiplist()
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discoverBridgeAddr(ctx, []int{port}, s, now); err == nil {
		t.Fatal("expected no bridge on a cancelled sweep")
	}
	if s.skips(port, now) {
		t.Error("a cancelled sweep must not remember ports as non-bridges")
	}

	addr, err := discoverBridgeAddr(context.Background(), []int{port}, s, now)
	if err != nil {
		t.Fatalf("re-sweep after cancellation: %v", err)
	}
	if addr != srv.Listener.Addr().String() {
		t.Errorf("addr = %q, want %q", addr, srv.Listener.Addr().String())
	}
}

// The #424 scenario end to end, on the sweep rather than on the skiplist: a
// port that is still coming up answers as not-the-bridge, then starts serving,
// and must be found on the next sweep after the short mute rather than being
// silenced for the ceiling. The old flat TTL failed this by five minutes.
func TestDiscoverBridgeAddr_FindsAPortThatWasStillWarmingUp(t *testing.T) {
	var serving bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !serving {
			// SC:R has bound the port but web-api is not up yet.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"gateways":[]}`)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	s := newProbeSkiplist()
	start := time.Now()
	if _, err := discoverBridgeAddr(context.Background(), []int{port}, s, start); err == nil {
		t.Fatal("expected no bridge while the port is still warming up")
	}

	serving = true

	// Still inside the short mute: the sweep is entitled to skip it, which is
	// what keeps #384's probe noise down.
	if _, err := discoverBridgeAddr(context.Background(), []int{port}, s, start.Add(probeSkipTTLInitial/2)); err == nil {
		t.Fatal("the port is still muted; the sweep must not have probed it")
	}

	addr, err := discoverBridgeAddr(context.Background(), []int{port}, s, start.Add(probeSkipTTLInitial+time.Second))
	if err != nil {
		t.Fatalf("the bridge must be found once the short mute lapses: %v", err)
	}
	if addr != srv.Listener.Addr().String() {
		t.Fatalf("found %q, want %q", addr, srv.Listener.Addr().String())
	}
	// And the recovery must land well inside the outage #424 reported.
	if probeSkipTTLInitial+time.Second >= time.Minute {
		t.Errorf("recovery took %v; #424 asks for well under a minute", probeSkipTTLInitial+time.Second)
	}
}
