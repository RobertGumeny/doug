//go:build !windows

package runlock

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on file without blocking. It
// returns ErrHeld when another process already holds the lock.
func lockFile(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ErrHeld
		}
		return err
	}
	return nil
}

// unlockFile releases the advisory lock held on file.
func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
