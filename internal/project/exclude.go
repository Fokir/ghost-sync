package project

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	excludeMarkerBegin = "# ghost-sync: AI agent files (managed automatically, do not edit)"
	excludeMarkerEnd   = "# ghost-sync: end"
)

// ExcludePathForRepo returns the path to the .git/info/exclude file for the given repo root.
func ExcludePathForRepo(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "info", "exclude")
}

// WriteExclude writes the given patterns into the ghost-sync marker block in the
// exclude file. If a block already exists it is replaced (idempotent). Content
// outside the block is preserved.
func WriteExclude(excludePath string, patterns []string) error {
	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var before, after []string
	inBlock := false
	foundBlock := false

	if len(existing) > 0 {
		lines := strings.Split(string(existing), "\n")
		// Remove trailing empty string caused by final newline
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		for _, line := range lines {
			if line == excludeMarkerBegin {
				inBlock = true
				foundBlock = true
				continue
			}
			if line == excludeMarkerEnd {
				inBlock = false
				continue
			}
			if !inBlock {
				if foundBlock {
					after = append(after, line)
				} else {
					before = append(before, line)
				}
			}
		}
	}

	// Build new block
	block := []string{excludeMarkerBegin}
	block = append(block, patterns...)
	block = append(block, excludeMarkerEnd)

	// Assemble full content
	var all []string
	all = append(all, before...)
	all = append(all, block...)
	all = append(all, after...)

	content := strings.Join(all, "\n") + "\n"

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(excludePath, []byte(content), 0o644)
}

// RemoveExclude removes the ghost-sync marker block from the exclude file,
// preserving all other content.
func RemoveExclude(excludePath string) error {
	existing, err := os.ReadFile(excludePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var kept []string
	inBlock := false

	lines := strings.Split(string(existing), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for _, line := range lines {
		if line == excludeMarkerBegin {
			inBlock = true
			continue
		}
		if line == excludeMarkerEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			kept = append(kept, line)
		}
	}

	content := strings.Join(kept, "\n")
	if len(kept) > 0 {
		content += "\n"
	}

	return os.WriteFile(excludePath, []byte(content), 0o644)
}
