package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConflictEntry represents a file that exists in both local and remote but has different content.
type ConflictEntry struct {
	Path        string
	LocalMtime  time.Time
	RemoteMtime time.Time
}

// DiffResult holds the categorized result of comparing two directories.
type DiffResult struct {
	LocalOnly  []string
	RemoteOnly []string
	Same       []string
	Conflicts  []ConflictEntry
}

// FileHash returns the SHA256 hex digest of the file at path.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DiffFiles compares all files in localDir and remoteDir and categorizes them.
func DiffFiles(localDir, remoteDir string) (*DiffResult, error) {
	localFiles, err := CollectFiles(localDir)
	if err != nil {
		return nil, err
	}
	remoteFiles, err := CollectFiles(remoteDir)
	if err != nil {
		return nil, err
	}

	result := &DiffResult{}

	for rel := range localFiles {
		if _, inRemote := remoteFiles[rel]; !inRemote {
			result.LocalOnly = append(result.LocalOnly, rel)
			continue
		}

		localPath := filepath.Join(localDir, filepath.FromSlash(rel))
		remotePath := filepath.Join(remoteDir, filepath.FromSlash(rel))

		localHash, err := FileHash(localPath)
		if err != nil {
			return nil, err
		}
		remoteHash, err := FileHash(remotePath)
		if err != nil {
			return nil, err
		}

		if localHash == remoteHash {
			result.Same = append(result.Same, rel)
		} else {
			localInfo, err := os.Stat(localPath)
			if err != nil {
				return nil, err
			}
			remoteInfo, err := os.Stat(remotePath)
			if err != nil {
				return nil, err
			}
			result.Conflicts = append(result.Conflicts, ConflictEntry{
				Path:        rel,
				LocalMtime:  localInfo.ModTime(),
				RemoteMtime: remoteInfo.ModTime(),
			})
		}
	}

	for rel := range remoteFiles {
		if _, inLocal := localFiles[rel]; !inLocal {
			result.RemoteOnly = append(result.RemoteOnly, rel)
		}
	}

	return result, nil
}

// CollectFiles walks dir and returns a map of slash-separated relative paths.
// Symlinks are skipped (not followed).
func CollectFiles(dir string) (map[string]struct{}, error) {
	files := make(map[string]struct{})
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks without following them.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// IsIgnored returns true if relPath matches any of the given ignore patterns.
// A pattern is matched as a prefix (e.g. ".claude/cache/" ignores everything under it).
// Additionally, ".git/" segments are always ignored at any depth to prevent syncing
// embedded git repositories.
func IsIgnored(relPath string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(relPath, pattern) {
			return true
		}
	}
	// Always ignore .git directories at any nesting level.
	if strings.HasPrefix(relPath, ".git/") || strings.Contains(relPath, "/.git/") {
		return true
	}
	return false
}
