package config

// ConfigVersion is the current config file format version.
const ConfigVersion = 1

// DefaultMaxFileSize is the default maximum file size for sync.
const DefaultMaxFileSize = "10MB"

// DefaultConflictStrategy is the default strategy when conflicts occur.
const DefaultConflictStrategy = "latest-wins"

// DefaultPatterns returns the default list of file patterns to sync.
func DefaultPatterns() []string {
	return []string{
		".claude/",
		".serena/",
		"docs/superpowers/",
		"CLAUDE.md",
		"GEMINI.md",
		"AGENTS.md",
	}
}

// DefaultIgnore returns the default list of patterns to ignore during sync.
func DefaultIgnore() []string {
	return []string{
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
}
