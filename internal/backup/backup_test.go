package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateAndPrune(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")

	// Create a source file.
	srcPath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("hello backup"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Create a backup.
	if err := Create(backupDir, srcPath, "source.txt"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify backup was created.
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup dir, got %d", len(entries))
	}

	// Prune with 0 age — removes everything.
	if err := Prune(backupDir, 0, 0); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	entries, err = os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir after prune: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 backup dirs after prune, got %d", len(entries))
	}
}

func TestPruneKeepsRecent(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")

	// Create a source file.
	srcPath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("keep me"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Create a backup.
	if err := Create(backupDir, srcPath, "source.txt"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Prune with 30 days — recent backup should be kept.
	if err := Prune(backupDir, 30*24*time.Hour, 0); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir after prune: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup dir after prune, got %d", len(entries))
	}
}

func TestDefaultDir(t *testing.T) {
	d := DefaultDir()
	if d == "" {
		t.Fatal("DefaultDir returned empty string")
	}
	if !strings.HasSuffix(d, "backups") {
		t.Errorf("unexpected default dir: %s", d)
	}
}

func TestCreateNestedRelPath(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")

	srcPath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(srcPath, []byte("nested"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	relPath := filepath.Join("sub", "dir", "file.txt")
	if err := Create(backupDir, srcPath, relPath); err != nil {
		t.Fatalf("Create with nested relPath: %v", err)
	}

	// Find the timestamp dir and verify nested path exists.
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("backup dir empty or error: %v", err)
	}
	destPath := filepath.Join(backupDir, entries[0].Name(), relPath)
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("nested backup file not found: %v", err)
	}
}

func TestPruneNonExistentDir(t *testing.T) {
	err := Prune("/nonexistent/backup/dir", 24*time.Hour, 0)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent backup dir, got: %v", err)
	}
}
