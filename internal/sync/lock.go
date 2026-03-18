package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock represents an exclusive file-based lock.
type Lock struct {
	path string
	file *os.File
}

// AcquireLock creates an exclusive file lock at path.
// It retries every 100ms until timeout is reached.
// On success it writes the current process PID to the lock file.
func AcquireLock(path string, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)

	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			// Write PID to the lock file.
			pid := os.Getpid()
			if _, werr := fmt.Fprintf(f, "%d\n", pid); werr != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, fmt.Errorf("writing pid to lock file: %w", werr)
			}
			return &Lock{path: path, file: f}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("opening lock file: %w", err)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for lock %s", path)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// ReleaseLock closes and removes the lock file.
// It is nil-safe and returns nil when lock is nil.
func ReleaseLock(lock *Lock) error {
	if lock == nil {
		return nil
	}

	var firstErr error

	if lock.file != nil {
		if err := lock.file.Close(); err != nil {
			firstErr = fmt.Errorf("closing lock file: %w", err)
		}
		lock.file = nil
	}

	if err := os.Remove(lock.path); err != nil && !os.IsNotExist(err) {
		if firstErr == nil {
			firstErr = fmt.Errorf("removing lock file: %w", err)
		}
	}

	return firstErr
}

// DefaultLockPath returns the default path for the ghost-sync lock file.
func DefaultLockPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ghost-sync", "ghost-sync.lock")
}
