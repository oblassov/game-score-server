package filesystem

import "errors"

// ErrLockTimeout is returned when the db lock can't be taken in time.
var ErrLockTimeout = errors.New("timed out waiting for the db lock")
