package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a minimal fake git repo in dir (just .git/HEAD).
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create .git dir: %v", err)
	}
	// Write a minimal HEAD so it looks like a real git repo.
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	return dir
}

func TestInstallHooks(t *testing.T) {
	repo := initGitRepo(t)

	if err := Install(repo); err != nil {
		t.Fatalf("Install: %v", err)
	}

	hooksDir := filepath.Join(repo, ".git", "hooks")

	for _, name := range []string{"post-commit", "post-merge"} {
		path := filepath.Join(hooksDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(data)
		if !strings.Contains(content, "ghost-sync") {
			t.Errorf("%s: expected ghost-sync content, got:\n%s", name, content)
		}
	}
}

func TestInstallPreservesExistingHooks(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	existingContent := "#!/bin/sh\necho 'existing hook'\n"
	postCommitPath := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(existingContent), 0755); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(repo); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(postCommitPath)
	if err != nil {
		t.Fatalf("read post-commit: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "existing hook") {
		t.Error("post-commit: existing content was lost")
	}
	if !strings.Contains(content, "ghost-sync") {
		t.Error("post-commit: ghost-sync content not added")
	}
}

func TestRemoveHooks(t *testing.T) {
	repo := initGitRepo(t)

	if err := Install(repo); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if err := Remove(repo); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	hooksDir := filepath.Join(repo, ".git", "hooks")

	for _, name := range []string{"post-commit", "post-merge"} {
		path := filepath.Join(hooksDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted — that is acceptable (content was only ghost-sync).
				continue
			}
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(data), "ghost-sync") {
			t.Errorf("%s: ghost-sync content still present after Remove", name)
		}
	}
}

func TestRemovePreservesOtherContent(t *testing.T) {
	repo := initGitRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	existingContent := "#!/bin/sh\necho 'other tool hook'\n"
	postCommitPath := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(existingContent), 0755); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(repo); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if err := Remove(repo); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	data, err := os.ReadFile(postCommitPath)
	if err != nil {
		t.Fatalf("read post-commit after Remove: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "other tool hook") {
		t.Error("post-commit: existing content was removed along with ghost-sync content")
	}
	if strings.Contains(content, "ghost-sync") {
		t.Error("post-commit: ghost-sync content still present after Remove")
	}
}
