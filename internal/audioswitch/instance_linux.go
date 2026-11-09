package audioswitch

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// EnsureSingleInstance locks a file in /tmp so only one instance runs.
// The returned function releases the lock.
func EnsureSingleInstance(appName string) (func(), error) {
	lockPath := filepath.Join(os.TempDir(), appName+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, ErrAlreadyRunning
	}
	return func() {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}, nil
}
