// Package netfacade is the single sanctioned surface for network operations in
// screpdb's Go binary. Per issue #135 the binary makes NO external outbound
// network calls: the only version-update check is a browser fetch() in the
// frontend, and the dashboard HTTP server binds to localhost only.
//
// The sole network-client operation the binary performs is a localhost TCP
// readiness probe (used to detect when the embedded dashboard server has come
// up). It lives here so the enforcement test in internal/iofacade can assert
// that no other package constructs an outbound HTTP client or dials a remote
// host.
//
// Inbound serving (http.Server / ListenAndServe bound to localhost) stays in
// the dashboard package and is intentionally allowed by the enforcement test.
package netfacade

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// errNotLocal guards WaitForLocalListener against being used to reach a remote
// host — this facade only ever talks to the loopback interface.
func isLocalAddr(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" || host == "" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// WaitForLocalListener polls a localhost address until a TCP connection
// succeeds or attempts are exhausted, returning nil once the listener accepts.
// It refuses any non-loopback address. This replaces the previous outbound
// http.Get(/api/health) readiness check: a successful dial means the server is
// listening (routes are registered before ListenAndServe), and it makes no HTTP
// request and cannot reach the network beyond loopback.
func WaitForLocalListener(addr string, attempts int, delay time.Duration) error {
	if !isLocalAddr(addr) {
		return fmt.Errorf("netfacade: refusing non-local address %q", addr)
	}
	for i := 0; i < attempts; i++ {
		conn, err := net.DialTimeout("tcp", addr, delay)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("netfacade: %s did not start listening after %d attempts", addr, attempts)
}
