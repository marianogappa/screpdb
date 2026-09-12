package bnetfacade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const gameInfoFixture = `{
	"avatars": {},
	"players": [],
	"replays": [
		{
			"attributes": {"replay_result": "1", "replay_humans": "8"},
			"create_time": 1787492037,
			"link": "",
			"md5": "fd0265b18d9072002bb991f765ae3e75",
			"url": "https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/fd0265b18d9072002bb991f765ae3e75.replay"
		},
		{
			"attributes": {},
			"create_time": 1787492099,
			"md5": "0123456789abcdef0123456789abcdef",
			"url": "https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/3/4/0123456789abcdef0123456789abcdef.replay"
		}
	]
}`

func TestFetchGameInfo_Success(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, gameInfoFixture)
	}))
	defer srv.Close()

	info, err := FetchGameInfo(context.Background(), srv.Listener.Addr().String(), "MM-97F83F0C", PriorityBackground)
	if err != nil {
		t.Fatalf("FetchGameInfo: %v", err)
	}
	if gotPath != "/web-api/v1/matchmaker-gameinfo-playerinfo/MM-97F83F0C" {
		t.Errorf("path: got %q", gotPath)
	}
	if len(info.Replays) != 2 {
		t.Fatalf("replays: got %d", len(info.Replays))
	}
	first := info.Replays[0]
	if first.MD5 != "fd0265b18d9072002bb991f765ae3e75" || first.CreateTime != 1787492037 {
		t.Errorf("first entry: %+v", first)
	}
	if first.Attributes["replay_result"] != "1" {
		t.Errorf("attributes: %v", first.Attributes)
	}
}

func TestFetchGameInfo_EmptyGameID(t *testing.T) {
	if _, err := FetchGameInfo(context.Background(), "127.0.0.1:1", " ", PriorityBackground); err == nil {
		t.Fatal("expected error for empty game id")
	}
}

func TestGCSReplayPath(t *testing.T) {
	path, err := GCSReplayPath("https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/abc.replay")
	if err != nil {
		t.Fatalf("GCSReplayPath: %v", err)
	}
	if path != "/starcraft-user-uploads-prod/S1-replays/1/2/abc.replay" {
		t.Errorf("path: got %q", path)
	}
	for _, bad := range []string{
		"https://evil.example.com/starcraft-user-uploads-prod/S1-replays/1/2/abc.replay",
		"http://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/abc.replay",
		"https://storage.googleapis.com/other-bucket/abc.replay",
	} {
		if _, err := GCSReplayPath(bad); err == nil {
			t.Errorf("GCSReplayPath(%q) accepted", bad)
		}
	}
}

func TestHeadReplayRefusesBadPath(t *testing.T) {
	if _, err := HeadReplay(context.Background(), "/etc/passwd"); !errors.Is(err, ErrForbiddenPath) {
		t.Fatalf("err: %v", err)
	}
}
