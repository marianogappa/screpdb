//go:build js

package iofacade

func NewDirWatcher() (DirWatcher, error) {
	return nil, ErrWatchUnsupported
}
