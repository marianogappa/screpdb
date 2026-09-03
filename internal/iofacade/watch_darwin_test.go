package iofacade

import (
	"errors"
	"testing"
)

func TestDirWatcherUnsupportedOnDarwin(t *testing.T) {
	w, err := NewDirWatcher()
	if w != nil || !errors.Is(err, ErrWatchUnsupported) {
		t.Fatalf("NewDirWatcher() = %v, %v; want nil, ErrWatchUnsupported", w, err)
	}
}
