package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRecoveryMiddleware_PanicBecomes500(t *testing.T) {
	// A panicking handler must be contained as a 500 rather than crashing the
	// process. (The crash-report file itself is covered by crashreport's tests.)
	// chdir so any crash-report file lands in a temp dir, not the package dir.
	dir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom in handler")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)

	recoveryMiddleware(panicking).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after handler panic, got %d", rec.Code)
	}
}

func TestRecoveryMiddleware_PassesThroughNonPanic(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)

	recoveryMiddleware(ok).ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("non-panicking handler should pass through, got %d", rec.Code)
	}
}
