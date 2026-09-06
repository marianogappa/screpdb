package bnetfacade

import (
	"net"
	"net/http"
	"time"
)

// Every client below owns its transport instead of falling through to
// http.DefaultTransport. Sharing the process-wide pool is what turned a
// misbehaving loopback peer into an unattributable app-level log line (#384):
// net/http reports "Unsolicited response received on idle HTTP channel" with a
// bare log.Printf from the shared read loop, so there is no ErrorLog hook and
// no way to say which request caused it. Keeping the pools apart, and keeping
// loopback connections out of a pool entirely, is the only lever we have.

// loopbackTransport carries every request to the local SC:R web-api bridge:
// discovery probes, state probes and real bridge calls.
//
// DisableKeepAlives is the point of it. Discovery GETs /web-api/v1/gateway on
// every loopback port it can find, so most probes land on unrelated local
// daemons; a service that is not an HTTP server may accept the connection,
// ignore the request and write binary garbage back at its own pace. Pooled,
// that connection is sitting idle when the bytes arrive and net/http logs them.
// Unpooled, the read loop has already exited and closed the socket. Nothing is
// lost: a loopback dial costs microseconds, probes are one-shot, and real
// bridge calls are rate-limited far below any idle timeout.
var loopbackTransport = &http.Transport{
	Proxy:                 nil, // loopback only; never route through a proxy
	DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	DisableKeepAlives:     true,
	ResponseHeaderTimeout: 5 * time.Second,
}

// gcsTransport carries replay downloads from storage.googleapis.com, a real
// remote host that unquestionably speaks HTTP, so it keeps connection pooling
// and TLS session reuse. It is a clone of the default transport rather than the
// default transport itself only to keep the pool separate from anything else in
// the process.
func newGCSTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

var (
	probeHTTPClient    = &http.Client{Transport: loopbackTransport}
	bridgeHTTPClient   = &http.Client{Transport: loopbackTransport, Timeout: 5 * time.Second}
	downloadHTTPClient = &http.Client{Transport: newGCSTransport(), Timeout: 60 * time.Second}
)
