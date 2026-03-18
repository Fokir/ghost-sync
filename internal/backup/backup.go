package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DefaultDir returns the default backup directory path.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ghost-sync", "backups")
	}
	return filepath.Join(home, ".ghost-sync", "backups")
}

// Create copies srcPath to <backupDir>/<timestamp>/<relPath>.
// The timestamp format is "2006-01-02T150405".
func Create(backupDir, srcPath, relPath string) error {
	ts := time.Now().Format("2006-01-02T150405")
	destPath := filepath.Join(backupDir, ts, relPath)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Prune removes backup directories older than maxAge, then removes the oldest
// directories until the total size is within maxSize bytes.
// A maxAge of 0 removes all backups.
func Prune(backupDir string, maxAge time.Duration, maxSize int64) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup dir: %w", err)
	}

	// Collect directory entries with their modification times.
	type dirEntry struct {
		name    string
		path    string
		modTime time.Time
	}

	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, dirEntry{
			name:    e.Name(),
			path:    filepath.Join(backupDir, e.Name()),
			modTime: info.ModTime(),
		})
	}

	// Sort oldest first.
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.Before(dirs[j].modTime)
	})

	now := time.Now()

	// Remove directories older than maxAge.
	if maxAge == 0 {
		// Remove all.
		for _, d := range dirs {
			if err := os.RemoveAll(d.path); err != nil {
				return fmt.Errorf("remove %s: %w", d.path, err)
			}
		}
		dirs = nil
	} else {
		var remaining []dirEntry
		for _, d := range dirs {
			age := now.Sub(d.modTime)
			if age > maxAge {
				if err := os.RemoveAll(d.path); err != nil {
					return fmt.Errorf("remove %s: %w", d.path, err)
				}
			} else {
				remaining = append(remaining, d)
			}
		}
		dirs = remaining
	}

	// Remove oldest until total size is within maxSize.
	if maxSize > 0 {
		for len(dirs) > 0 {
			total, err := dirSize(backupDir)
			if err != nil {
				return fmt.Errorf("calculate dir size: %w", err)
			}
			if total <= maxSize {
				break
			}
			oldest := dirs[0]
			dirs = dirs[1:]
			if err := os.RemoveAll(oldest.path); err != nil {
				return fmt.Errorf("remove %s: %w", oldest.path, err)
			}
		}
	}

	return nil
}

// dirSize recursively calculates the total size of files under path.
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
