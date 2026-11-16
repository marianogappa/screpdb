package sbfacade

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const settledGame = `{"game":{"id":"01a1da1c-8486-7801-827d-b56d61fe89e8","startTime":1794660664000,"config":{"teams":[[{"id":17185,"race":"t","isComputer":false},{"id":18606,"race":"t","isComputer":false}]],"gameType":"oneVOne","gameSource":"MATCHMAKING","gameSourceExtra":{"type":"1v1"}},"gameLength":719502,"results":[[17185,{"apm":0,"race":"t","result":"win"}],[18606,{"apm":0,"race":"t","result":"loss"}]],"canceledAt":null},"mmrChanges":[{"userId":17185,"matchmakingType":"1v1","rating":2366.9,"ratingChange":5.4,"outcome":"win"}],"replay":{"gameId":"01a1da1c-8486-7801-827d-b56d61fe89e8","id":"01a1da27-b27b-7eb7-9ac4-9cc6e6d40b66","url":"https://cdn.shieldbattery.net/replays/01a1da27-b27b-7eb7-9ac4-9cc6e6d40b66.rep?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc","hash":"85383a77","filename":"SB-1v1-Backrooms_1.1-1794660664"}}`

func serve(t *testing.T, status int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchGameReadsResultsByUserID(t *testing.T) {
	game, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, settledGame))
	if err != nil {
		t.Fatal(err)
	}
	if game.Results[17185] != ResultWin || game.Results[18606] != ResultLoss || game.Canceled {
		t.Fatalf("got %+v", game)
	}
	if !game.StartTime.Equal(time.UnixMilli(1794660664000)) || game.GameLength != 719502*time.Millisecond ||
		game.GameSource != SourceMatchmaking || game.MatchmakingType != "1v1" {
		t.Fatalf("got %+v", game)
	}
	if game.ReplayURL != "https://cdn.shieldbattery.net/replays/01a1da27-b27b-7eb7-9ac4-9cc6e6d40b66.rep?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc" ||
		game.ReplayHash != "85383a77" || game.ReplayFilename != "SB-1v1-Backrooms_1.1-1794660664" {
		t.Fatalf("replay %+v", game)
	}
	if err := checkReplayURL(game.ReplayURL); err != nil {
		t.Fatalf("real replay URL refused: %v", err)
	}
}

func TestFetchGameWithoutReplayOrLength(t *testing.T) {
	game, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, `{"game":{"id":"x","startTime":1794660664000,"config":{"gameSource":"LOBBY"},"gameLength":null,"results":null,"canceledAt":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if game.ReplayURL != "" || game.ReplayHash != "" || game.GameLength != 0 || game.GameSource != SourceLobby || game.MatchmakingType != "" {
		t.Fatalf("got %+v", game)
	}
}

func TestFetchGameBeforeResultsAreReconciled(t *testing.T) {
	game, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, `{"game":{"id":"x","results":null,"canceledAt":null}}`))
	if err != nil || game.Results != nil || game.Canceled {
		t.Fatalf("got %+v %v", game, err)
	}
	canceled, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, `{"game":{"id":"x","results":null,"canceledAt":1794661395994}}`))
	if err != nil || !canceled.Canceled {
		t.Fatalf("got %+v %v", canceled, err)
	}
}

func TestFetchGameErrors(t *testing.T) {
	if _, err := fetchGameFrom(context.Background(), serve(t, http.StatusNotFound, "")); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := fetchGameFrom(context.Background(), serve(t, http.StatusInternalServerError, "")); err == nil {
		t.Error("500 accepted")
	}
	if _, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, `{"game":{"results":[[1]]}}`)); err == nil {
		t.Error("malformed result accepted")
	}
	if _, err := fetchGameFrom(context.Background(), serve(t, http.StatusOK, `not json`)); err == nil {
		t.Error("non JSON accepted")
	}
}

func TestRateLimitPausesEveryRequest(t *testing.T) {
	saved := thePacer.next
	t.Cleanup(func() { thePacer.next, thePacer.paused = saved, time.Time{} })
	if _, err := fetchGameFrom(context.Background(), serve(t, http.StatusTooManyRequests, "")); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("429: %v", err)
	}
	if until := time.Until(thePacer.next); until < rateLimitPause-time.Second {
		t.Fatalf("paused only %v", until)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := thePacer.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait during pause: %v", err)
	}
	if _, pausedUntil := Usage(time.Now()); pausedUntil.IsZero() {
		t.Fatal("usage does not report the pause")
	}
}

func TestUsageCountsTodaysRequests(t *testing.T) {
	next, day, used := thePacer.next, thePacer.day, thePacer.used
	t.Cleanup(func() { thePacer.next, thePacer.day, thePacer.used = next, day, used })
	thePacer.next = time.Time{}
	before, _ := Usage(time.Now())
	if err := thePacer.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after, _ := Usage(time.Now()); after != before+1 {
		t.Fatalf("used %d after one request, %d before", after, before)
	}
	if used, _ := Usage(time.Now().AddDate(0, 0, 1)); used != 0 {
		t.Fatalf("tomorrow starts at %d", used)
	}
}

func TestFetchGameRefusesMalformedIDs(t *testing.T) {
	for _, id := range []string{"", "../users/1/profile", "01A1DA1C-8486-7801-827D-B56D61FE89E8", "01a1da1c-8486-7801-827d-b56d61fe89e8?x=1"} {
		if _, err := FetchGame(context.Background(), id); !errors.Is(err, ErrBadGameID) {
			t.Errorf("%q: %v", id, err)
		}
	}
}

func TestPacerSpacesRequests(t *testing.T) {
	p := &pacer{}
	start := time.Now()
	for range 2 {
		if err := p.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < requestInterval-100*time.Millisecond {
		t.Fatalf("two requests %v apart", elapsed)
	}
}
