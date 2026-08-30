package bnetfacade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func swapBudgetsForTest(t *testing.T, b *budgets) {
	t.Helper()
	old := theBudgets
	theBudgets = b
	t.Cleanup(func() { theBudgets = old })
}

func fastBudgets(tokenInterval time.Duration, burst int) *budgets {
	b := newBudgets()
	b.bridgeLim = newLimiter(tokenInterval, burst)
	b.downloadLim = newLimiter(tokenInterval, burst)
	return b
}

func TestLimiterBurstThenWaits(t *testing.T) {
	l := newLimiter(50*time.Millisecond, 2)
	ctx := context.Background()

	start := time.Now()
	for range 2 {
		if err := l.acquire(ctx, PriorityBackground); err != nil {
			t.Fatalf("burst acquire: %v", err)
		}
	}
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("burst acquires should be immediate, took %v", elapsed)
	}

	start = time.Now()
	if err := l.acquire(ctx, PriorityBackground); err != nil {
		t.Fatalf("third acquire: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Fatalf("third acquire should have waited ~50ms, took %v", elapsed)
	}
}

func TestLimiterUserBeatsBackground(t *testing.T) {
	l := newLimiter(50*time.Millisecond, 1)
	ctx := context.Background()

	if err := l.acquire(ctx, PriorityBackground); err != nil {
		t.Fatalf("drain token: %v", err)
	}

	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	acquireAs := func(name string, prio Priority) {
		defer wg.Done()
		if err := l.acquire(ctx, prio); err != nil {
			t.Errorf("%s acquire: %v", name, err)
			return
		}
		mu.Lock()
		order = append(order, name)
		mu.Unlock()
	}

	wg.Add(1)
	go acquireAs("background", PriorityBackground)
	time.Sleep(10 * time.Millisecond) // background is queued first
	wg.Add(1)
	go acquireAs("user", PriorityUser)
	wg.Wait()

	if len(order) != 2 || order[0] != "user" {
		t.Fatalf("user should be served before the earlier-queued background waiter, got %v", order)
	}
}

func TestLimiterAcquireHonoursContextCancel(t *testing.T) {
	l := newLimiter(time.Hour, 1)
	if err := l.acquire(context.Background(), PriorityUser); err != nil {
		t.Fatalf("drain token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.acquire(ctx, PriorityUser); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want DeadlineExceeded", err)
	}
	if got := len(l.queues[PriorityUser]); got != 0 {
		t.Fatalf("abandoned waiter left in queue: %d", got)
	}
}

func TestBridgeDailyCapExhaustsAndRolls(t *testing.T) {
	b := fastBudgets(time.Millisecond, 10)
	b.bridgeCap = 2
	now := time.Date(2026, 8, 30, 23, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ctx := context.Background()

	for range 2 {
		if err := b.acquireBridge(ctx, PriorityUser); err != nil {
			t.Fatalf("acquire under cap: %v", err)
		}
	}
	if err := b.acquireBridge(ctx, PriorityUser); !errors.Is(err, ErrBridgeBudgetExhausted) {
		t.Fatalf("got %v, want ErrBridgeBudgetExhausted", err)
	}

	now = now.Add(2 * time.Hour) // past midnight
	if err := b.acquireBridge(ctx, PriorityUser); err != nil {
		t.Fatalf("acquire after day roll: %v", err)
	}
	if s := b.snapshot(); s.BridgeUsedToday != 1 {
		t.Fatalf("day roll should reset the counter, got %d", s.BridgeUsedToday)
	}
}

func TestDownloadBudgetIsSeparate(t *testing.T) {
	b := fastBudgets(time.Millisecond, 10)
	b.bridgeCap = 1
	ctx := context.Background()

	if err := b.acquireBridge(ctx, PriorityUser); err != nil {
		t.Fatalf("bridge acquire: %v", err)
	}
	if err := b.acquireBridge(ctx, PriorityUser); !errors.Is(err, ErrBridgeBudgetExhausted) {
		t.Fatalf("got %v, want ErrBridgeBudgetExhausted", err)
	}
	if err := b.acquireDownload(ctx, PriorityUser); err != nil {
		t.Fatalf("download budget must not be affected by an exhausted bridge budget: %v", err)
	}
}

func TestBridgeCooldownBackoff(t *testing.T) {
	b := fastBudgets(time.Millisecond, 10)
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ctx := context.Background()

	until := b.noteBridgeRateLimited()
	if got := until.Sub(now); got != cooldownBase {
		t.Fatalf("first cooldown: got %v, want %v", got, cooldownBase)
	}
	if err := b.acquireBridge(ctx, PriorityUser); !errors.Is(err, ErrBridgeCoolingDown) {
		t.Fatalf("got %v, want ErrBridgeCoolingDown", err)
	}
	if err := b.acquireDownload(ctx, PriorityUser); err != nil {
		t.Fatalf("downloads must not be blocked by a bridge cooldown: %v", err)
	}

	now = until.Add(time.Second)
	until = b.noteBridgeRateLimited()
	if got := until.Sub(now); got != 2*cooldownBase {
		t.Fatalf("second consecutive cooldown: got %v, want %v", got, 2*cooldownBase)
	}

	now = until.Add(time.Second)
	if err := b.acquireBridge(ctx, PriorityUser); err != nil {
		t.Fatalf("acquire after cooldown expiry: %v", err)
	}
	b.noteBridgeSuccess()
	until = b.noteBridgeRateLimited()
	if got := until.Sub(now); got != cooldownBase {
		t.Fatalf("cooldown after a success should reset to base: got %v, want %v", got, cooldownBase)
	}

	for range 20 {
		now = b.cooldownUntil.Add(time.Second)
		until = b.noteBridgeRateLimited()
	}
	if got := until.Sub(now); got != cooldownMax {
		t.Fatalf("cooldown must cap at %v, got %v", cooldownMax, got)
	}
}

func TestBudgetPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := iofacade.AllowDir(dir); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	path := filepath.Join(dir, budgetFileName)
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	b := fastBudgets(time.Millisecond, 10)
	b.now = func() time.Time { return now }
	if err := b.enablePersistence(path); err != nil {
		t.Fatalf("enablePersistence: %v", err)
	}
	ctx := context.Background()
	for range 3 {
		if err := b.acquireBridge(ctx, PriorityUser); err != nil {
			t.Fatalf("acquire: %v", err)
		}
	}
	if err := b.acquireDownload(ctx, PriorityUser); err != nil {
		t.Fatalf("download acquire: %v", err)
	}
	cooldownUntil := b.noteBridgeRateLimited()

	restarted := fastBudgets(time.Millisecond, 10)
	restarted.now = func() time.Time { return now.Add(time.Minute) }
	if err := restarted.enablePersistence(path); err != nil {
		t.Fatalf("enablePersistence after restart: %v", err)
	}
	s := restarted.snapshot()
	if s.BridgeUsedToday != 3 || s.DownloadsUsedToday != 1 {
		t.Fatalf("restart must not reset counters: got bridge=%d downloads=%d", s.BridgeUsedToday, s.DownloadsUsedToday)
	}
	if !s.CooldownUntil.Equal(cooldownUntil) {
		t.Fatalf("restart must not reset cooldown: got %v, want %v", s.CooldownUntil, cooldownUntil)
	}

	nextDay := fastBudgets(time.Millisecond, 10)
	nextDay.now = func() time.Time { return now.Add(24 * time.Hour) }
	if err := nextDay.enablePersistence(path); err != nil {
		t.Fatalf("enablePersistence next day: %v", err)
	}
	if s := nextDay.snapshot(); s.BridgeUsedToday != 0 {
		t.Fatalf("stale counters from a previous day must not be restored: got %d", s.BridgeUsedToday)
	}
}

func TestBridgeGetRateLimitedResponseTriggersCooldown(t *testing.T) {
	swapBudgetsForTest(t, fastBudgets(time.Millisecond, 10))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `"Rate Limited"`)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()
	_, err := BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser)
	if !errors.Is(err, ErrBridgeRateLimited) {
		t.Fatalf("got %v, want ErrBridgeRateLimited", err)
	}
	_, err = BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser)
	if !errors.Is(err, ErrBridgeCoolingDown) {
		t.Fatalf("got %v, want ErrBridgeCoolingDown", err)
	}
	if s := BudgetSnapshot(); s.CooldownUntil.IsZero() {
		t.Fatal("snapshot should expose the cooldown")
	}
}

func TestBridgeGetSpendsBudget(t *testing.T) {
	b := fastBudgets(time.Millisecond, 10)
	swapBudgetsForTest(t, b)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()
	if _, err := BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser); err != nil {
		t.Fatalf("BridgeGet: %v", err)
	}
	if s := BudgetSnapshot(); s.BridgeUsedToday != 1 {
		t.Fatalf("BridgeGet must spend the bridge budget: got %d", s.BridgeUsedToday)
	}

	b.bridgeCap = 1
	_, err := BridgeGet(context.Background(), addr, "/web-api/v1/profile", PriorityUser)
	if !errors.Is(err, ErrBridgeBudgetExhausted) {
		t.Fatalf("got %v, want ErrBridgeBudgetExhausted", err)
	}
}

func TestIsRateLimitedResponse(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   bool
	}{
		{http.StatusTooManyRequests, "", true},
		{http.StatusOK, `"Rate Limited"`, true},
		{http.StatusOK, "rate limited", true},
		{http.StatusOK, `{"name":"Rate Limited Andy","games":[...padding to exceed the small-payload cutoff...]}`, false},
		{http.StatusOK, `{"ok":true}`, false},
	}
	for _, c := range cases {
		if got := isRateLimitedResponse(c.status, []byte(c.body)); got != c.want {
			t.Errorf("isRateLimitedResponse(%d, %q) = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}
