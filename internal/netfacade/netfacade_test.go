package netfacade

import (
	"net"
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
