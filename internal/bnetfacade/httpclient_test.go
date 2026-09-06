package bnetfacade

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// binaryGarbageListener stands in for the loopback services discovery walks
// over that are not HTTP servers: it accepts the connection, replies with just
// enough HTTP to be parsed, then writes the kind of binary payload that showed
// up in #384's log line. If the client pools the connection, net/http's read
// loop finds those bytes on an idle channel and log.Printf's about it.
func binaryGarbageListener(t *testing.T, garbage []byte) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		ln.Close()
		wg.Wait()
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
				time.Sleep(50 * time.Millisecond)
				conn.Write(garbage)
				time.Sleep(200 * time.Millisecond)
			}()
		}
	}()
	return ln
}

// captureDefaultLog redirects the stdlib default logger, which is where
// net/http's "Unsolicited response received on idle HTTP channel" goes: the
// transport emits it with a bare log.Printf, so there is no ErrorLog hook to
// intercept it and the only way to stop it is to keep the connection out of
// the idle pool.
func captureDefaultLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	var mu sync.Mutex
	prevOut, prevFlags, prevPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}))
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
	})
	return buf
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestProbeBridge_NoUnsolicitedResponseLog(t *testing.T) {
	garbage := []byte{0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x7f}
	ln := binaryGarbageListener(t, garbage)
	logged := captureDefaultLog(t)

	// A non-JSON 200 is not the bridge; what matters is that probing does not
	// leave a pooled connection behind for the garbage to land on.
	if state := ProbeBridge(context.Background(), ln.Addr().String()); state != BridgeNotRunning {
		t.Errorf("state = %q, want %q", state, BridgeNotRunning)
	}
	time.Sleep(400 * time.Millisecond)

	if got := logged.String(); strings.Contains(got, "Unsolicited response") {
		t.Errorf("probing a non-HTTP loopback peer logged transport noise: %s", got)
	}
}

func TestLoopbackTransport_DisablesKeepAlives(t *testing.T) {
	if !loopbackTransport.DisableKeepAlives {
		t.Error("loopback transport must not pool connections; see #384")
	}
	if loopbackTransport.Proxy != nil {
		t.Error("loopback requests must never be routed through a proxy")
	}
}

func TestClients_DoNotShareDefaultTransport(t *testing.T) {
	for name, client := range map[string]*http.Client{
		"probe":    probeHTTPClient,
		"bridge":   bridgeHTTPClient,
		"download": downloadHTTPClient,
	} {
		if client.Transport == nil {
			t.Errorf("%s client has a nil Transport, so it uses the process-wide default pool", name)
		}
		if client.Transport == http.DefaultTransport {
			t.Errorf("%s client uses http.DefaultTransport directly", name)
		}
	}
}
