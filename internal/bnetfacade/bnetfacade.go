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
// start with /web-api/. Returns the raw response body; use DecodeBridgeJSON to
// handle the non-UTF-8 encoding quirk before JSON unmarshalling.
func BridgeGet(ctx context.Context, addr, path string) ([]byte, error) {
	if !isLocalAddr(addr) {
		return nil, fmt.Errorf("%w: %s", ErrNotLocal, addr)
	}
	if !strings.HasPrefix(path, bridgePathPrefix) {
		return nil, fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, path, bridgePathPrefix)
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bnetfacade: bridge returned %s for %s", resp.Status, path)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBridgeResponse))
}

// DecodeBridgeJSON unmarshals a bridge response into dst, handling the
// non-UTF-8 encoding quirk: map titles may contain raw cp949 (Korean) or
// latin-1 bytes that make the payload invalid UTF-8. The function tries the raw
// bytes first (fast path for ASCII/UTF-8 payloads), then falls back to cp949
// and ISO 8859-1 transcoding.
func DecodeBridgeJSON(data []byte, dst any) error {
	if utf8.Valid(data) {
		return json.Unmarshal(data, dst)
	}
	decoded, err := korean.EUCKR.NewDecoder().Bytes(data)
	if err == nil && utf8.Valid(decoded) && !bytes.ContainsRune(decoded, utf8.RuneError) {
		return json.Unmarshal(decoded, dst)
	}
	decoded, err = charmap.ISO8859_1.NewDecoder().Bytes(data)
	if err != nil {
		return fmt.Errorf("bnetfacade: could not decode bridge payload: %w", err)
	}
	return json.Unmarshal(decoded, dst)
}

// DownloadReplay fetches a replay from the GCS starcraft-user-uploads-prod
// bucket. The path must start with /starcraft-user-uploads-prod/S1-replays/.
// Downloaded bytes are validated: they must be at least 16 bytes long and carry
// the seRS magic at offset 12.
func DownloadReplay(ctx context.Context, path string) ([]byte, error) {
	if !strings.HasPrefix(path, gcsReplayPrefix) {
		return nil, fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, path, gcsReplayPrefix)
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
	_, _ = io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		return BridgeConnected
	case http.StatusUnauthorized:
		return BridgeOffline
	default:
		return BridgeNotRunning
	}
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
