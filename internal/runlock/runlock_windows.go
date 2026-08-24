//go:build windows

package runlock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Windows file locks are mandatory rather than advisory: a locked byte range
// is unreadable by every other handle, including plain os.ReadFile and git's
// indexer. ReadMetadata is specifically meant to read run.lock *while another
// process holds the lock, so the locked range must not overlap the metadata.
//
// Locking a range past end-of-file is legal on Windows, so the lock byte sits
// far beyond any plausible run.lock content. Mutual exclusion still holds —
// every doug process locks the same range — while the metadata bytes at the
// start of the file stay readable.
const (
	lockRegionOffsetHigh = 0x4000_0000 // byte offset 2^62
	lockByteCount        = 1
)

func lockRegion() *windows.Overlapped {
	return &windows.Overlapped{OffsetHigh: lockRegionOffsetHigh}
}

// lockFile takes an exclusive lock on file without blocking. It returns ErrHeld
// when another process already holds the lock.
//
// LOCKFILE_FAIL_IMMEDIATELY is the Windows analogue of LOCK_NB: without it
// LockFileEx blocks until the holder releases, which would hang a lifecycle
// driver instead of reporting a held lock.
func lockFile(file *os.File) error {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		lockByteCount,
		0,
		lockRegion(),
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
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		lockByteCount,
		0,
		lockRegion(),
	)
}
