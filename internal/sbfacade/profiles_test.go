package sbfacade

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

const profileSample = `{
"user":{"id":17185,"name":"K_BishOp","created":1793478563170,"avatarUrl":"https://cdn.shieldbattery.net/user-avatars/x.png"},
"profile":{"userId":17185,
 "ladder":{
  "1v1":{"userId":17185,"matchmakingType":"1v1","seasonId":11,"rank":4,"rating":2363.0508,"points":11829.285,"wins":130,"losses":57,"lifetimeGames":187,"pWins":0,"pLosses":0,"tWins":123,"tLosses":56,"zWins":3,"zLosses":0,"rWins":4,"rLosses":1,"rPWins":0,"rPLosses":0,"lastPlayedDate":1794783727810},
  "2v2":{"userId":17185,"matchmakingType":"2v2","seasonId":11,"rank":3,"rating":1988.9413,"wins":38,"losses":7,"lifetimeGames":45,"pWins":5,"pLosses":0,"tWins":9,"tLosses":1,"zWins":12,"zLosses":1,"rWins":12,"rLosses":5,"lastPlayedDate":1794784399305}},
 "seasonId":11,
 "userStats":{"userId":17185,"pWins":6,"pLosses":0,"tWins":132,"tLosses":57,"zWins":15,"zLosses":1,"rWins":20,"rLosses":6,"rPWins":4}},
"matchHistory":{
 "games":[
  {"id":"01a1e176-07ff-7825-b3df-0de400d512f4","startTime":1794783971325,"mapId":"m1","config":{"teams":[[{"id":14596,"race":"r","isComputer":false},{"id":14009,"race":"r","isComputer":false}],[{"id":17185,"race":"z","isComputer":false},{"id":17292,"race":"p","isComputer":false}]],"gameType":"topVBottom","gameSource":"MATCHMAKING","gameSourceExtra":{"type":"2v2","parties":[[14596],[14009],[17185],[17292]]}},"gameLength":427980,"results":[[14596,{"apm":0,"race":"z","result":"loss"}],[14009,{"apm":0,"race":"t","result":"loss"}],[17185,{"apm":0,"race":"z","result":"win"}],[17292,{"apm":0,"race":"p","result":"win"}]],"canceledAt":null},
  {"id":"01a1da1c-8486-7801-827d-b56d61fe89e8","startTime":1794660664000,"mapId":"m2","config":{"teams":[[{"id":17185,"race":"t","isComputer":false},{"id":18606,"race":"r","isComputer":false}]],"gameSource":"MATCHMAKING","gameSourceExtra":{"type":"1v1"}},"gameLength":null,"results":null,"canceledAt":null},
  {"id":"01a1da1c-0000-7801-827d-b56d61fe89e8","startTime":1794660000000,"mapId":"unknown","config":{"teams":[[{"id":17185,"race":"p","isComputer":false},{"id":0,"race":"z","isComputer":true}]],"gameSource":"LOBBY","gameSourceExtra":null},"gameLength":null,"results":null,"canceledAt":1794661395994}],
 "maps":[{"id":"m1","hash":"a5","name":"Fighting Spirit 1.4"},{"id":"m2","name":"Backrooms 1.1"}],
 "users":[{"id":14596,"name":"A"},{"id":14009,"name":"B"},{"id":17292,"name":"C"},{"id":18606,"name":"D"}],
 "replays":[]},
"seasons":[{"id":11,"name":"2026 - Season 2"}]}`

func TestFetchProfile(t *testing.T) {
	p, err := fetchProfileFrom(context.Background(), serve(t, http.StatusOK, profileSample))
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != 17185 || p.Name != "K_BishOp" || !p.Created.Equal(time.UnixMilli(1793478563170)) || p.SeasonID != 11 {
		t.Fatalf("user %+v", p)
	}
	one := p.Ladder["1v1"]
	if one.Rank != 4 || one.Rating != 2363.0508 || one.Wins != 130 || one.Losses != 57 || one.LifetimeGames != 187 ||
		!one.LastPlayed.Equal(time.UnixMilli(1794783727810)) || one.ByRace["t"] != (WinLoss{123, 56}) || one.ByRace["r"] != (WinLoss{4, 1}) {
		t.Fatalf("1v1 %+v", one)
	}
	if len(p.Ladder) != 2 || p.Ladder["2v2"].ByRace["z"] != (WinLoss{12, 1}) {
		t.Fatalf("ladder %+v", p.Ladder)
	}
	if p.Lifetime["t"] != (WinLoss{132, 57}) || p.Lifetime["r"] != (WinLoss{20, 6}) || p.Lifetime["p"] != (WinLoss{6, 0}) {
		t.Fatalf("lifetime %+v", p.Lifetime)
	}
	if len(p.RecentGames) != 3 {
		t.Fatalf("recent %+v", p.RecentGames)
	}

	settled := p.RecentGames[0]
	if settled.MapName != "Fighting Spirit 1.4" || settled.GameSource != SourceMatchmaking || settled.MatchmakingType != "2v2" ||
		settled.GameLength != 427980*time.Millisecond || !settled.StartTime.Equal(time.UnixMilli(1794783971325)) || settled.Canceled {
		t.Fatalf("settled %+v", settled)
	}
	wantPlayers := []RecentPlayer{
		{UserID: 14596, Name: "A", Race: "z", Team: 0, Result: ResultLoss},
		{UserID: 14009, Name: "B", Race: "t", Team: 0, Result: ResultLoss},
		{UserID: 17185, Name: "K_BishOp", Race: "z", Team: 1, Result: ResultWin},
		{UserID: 17292, Name: "C", Race: "p", Team: 1, Result: ResultWin},
	}
	if len(settled.Players) != len(wantPlayers) {
		t.Fatalf("players %+v", settled.Players)
	}
	for i, want := range wantPlayers {
		if settled.Players[i] != want {
			t.Errorf("player %d: got %+v want %+v", i, settled.Players[i], want)
		}
	}

	unsettled := p.RecentGames[1]
	if unsettled.GameLength != 0 || unsettled.MapName != "Backrooms 1.1" || unsettled.Players[1] != (RecentPlayer{UserID: 18606, Name: "D", Race: "r"}) {
		t.Fatalf("unsettled %+v", unsettled)
	}

	canceled := p.RecentGames[2]
	if !canceled.Canceled || canceled.MapName != "" || canceled.GameSource != SourceLobby || len(canceled.Players) != 1 {
		t.Fatalf("canceled %+v", canceled)
	}
}

func TestFetchProfileErrors(t *testing.T) {
	if _, err := fetchProfileFrom(context.Background(), serve(t, http.StatusNotFound, `{"error":"not found"}`)); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := fetchProfileFrom(context.Background(), serve(t, http.StatusOK, `{"matchHistory":{"games":[{"results":[[1]]}]}}`)); err == nil {
		t.Error("malformed result accepted")
	}
	if _, err := FetchProfile(context.Background(), 0); !errors.Is(err, ErrBadUserID) {
		t.Errorf("user 0: %v", err)
	}
}

func TestFetchUsersChunksAtFifty(t *testing.T) {
	var mu sync.Mutex
	var batches [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query()["u"]
		mu.Lock()
		batches = append(batches, ids)
		mu.Unlock()
		var resp struct {
			UserInfos []map[string]any `json:"userInfos"`
		}
		for _, id := range ids {
			if id == "99" {
				continue
			}
			n, _ := strconv.Atoi(id)
			resp.UserInfos = append(resp.UserInfos, map[string]any{"id": n, "name": "user" + id})
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	ids := make([]uint32, 0, 120)
	for i := 1; i <= 120; i++ {
		ids = append(ids, uint32(i))
	}
	waits := 0
	names, err := fetchUsersFrom(context.Background(), srv.URL, ids, func(context.Context) error { waits++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || len(batches[0]) != 50 || len(batches[1]) != 50 || len(batches[2]) != 20 || waits != 3 {
		t.Fatalf("batches %d (%d waits)", len(batches), waits)
	}
	if len(names) != 119 || names[1] != "user1" || names[120] != "user120" {
		t.Fatalf("names %d %q %q", len(names), names[1], names[120])
	}
	if _, ok := names[99]; ok {
		t.Fatal("unknown id resolved")
	}
}

func TestFetchUsersEmptyMakesNoRequest(t *testing.T) {
	names, err := fetchUsersFrom(context.Background(), "http://127.0.0.1:1", nil, func(context.Context) error {
		t.Fatal("waited for a request")
		return nil
	})
	if err != nil || len(names) != 0 {
		t.Fatalf("%v %v", names, err)
	}
	if _, err := FetchUsers(context.Background(), []uint32{1, 0}); !errors.Is(err, ErrBadUserID) {
		t.Fatalf("user 0: %v", err)
	}
}
