package bnetfacade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/korean"
)

var (
	ErrNotLocal      = errors.New("bnetfacade: refusing non-loopback address")
	ErrForbiddenPath = errors.New("bnetfacade: path outside allowed prefix")
	ErrForbiddenHost = errors.New("bnetfacade: host outside allowlist")
	ErrInvalidReplay = errors.New("bnetfacade: downloaded bytes are not a valid replay")
)

const (
	bridgePathPrefix  = "/web-api/"
	gcsHost           = "storage.googleapis.com"
	gcsReplayPrefix   = "/starcraft-user-uploads-prod/S1-replays/"
	maxReplaySize     = 20 << 20 // 20 MiB; SC:R replays are typically under 1 MiB.
	maxBridgeResponse = 1 << 20  // 1 MiB ceiling for bridge JSON responses.
	replayMagic       = "seRS"
	replayMagicOffset = 12
	replayMinSize     = replayMagicOffset + len(replayMagic) // 16 bytes
)

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

func bridgeClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

func gcsClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// BridgeGet performs a GET request against SC:R's local web-api bridge. The
// addr must resolve to loopback (127.0.0.1 / ::1 / localhost), and path must
// start with /web-api/. Every call spends the bridge budget (token bucket +
// persisted daily cap) and respects the rate-limit cooldown; prio decides who
// wins when calls queue on the bucket. Returns the raw response body; use
// DecodeBridgeJSON to handle the non-UTF-8 encoding quirk before JSON
// unmarshalling.
func BridgeGet(ctx context.Context, addr, path string, prio Priority) ([]byte, error) {
	if !isLocalAddr(addr) {
		return nil, fmt.Errorf("%w: %s", ErrNotLocal, addr)
	}
	if !strings.HasPrefix(path, bridgePathPrefix) {
		return nil, fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, path, bridgePathPrefix)
	}
	if err := theBudgets.acquireBridge(ctx, prio); err != nil {
		return nil, err
	}
	url := "http://" + addr + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bridgeClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBridgeResponse))
	if err != nil {
		return nil, err
	}
	if isRateLimitedResponse(resp.StatusCode, body) {
		until := theBudgets.noteBridgeRateLimited()
		return nil, fmt.Errorf("%w (cooling down until %s)", ErrBridgeRateLimited, until.Format(time.RFC3339))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bnetfacade: bridge returned %s for %s", resp.Status, path)
	}
	theBudgets.noteBridgeSuccess()
	return body, nil
}

// DecodeBridgeJSON unmarshals a bridge response into dst, handling the
// non-UTF-8 encoding quirk: map titles may contain raw cp949 (Korean) or
// latin-1 bytes that make the payload invalid UTF-8.
func DecodeBridgeJSON(data []byte, dst any) error {
	return json.Unmarshal(normalizeBridgeJSON(data), dst)
}

// normalizeBridgeJSON repairs only the byte runs that are not valid UTF-8 and
// copies every well-formed UTF-8 sequence through untouched.
//
// Transcoding the whole payload is wrong here. Bridge responses are UTF-8 that
// occasionally carry a handful of legacy-encoded or byte-truncated fields (SC:R
// writes map titles and game names into fixed-width buffers, so a multi-byte
// character can be cut mid-sequence). Re-decoding the entire response as
// cp949 or ISO 8859-1 because of those few bytes mojibakes every correct
// string in it, which is how Cyrillic and Korean battle tags came back
// double-encoded.
//
// Runs only ever contain bytes >= 0x80, since ASCII always decodes cleanly, and
// neither legacy decoder emits ASCII for such bytes. The JSON structure
// therefore survives the substitution.
func normalizeBridgeJSON(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	out := make([]byte, 0, len(data)+len(data)/8)
	for i := 0; i < len(data); {
		if r, size := utf8.DecodeRune(data[i:]); r != utf8.RuneError || size > 1 {
			out = append(out, data[i:i+size]...)
			i += size
			continue
		}
		j := i + 1
		for j < len(data) {
			if r, size := utf8.DecodeRune(data[j:]); r != utf8.RuneError || size > 1 {
				break
			}
			j++
		}
		out = append(out, decodeLegacyRun(data[i:j])...)
		i = j
	}
	return out
}

// decodeLegacyRun transcodes one run of bytes that is not valid UTF-8. cp949 is
// tried first because Korean map titles are the common case; ISO 8859-1 is the
// fallback, and it cannot fail because every byte maps to a code point. A run
// that is really the truncated head of a UTF-8 character is indistinguishable
// from legacy bytes, so it decodes to whichever character the run spells — the
// field is already damaged upstream either way.
func decodeLegacyRun(run []byte) []byte {
	if decoded, err := korean.EUCKR.NewDecoder().Bytes(run); err == nil &&
		!bytes.ContainsRune(decoded, utf8.RuneError) {
		return decoded
	}
	decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(run)
	if err != nil {
		return bytes.Repeat([]byte(string(utf8.RuneError)), len(run))
	}
	return decoded
}

// IsMojibakedPayload reports whether data is a cached bridge payload that an
// older build corrupted by transcoding a whole response as ISO 8859-1. Callers
// use it to invalidate such an entry so it refetches, never to repair the text
// in place.
//
// The signature is structural and whole-payload, which is what makes it safe:
// a latin-1 expansion leaves every rune in U+0000..U+00FF, and re-encoding
// recovers the original UTF-8 sequences. A payload holding real non-latin-1
// text cannot have been produced that way, and genuine latin-1 text yields
// stray high bytes rather than well-formed multi-byte sequences. Repairing a
// single field on its own would have no such evidence behind it.
func IsMojibakedPayload(data []byte) bool {
	if len(data) == 0 || !utf8.Valid(data) {
		return false
	}
	latin1 := make([]byte, 0, len(data))
	for _, r := range string(data) {
		if r > 0xFF {
			return false
		}
		latin1 = append(latin1, byte(r))
	}
	var recovered, stray int
	for i := 0; i < len(latin1); {
		if latin1[i] < utf8.RuneSelf {
			i++
			continue
		}
		if r, size := utf8.DecodeRune(latin1[i:]); size > 1 && r != utf8.RuneError {
			recovered += size
			i += size
			continue
		}
		stray++
		i++
	}
	return recovered > 0 && recovered >= stray
}

// DownloadReplay fetches a replay from the GCS starcraft-user-uploads-prod
// bucket. The path must start with /starcraft-user-uploads-prod/S1-replays/.
// Every call spends the download budget, which is separate from the bridge
// budget: GCS never touches the user's Blizzard session. Downloaded bytes are
// validated: they must be at least 16 bytes long and carry the seRS magic at
// offset 12.
func DownloadReplay(ctx context.Context, path string, prio Priority) ([]byte, error) {
	if !strings.HasPrefix(path, gcsReplayPrefix) {
		return nil, fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, path, gcsReplayPrefix)
	}
	if err := theBudgets.acquireDownload(ctx, prio); err != nil {
		return nil, err
	}
	return downloadReplayFrom(ctx, "https://"+gcsHost+path)
}

func downloadReplayFrom(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := gcsClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bnetfacade: GCS returned %s for %s", resp.Status, url)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReplaySize))
	if err != nil {
		return nil, fmt.Errorf("bnetfacade: reading replay: %w", err)
	}
	if err := validateReplay(data); err != nil {
		return nil, err
	}
	return data, nil
}

// BridgeState describes the connection status with SC:R's local web-api bridge.
type BridgeState string

const (
	BridgeNotRunning BridgeState = "not_running" // no SC:R bridge found
	BridgeOffline    BridgeState = "offline"     // bridge reachable but 401 (not logged in)
	BridgeConnected  BridgeState = "connected"   // bridge reachable and authenticated
)

const probeTimeout = 2 * time.Second

// ProbeBridge checks the SC:R bridge at addr by GETting /web-api/v1/gateway.
// It returns BridgeConnected (200), BridgeOffline (401), or BridgeNotRunning
// (connection refused, timeout, or any other failure). The addr must be
// loopback; non-loopback addresses return BridgeNotRunning.
func ProbeBridge(ctx context.Context, addr string) BridgeState {
	if !isLocalAddr(addr) {
		return BridgeNotRunning
	}
	return probeBridgeURL(ctx, "http://"+addr+"/web-api/v1/gateway")
}

func probeBridgeURL(ctx context.Context, url string) BridgeState {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return BridgeNotRunning
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return BridgeNotRunning
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK:
		// SC:R's /web-api/v1/gateway returns JSON. Other servers (including
		// screpdb itself via the SPA fallback) may return 200 with HTML. Only
		// treat it as a bridge if the response looks like JSON.
		if len(body) == 0 || body[0] != '{' {
			return BridgeNotRunning
		}
		return BridgeConnected
	case http.StatusUnauthorized:
		return BridgeOffline
	default:
		return BridgeNotRunning
	}
}

// ProbeGateway checks the SC:R bridge at addr and extracts the active gateway.
// It returns the BridgeState plus the gateway number (0 if unavailable or the
// body doesn't contain a recognisable gateway id).
func ProbeGateway(ctx context.Context, addr string) (BridgeState, int) {
	if !isLocalAddr(addr) {
		return BridgeNotRunning, 0
	}
	return probeGatewayURL(ctx, "http://"+addr+"/web-api/v1/gateway")
}

func probeGatewayURL(ctx context.Context, url string) (BridgeState, int) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return BridgeNotRunning, 0
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return BridgeNotRunning, 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK:
		if len(body) == 0 || body[0] != '{' {
			return BridgeNotRunning, 0
		}
		return BridgeConnected, parseGatewayFromBody(body)
	case http.StatusUnauthorized:
		return BridgeOffline, 0
	default:
		return BridgeNotRunning, 0
	}
}

func parseGatewayFromBody(body []byte) int {
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return 0
	}
	for _, key := range []string{"gateway", "gateway_id"} {
		if v, ok := raw[key]; ok {
			var n int
			if json.Unmarshal(v, &n) == nil && n > 0 {
				return n
			}
		}
	}
	if v, ok := raw["gateways"]; ok {
		var arr []map[string]json.RawMessage
		if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
			for _, key := range []string{"id", "gateway_id", "gateway"} {
				if gv, ok := arr[0][key]; ok {
					var n int
					if json.Unmarshal(gv, &n) == nil && n > 0 {
						return n
					}
				}
			}
		}
	}
	return 0
}

func validateReplay(data []byte) error {
	if len(data) < replayMinSize {
		return fmt.Errorf("%w: too short (%d bytes, need at least %d)", ErrInvalidReplay, len(data), replayMinSize)
	}
	if !bytes.Equal(data[replayMagicOffset:replayMagicOffset+len(replayMagic)], []byte(replayMagic)) {
		return fmt.Errorf("%w: missing seRS magic at offset %d", ErrInvalidReplay, replayMagicOffset)
	}
	return nil
}

// portDialTimeout bounds the loopback connect used by BridgePortOpen. A
// loopback connect either completes in microseconds or is refused outright, so
// this only ever trips when the kernel's accept queue is saturated.
const portDialTimeout = 250 * time.Millisecond

// BridgePortOpen reports whether anything is listening on addr, by opening and
// immediately closing a loopback TCP connection. It refuses any non-loopback
// address.
//
// This is deliberately *not* an HTTP request and is unmetered, like
// ProbeBridge. It sends zero bytes, so the SC:R client has nothing to forward
// upstream to Battle.net and no rate-limit budget is spent; the connection is
// accepted and closed. That makes it cheap enough (~100µs) to poll on a fast
// tick, which is the point: the monitor watches the port for transitions and
// only spends a real HTTP probe when one actually happens.
//
// A bind-probe would be cheaper still and touch SC:R not at all, but it can win
// the race for the port while SC:R is starting up and stop the game binding its
// own listener — so the dial is the safer of the two.
func BridgePortOpen(ctx context.Context, addr string) bool {
	if !isLocalAddr(addr) {
		return false
	}
	dialer := net.Dialer{Timeout: portDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
