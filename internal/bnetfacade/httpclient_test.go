package bnetfacade

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestDownloadClient_DoesNotShareDefaultTransport(t *testing.T) {
	if downloadHTTPClient.Transport == nil {
		t.Error("download client has a nil Transport, so it uses the process-wide default pool")
	}
	if downloadHTTPClient.Transport == http.DefaultTransport {
		t.Error("download client uses http.DefaultTransport directly")
	}
}

// greetingListener is the case an unpooled transport still could not cover: a
// peer that writes its protocol banner the moment the connection is accepted,
// the way MySQL, SSH and most binary daemons do. net/http starts a connection's
// read loop before roundTrip claims it, so such a greeting can arrive while the
// transport still counts zero requests in flight — the same log line, on a
// connection that was never pooled at all. It then drains our request and hangs
// up, which is what a daemon does with bytes it cannot parse.
func greetingListener(t *testing.T, greeting []byte) net.Listener {
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
				conn.Write(greeting)
				br := bufio.NewReader(conn)
				for {
					line, err := br.ReadString('\n')
					if err != nil || line == "\r\n" {
						return
					}
				}
			}()
		}
	}()
	return ln
}

func TestProbeBridge_GreetingPeerLogsNothing(t *testing.T) {
	greeting := []byte{0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x7f}
	ln := greetingListener(t, greeting)
	logged := captureDefaultLog(t)

	// Repeated because losing the race is what produces the log line and a
	// single probe usually wins it: the pooled transport this replaced loses it
	// about once in 750 probes, so a few thousand make the regression reliable
	// while still costing a couple of seconds.
	for range 3000 {
		if state := ProbeBridge(context.Background(), ln.Addr().String()); state != BridgeNotRunning {
			t.Fatalf("state = %q, want %q", state, BridgeNotRunning)
		}
	}
	if got := logged.String(); strings.Contains(got, "Unsolicited response") {
		t.Errorf("probing a greeting peer logged transport noise: %s", got)
	}
}

// TestLoopbackGet_DoesNotLeaveLoopback is the other reason to own the
// connection: an http.Client follows redirects, so a confused local daemon
// could have bounced a discovery probe off the machine.
func TestLoopbackGet_DoesNotLeaveLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://example.com/", http.StatusFound)
	}))
	defer srv.Close()

	resp, err := loopbackGet(context.Background(), srv.URL+"/web-api/v1/gateway", probeTimeout, maxProbeResponse)
	if err != nil {
		t.Fatalf("loopbackGet: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want the 302 returned unfollowed", resp.StatusCode)
	}
}

func TestLoopbackGet_RefusesNonLoopback(t *testing.T) {
	_, err := loopbackGet(context.Background(), "http://example.com/web-api/v1/gateway",
		probeTimeout, maxProbeResponse)
	if !errors.Is(err, ErrNotLocal) {
		t.Errorf("err = %v, want ErrNotLocal", err)
	}
}

func TestLoopbackGet_RefusesNonHTTPScheme(t *testing.T) {
	_, err := loopbackGet(context.Background(), "https://127.0.0.1:6119/web-api/v1/gateway",
		probeTimeout, maxProbeResponse)
	if err == nil || !strings.Contains(err.Error(), "requires http") {
		t.Errorf("err = %v, want a scheme error", err)
	}
}

func TestLoopbackGet_CapsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 10_000))
	}))
	defer srv.Close()

	resp, err := loopbackGet(context.Background(), srv.URL+"/web-api/v1/gateway", probeTimeout, 128)
	if err != nil {
		t.Fatalf("loopbackGet: %v", err)
	}
	if len(resp.Body) != 128 {
		t.Errorf("read %d bytes, want 128", len(resp.Body))
	}
}

// TestLoopbackGet_HonoursCancelledContext covers a peer that accepts and then
// says nothing: only the AfterFunc close can unblock the read before the
// timeout deadline fires.
func TestLoopbackGet_HonoursCancelledContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { conn.Close() })
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := loopbackGet(ctx, "http://"+ln.Addr().String()+"/web-api/v1/gateway",
		time.Minute, maxProbeResponse); err == nil {
		t.Fatal("err = nil, want a cancellation failure")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s; cancellation did not unblock the read", elapsed)
	}
}
