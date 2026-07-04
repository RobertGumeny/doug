// Package runlock provides Doug's shared advisory lock for lifecycle drivers.
package runlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const FileName = "run.lock"

var ErrHeld = errors.New("doug run lock is held")

// Lock is an acquired advisory lifecycle lock. Close releases it.
type Lock struct {
	path string
	file *os.File
}

func Path(dougDir string) string {
	return filepath.Join(dougDir, FileName)
}

// TryAcquire attempts to claim .doug/run.lock without blocking. The lock is an
// OS advisory flock, so a stale lock file left by a crashed process is reusable
// as soon as no process still holds the descriptor lock.
func TryAcquire(dougDir, owner string) (*Lock, error) {
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .doug directory for run lock: %w", err)
	}
	path := Path(dougDir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open run lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrHeld
		}
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}
	if owner == "" {
		owner = "doug"
	}
	metadata := fmt.Sprintf("owner: %s\npid: %d\nacquired_at: %s\n", owner, os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := file.Truncate(0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("truncate run lock metadata: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("seek run lock metadata: %w", err)
	}
	if _, err := file.WriteString(metadata); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("write run lock metadata: %w", err)
	}
	return &Lock{path: path, file: file}, nil
}

func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
