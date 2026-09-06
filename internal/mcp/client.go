package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/netfacade"
)

const (
	// requestTimeout bounds one API read. Game detail over a big corpus is the
	// slowest of them and still answers well inside this.
	requestTimeout = 30 * time.Second
	// maxResponseBytes caps what one call can return. Game detail is the
	// largest payload by far; anything past this is truncated with a note
	// rather than flooding the model's context.
	maxResponseBytes = 4 << 20
)

// Client reads the dashboard's JSON API over loopback. It owns no corpus
// state: every answer comes from the one process that loaded the replays.
type Client struct {
	addr string
}

// NewClient returns a client for a loopback host:port.
func NewClient(addr string) *Client { return &Client{addr: addr} }

func (c *Client) Addr() string { return c.addr }

// Get reads an API path (which may carry a query string) and returns the raw
// body. A non-2xx response is an error carrying the server's own message.
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	status, body, err := netfacade.LocalAPIGet(ctx, c.addr, path, requestTimeout, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf("GET %s: the dashboard answered %d: %s", path, status, firstLine(body))
	}
	return body, nil
}

// Health reads /api/health, the endpoint that reports corpus size and load
// progress.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var out Health
	body, err := c.Get(ctx, "/api/health")
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("parse /api/health: %w", err)
	}
	return out, nil
}

// Health is the subset of /api/health this package acts on.
type Health struct {
	App          string `json:"app"`
	TotalReplays int    `json:"total_replays"`
	Version      string `json:"version"`
	Library      struct {
		Phase     string `json:"phase"`
		Loaded    int    `json:"loaded"`
		Total     int    `json:"total"`
		Complete  bool   `json:"complete"`
		ReplayDir string `json:"replay_dir"`
	} `json:"library"`
}

func firstLine(body []byte) string {
	text := strings.TrimSpace(string(body))
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	if text == "" {
		return "(empty response)"
	}
	return text
}
