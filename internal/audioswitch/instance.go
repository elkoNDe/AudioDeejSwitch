package audioswitch

import "errors"

// ErrAlreadyRunning is returned when another instance holds the single-instance lock.
var ErrAlreadyRunning = errors.New("another instance is already running")
