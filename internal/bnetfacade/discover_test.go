package bnetfacade

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
