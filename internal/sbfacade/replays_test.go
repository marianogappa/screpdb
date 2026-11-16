package sbfacade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeReplay() []byte {
	data := make([]byte, 64)
	copy(data[replayMagicOffset:], replayMagic)
	return data
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestReplayURLAllowlist(t *testing.T) {
	good := []string{
		"https://cdn.shieldbattery.net/replays/01a1da27-b27b-7eb7-9ac4-9cc6e6d40b66.rep",
		"https://cdn.shieldbattery.net/replays/01a1da27-b27b-7eb7-9ac4-9cc6e6d40b66.rep?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=7f",
	}
	for _, u := range good {
		if err := checkReplayURL(u); err != nil {
			t.Errorf("%s refused: %v", u, err)
		}
	}
	bad := []string{
		"http://cdn.shieldbattery.net/replays/a.rep",
		"https://shieldbattery.net/replays/a.rep",
		"https://evil.example/replays/a.rep",
		"https://cdn.shieldbattery.net.evil.example/replays/a.rep",
		"https://cdn.shieldbattery.net:8443/replays/a.rep",
		"https://user:pw@cdn.shieldbattery.net/replays/a.rep",
		"https://cdn.shieldbattery.net/avatars/a.png",
		"https://cdn.shieldbattery.net/replays/",
		"https://cdn.shieldbattery.net/replays/../user-avatars/a.png",
		"https://cdn.shieldbattery.net/replays/..",
		"https://cdn.shieldbattery.net/replays/%2e%2e/a.png",
		"https://cdn.shieldbattery.net/replays/a%2Fb.rep",
		"https://cdn.shieldbattery.net/replays/sub/a.rep",
		"https://cdn.shieldbattery.net/replaysx/a.rep",
		"file:///replays/a.rep",
		"/replays/a.rep",
		"",
	}
	for _, u := range bad {
		if err := checkReplayURL(u); !errors.Is(err, ErrBadReplayURL) {
			t.Errorf("%q accepted: %v", u, err)
		}
		if _, err := DownloadReplay(context.Background(), u, hashOf(fakeReplay())); !errors.Is(err, ErrBadReplayURL) {
			t.Errorf("DownloadReplay %q: %v", u, err)
		}
	}
}

func TestDownloadReplayRefusesMalformedHash(t *testing.T) {
	if _, err := DownloadReplay(context.Background(), "https://cdn.shieldbattery.net/replays/a.rep", "85383a77"); !errors.Is(err, ErrInvalidReplay) {
		t.Fatalf("short hash: %v", err)
	}
}

func TestDownloadReplayVerifiesBytes(t *testing.T) {
	replay := fakeReplay()
	got, err := downloadReplayFrom(context.Background(), serve(t, http.StatusOK, string(replay))+"/replays/a.rep?sig=1", strings.ToUpper(hashOf(replay)))
	if err != nil || !bytes.Equal(got, replay) {
		t.Fatalf("good replay: %v", err)
	}

	if _, err := downloadReplayFrom(context.Background(), serve(t, http.StatusOK, string(replay)), hashOf([]byte("other"))); !errors.Is(err, ErrInvalidReplay) {
		t.Errorf("hash mismatch: %v", err)
	}
	notReplay := make([]byte, 64)
	if _, err := downloadReplayFrom(context.Background(), serve(t, http.StatusOK, string(notReplay)), hashOf(notReplay)); !errors.Is(err, ErrInvalidReplay) {
		t.Errorf("bad magic: %v", err)
	}
	short := []byte("seRS")
	if _, err := downloadReplayFrom(context.Background(), serve(t, http.StatusOK, string(short)), hashOf(short)); !errors.Is(err, ErrInvalidReplay) {
		t.Errorf("short: %v", err)
	}
	if _, err := downloadReplayFrom(context.Background(), serve(t, http.StatusNotFound, ""), hashOf(replay)); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
}

func TestDownloadReplayRefusesRedirects(t *testing.T) {
	replay := fakeReplay()
	target := serve(t, http.StatusOK, string(replay))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	if _, err := downloadReplayFrom(context.Background(), srv.URL+"/replays/a.rep", hashOf(replay)); err == nil {
		t.Fatal("redirect followed")
	}
}

func TestDownloadReplaySizeCap(t *testing.T) {
	big := make([]byte, maxReplaySize+1)
	copy(big[replayMagicOffset:], replayMagic)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(big)
	}))
	t.Cleanup(srv.Close)
	if _, err := downloadReplayFrom(context.Background(), srv.URL, hashOf(big)); !errors.Is(err, ErrInvalidReplay) {
		t.Fatalf("oversized: %v", err)
	}
}
