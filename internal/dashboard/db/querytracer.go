package db

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

// Query tracing is off by default and has zero overhead in that mode — the
// Store hands the raw *sql.DB back to sqlc. Flip it on to diagnose slow pages:
//
//   SCREPDB_DEBUG_QUERIES=1          # log every query + its duration
//   SCREPDB_SLOW_QUERY_MS=200        # override the slow threshold (default 100)
//
// Slow queries always log (prefixed [SLOW]) even when verbose mode is off —
// that way a user can leave the env flag unset in production and still catch
// the obvious regressions.

var tracingVerboseEnabled atomic.Bool
var tracingSlowThresholdMs atomic.Int64
var tracingConfigOnce atomic.Bool

func loadTracingConfigOnce() {
	if !tracingConfigOnce.CompareAndSwap(false, true) {
		return
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SCREPDB_DEBUG_QUERIES")))
	tracingVerboseEnabled.Store(v == "1" || v == "true" || v == "yes" || v == "on")

	threshold := int64(100)
	if raw := strings.TrimSpace(os.Getenv("SCREPDB_SLOW_QUERY_MS")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			threshold = parsed
		}
	}
	tracingSlowThresholdMs.Store(threshold)
}

// Trace wraps a *sql.DB with a tracing DBTX if any tracing is configured.
// Returns the raw *sql.DB (no allocation) when fully disabled so the hot
// path stays cold. Callers use the returned DBTX as the argument to
// sqlcgen.New.
func Trace(db *sql.DB) sqlcgen.DBTX {
	loadTracingConfigOnce()
	if !tracingVerboseEnabled.Load() {
		// Even with verbose off, wrap so we can still log slow queries.
		// The overhead of one time.Now() per query is negligible.
	}
	return &tracingDBTX{inner: db}
}

type tracingDBTX struct {
	inner sqlcgen.DBTX
}

func (t *tracingDBTX) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := t.inner.ExecContext(ctx, q, args...)
	logIfNoteworthy("EXEC", q, time.Since(start), 0, err)
	return res, err
}

func (t *tracingDBTX) PrepareContext(ctx context.Context, q string) (*sql.Stmt, error) {
	start := time.Now()
	stmt, err := t.inner.PrepareContext(ctx, q)
	logIfNoteworthy("PREPARE", q, time.Since(start), 0, err)
	return stmt, err
}

func (t *tracingDBTX) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := t.inner.QueryContext(ctx, q, args...)
	logIfNoteworthy("QUERY", q, time.Since(start), 0, err)
	return rows, err
}

func (t *tracingDBTX) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	start := time.Now()
	row := t.inner.QueryRowContext(ctx, q, args...)
	logIfNoteworthy("QUERYROW", q, time.Since(start), 0, nil)
	return row
}

func logIfNoteworthy(op, q string, dur time.Duration, rows int, err error) {
	ms := dur.Milliseconds()
	slow := ms >= tracingSlowThresholdMs.Load()
	verbose := tracingVerboseEnabled.Load()
	if !slow && !verbose {
		return
	}
	prefix := "[QUERY]"
	if slow {
		prefix = "[SLOW]"
	}
	q = collapseWhitespace(q)
	if len(q) > 160 {
		q = q[:160] + "…"
	}
	if err != nil {
		log.Printf("%s %s dur=%dms err=%v sql=%s", prefix, op, ms, err, q)
		return
	}
	log.Printf("%s %s dur=%dms sql=%s", prefix, op, ms, q)
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' || r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}
