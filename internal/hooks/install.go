package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ghostSyncMarker = "ghost-sync"

// Install installs post-commit and post-merge hooks into <repoRoot>/.git/hooks/.
// If a hook file already exists, ghost-sync content is appended (preserving existing content).
// The hooks directory is created if it does not exist.
func Install(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hooks := map[string]string{
		"post-commit": PostCommitScript(),
		"post-merge":  PostMergeScript(),
	}

	for name, script := range hooks {
		path := filepath.Join(hooksDir, name)
		if err := installHook(path, script); err != nil {
			return fmt.Errorf("install %s hook: %w", name, err)
		}
	}

	return nil
}

// Remove removes ghost-sync content from post-commit and post-merge hooks,
// preserving any other content. If the hook file becomes empty after removal,
// it is deleted.
func Remove(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")

	hookNames := []string{"post-commit", "post-merge"}
	for _, name := range hookNames {
		path := filepath.Join(hooksDir, name)
		if err := removeHookContent(path); err != nil {
			return fmt.Errorf("remove %s hook content: %w", name, err)
		}
	}

	return nil
}

// installHook writes script to path. If the file already exists and does not
// already contain ghost-sync content, the script is appended. If ghost-sync
// content is already present, it is replaced with the new script.
func installHook(path, script string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read hook: %w", err)
	}

	var content string
	if os.IsNotExist(err) || len(existing) == 0 {
		// New file — write the script directly.
		content = script
	} else {
		existingStr := string(existing)
		if strings.Contains(existingStr, ghostSyncMarker) {
			// Replace existing ghost-sync block.
			content = removeGhostSyncContent(existingStr) + script
		} else {
			// Append ghost-sync block to existing content.
			content = existingStr
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n" + script
		}
	}

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	return nil
}

// removeHookContent removes ghost-sync content from the file at path.
// If the remaining content is empty (or only whitespace), the file is deleted.
func removeHookContent(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hook: %w", err)
	}

	cleaned := removeGhostSyncContent(string(data))

	if strings.TrimSpace(cleaned) == "" {
		return os.Remove(path)
	}

	return os.WriteFile(path, []byte(cleaned), 0755)
}

// removeGhostSyncContent strips the ghost-sync block from content.
// The block starts at the first line containing "# ghost-sync" (a comment)
// and ends at the next top-level "exit 0" line (not indented, i.e., no
// leading whitespace).
func removeGhostSyncContent(content string) string {
	lines := strings.Split(content, "\n")

	var result []string
	inBlock := false

	for _, line := range lines {
		if !inBlock {
			// Detect the start of a ghost-sync managed block (comment line).
			if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "# ghost-sync") {
				inBlock = true
				continue
			}
			result = append(result, line)
		} else {
			// Inside ghost-sync block — skip lines until a top-level "exit 0"
			// (line that starts with "exit 0", not indented inside an if-block).
			trimmed := strings.TrimSpace(line)
			if trimmed == "exit 0" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inBlock = false
			}
			// Skip this line (it is part of the ghost-sync block).
		}
	}

	return strings.Join(result, "\n")
}
