package bnetfacade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"unicode/utf8"
)

const profileFixture = `{
	"aurora_id": 1234567,
	"battle_tag": "Player#1234",
	"country_code": "KR",
	"game_results": [{"game_id": "g1"}],
	"replays": [{"md5": "abc", "map_title": "%s"}],
	"toons": [{"toon": "Player", "games_last_week": 3}],
	"matchmaked_stats": [{"rating": 1500}],
	"stats": [{"race": "zerg"}]
}`

func TestFetchAuroraProfile_Success(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		// EUC-KR encoding of Korean characters makes the payload invalid UTF-8.
		cp949Seoul := []byte{0xBC, 0xAD, 0xBF, 0xEF}
		fmt.Fprintf(w, profileFixture, cp949Seoul)
	}))
	defer srv.Close()

	p, err := FetchAuroraProfile(context.Background(), srv.Listener.Addr().String(), "Player", 30, PriorityUser)
	if err != nil {
		t.Fatalf("FetchAuroraProfile: %v", err)
	}
	if gotPath != "/web-api/v2/aurora-profile-by-toon/Player/30" {
		t.Errorf("path: got %q", gotPath)
	}
	if gotQuery != "request_flags=scr_profile" {
		t.Errorf("query: got %q", gotQuery)
	}
	if !p.Found() {
		t.Error("Found() = false, want true")
	}
	if p.AuroraID != 1234567 || p.BattleTag != "Player#1234" || p.CountryCode != "KR" {
		t.Errorf("scalars: got %d %q %q", p.AuroraID, p.BattleTag, p.CountryCode)
	}
	if len(p.GameResults) == 0 || len(p.Replays) == 0 || len(p.Toons) == 0 || len(p.MatchmakedStats) == 0 || len(p.Stats) == 0 {
		t.Error("expected all raw sections populated")
	}
	if !utf8.Valid(p.Raw) {
		t.Error("Raw is not valid UTF-8 after normalization")
	}
}

func TestFetchAuroraProfile_UnknownToon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"aurora_id": 0, "game_results": [], "replays": [], "toons": [], "matchmaked_stats": [], "stats": []}`)
	}))
	defer srv.Close()

	p, err := FetchAuroraProfile(context.Background(), srv.Listener.Addr().String(), "NoSuchToon", 20, PriorityUser)
	if err != nil {
		t.Fatalf("FetchAuroraProfile: %v", err)
	}
	if p.Found() {
		t.Error("Found() = true for aurora_id 0, want false")
	}
}

func TestFetchAuroraProfile_UnknownGateway(t *testing.T) {
	_, err := FetchAuroraProfile(context.Background(), "127.0.0.1:6119", "Player", 99, PriorityUser)
	if !errors.Is(err, ErrUnknownGateway) {
		t.Errorf("got %v, want ErrUnknownGateway", err)
	}
}

func TestFetchAuroraProfile_EscapesToon(t *testing.T) {
	var gotEscaped string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscaped = r.URL.EscapedPath()
		fmt.Fprint(w, `{"aurora_id": 1}`)
	}))
	defer srv.Close()

	if _, err := FetchAuroraProfile(context.Background(), srv.Listener.Addr().String(), "with space/slash", 10, PriorityUser); err != nil {
		t.Fatalf("FetchAuroraProfile: %v", err)
	}
	if gotEscaped != "/web-api/v2/aurora-profile-by-toon/with%20space%2Fslash/10" {
		t.Errorf("escaped path: got %q", gotEscaped)
	}
}
