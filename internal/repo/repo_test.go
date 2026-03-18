package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary directory with a git repository
// that has an initial commit, suitable for use in tests.
func setupTestRepo(t *testing.T) *Repo {
	t.Helper()

	dir := t.TempDir()
	r := New(dir)

	mustGit := func(args ...string) {
		t.Helper()
		if _, err := r.git(args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	mustGit("init")
	mustGit("config", "user.email", "test@test.com")
	mustGit("config", "user.name", "Test User")

	// Create .gitattributes
	gaPath := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(gaPath, []byte("* text=auto\n"), 0o644); err != nil {
		t.Fatalf("write .gitattributes: %v", err)
	}

	mustGit("add", "-A")
	mustGit("commit", "-m", "initial commit")

	return r
}

func TestCommitAndGetHEAD(t *testing.T) {
	r := setupTestRepo(t)

	// Write a new file.
	filePath := filepath.Join(r.Path, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sha, err := r.Commit("add test.txt")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if sha == "" {
		t.Fatal("expected non-empty SHA from Commit")
	}

	head, err := r.HEAD()
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	if head == "" {
		t.Fatal("expected non-empty SHA from HEAD")
	}
	if sha != head {
		t.Fatalf("Commit SHA %q != HEAD %q", sha, head)
	}
}

func TestCommitNoChanges(t *testing.T) {
	r := setupTestRepo(t)

	_, err := r.Commit("should fail")
	if err == nil {
		t.Fatal("expected error when nothing to commit, got nil")
	}
}

func TestInitSyncRepo(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "sync-repo")

	r, err := InitSyncRepo(localPath)
	if err != nil {
		t.Fatalf("InitSyncRepo: %v", err)
	}

	// Verify .gitattributes exists.
	gaPath := filepath.Join(r.Path, ".gitattributes")
	if _, err := os.Stat(gaPath); os.IsNotExist(err) {
		t.Fatal(".gitattributes does not exist")
	}

	// Verify it is a valid git repo by calling HEAD.
	sha, err := r.HEAD()
	if err != nil {
		t.Fatalf("HEAD after InitSyncRepo: %v", err)
	}
	if sha == "" {
		t.Fatal("expected non-empty SHA after InitSyncRepo")
	}

	// Verify projects/ and global/ directories exist.
	for _, sub := range []string{"projects", "global"} {
		subPath := filepath.Join(r.Path, sub)
		if _, err := os.Stat(subPath); os.IsNotExist(err) {
			t.Fatalf("directory %s does not exist", sub)
		}
	}
}
