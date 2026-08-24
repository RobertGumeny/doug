// Package runlock provides Doug's shared advisory lock for lifecycle drivers.
package runlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const FileName = "run.lock"

var ErrHeld = errors.New("doug run lock is held")

// Lock is an acquired advisory lifecycle lock. Close releases it.
type Lock struct {
	path string
	file *os.File
}

// Metadata is the best-effort human-readable owner information stored in
// .doug/run.lock while a lifecycle driver holds the advisory lock.
type Metadata struct {
	Owner      string
	PID        int
	AcquiredAt string
}

func Path(dougDir string) string {
	return filepath.Join(dougDir, FileName)
}

// ReadMetadata returns best-effort lock-owner metadata from .doug/run.lock.
// The boolean reports whether at least one metadata field was available.
func ReadMetadata(dougDir string) (Metadata, bool) {
	data, err := os.ReadFile(Path(dougDir))
	if err != nil {
		return Metadata{}, false
	}
	var metadata Metadata
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "owner":
			metadata.Owner = value
		case "pid":
			pid, err := strconv.Atoi(value)
			if err == nil {
				metadata.PID = pid
			}
		case "acquired_at":
			metadata.AcquiredAt = value
		}
	}
	return metadata, metadata.Owner != "" || metadata.PID != 0 || metadata.AcquiredAt != ""
}

// HeldDetails formats lock metadata for lock-held remediation messages.
func HeldDetails(dougDir string) string {
	metadata, ok := ReadMetadata(dougDir)
	if !ok {
		return ""
	}
	parts := []string{}
	if metadata.Owner != "" {
		parts = append(parts, fmt.Sprintf("owner=%q", metadata.Owner))
	}
	if metadata.PID != 0 {
		parts = append(parts, fmt.Sprintf("pid=%d", metadata.PID))
	}
	if metadata.AcquiredAt != "" {
		parts = append(parts, fmt.Sprintf("acquired_at=%s", metadata.AcquiredAt))
	}
	return strings.Join(parts, " ")
}

// TryAcquire attempts to claim .doug/run.lock without blocking. The lock is an
// OS-level file lock, so a stale lock file left by a crashed process is reusable
// as soon as no process still holds the descriptor lock. See lockFile for the
// per-platform primitive.
func TryAcquire(dougDir, owner string) (*Lock, error) {
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .doug directory for run lock: %w", err)
	}
	path := Path(dougDir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open run lock: %w", err)
	}
	if err := lockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, ErrHeld) {
			return nil, ErrHeld
		}
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}
	if owner == "" {
		owner = "doug"
	}
	metadata := fmt.Sprintf("owner: %s\npid: %d\nacquired_at: %s\n", owner, os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := file.Truncate(0); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("truncate run lock metadata: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("seek run lock metadata: %w", err)
	}
	if _, err := file.WriteString(metadata); err != nil {
		_ = unlockFile(file)
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
	unlockErr := unlockFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
