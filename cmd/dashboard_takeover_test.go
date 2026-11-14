package cmd

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
)

func startFakeScrepdb(t *testing.T, version string) (port int, quitCalled func() bool) {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu   sync.Mutex
		quit bool
	)
	srv := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"app":"screpdb","version":"` + version + `"}`))
	})
	mux.HandleFunc("/api/custom/quit", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		quit = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		// Shutdown, unlike Close, lets this reply reach the client before the connection goes, as the real
		// handler does by flushing and waiting before it exits.
		go func() { _ = srv.Shutdown(context.Background()) }()
	})
	srv.Handler = mux
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().(*net.TCPAddr).Port, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return quit
	}
}

func TestResolveDashboardPortReportsRunningVersion(t *testing.T) {
	port, _ := startFakeScrepdb(t, "v1.36.0")
	got, version, running, err := resolveDashboardPort(port)
	if err != nil || !running || got != port || version != "v1.36.0" {
		t.Fatalf("got port=%d version=%q running=%v err=%v", got, version, running, err)
	}
}

func TestTakeOverOlderInstanceFreesItsPort(t *testing.T) {
	port, quitCalled := startFakeScrepdb(t, "v1.36.0")
	if err := takeOverOlderInstance(net.JoinHostPort("localhost", strconv.Itoa(port)), "v1.36.0"); err != nil {
		t.Fatal(err)
	}
	if !quitCalled() {
		t.Fatal("older instance was not asked to quit")
	}
	got, _, running, err := resolveDashboardPort(port)
	if err != nil || running || got != port {
		t.Fatalf("port not reclaimed: port=%d running=%v err=%v", got, running, err)
	}
}
