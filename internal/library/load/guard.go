package load

import (
	"fmt"
	"runtime/debug"
)

// guarded runs fn under a panic guard. Parsing and detection run a lot of
// data-dependent code against replays that can be old, truncated or from
// unusual game modes; without this one bad file would take down the whole
// load. The stack is included so a tester can paste it into a bug report.
//
// recover cannot catch a runtime fatal such as "concurrent map writes"; the
// parse and detect path is free of shared mutable state, so the only crash
// mode left here is an ordinary, recoverable panic.
func guarded(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while processing replay (this is a bug, please report it at "+
				"https://github.com/marianogappa/screpdb/issues): %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
