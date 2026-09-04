package iofacade

import "errors"

// ErrWatchUnsupported is returned by NewDirWatcher on platforms where native
// directory notifications are not used; callers fall back to periodic
// reconciliation walks.
var ErrWatchUnsupported = errors.New("iofacade: directory watching unsupported on this platform")

type FSOp uint8

const (
	FSCreate FSOp = iota + 1
	FSWrite
	FSRemove
	FSRename
)

func (op FSOp) String() string {
	switch op {
	case FSCreate:
		return "create"
	case FSWrite:
		return "write"
	case FSRemove:
		return "remove"
	case FSRename:
		return "rename"
	}
	return "unknown"
}

type FSEvent struct {
	Path string
	Op   FSOp
}

// DirWatcher delivers native filesystem notifications for directories inside
// the permitted roots. It is non-recursive: callers add each directory they
// care about. Events may be dropped under load, so consumers must treat the
// stream as a hint and reconcile against a directory walk.
type DirWatcher interface {
	Add(dir string) error
	Remove(dir string) error
	Events() <-chan FSEvent
	Errors() <-chan error
	Close() error
}
