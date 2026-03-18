package project

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateProjectID returns the first 10 hex characters of the SHA256 hash
// of the normalized remote URL (trimmed, lowercased, .git suffix stripped).
func GenerateProjectID(remoteURL string) string {
	normalized := strings.TrimSpace(remoteURL)
	normalized = strings.ToLower(normalized)
	normalized = strings.TrimSuffix(normalized, ".git")
	sum := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", sum)[:10]
}

// ExtractRepoName extracts the repository name from an SSH or HTTPS remote URL.
// e.g. git@github.com:team/my-app.git → my-app
// e.g. https://github.com/team/my-app.git → my-app
func ExtractRepoName(remoteURL string) string {
	u := strings.TrimSpace(remoteURL)

	// Handle SSH format: git@host:path/repo.git
	if idx := strings.LastIndex(u, ":"); idx != -1 && !strings.HasPrefix(u, "http") {
		u = u[idx+1:]
	}

	// Take the last path segment
	u = filepath.ToSlash(u)
	parts := strings.Split(u, "/")
	name := parts[len(parts)-1]

	// Strip .git suffix
	name = strings.TrimSuffix(name, ".git")
	return name
}

// DetectProject runs git to find the remote URL and repo name for the given directory.
func DetectProject(dir string) (remoteURL, repoName string, err error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	remoteURL = strings.TrimSpace(string(out))
	repoName = ExtractRepoName(remoteURL)
	return remoteURL, repoName, nil
}

// ProjectDirName returns the canonical directory name for a project: <name>--<id>.
func ProjectDirName(name, id string) string {
	return name + "--" + id
}

// DetectGitRoot returns the absolute path to the git repository root for the given directory.
func DetectGitRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}
