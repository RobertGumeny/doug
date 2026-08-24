//go:build windows

package runlock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockByteCount is the length of the byte range locked in run.lock. Windows
// file locks are range-based rather than whole-descriptor, so every caller must
// agree on the same range. Locking a range past end-of-file is legal, which
// matters because TryAcquire truncates the file after taking the lock.
const lockByteCount = 1

// lockFile takes an exclusive lock on file without blocking. It returns ErrHeld
// when another process already holds the lock.
//
// LOCKFILE_FAIL_IMMEDIATELY is the Windows analogue of LOCK_NB: without it
// LockFileEx blocks until the holder releases, which would hang a lifecycle
// driver instead of reporting a held lock.
func lockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		lockByteCount,
		0,
		overlapped,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return ErrHeld
		}
		return err
	}
	return nil
}

// unlockFile releases the lock held on file.
func unlockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		lockByteCount,
		0,
		overlapped,
	)
}
