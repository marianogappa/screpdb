package bnetfacade

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Nothing here falls through to http.DefaultTransport. Sharing the process-wide
// pool is what turned a misbehaving loopback peer into an unattributable
// app-level log line (#384): net/http reports "Unsolicited response received on
// idle HTTP channel" with a bare log.Printf from the shared read loop, so there
// is no ErrorLog hook and no way to say which request caused it.

// gcsTransport carries replay downloads from storage.googleapis.com, a real
// remote host that unquestionably speaks HTTP, so it keeps connection pooling
// and TLS session reuse. It is a clone of the default transport rather than the
// default transport itself only to keep the pool separate from anything else in
// the process.
func newGCSTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

var downloadHTTPClient = &http.Client{Transport: newGCSTransport(), Timeout: 60 * time.Second}

const (
	loopbackDialTimeout = 5 * time.Second
	bridgeTimeout       = 5 * time.Second
)

type loopbackResponse struct {
	StatusCode int
	Status     string
	Body       []byte
}

// loopbackGet performs a one-shot HTTP/1.1 GET against a loopback address on a
// connection this package owns end to end, rather than through any
// http.Transport. It carries every request to the local SC:R web-api bridge:
// discovery probes, state probes and real bridge calls.
//
// Owning the connection, rather than merely giving the probes an unpooled
// transport, is what makes the #384 log line impossible. The stdlib emits it
// from readLoopPeekFailLocked, which is reached whenever bytes arrive with no
// request in flight — and that is true in two states, not one. The pooled idle
// connection is the obvious one. The other is a connection that was only just
// dialled: dialConn starts readLoop before roundTrip claims it, so a peer that
// greets on connect (MySQL, SSH and most binary daemons do) can land its
// greeting in that window. Discovery GETs every loopback listening port on the
// machine, so it meets exactly those peers, and DisableKeepAlives does not help
// there because nothing was ever pooled. Measured against a greeting peer, an
// unpooled transport still logged 27 lines in 20,000 probes; this logs none.
//
// It also keeps the facade's loopback promise literal: an http.Client follows
// redirects, so a confused local daemon could bounce a probe off the machine.
func loopbackGet(ctx context.Context, rawURL string, timeout time.Duration, maxBody int64) (*loopbackResponse, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("bnetfacade: loopback GET requires http, got %q", u.Scheme)
	}
	if !isLocalAddr(u.Host) {
		return nil, fmt.Errorf("%w: %s", ErrNotLocal, u.Host)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Close = true

	dialer := net.Dialer{Timeout: loopbackDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort(u))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	// The deadline covers the timeout; AfterFunc covers a caller cancelling the
	// parent context earlier, which a deadline alone would not notice.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()

	if err := req.Write(conn); err != nil {
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	return &loopbackResponse{StatusCode: resp.StatusCode, Status: resp.Status, Body: body}, nil
}

func hostPort(u *url.URL) string {
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		return net.JoinHostPort(u.Host, "80")
	}
	return u.Host
}
