package db

import "database/sql"

// ErrNotFound is what a read returns when the thing asked for is not in the
// corpus. It aliases sql.ErrNoRows while the SQL store still exists so both
// implementations report a miss the same way; it becomes a standalone error
// when the SQL store is deleted.
var ErrNotFound = sql.ErrNoRows
