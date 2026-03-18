package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

// ConflictAction describes how to resolve a conflict.
type ConflictAction int

const (
	TakeLocal  ConflictAction = iota
	TakeRemote ConflictAction = iota
	Skip       ConflictAction = iota
)

// ResolveLatestWins returns TakeRemote if remote mtime is strictly after local, else TakeLocal.
func ResolveLatestWins(entry ConflictEntry) ConflictAction {
	if entry.RemoteMtime.After(entry.LocalMtime) {
		return TakeRemote
	}
	return TakeLocal
}

// BackupFile copies the file at srcPath to <backupDir>/<timestamp>/<relPath>.
// Timestamp format: 2006-01-02T150405
func BackupFile(srcPath, backupDir, relPath string) error {
	timestamp := time.Now().Format("2006-01-02T150405")
	destPath := filepath.Join(backupDir, timestamp, relPath)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("backup mkdir: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("backup open src: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("backup create dst: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("backup copy: %w", err)
	}
	return nil
}

// GetConflictStrategy returns the strategy for relPath by matching against rules
// (prefix or exact match). Falls back to defaultStrategy if no rule matches.
func GetConflictStrategy(relPath string, rules []config.ConflictRule, defaultStrategy string) string {
	for _, rule := range rules {
		pattern := rule.Pattern
		if pattern == relPath {
			return rule.Strategy
		}
		// prefix match: pattern must end with / or relPath starts with pattern+/
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, pattern) {
				return rule.Strategy
			}
		} else if strings.HasPrefix(relPath, pattern+"/") {
			return rule.Strategy
		}
	}
	return defaultStrategy
}

// DefaultBackupDir returns the default backup directory: ~/.ghost-sync/backups
func DefaultBackupDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ghost-sync/backups"
	}
	return filepath.Join(home, ".ghost-sync", "backups")
}

// PruneBackups removes backup dirs older than maxAge, then removes oldest dirs
// until total size is under maxSizeBytes.
func PruneBackups(backupDir string, maxAge time.Duration, maxSizeBytes int64) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("prune read dir: %w", err)
	}

	now := time.Now()

	// Collect dirs with their info
	type dirInfo struct {
		name    string
		modTime time.Time
		size    int64
	}

	var dirs []dirInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(backupDir, e.Name())
		size := dirSize(fullPath)
		dirs = append(dirs, dirInfo{
			name:    e.Name(),
			modTime: info.ModTime(),
			size:    size,
		})
	}

	// Remove dirs older than maxAge
	var remaining []dirInfo
	for _, d := range dirs {
		if maxAge > 0 && now.Sub(d.modTime) > maxAge {
			if err := os.RemoveAll(filepath.Join(backupDir, d.name)); err != nil {
				return fmt.Errorf("prune remove old: %w", err)
			}
		} else {
			remaining = append(remaining, d)
		}
	}

	// If maxSizeBytes > 0, enforce total size limit by removing oldest first
	if maxSizeBytes > 0 {
		// Sort by modTime ascending (oldest first)
		sort.Slice(remaining, func(i, j int) bool {
			return remaining[i].modTime.Before(remaining[j].modTime)
		})

		var totalSize int64
		for _, d := range remaining {
			totalSize += d.size
		}

		for len(remaining) > 0 && totalSize > maxSizeBytes {
			oldest := remaining[0]
			remaining = remaining[1:]
			totalSize -= oldest.size
			if err := os.RemoveAll(filepath.Join(backupDir, oldest.name)); err != nil {
				return fmt.Errorf("prune remove oversized: %w", err)
			}
		}
	}

	return nil
}

// dirSize returns the total size of all files under path.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
