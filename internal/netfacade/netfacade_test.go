package netfacade

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForLocalListenerSucceedsWhenListening(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	if err := WaitForLocalListener(ln.Addr().String(), 10, 50*time.Millisecond); err != nil {
		t.Fatalf("WaitForLocalListener on a live listener: %v", err)
	}
}

func TestWaitForLocalListenerTimesOutWhenClosed(t *testing.T) {
	// Bind then immediately close to obtain a port nothing is listening on.
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	if err := WaitForLocalListener(addr, 2, 20*time.Millisecond); err == nil {
		t.Fatalf("WaitForLocalListener: expected timeout error on closed port")
	}
}

func TestWaitForLocalListenerRefusesRemoteAddr(t *testing.T) {
	if err := WaitForLocalListener("example.com:80", 1, time.Millisecond); err == nil {
		t.Fatalf("expected refusal of non-local address")
	}
	if err := WaitForLocalListener("8.8.8.8:53", 1, time.Millisecond); err == nil {
		t.Fatalf("expected refusal of non-loopback IP")
	}
}

func TestLocalAPIGetReadsALoopbackResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/api/games?limit=2" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	status, body, err := LocalAPIGet(context.Background(), addr, "/api/games?limit=2", time.Second, 1<<20)
	if err != nil {
		t.Fatalf("LocalAPIGet: %v", err)
	}
	if status != http.StatusOK || string(body) != `{"ok":true}` {
		t.Fatalf("status=%d body=%q", status, body)
	}

	// A path without a leading slash is accepted and normalized.
	if status, _, err := LocalAPIGet(context.Background(), addr, "api/games?limit=2", time.Second, 1<<20); err != nil || status != http.StatusOK {
		t.Fatalf("bare path: status=%d err=%v", status, err)
	}
}

func TestLocalAPIGetReturnsTheStatusOfAnErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	status, body, err := LocalAPIGet(context.Background(), strings.TrimPrefix(srv.URL, "http://"), "/api/games/9", time.Second, 1<<20)
	if err != nil {
		t.Fatalf("LocalAPIGet: %v", err)
	}
	if status != http.StatusNotFound || string(body) != "nope" {
		t.Fatalf("status=%d body=%q", status, body)
	}
}

func TestLocalAPIGetCapsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1000))
	}))
	defer srv.Close()

	_, body, err := LocalAPIGet(context.Background(), strings.TrimPrefix(srv.URL, "http://"), "/api/games", time.Second, 16)
	if err != nil {
		t.Fatalf("LocalAPIGet: %v", err)
	}
	if len(body) != 16 {
		t.Fatalf("read %d bytes, want the 16-byte cap", len(body))
	}
}

func TestLocalAPIGetRefusesRemoteAddr(t *testing.T) {
	for _, addr := range []string{"example.com:80", "8.8.8.8:53"} {
		_, _, err := LocalAPIGet(context.Background(), addr, "/api/games", time.Millisecond, 1<<20)
		if !errors.Is(err, ErrNotLoopback) {
			t.Errorf("LocalAPIGet(%q) = %v, want ErrNotLoopback", addr, err)
		}
	}
}
