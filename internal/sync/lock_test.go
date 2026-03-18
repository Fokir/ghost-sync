package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempLockPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.lock")
}

func TestAcquireAndRelease(t *testing.T) {
	path := tempLockPath(t)

	lock, err := AcquireLock(path, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: unexpected error: %v", err)
	}

	// Lock file must exist on disk.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file should exist after acquire: %v", err)
	}

	if err := ReleaseLock(lock); err != nil {
		t.Fatalf("ReleaseLock: unexpected error: %v", err)
	}

	// Lock file must be removed after release.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("lock file should be removed after release")
	}
}

func TestAcquireLockTimeout(t *testing.T) {
	path := tempLockPath(t)

	first, err := AcquireLock(path, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock (first): unexpected error: %v", err)
	}
	defer ReleaseLock(first) //nolint:errcheck

	// Second acquire must fail quickly.
	_, err = AcquireLock(path, 100*time.Millisecond)
	if err == nil {
		t.Fatal("AcquireLock (second): expected timeout error, got nil")
	}
}

func TestReleaseLockNilSafe(t *testing.T) {
	if err := ReleaseLock(nil); err != nil {
		t.Fatalf("ReleaseLock(nil): expected nil error, got %v", err)
	}
}

func TestDefaultLockPath(t *testing.T) {
	p := DefaultLockPath()
	if p == "" {
		t.Fatal("DefaultLockPath returned empty string")
	}
	// Must end with the expected filename.
	if filepath.Base(p) != "ghost-sync.lock" {
		t.Errorf("expected filename ghost-sync.lock, got %s", filepath.Base(p))
	}
}
