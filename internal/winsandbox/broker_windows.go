//go:build windows

package winsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/pkg/browser"
)

// The Low worker asks the Medium launcher to perform two things it cannot do
// itself, via a file-drop broker under the (Low-writable) app-data dir:
//
//   - See-replay ("watch me"): stage a replay into the user's replays folder so
//     StarCraft can load it. That folder is read-only to the Low worker.
//   - open-url: bring up the dashboard in the default browser. A ShellExecute
//     from a Low-integrity process silently fails to spawn a window (issue #237),
//     so the Medium launcher opens the browser on the worker's behalf.
//
// Both request kinds are locked down so a compromised worker gains nothing: the
// See-replay destination is fixed, and open-url only ever opens a loopback
// dashboard URL — never an arbitrary string ShellExecute could turn into a
// launched program.
const (
	brokerDirName     = "broker"
	seeReplayFolder   = "000_screpdb_watch_me"
	seeReplayFilename = "watch_me.rep"
	requestSuffix     = ".request.json"
	responseSuffix    = ".response.json"

	kindSeeReplay = "see-replay"
	kindOpenURL   = "open-url"

	brokerPollInterval = 200 * time.Millisecond
	clientPollInterval = 50 * time.Millisecond
	clientTimeout      = 15 * time.Second
)

type brokerRequest struct {
	Kind string `json:"kind"`

	// See-replay fields.
	Source       string `json:"source,omitempty"`        // absolute path inside app-data
	ReplayFolder string `json:"replay_folder,omitempty"` // current ingest dir (write target parent)

	// open-url fields.
	URL string `json:"url,omitempty"`
}

type brokerResponse struct {
	Success bool   `json:"success"`
	Dest    string `json:"dest,omitempty"`
	Error   string `json:"error,omitempty"`
}

var brokerSeq atomic.Uint64

// brokerRoundtrip (worker side) drops req under appDataDir/broker and waits for
// the launcher's response.
func brokerRoundtrip(appDataDir string, req brokerRequest) (brokerResponse, error) {
	dir := filepath.Join(appDataDir, brokerDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return brokerResponse{}, fmt.Errorf("create broker dir: %w", err)
	}
	nonce := strconv.Itoa(os.Getpid()) + "-" + strconv.FormatUint(brokerSeq.Add(1), 10)
	reqPath := filepath.Join(dir, nonce+requestSuffix)
	respPath := filepath.Join(dir, nonce+responseSuffix)

	body, err := json.Marshal(req)
	if err != nil {
		return brokerResponse{}, err
	}
	if err := os.WriteFile(reqPath, body, 0o644); err != nil {
		return brokerResponse{}, fmt.Errorf("write broker request: %w", err)
	}
	defer os.Remove(reqPath)
	defer os.Remove(respPath)

	deadline := time.Now().Add(clientTimeout)
	for {
		if data, err := os.ReadFile(respPath); err == nil {
			var resp brokerResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return brokerResponse{}, fmt.Errorf("decode broker response: %w", err)
			}
			return resp, nil
		}
		if time.Now().After(deadline) {
			return brokerResponse{}, fmt.Errorf("broker request timed out after %s", clientTimeout)
		}
		time.Sleep(clientPollInterval)
	}
}

// BrokerSeeReplay (worker side) drops a request for the launcher to copy source
// into <replayFolder>/000_screpdb_watch_me/watch_me.rep and waits for the
// response. Returns the destination path on success.
func BrokerSeeReplay(appDataDir, source, replayFolder string) (string, error) {
	resp, err := brokerRoundtrip(appDataDir, brokerRequest{
		Kind:         kindSeeReplay,
		Source:       source,
		ReplayFolder: replayFolder,
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("broker: %s", resp.Error)
	}
	return resp.Dest, nil
}

// BrokerOpenURL (worker side) asks the Medium launcher to open url in the
// default browser. url must be a loopback dashboard URL; the launcher rejects
// anything else.
func BrokerOpenURL(appDataDir, url string) error {
	resp, err := brokerRoundtrip(appDataDir, brokerRequest{Kind: kindOpenURL, URL: url})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("broker: %s", resp.Error)
	}
	return nil
}

// StartBroker (launcher side) serves worker requests dropped under
// appDataDir/broker until ctx is cancelled or the returned stop func is called.
// It runs in its own goroutine.
func StartBroker(ctx context.Context, appDataDir string) (func(), error) {
	dir := filepath.Join(appDataDir, brokerDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return func() {}, fmt.Errorf("create broker dir: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer crashreport.GuardNonFatal(nil)
		ticker := time.NewTicker(brokerPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				serveBrokerRequests(dir)
			}
		}
	}()
	return cancel, nil
}

func serveBrokerRequests(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), requestSuffix) {
			continue
		}
		reqPath := filepath.Join(dir, e.Name())
		respPath := strings.TrimSuffix(reqPath, requestSuffix) + responseSuffix
		resp := handleBrokerRequest(reqPath)
		if body, err := json.Marshal(resp); err == nil {
			_ = os.WriteFile(respPath, body, 0o644)
		}
		_ = os.Remove(reqPath)
	}
}

func handleBrokerRequest(reqPath string) brokerResponse {
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return brokerResponse{Error: "read request: " + err.Error()}
	}
	var req brokerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return brokerResponse{Error: "decode request: " + err.Error()}
	}
	switch req.Kind {
	case kindOpenURL:
		return handleOpenURLRequest(req)
	case kindSeeReplay, "":
		return handleSeeRequest(req)
	default:
		return brokerResponse{Error: "unknown broker request kind: " + req.Kind}
	}
}

func handleSeeRequest(req brokerRequest) brokerResponse {
	// Source is the replay to stage (read-down access is always allowed, so the
	// launcher grants no new read capability); require it to be a regular file.
	if info, err := os.Stat(req.Source); err != nil || info.IsDir() {
		return brokerResponse{Error: "source is not a readable file"}
	}
	replayFolder := strings.TrimSpace(req.ReplayFolder)
	if replayFolder == "" {
		return brokerResponse{Error: "empty replay folder"}
	}
	if info, err := os.Stat(replayFolder); err != nil || !info.IsDir() {
		return brokerResponse{Error: "replay folder is not a directory"}
	}

	// The destination is fixed — a compromised worker cannot direct the
	// Medium launcher to write anywhere but this one file.
	destDir := filepath.Join(replayFolder, seeReplayFolder)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return brokerResponse{Error: "create dest dir: " + err.Error()}
	}
	dest := filepath.Join(destDir, seeReplayFilename)
	input, err := os.ReadFile(req.Source)
	if err != nil {
		return brokerResponse{Error: "read source: " + err.Error()}
	}
	if err := os.WriteFile(dest, input, 0o644); err != nil {
		return brokerResponse{Error: "write dest: " + err.Error()}
	}
	return brokerResponse{Success: true, Dest: dest}
}

func handleOpenURLRequest(req brokerRequest) brokerResponse {
	// Only ever open a URL the worker legitimately needs the Medium launcher to
	// open for it: the loopback dashboard, or a prefilled screpdb GitHub issue
	// from the crash reporter. This keeps the Low sandbox meaningful — a
	// compromised worker cannot hand the launcher an arbitrary string for
	// ShellExecute to turn into a launched program or a non-http protocol handler.
	if err := validateBrokerURL(req.URL); err != nil {
		return brokerResponse{Error: err.Error()}
	}
	if err := browser.OpenURL(req.URL); err != nil {
		return brokerResponse{Error: "open browser: " + err.Error()}
	}
	return brokerResponse{Success: true}
}

func validateBrokerURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	switch {
	case u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"):
		if port := u.Port(); port != "" {
			if _, err := strconv.Atoi(port); err != nil {
				return fmt.Errorf("invalid port %q", port)
			}
		}
		return nil
	case u.Scheme == "https" && u.Hostname() == "github.com" && strings.HasPrefix(u.Path, "/marianogappa/screpdb"):
		return nil
	default:
		return fmt.Errorf("refusing to open url %q", raw)
	}
}
