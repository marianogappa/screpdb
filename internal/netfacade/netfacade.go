// Package netfacade is the sanctioned surface for general network operations in
// screpdb's Go binary. Per issue #135 the binary's only network-client
// operations here talk to the loopback interface: a localhost TCP readiness
// probe (used to detect when the embedded dashboard server has come up) and a
// localhost /api/health GET used for single-instance detection (deciding whether
// a busy port is already served by another screpdb). The dashboard HTTP server
// binds to localhost only.
//
// The one deliberate exception to "no outbound calls" is in-binary self-update
// (issue #212): internal/selfupdate is a separate sanctioned surface that
// queries the GitHub Releases API and downloads the matching asset, verifying
// every byte against a minisign-signed SHA256SUMS before any swap. Both this
// package and internal/selfupdate are exempt from the enforcement test in
// internal/iofacade, which asserts that no other package constructs an outbound
// HTTP client or dials a remote host.
//
// Inbound serving (http.Server / ListenAndServe bound to localhost) stays in
// the dashboard package and is intentionally allowed by the enforcement test.
package netfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

// LocalPortAvailable reports whether nothing is currently listening on the given
// loopback address, by attempting a short-lived bind. It refuses any
// non-loopback address. Note the usual bind race: the port can be taken between
// this check and an actual ListenAndServe, so callers must still handle a bind
// failure downstream.
func LocalPortAvailable(addr string) bool {
	if !isLocalAddr(addr) {
		return false
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// IsLocalScrepdb reports whether the loopback address is served by a screpdb
// instance, by GETting /api/health and checking for screpdb's identity marker.
// It refuses any non-loopback address and makes no request off the loopback
// interface. Used for single-instance detection: a busy port that answers as
// screpdb should be reused rather than treated as a fatal conflict.
func IsLocalScrepdb(addr string, timeout time.Duration) bool {
	if !isLocalAddr(addr) {
		return false
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		App string `json:"app"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return false
	}
	return body.App == "screpdb"
}

// ErrNotLoopback is returned when a caller asks this facade to reach an
// address that is not on the loopback interface.
var ErrNotLoopback = errors.New("netfacade: only loopback addresses are allowed")

// LocalAPIGet issues a GET against a loopback screpdb API and returns the
// status code and the response body, capped at maxBytes. It refuses any
// non-loopback address, so the MCP server (issue #381), which reads the
// dashboard's JSON API instead of opening a database, cannot be pointed at a
// remote host by the model driving it.
func LocalAPIGet(ctx context.Context, addr, path string, timeout time.Duration, maxBytes int64) (int, []byte, error) {
	if !isLocalAddr(addr) {
		return 0, nil, fmt.Errorf("%w: %s", ErrNotLoopback, addr)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		return 0, nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
