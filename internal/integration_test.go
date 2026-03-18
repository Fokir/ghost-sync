package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

// gitRun runs a git command in dir, failing the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// disableGlobalGitignore configures the repo at dir to use an empty excludes
// file, preventing global .gitignore rules from interfering with tests.
func disableGlobalGitignore(t *testing.T, dir string) {
	t.Helper()
	emptyIgnore := filepath.Join(t.TempDir(), "empty-gitignore")
	if err := os.WriteFile(emptyIgnore, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty gitignore: %v", err)
	}
	// Forward slashes required for git config on Windows.
	gitRun(t, dir, "config", "core.excludesFile", filepath.ToSlash(emptyIgnore))
}

// gitInit creates a git repo with an initial commit in dir.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test User")
	disableGlobalGitignore(t, dir)

	// Create an initial file so the first commit is valid.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "initial commit")
}

// writeFile writes content to path, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFullSyncCycle(t *testing.T) {
	patterns := config.DefaultPatterns()
	ignore := config.DefaultIgnore()
	maxFileSize, err := config.ParseFileSize(config.DefaultMaxFileSize)
	if err != nil {
		t.Fatalf("ParseFileSize: %v", err)
	}

	// -----------------------------------------------------------------------
	// 1. Setup
	// -----------------------------------------------------------------------

	// Create sync repo.
	syncRepoDir := filepath.Join(t.TempDir(), "sync-repo")
	syncRepo, err := repo.InitSyncRepo(syncRepoDir)
	if err != nil {
		t.Fatalf("InitSyncRepo: %v", err)
	}
	// Bypass global .gitignore so test files like CLAUDE.md are not ignored.
	disableGlobalGitignore(t, syncRepo.Path)

	// Create fake working project (git init + initial commit).
	workDir := t.TempDir()
	gitInit(t, workDir)

	// Determine the project directory name inside the sync repo.
	// We use a fixed remote URL (fake) to generate a stable ID.
	fakeRemote := "https://github.com/test/work-project.git"
	projectID := project.GenerateProjectID(fakeRemote)
	projectName := project.ExtractRepoName(fakeRemote)
	projectDirName := project.ProjectDirName(projectName, projectID)
	syncProjectDir := filepath.Join(syncRepo.Path, "projects", projectDirName)

	// -----------------------------------------------------------------------
	// 2. Create AI files in the working project
	// -----------------------------------------------------------------------
	writeFile(t, filepath.Join(workDir, ".claude", "skills", "test.md"), "# test skill\n")
	writeFile(t, filepath.Join(workDir, "CLAUDE.md"), "# CLAUDE\n")

	// -----------------------------------------------------------------------
	// 3. Push: copy AI files from working project to sync repo, commit
	// -----------------------------------------------------------------------
	if err := os.MkdirAll(syncProjectDir, 0o755); err != nil {
		t.Fatalf("mkdir syncProjectDir: %v", err)
	}

	copied, err := gosync.CopyByPatterns(workDir, syncProjectDir, patterns, ignore, maxFileSize)
	if err != nil {
		t.Fatalf("CopyByPatterns (push): %v", err)
	}
	if copied == 0 {
		t.Fatal("expected files to be copied to sync repo, got 0")
	}

	_, err = syncRepo.Commit("push: sync AI files")
	if err != nil {
		t.Fatalf("Commit after push: %v", err)
	}

	// -----------------------------------------------------------------------
	// 4. Pull to a clean project: create another temp dir, copy from sync repo
	// -----------------------------------------------------------------------
	cleanDir := t.TempDir()
	gitInit(t, cleanDir)

	_, err = gosync.CopyByPatterns(syncProjectDir, cleanDir, nil, ignore, maxFileSize)
	if err != nil {
		t.Fatalf("CopyByPatterns (pull): %v", err)
	}

	// Verify the files exist in clean project.
	for _, rel := range []string{
		filepath.Join(".claude", "skills", "test.md"),
		"CLAUDE.md",
	} {
		p := filepath.Join(cleanDir, rel)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s in clean project, but it does not exist", rel)
		}
	}

	// -----------------------------------------------------------------------
	// 5. Diff: run DiffFiles between clean project and sync repo
	//    Expect 0 conflicts, all AI files in "Same"
	// -----------------------------------------------------------------------
	diff, err := gosync.DiffFiles(cleanDir, syncProjectDir)
	if err != nil {
		t.Fatalf("DiffFiles: %v", err)
	}

	if len(diff.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(diff.Conflicts), diff.Conflicts)
	}

	// The AI files we copied should appear in Same (files present in both dirs
	// with identical content). Allow for any extra files (e.g. README.md) to
	// appear in LocalOnly — only assert that our two AI files are in Same.
	sameSet := make(map[string]bool, len(diff.Same))
	for _, s := range diff.Same {
		sameSet[s] = true
	}
	for _, rel := range []string{".claude/skills/test.md", "CLAUDE.md"} {
		if !sameSet[rel] {
			t.Errorf("expected %s in Same, got Same=%v", rel, diff.Same)
		}
	}

	// -----------------------------------------------------------------------
	// 6. Deletion propagation: delete CLAUDE.md from working project,
	//    push again (copy + detect stale files in sync repo), verify removed
	// -----------------------------------------------------------------------
	if err := os.Remove(filepath.Join(workDir, "CLAUDE.md")); err != nil {
		t.Fatalf("remove CLAUDE.md: %v", err)
	}

	// Re-copy from working project to sync repo (updated files).
	_, err = gosync.CopyByPatterns(workDir, syncProjectDir, patterns, ignore, maxFileSize)
	if err != nil {
		t.Fatalf("CopyByPatterns (push after delete): %v", err)
	}

	// Detect stale files: files in sync repo that match patterns but no longer
	// exist in the working project.
	syncFiles, err := gosync.CollectFiles(syncProjectDir)
	if err != nil {
		t.Fatalf("CollectFiles (syncProjectDir): %v", err)
	}

	var stale []string
	for rel := range syncFiles {
		if !gosync.MatchesPatterns(rel, patterns) {
			continue
		}
		workPath := filepath.Join(workDir, rel)
		if _, err := os.Stat(workPath); os.IsNotExist(err) {
			stale = append(stale, rel)
		}
	}

	if err := gosync.DeleteFiles(syncProjectDir, stale); err != nil {
		t.Fatalf("DeleteFiles: %v", err)
	}

	// Commit deletion.
	_, err = syncRepo.Commit("push: remove deleted AI files")
	if err != nil {
		t.Fatalf("Commit after deletion: %v", err)
	}

	// Verify CLAUDE.md is no longer in sync repo.
	claudePath := filepath.Join(syncProjectDir, "CLAUDE.md")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Error("expected CLAUDE.md to be removed from sync repo, but it still exists")
	}

	// Verify the other file is still present.
	skillPath := filepath.Join(syncProjectDir, ".claude", "skills", "test.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("expected .claude/skills/test.md to remain in sync repo after deletion of CLAUDE.md")
	}
}
