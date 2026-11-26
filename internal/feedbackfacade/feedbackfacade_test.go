package feedbackfacade

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var png = append([]byte{0x89, 'P', 'N', 'G'}, bytes.Repeat([]byte{1}, 16)...)

func TestSendPostsJSONPayload(t *testing.T) {
	var got payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := sendTo(context.Background(), srv.URL, Submission{
		Message:     "  the hotkeys tab is wrong  ",
		Contact:     "someone#discord",
		Language:    "ko",
		Screenshots: [][]byte{png},
	})
	if err != nil {
		t.Fatalf("sendTo: %v", err)
	}
	if got.Message != "the hotkeys tab is wrong" || got.Contact != "someone#discord" || got.Language != "ko" {
		t.Fatalf("unexpected payload %+v", got)
	}
	if got.Version == "" || !strings.Contains(got.OS, "/") {
		t.Fatalf("missing version or os: %+v", got)
	}
	if len(got.Screenshots) != 1 || got.Screenshots[0] != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("unexpected screenshots %v", got.Screenshots)
	}
}

func TestSendMapsStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusBadRequest, ErrInvalid},
		{http.StatusRequestEntityTooLarge, ErrInvalid},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
		}))
		err := sendTo(context.Background(), srv.URL, Submission{Message: "hi"})
		srv.Close()
		if !errors.Is(err, c.want) {
			t.Fatalf("status %d: got %v, want %v", c.status, err, c.want)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if err := sendTo(context.Background(), srv.URL, Submission{Message: "hi"}); err == nil || errors.Is(err, ErrInvalid) {
		t.Fatalf("502 should be a plain error, got %v", err)
	}
}

func TestSendDoesNotFollowRedirects(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	if err := sendTo(context.Background(), srv.URL, Submission{Message: "hi"}); err == nil {
		t.Fatal("redirect should be an error")
	}
	if hit {
		t.Fatal("redirect was followed")
	}
}

func TestBuildPayloadRejectsInvalidSubmissions(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0}
	cases := map[string]Submission{
		"empty message":        {Message: "   "},
		"long message":         {Message: strings.Repeat("가", MaxMessageChars+1)},
		"long contact":         {Message: "hi", Contact: strings.Repeat("a", MaxContactChars+1)},
		"long language":        {Message: "hi", Language: strings.Repeat("a", MaxLanguageChars+1)},
		"too many screenshots": {Message: "hi", Screenshots: [][]byte{jpeg, jpeg, jpeg, jpeg, jpeg}},
		"not an image":         {Message: "hi", Screenshots: [][]byte{[]byte("GIF89a")}},
		"too large":            {Message: "hi", Screenshots: [][]byte{append(append([]byte{}, png...), make([]byte, MaxScreenshotBytes)...)}},
	}
	for name, s := range cases {
		if _, err := buildPayload(s); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
	if _, err := buildPayload(Submission{Message: strings.Repeat("가", MaxMessageChars), Screenshots: [][]byte{jpeg}}); err != nil {
		t.Fatalf("limits should be inclusive: %v", err)
	}
}
