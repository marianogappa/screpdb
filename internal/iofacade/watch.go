//go:build !darwin

package iofacade

import "github.com/fsnotify/fsnotify"

type fsnotifyWatcher struct {
	watcher *fsnotify.Watcher
	events  chan FSEvent
	errs    chan error
}

// NewDirWatcher returns a DirWatcher backed by fsnotify. Every Add is checked
// against the allowlist like any other facade call.
func NewDirWatcher() (DirWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fw := &fsnotifyWatcher{
		watcher: watcher,
		events:  make(chan FSEvent, 256),
		errs:    make(chan error, 8),
	}
	go fw.forward()
	return fw, nil
}

func (f *fsnotifyWatcher) forward() {
	defer close(f.events)
	for {
		select {
		case ev, ok := <-f.watcher.Events:
			if !ok {
				return
			}
			op := mapFSOp(ev.Op)
			if op == 0 {
				continue
			}
			// Dropping under backpressure is deliberate: the consumer's
			// reconciliation walk is the source of truth, notifications only
			// lower latency, and blocking here would stall fsnotify's reader.
			select {
			case f.events <- FSEvent{Path: ev.Name, Op: op}:
			default:
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			select {
			case f.errs <- err:
			default:
			}
		}
	}
}

func mapFSOp(op fsnotify.Op) FSOp {
	switch {
	case op.Has(fsnotify.Create):
		return FSCreate
	case op.Has(fsnotify.Write):
		return FSWrite
	case op.Has(fsnotify.Remove):
		return FSRemove
	case op.Has(fsnotify.Rename):
		return FSRename
	}
	return 0
}

func (f *fsnotifyWatcher) Add(dir string) error {
	p, err := resolve(dir)
	if err != nil {
		return err
	}
	return f.watcher.Add(p)
}

func (f *fsnotifyWatcher) Remove(dir string) error {
	p, err := resolve(dir)
	if err != nil {
		return err
	}
	return f.watcher.Remove(p)
}

func (f *fsnotifyWatcher) Events() <-chan FSEvent { return f.events }

func (f *fsnotifyWatcher) Errors() <-chan error { return f.errs }

func (f *fsnotifyWatcher) Close() error { return f.watcher.Close() }
