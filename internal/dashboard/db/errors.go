package db

import "errors"

// ErrNotFound is what a read returns when the thing asked for is not in the
// corpus.
var ErrNotFound = errors.New("not found in the replay library")
