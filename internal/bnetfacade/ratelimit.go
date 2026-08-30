package bnetfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

// Priority orders waiters on the rate-limiter queues. A user-initiated call
// (someone clicked a player) is always served before any queued background
// sweep.
type Priority int

const (
	PriorityBackground Priority = iota
	PriorityUser
)

var (
	ErrBridgeBudgetExhausted   = errors.New("bnetfacade: bridge daily request budget exhausted")
	ErrDownloadBudgetExhausted = errors.New("bnetfacade: replay download daily budget exhausted")
	ErrBridgeCoolingDown       = errors.New("bnetfacade: bridge cooling down after rate limiting")
	ErrBridgeRateLimited       = errors.New("bnetfacade: bridge reported rate limiting")
)

// Bridge calls ride the user's Blizzard session: being chatty risks a mid-game
// disconnection, so the sustained rate stays conservative and the daily cap
// survives restarts. The burst covers a game-detail page (8 players) plus a
// couple of clicks; the daily cap covers a once-per-day sweep of the several
// hundred distinct players a ~1000-replay database typically holds, with
// headroom for interactive browsing. GCS downloads never touch that session —
// they cost bandwidth, not disconnection risk — hence the separate budget.
const (
	bridgeTokenInterval = 2 * time.Second
	bridgeBurst         = 12
	bridgeDailyCap      = 600

	downloadTokenInterval = 2 * time.Second
	downloadBurst         = 6
	downloadDailyCap      = 750

	cooldownBase = 15 * time.Minute
	cooldownMax  = 6 * time.Hour

	budgetFileName = "bnet_budget.json"
)

// limiter is a token bucket with two priority-ordered FIFO wait queues.
type limiter struct {
	mu            sync.Mutex
	tokenInterval time.Duration
	burst         float64
	tokens        float64
	last          time.Time
	queues        [2][]chan struct{}
	timer         *time.Timer
}

func newLimiter(tokenInterval time.Duration, burst int) *limiter {
	return &limiter{
		tokenInterval: tokenInterval,
		burst:         float64(burst),
		tokens:        float64(burst),
		last:          time.Now(),
	}
}

func (l *limiter) refillLocked(now time.Time) {
	if !now.After(l.last) {
		return
	}
	l.tokens = math.Min(l.burst, l.tokens+float64(now.Sub(l.last))/float64(l.tokenInterval))
	l.last = now
}

func (l *limiter) acquire(ctx context.Context, prio Priority) error {
	l.mu.Lock()
	l.refillLocked(time.Now())
	if l.tokens >= 1 && !l.hasWaitersAheadLocked(prio) {
		l.tokens--
		l.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	l.queues[prio] = append(l.queues[prio], ch)
	l.scheduleDispatchLocked()
	l.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		l.abandon(prio, ch)
		return ctx.Err()
	}
}

func (l *limiter) hasWaitersAheadLocked(prio Priority) bool {
	if prio == PriorityUser {
		return len(l.queues[PriorityUser]) > 0
	}
	return len(l.queues[PriorityUser]) > 0 || len(l.queues[PriorityBackground]) > 0
}

func (l *limiter) dispatch() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refillLocked(time.Now())
	for l.tokens >= 1 {
		var ch chan struct{}
		switch {
		case len(l.queues[PriorityUser]) > 0:
			ch, l.queues[PriorityUser] = l.queues[PriorityUser][0], l.queues[PriorityUser][1:]
		case len(l.queues[PriorityBackground]) > 0:
			ch, l.queues[PriorityBackground] = l.queues[PriorityBackground][0], l.queues[PriorityBackground][1:]
		default:
			return
		}
		l.tokens--
		close(ch)
	}
	l.scheduleDispatchLocked()
}

func (l *limiter) scheduleDispatchLocked() {
	if len(l.queues[PriorityUser])+len(l.queues[PriorityBackground]) == 0 {
		return
	}
	wait := max(time.Duration((1-l.tokens)*float64(l.tokenInterval)), time.Millisecond)
	if l.timer == nil {
		l.timer = time.AfterFunc(wait, l.dispatch)
	} else {
		l.timer.Reset(wait)
	}
}

// abandon removes a waiter whose context was cancelled. If the waiter was
// granted a token between cancellation and removal, the token is refunded.
func (l *limiter) abandon(prio Priority, ch chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	q := l.queues[prio]
	for i, c := range q {
		if c == ch {
			l.queues[prio] = append(q[:i], q[i+1:]...)
			return
		}
	}
	l.tokens = math.Min(l.burst, l.tokens+1)
}

// budgets holds both daily budgets, the bridge cooldown state, and the
// optional persistence path. Enforcement is in-memory always; persistence only
// makes the daily counters and the cooldown survive restarts.
type budgets struct {
	bridgeLim   *limiter
	downloadLim *limiter

	mu            sync.Mutex
	now           func() time.Time
	day           string
	bridgeUsed    int
	downloadsUsed int
	bridgeCap     int
	downloadCap   int
	cooldownN     int
	cooldownUntil time.Time
	persistPath   string
}

func newBudgets() *budgets {
	return &budgets{
		bridgeLim:   newLimiter(bridgeTokenInterval, bridgeBurst),
		downloadLim: newLimiter(downloadTokenInterval, downloadBurst),
		now:         time.Now,
		bridgeCap:   bridgeDailyCap,
		downloadCap: downloadDailyCap,
	}
}

var theBudgets = newBudgets()

type budgetFile struct {
	Day                 string    `json:"day"`
	BridgeUsed          int       `json:"bridge_used"`
	DownloadsUsed       int       `json:"downloads_used"`
	CooldownUntil       time.Time `json:"cooldown_until,omitzero"`
	CooldownConsecutive int       `json:"cooldown_consecutive,omitempty"`
}

// EnableBudgetPersistence loads any persisted daily counters and cooldown from
// the app-data root and arms saving, so restarting screpdb does not reset the
// daily caps. Without this call the budgets still enforce, from zero, in
// memory only.
func EnableBudgetPersistence() error {
	dir, err := appdata.Dir()
	if err != nil {
		return err
	}
	return theBudgets.enablePersistence(filepath.Join(dir, budgetFileName))
}

func (b *budgets) enablePersistence(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	b.rollDayLocked(now)
	if data, err := iofacade.ReadFile(path); err == nil {
		var f budgetFile
		if err := json.Unmarshal(data, &f); err == nil {
			if f.Day == b.day {
				b.bridgeUsed = f.BridgeUsed
				b.downloadsUsed = f.DownloadsUsed
			}
			if f.CooldownUntil.After(now) {
				b.cooldownUntil = f.CooldownUntil
				b.cooldownN = f.CooldownConsecutive
			}
		}
	}
	b.persistPath = path
	b.saveLocked()
	return nil
}

func (b *budgets) rollDayLocked(now time.Time) {
	day := now.Format("2006-01-02")
	if day == b.day {
		return
	}
	b.day = day
	b.bridgeUsed = 0
	b.downloadsUsed = 0
}

func (b *budgets) saveLocked() {
	if b.persistPath == "" {
		return
	}
	data, err := json.Marshal(budgetFile{
		Day:                 b.day,
		BridgeUsed:          b.bridgeUsed,
		DownloadsUsed:       b.downloadsUsed,
		CooldownUntil:       b.cooldownUntil,
		CooldownConsecutive: b.cooldownN,
	})
	if err != nil {
		return
	}
	// Best-effort: a failed save costs at most one day's counter on restart.
	_ = iofacade.WriteFile(b.persistPath, data, 0o644)
}

func (b *budgets) acquireBridge(ctx context.Context, prio Priority) error {
	b.mu.Lock()
	now := b.now()
	b.rollDayLocked(now)
	if now.Before(b.cooldownUntil) {
		until := b.cooldownUntil
		b.mu.Unlock()
		return fmt.Errorf("%w (until %s)", ErrBridgeCoolingDown, until.Format(time.RFC3339))
	}
	if b.bridgeUsed >= b.bridgeCap {
		b.mu.Unlock()
		return fmt.Errorf("%w (%d today)", ErrBridgeBudgetExhausted, b.bridgeCap)
	}
	b.bridgeUsed++
	b.saveLocked()
	b.mu.Unlock()

	if err := b.bridgeLim.acquire(ctx, prio); err != nil {
		b.mu.Lock()
		b.bridgeUsed--
		b.saveLocked()
		b.mu.Unlock()
		return err
	}
	return nil
}

func (b *budgets) acquireDownload(ctx context.Context, prio Priority) error {
	b.mu.Lock()
	b.rollDayLocked(b.now())
	if b.downloadsUsed >= b.downloadCap {
		b.mu.Unlock()
		return fmt.Errorf("%w (%d today)", ErrDownloadBudgetExhausted, b.downloadCap)
	}
	b.downloadsUsed++
	b.saveLocked()
	b.mu.Unlock()

	if err := b.downloadLim.acquire(ctx, prio); err != nil {
		b.mu.Lock()
		b.downloadsUsed--
		b.saveLocked()
		b.mu.Unlock()
		return err
	}
	return nil
}

func (b *budgets) noteBridgeRateLimited() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cooldownN++
	shift := min(b.cooldownN-1, 8)
	d := min(cooldownBase<<shift, cooldownMax)
	b.cooldownUntil = b.now().Add(d)
	b.saveLocked()
	return b.cooldownUntil
}

func (b *budgets) noteBridgeSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cooldownN == 0 && b.cooldownUntil.IsZero() {
		return
	}
	b.cooldownN = 0
	b.cooldownUntil = time.Time{}
	b.saveLocked()
}

// BudgetStatus is a read-only snapshot of both daily budgets, surfaced to the
// dashboard's "requests today" meter.
type BudgetStatus struct {
	BridgeUsedToday    int
	BridgeDailyCap     int
	DownloadsUsedToday int
	DownloadsDailyCap  int
	CooldownUntil      time.Time
}

func BudgetSnapshot() BudgetStatus {
	return theBudgets.snapshot()
}

func (b *budgets) snapshot() BudgetStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	b.rollDayLocked(now)
	s := BudgetStatus{
		BridgeUsedToday:    b.bridgeUsed,
		BridgeDailyCap:     b.bridgeCap,
		DownloadsUsedToday: b.downloadsUsed,
		DownloadsDailyCap:  b.downloadCap,
	}
	if now.Before(b.cooldownUntil) {
		s.CooldownUntil = b.cooldownUntil
	}
	return s
}

// isRateLimitedResponse detects the bridge's explicit throttling signal:
// either HTTP 429 or the distinct "Rate Limited" status string the bridge
// returns as a tiny payload. Only small bodies are matched so a genuine
// payload containing the phrase (e.g. inside a player name) is never mistaken
// for throttling.
func isRateLimitedResponse(statusCode int, body []byte) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	const maxStatusPayload = 64
	if len(body) > maxStatusPayload {
		return false
	}
	return strings.Contains(strings.ToLower(string(body)), "rate limited")
}
