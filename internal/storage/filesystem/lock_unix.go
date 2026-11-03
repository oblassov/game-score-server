//go:build unix

package filesystem

import (
	"errors"
	"fmt"
	"os"
	"syscall"
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

func flock(file *os.File, how int) error {
	for {
		err := syscall.Flock(int(file.Fd()), how)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("flock %s: %w", file.Name(), err)
		}

		return nil
	}
}
