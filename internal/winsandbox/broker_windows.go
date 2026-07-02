//go:build windows

package winsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
)

// The See-replay ("watch me") feature stages a replay into the user's replays
// folder so StarCraft can load it. That folder is read-only to the Low worker,
// so the worker asks the Medium launcher to perform the single write via a
// file-drop broker under the (Low-writable) app-data dir.
//
// Fixed, non-negotiable destination — the launcher never honors a
// worker-supplied filename/subpath, so a compromised worker can at worst
// overwrite this one file with bytes it could already read from app-data.
const (
	brokerDirName     = "broker"
	seeReplayFolder   = "000_screpdb_watch_me"
	seeReplayFilename = "watch_me.rep"
	requestSuffix     = ".request.json"
	responseSuffix    = ".response.json"

	brokerPollInterval = 200 * time.Millisecond
	clientPollInterval = 50 * time.Millisecond
	clientTimeout      = 15 * time.Second
)

type seeRequest struct {
	Source       string `json:"source"`        // absolute path inside app-data
	ReplayFolder string `json:"replay_folder"` // current ingest dir (write target parent)
}

type seeResponse struct {
	Success bool   `json:"success"`
	Dest    string `json:"dest,omitempty"`
	Error   string `json:"error,omitempty"`
}

var brokerSeq atomic.Uint64

// BrokerSeeReplay (worker side) drops a request for the launcher to copy source
// into <replayFolder>/000_screpdb_watch_me/watch_me.rep and waits for the
// response. Returns the destination path on success.
func BrokerSeeReplay(appDataDir, source, replayFolder string) (string, error) {
	dir := filepath.Join(appDataDir, brokerDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create broker dir: %w", err)
	}
	nonce := strconv.Itoa(os.Getpid()) + "-" + strconv.FormatUint(brokerSeq.Add(1), 10)
	reqPath := filepath.Join(dir, nonce+requestSuffix)
	respPath := filepath.Join(dir, nonce+responseSuffix)

	body, err := json.Marshal(seeRequest{Source: source, ReplayFolder: replayFolder})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(reqPath, body, 0o644); err != nil {
		return "", fmt.Errorf("write broker request: %w", err)
	}
	defer os.Remove(reqPath)
	defer os.Remove(respPath)

	deadline := time.Now().Add(clientTimeout)
	for {
		if data, err := os.ReadFile(respPath); err == nil {
			var resp seeResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return "", fmt.Errorf("decode broker response: %w", err)
			}
			if !resp.Success {
				return "", fmt.Errorf("broker: %s", resp.Error)
			}
			return resp.Dest, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("broker request timed out after %s", clientTimeout)
		}
		time.Sleep(clientPollInterval)
	}
}

// StartBroker (launcher side) serves See-replay requests dropped under
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
		resp := handleSeeRequest(reqPath)
		if body, err := json.Marshal(resp); err == nil {
			_ = os.WriteFile(respPath, body, 0o644)
		}
		_ = os.Remove(reqPath)
	}
}

func handleSeeRequest(reqPath string) seeResponse {
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return seeResponse{Error: "read request: " + err.Error()}
	}
	var req seeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return seeResponse{Error: "decode request: " + err.Error()}
	}
	// Source is the replay to stage (read-down access is always allowed, so the
	// launcher grants no new read capability); require it to be a regular file.
	if info, err := os.Stat(req.Source); err != nil || info.IsDir() {
		return seeResponse{Error: "source is not a readable file"}
	}
	replayFolder := strings.TrimSpace(req.ReplayFolder)
	if replayFolder == "" {
		return seeResponse{Error: "empty replay folder"}
	}
	if info, err := os.Stat(replayFolder); err != nil || !info.IsDir() {
		return seeResponse{Error: "replay folder is not a directory"}
	}

	// The destination is fixed — a compromised worker cannot direct the
	// Medium launcher to write anywhere but this one file.
	destDir := filepath.Join(replayFolder, seeReplayFolder)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return seeResponse{Error: "create dest dir: " + err.Error()}
	}
	dest := filepath.Join(destDir, seeReplayFilename)
	input, err := os.ReadFile(req.Source)
	if err != nil {
		return seeResponse{Error: "read source: " + err.Error()}
	}
	if err := os.WriteFile(dest, input, 0o644); err != nil {
		return seeResponse{Error: "write dest: " + err.Error()}
	}
	return seeResponse{Success: true, Dest: dest}
}
