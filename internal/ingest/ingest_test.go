package ingest

import (
	"errors"
	"strings"
	"testing"
)

func TestRunGuardedPassesThroughSuccess(t *testing.T) {
	if err := runGuarded(func() error { return nil }); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRunGuardedPassesThroughError(t *testing.T) {
	sentinel := errors.New("ordinary failure")
	err := runGuarded(func() error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the original error to pass through, got %v", err)
	}
}

func TestRunGuardedRecoversPanic(t *testing.T) {
	err := runGuarded(func() error {
		var p *int
		_ = *p // nil dereference panic, like a malformed replay could trigger
		return nil
	})
	if err == nil {
		t.Fatal("expected a recovered panic to surface as an error, got nil")
	}
	if !strings.Contains(err.Error(), "panic while processing replay") {
		t.Errorf("error should identify the panic; got: %v", err)
	}
	if !strings.Contains(err.Error(), "github.com/marianogappa/screpdb/issues") {
		t.Errorf("error should point the user at the issue tracker; got: %v", err)
	}
	// The recovered error must carry a stack trace for bug reports.
	if !strings.Contains(err.Error(), "ingest.go") {
		t.Errorf("error should include a stack trace; got: %v", err)
	}
}
