//go:build unix

package filesystem

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func lockShared(file *os.File) error {
	return flock(file, syscall.LOCK_SH)
}

func lockExclusive(file *os.File) error {
	return flock(file, syscall.LOCK_EX)
}

func unlock(file *os.File) error {
	return flock(file, syscall.LOCK_UN)
}

const (
	lockTimeout   = 5 * time.Second
	lockRetryWait = 10 * time.Millisecond
)

// flock takes the lock without blocking, retrying until lockTimeout so a hung
// holder can't stall the caller forever.
func flock(file *os.File, how int) error {
	deadline := time.Now().Add(lockTimeout)

	for {
		err := syscall.Flock(int(file.Fd()), how|syscall.LOCK_NB)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if errors.Is(err, syscall.EWOULDBLOCK) {
			if !time.Now().Before(deadline) {
				return fmt.Errorf("flock %s: %w", file.Name(), ErrLockTimeout)
			}
			time.Sleep(lockRetryWait)

			continue
		}
		if err != nil {
			return fmt.Errorf("flock %s: %w", file.Name(), err)
		}

		return nil
	}
}
