package iofacade

// NewDirWatcher is unsupported on macOS: fsnotify's kqueue backend opens one
// descriptor per file inside every watched directory, which a replay folder
// with thousands of games would exhaust. Callers reconcile with periodic walks.
func NewDirWatcher() (DirWatcher, error) {
	return nil, ErrWatchUnsupported
}
