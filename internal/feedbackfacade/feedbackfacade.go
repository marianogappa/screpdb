// Package feedbackfacade is the sanctioned surface for sending the user's
// private feedback to the author from screpdb's Go binary.
//
// It makes exactly one kind of request: an HTTPS POST of a small JSON document
// to https://screpdb-feedback.spinetta.workers.dev/, a Cloudflare Worker (its
// source is in feedback-worker/) that relays it to a private Discord channel
// only the author can read. It sends only what the user typed and chose to
// attach (a message, an optional contact, at most four PNG or JPEG
// screenshots), plus the app version, the OS and the UI language. It sends
// nothing unless the user presses Send, never follows redirects and never
// sends credentials or cookies.
//
// This package is exempt from TestNoDirectIOOutsideFacades alongside
// internal/netfacade, internal/bnetfacade, internal/sbfacade and
// internal/selfupdate.
package feedbackfacade

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

const (
	endpoint = "https://screpdb-feedback.spinetta.workers.dev/"

	MaxMessageChars    = 4000
	MaxContactChars    = 200
	MaxLanguageChars   = 16
	MaxScreenshots     = 4
	MaxScreenshotBytes = 9 << 20

	maxReplyBytes = 4 << 10
)

var (
	ErrInvalid     = errors.New("feedbackfacade: invalid feedback")
	ErrRateLimited = errors.New("feedbackfacade: rate limited")

	pngMagic  = []byte{0x89, 'P', 'N', 'G'}
	jpegMagic = []byte{0xff, 0xd8, 0xff}

	httpClient = &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

type Submission struct {
	Message     string
	Contact     string
	Language    string
	Screenshots [][]byte
}

type payload struct {
	Message     string   `json:"message"`
	Contact     string   `json:"contact,omitempty"`
	Language    string   `json:"language,omitempty"`
	Version     string   `json:"version"`
	OS          string   `json:"os"`
	Screenshots []string `json:"screenshots,omitempty"`
}

func Send(ctx context.Context, s Submission) error {
	return sendTo(ctx, endpoint, s)
}

func sendTo(ctx context.Context, url string, s Submission) error {
	p, err := buildPayload(s)
	if err != nil {
		return err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge:
		return fmt.Errorf("%w: %s", ErrInvalid, strings.TrimSpace(string(reply)))
	default:
		return fmt.Errorf("feedbackfacade: %s", resp.Status)
	}
}

func buildPayload(s Submission) (payload, error) {
	message := strings.TrimSpace(s.Message)
	contact := strings.TrimSpace(s.Contact)
	language := strings.TrimSpace(s.Language)
	switch {
	case message == "":
		return payload{}, fmt.Errorf("%w: empty message", ErrInvalid)
	case utf8.RuneCountInString(message) > MaxMessageChars:
		return payload{}, fmt.Errorf("%w: message too long", ErrInvalid)
	case utf8.RuneCountInString(contact) > MaxContactChars:
		return payload{}, fmt.Errorf("%w: contact too long", ErrInvalid)
	case utf8.RuneCountInString(language) > MaxLanguageChars:
		return payload{}, fmt.Errorf("%w: language too long", ErrInvalid)
	case len(s.Screenshots) > MaxScreenshots:
		return payload{}, fmt.Errorf("%w: too many screenshots", ErrInvalid)
	}
	total := 0
	encoded := make([]string, 0, len(s.Screenshots))
	for _, shot := range s.Screenshots {
		if !bytes.HasPrefix(shot, pngMagic) && !bytes.HasPrefix(shot, jpegMagic) {
			return payload{}, fmt.Errorf("%w: screenshots must be PNG or JPEG", ErrInvalid)
		}
		total += len(shot)
		if total > MaxScreenshotBytes {
			return payload{}, fmt.Errorf("%w: screenshots too large", ErrInvalid)
		}
		encoded = append(encoded, base64.StdEncoding.EncodeToString(shot))
	}
	return payload{
		Message:     message,
		Contact:     contact,
		Language:    language,
		Version:     buildinfo.Version,
		OS:          runtime.GOOS + "/" + runtime.GOARCH,
		Screenshots: encoded,
	}, nil
}
