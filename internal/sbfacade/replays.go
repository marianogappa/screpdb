package sbfacade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	replayHost = "cdn.shieldbattery.net"
	// maxReplaySize is far above any real replay; it bounds what a
	// misbehaving server can make screpdb hold in memory.
	maxReplaySize = 16 << 20

	replayMagic       = "seRS"
	replayMagicOffset = 12
	replayMinSize     = replayMagicOffset + len(replayMagic)
)

var (
	ErrBadReplayURL  = errors.New("sbfacade: replay URL is not a ShieldBattery replay")
	ErrInvalidReplay = errors.New("sbfacade: downloaded bytes are not a valid replay")

	replayPathPattern = regexp.MustCompile(`^/replays/[A-Za-z0-9_-][A-Za-z0-9._-]*$`)
	sha256HexPattern  = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// DownloadReplay downloads the replay a game record points at and returns its
// bytes only if they hash to wantSHA256Hex and look like a replay.
func DownloadReplay(ctx context.Context, rawURL, wantSHA256Hex string) ([]byte, error) {
	if err := checkReplayURL(rawURL); err != nil {
		return nil, err
	}
	if !sha256HexPattern.MatchString(wantSHA256Hex) {
		return nil, fmt.Errorf("%w: malformed sha256 %q", ErrInvalidReplay, wantSHA256Hex)
	}
	if err := thePacer.wait(ctx); err != nil {
		return nil, err
	}
	return downloadReplayFrom(ctx, rawURL, wantSHA256Hex)
}

func checkReplayURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadReplayURL, err)
	}
	switch {
	case u.Scheme != "https":
		return fmt.Errorf("%w: scheme %q", ErrBadReplayURL, u.Scheme)
	case u.Host != replayHost || u.User != nil:
		return fmt.Errorf("%w: host %q", ErrBadReplayURL, u.Host)
	case !replayPathPattern.MatchString(u.Path) || (u.RawPath != "" && u.RawPath != u.Path):
		return fmt.Errorf("%w: path %q", ErrBadReplayURL, u.EscapedPath())
	}
	return nil
}

func downloadReplayFrom(ctx context.Context, rawURL, wantSHA256Hex string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	unsigned := *req.URL
	unsigned.RawQuery = ""
	if err := checkStatus(resp, unsigned.String()); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReplaySize+1))
	if err != nil {
		return nil, fmt.Errorf("sbfacade: reading replay: %w", err)
	}
	if len(data) > maxReplaySize {
		return nil, fmt.Errorf("%w: larger than %d bytes", ErrInvalidReplay, maxReplaySize)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != strings.ToLower(wantSHA256Hex) {
		return nil, fmt.Errorf("%w: sha256 %s, want %s", ErrInvalidReplay, got, strings.ToLower(wantSHA256Hex))
	}
	if len(data) < replayMinSize {
		return nil, fmt.Errorf("%w: too short (%d bytes)", ErrInvalidReplay, len(data))
	}
	if !bytes.Equal(data[replayMagicOffset:replayMinSize], []byte(replayMagic)) {
		return nil, fmt.Errorf("%w: missing seRS magic at offset %d", ErrInvalidReplay, replayMagicOffset)
	}
	return data, nil
}
