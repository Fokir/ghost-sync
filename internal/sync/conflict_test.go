package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

func TestResolveLatestWins_RemoteNewer(t *testing.T) {
	now := time.Now()
	entry := ConflictEntry{
		Path:        "file.txt",
		LocalMtime:  now.Add(-10 * time.Second),
		RemoteMtime: now,
	}
	action := ResolveLatestWins(entry)
	if action != TakeRemote {
		t.Errorf("expected TakeRemote, got %v", action)
	}
}

func TestResolveLatestWins_LocalNewer(t *testing.T) {
	now := time.Now()
	entry := ConflictEntry{
		Path:        "file.txt",
		LocalMtime:  now,
		RemoteMtime: now.Add(-10 * time.Second),
	}
	action := ResolveLatestWins(entry)
	if action != TakeLocal {
		t.Errorf("expected TakeLocal, got %v", action)
	}
}

func TestResolveLatestWins_SameTime(t *testing.T) {
	now := time.Now()
	entry := ConflictEntry{
		Path:        "file.txt",
		LocalMtime:  now,
		RemoteMtime: now,
	}
	action := ResolveLatestWins(entry)
	if action != TakeLocal {
		t.Errorf("expected TakeLocal for equal times, got %v", action)
	}
}

func TestBackupFile(t *testing.T) {
	// Create a temp source file
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	content := []byte("hello backup")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir := t.TempDir()
	relPath := "subdir/source.txt"

	if err := BackupFile(srcPath, backupDir, relPath); err != nil {
		t.Fatalf("BackupFile error: %v", err)
	}

	// Find the backed-up file — it should be under backupDir/<timestamp>/subdir/source.txt
	matches, err := filepath.Glob(filepath.Join(backupDir, "*", "subdir", "source.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("backup file not found")
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("backup content mismatch: got %q want %q", data, content)
	}
}

func TestGetConflictStrategy_ExactMatch(t *testing.T) {
	rules := []config.ConflictRule{
		{Pattern: "foo/bar.txt", Strategy: "skip"},
		{Pattern: "docs", Strategy: "take_remote"},
	}
	got := GetConflictStrategy("foo/bar.txt", rules, "latest_wins")
	if got != "skip" {
		t.Errorf("expected skip, got %q", got)
	}
}

func TestGetConflictStrategy_PrefixMatch(t *testing.T) {
	rules := []config.ConflictRule{
		{Pattern: "docs", Strategy: "take_remote"},
	}
	got := GetConflictStrategy("docs/readme.md", rules, "latest_wins")
	if got != "take_remote" {
		t.Errorf("expected take_remote, got %q", got)
	}
}

func TestGetConflictStrategy_FallbackToDefault(t *testing.T) {
	rules := []config.ConflictRule{
		{Pattern: "other", Strategy: "skip"},
	}
	got := GetConflictStrategy("unmatched/file.go", rules, "latest_wins")
	if got != "latest_wins" {
		t.Errorf("expected latest_wins, got %q", got)
	}
}

func TestGetConflictStrategy_NoRules(t *testing.T) {
	got := GetConflictStrategy("anything.txt", nil, "take_local")
	if got != "take_local" {
		t.Errorf("expected take_local, got %q", got)
	}
}

func TestPruneBackups_RemovesOld(t *testing.T) {
	backupDir := t.TempDir()

	// Create an "old" backup dir by manipulating mtime via os.Chtimes
	oldDir := filepath.Join(backupDir, "2020-01-01T000000")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldDir, "file.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a recent backup dir
	recentDir := filepath.Join(backupDir, "2099-01-01T000000")
	if err := os.MkdirAll(recentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recentFile := filepath.Join(recentDir, "file.txt")
	if err := os.WriteFile(recentFile, []byte("recent"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Prune with maxAge=24h
	if err := PruneBackups(backupDir, 24*time.Hour, 0); err != nil {
		t.Fatalf("PruneBackups error: %v", err)
	}

	// Old dir should be gone
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("expected old backup dir to be removed")
	}

	// Recent dir should remain
	if _, err := os.Stat(recentDir); err != nil {
		t.Errorf("expected recent backup dir to exist: %v", err)
	}
}

func TestPruneBackups_KeepsRecent(t *testing.T) {
	backupDir := t.TempDir()

	// Two recent dirs
	for _, name := range []string{"2099-01-01T000001", "2099-01-01T000002"} {
		dir := filepath.Join(backupDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := PruneBackups(backupDir, 24*time.Hour, 0); err != nil {
		t.Fatalf("PruneBackups error: %v", err)
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 2 {
		t.Errorf("expected 2 dirs remaining, got %d", len(entries))
	}
}

func TestPruneBackups_NonExistentDir(t *testing.T) {
	err := PruneBackups("/nonexistent/path/that/does/not/exist", time.Hour, 0)
	if err != nil {
		t.Errorf("expected no error for non-existent dir, got: %v", err)
	}
}
