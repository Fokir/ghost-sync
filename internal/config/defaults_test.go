package config

import (
	"testing"
)

func TestDefaultPatterns(t *testing.T) {
	patterns := DefaultPatterns()

	expected := []string{
		".claude/",
		".serena/",
		"docs/superpowers/",
		"CLAUDE.md",
		"GEMINI.md",
		"AGENTS.md",
	}

	if len(patterns) != len(expected) {
		t.Fatalf("DefaultPatterns() returned %d patterns, want %d", len(patterns), len(expected))
	}

	patternSet := make(map[string]bool, len(patterns))
	for _, p := range patterns {
		patternSet[p] = true
	}

	for _, e := range expected {
		if !patternSet[e] {
			t.Errorf("DefaultPatterns() missing expected pattern %q", e)
		}
	}
}

func TestDefaultIgnore(t *testing.T) {
	ignore := DefaultIgnore()

	expected := []string{
		"node_modules/",
		".git/",
		".claude/.credentials.json",
		".claude/cache/",
		".claude/backups/",
		".claude/chrome/",
		".claude/debug/",
		".claude/file-history/",
		".claude/history.jsonl",
		".claude/ide/",
		".claude/mcp-needs-auth-cache.json",
		".claude/paste-cache/",
		".claude/plans/",
		".claude/plans-temp-output.md",
		".claude/plugins/",
		".claude/projects/",
		".claude/sessions/",
		".claude/shell-snapshots/",
		".claude/stats-cache.json",
		".claude/statsig/",
		".claude/statusline-command.sh",
		".claude/tasks/",
		".claude/telemetry/",
		".claude/todos/",
		".serena/language_servers/",
		".serena/logs/",
	}

	if len(ignore) != len(expected) {
		t.Fatalf("DefaultIgnore() returned %d entries, want %d", len(ignore), len(expected))
	}

	ignoreSet := make(map[string]bool, len(ignore))
	for _, i := range ignore {
		ignoreSet[i] = true
	}

	for _, e := range expected {
		if !ignoreSet[e] {
			t.Errorf("DefaultIgnore() missing expected entry %q", e)
		}
	}
}

func TestConstants(t *testing.T) {
	if ConfigVersion != 1 {
		t.Errorf("ConfigVersion = %d, want 1", ConfigVersion)
	}
	if DefaultMaxFileSize != "10MB" {
		t.Errorf("DefaultMaxFileSize = %q, want \"10MB\"", DefaultMaxFileSize)
	}
	if DefaultConflictStrategy != "latest-wins" {
		t.Errorf("DefaultConflictStrategy = %q, want \"latest-wins\"", DefaultConflictStrategy)
	}
}
