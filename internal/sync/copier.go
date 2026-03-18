package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CopyByPatterns walks srcDir and copies files that match at least one pattern,
// are not ignored, and do not exceed maxFileSize bytes (0 = no limit).
// Returns the number of files successfully copied.
func CopyByPatterns(srcDir, dstDir string, patterns, ignore []string, maxFileSize int64) (int, error) {
	count := 0
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
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

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Check ignore patterns.
		if IsIgnored(rel, ignore) {
			return nil
		}

		// Check if matches at least one pattern.
		if len(patterns) > 0 && !matchesPatterns(rel, patterns) {
			return nil
		}

		// Check file size limit.
		if maxFileSize > 0 {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() > maxFileSize {
				return nil
			}
		}

		src := path
		dst := filepath.Join(dstDir, filepath.FromSlash(rel))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}

// matchesPatterns returns true if relPath matches any of the given patterns.
// A pattern ending with "/" is a directory prefix match; otherwise it is an exact match.
func matchesPatterns(relPath string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/") {
			// Directory pattern: match any file under this directory.
			if strings.HasPrefix(relPath, p) {
				return true
			}
		} else {
			// Exact file match.
			if relPath == p {
				return true
			}
		}
	}
	return false
}

// copyFile copies src to dst, preserving permissions and mtime.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	// Preserve modification time.
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

// DeleteFiles removes the listed files (relative paths) from dir.
func DeleteFiles(dir string, paths []string) error {
	for _, rel := range paths {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete %s: %w", rel, err)
		}
	}
	return nil
}
