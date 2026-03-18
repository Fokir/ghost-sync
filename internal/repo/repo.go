package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo represents a git repository at a given path.
type Repo struct {
	Path string
}

// New returns a new Repo for the given path.
func New(path string) *Repo {
	return &Repo{Path: path}
}

// git runs a git command in the repository directory and returns trimmed stdout.
func (r *Repo) git(args ...string) (string, error) {
	cmdArgs := append([]string{"-C", r.Path}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %w\nstderr: %s", strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Commit stages all changes and commits them with the given message.
// Returns the short SHA of the new commit.
// Returns an error if there are no changes to commit.
func (r *Repo) Commit(message string) (string, error) {
	if _, err := r.git("add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}

	status, err := r.git("status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	if status == "" {
		return "", errors.New("nothing to commit: working tree clean")
	}

	if _, err := r.git("commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	return r.HEAD()
}

// HEAD returns the short SHA of the current HEAD commit.
func (r *Repo) HEAD() (string, error) {
	sha, err := r.git("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return sha, nil
}

// Pull performs a git pull --rebase.
func (r *Repo) Pull() error {
	_, err := r.git("pull", "--rebase")
	return err
}

// Push performs a git push.
func (r *Repo) Push() error {
	_, err := r.git("push")
	return err
}

// HasRemote returns true if the repository has an "origin" remote configured.
func (r *Repo) HasRemote() bool {
	out, err := r.git("remote")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "origin" {
			return true
		}
	}
	return false
}

// CloneSyncRepo clones the remote repository at remoteURL into localPath.
func CloneSyncRepo(remoteURL, localPath string) (*Repo, error) {
	cmd := exec.Command("git", "clone", remoteURL, localPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone: %w\noutput: %s", err, string(out))
	}
	return New(localPath), nil
}

// InitSyncRepo initialises a new sync repository at localPath.
// It creates the directory, runs git init, configures user details,
// creates .gitattributes, projects/ and global/ directories,
// and makes an initial commit.
func InitSyncRepo(localPath string) (*Repo, error) {
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", localPath, err)
	}

	r := New(localPath)

	if _, err := r.git("init"); err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}

	if _, err := r.git("config", "user.email", "ghost-sync@local"); err != nil {
		return nil, fmt.Errorf("git config email: %w", err)
	}
	if _, err := r.git("config", "user.name", "ghost-sync"); err != nil {
		return nil, fmt.Errorf("git config name: %w", err)
	}

	gitattributes := filepath.Join(localPath, ".gitattributes")
	if err := os.WriteFile(gitattributes, []byte("* text=auto\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write .gitattributes: %w", err)
	}

	for _, dir := range []string{"projects", "global"} {
		dirPath := filepath.Join(localPath, dir)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
		// Add a .gitkeep so empty directories are tracked.
		keepFile := filepath.Join(dirPath, ".gitkeep")
		if err := os.WriteFile(keepFile, []byte{}, 0o644); err != nil {
			return nil, fmt.Errorf("write .gitkeep in %s: %w", dir, err)
		}
	}

	if _, err := r.Commit("init: initial sync repo setup"); err != nil {
		return nil, fmt.Errorf("initial commit: %w", err)
	}

	return r, nil
}
