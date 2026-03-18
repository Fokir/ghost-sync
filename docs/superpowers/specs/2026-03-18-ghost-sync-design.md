# ghost-sync: Design Specification

## Overview

**ghost-sync** is a cross-platform CLI tool (Go) that synchronizes AI-agent generated files (Claude, Gemini, Serena, Superpowers, etc.) across projects and machines via a private git repository, leaving zero traces in working project repositories.

## Problem

AI agents generate configuration and artifact files (`.claude/`, `.serena/`, `CLAUDE.md`, `docs/superpowers/`, etc.) inside working projects. These files:

- Should not appear in project git history (no traces of AI usage)
- Need to be shared across team members and machines
- Need to survive project cloning / re-setup

## Solution: Copy-sync via private git repo

A dedicated private git repository stores canonical copies of all AI files. A Go CLI tool copies files between working projects and the sync repo, using git hooks for automation.

## Architecture

### Sync Repository Structure

```
<sync-repo>/
├── .gitattributes               # * text=auto (line ending normalization)
├── global/
│   └── <machine-id>/            # Global configs per machine (hostname or user-defined)
│       ├── claude/
│       └── serena/
├── projects/
│   ├── my-app--a1b2c3d4e5/        # <repo-name>--<10-char hash from remote URL>
│   │   ├── .claude/
│   │   ├── .serena/
│   │   ├── docs/superpowers/
│   │   └── .ghost-sync.meta     # See Meta File Schema below
│   └── another-project--d4e5f6a7b8/
```

**Note:** The sync data repo is separate from the ghost-sync tool source code repo. The tool source lives in its own repository; the sync repo only contains AI files.

### Meta File Schema (`.ghost-sync.meta`)

```yaml
remote_url: "git@github.com:team/my-app.git"
project_name: "my-app"
project_id: "a1b2c3d4e5"
last_sync_at: "2026-03-18T15:30:00Z"
last_sync_commit: "abc1234"            # Short SHA of last synced commit in working project
synced_by: "sokol@workstation"         # <user>@<hostname>
```

### Project Identification

- `id` = first 10 characters of SHA256 hex digest of git remote origin URL (normalized: trimmed, lowercased, `.git` suffix stripped)
- 10-char hex prefix provides ~1 trillion unique values, collision-safe for any practical number of projects
- ID is deterministic: same remote URL always produces the same ID on any machine
- All team members get the same project ID for the same repository
- `name` = human-readable name (auto-detected from repo name)
- `path` = local filesystem path (stored only in local config, not synced)

### Global Config (`~/.ghost-sync/config.yaml`)

```yaml
version: 1                         # Config schema version (for future migrations)

sync_repo: "git@github.com:user/ai-ghost-sync.git"       # Remote URL (SSH or HTTPS)
sync_repo_path: "C:/Users/sokol/Documents/ai-ghost-sync"  # Local clone directory

machine_id: "sokol-workstation"    # Auto-generated from hostname, overridable

patterns:
  - ".claude/"
  - ".serena/"
  - "docs/superpowers/"
  - "CLAUDE.md"
  - "GEMINI.md"
  - "AGENTS.md"

ignore:
  - "node_modules/"
  - ".claude/cache/"

max_file_size: "10MB"

conflict_strategy: "latest-wins"  # latest-wins | manual | backup-and-overwrite

conflict_rules:
  - pattern: "CLAUDE.md"
    strategy: "manual"
  - pattern: ".claude/skills/"
    strategy: "latest-wins"

projects:
  - name: "my-app"
    remote: "git@github.com:team/my-app.git"
    id: "a1b2c3d4e5"
    path: "C:/Users/sokol/Documents/my-app"
    patterns_override:             # Optional: per-project pattern additions/exclusions
      add:
        - ".cursor/"
      exclude:
        - ".serena/"               # This project doesn't use Serena

global_sync:
  enabled: true
  paths:
    - "~/.claude/"
    - "~/.serena/"
```

Config is auto-created with sensible defaults on first `ghost-sync init`.

On config schema changes, the `version` field enables auto-migration: if the loaded version is older than current, ghost-sync migrates the config in-place and prints a notice.

### Symlinks

Symlinks within synced directories are **skipped** during copy with a warning. AI agent files should not contain symlinks; if they do, it indicates a configuration issue that the user should resolve.

## CLI Commands

| Command | Description |
|---------|-------------|
| `ghost-sync init` | Initialize sync repo (clone existing or create new). Creates `.gitattributes` with `* text=auto` for cross-platform line ending normalization |
| `ghost-sync add` | Register current working project in sync repo |
| `ghost-sync remove` | Unregister project from registry, clean up `.git/info/exclude` block (files in sync repo remain) |
| `ghost-sync push` | Copy AI files from working project to sync repo, commit and push |
| `ghost-sync pull` | Pull sync repo, copy AI files into working project |
| `ghost-sync sync` | Pull first, resolve conflicts, then push changed files (see Sync Flow) |
| `ghost-sync status` | Show diff between working project and sync repo |
| `ghost-sync check` | Session hook: detect project, report sync status |
| `ghost-sync hooks install` | Install git hooks in current working project |
| `ghost-sync hooks remove` | Remove git hooks |
| `ghost-sync config` | Show/edit global config |
| `ghost-sync list` | List registered projects |
| `ghost-sync log` | Show recent sync operations (last 50 entries by default; supports `--lines N`, `--project <name>`) |
| `ghost-sync version` | Show ghost-sync version and build info |

### Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be done without executing |
| `--force` | Overwrite without conflict checks |
| `--global` | Operate on global configs instead of project-level |
| `--verbose` | Detailed output |
| `--pattern <glob>` | Sync only files matching pattern |

### Usage Examples

```bash
# First-time setup
ghost-sync init --repo git@github.com:me/ai-ghost-sync.git

# Register a working project
cd ~/projects/my-app
ghost-sync add
ghost-sync hooks install

# Manual sync
ghost-sync pull
ghost-sync push
ghost-sync status

# Global configs
ghost-sync push --global
ghost-sync pull --global
```

## Sync Flow (`ghost-sync sync`)

Detailed sequence for the `sync` command:

1. **Pull sync repo** — `git pull` in the sync data repo
2. **Compare** — diff local AI files vs sync repo copy (content hash + mtime)
3. **Resolve conflicts** — apply configured strategy per file:
   - `latest-wins`: auto-resolve, continue
   - `manual`: prompt user for each conflict; if non-interactive (hook), skip file and log
   - `backup-and-overwrite`: backup local, take remote
4. **Copy remote → local** — update local AI files from sync repo (resolved files only)
5. **Copy local → sync repo** — copy locally-changed AI files to sync repo
6. **Commit & push** — commit changes in sync repo, push in background

If conflicts remain unresolved (manual mode, user skipped): push proceeds with non-conflicted files only. Skipped files are logged to `~/.ghost-sync/logs/` and reported on next `ghost-sync status`.

## File Deletion Policy

- **Remote file deleted** (exists locally, removed from sync repo): on `ghost-sync pull`, the local file is **deleted** to match sync repo state. A backup is saved in `~/.ghost-sync/backups/<timestamp>/` regardless of conflict strategy.
- **Local file deleted** (removed from working project, exists in sync repo): on `ghost-sync push`, the file is **deleted from sync repo**. The deletion is committed with message `"sync: my-app — deleted <file>"`.
- **Both deleted**: no action needed, clean state.

This ensures deletions propagate in both directions. Backups protect against accidental loss.

## Logging

All operations are logged to `~/.ghost-sync/logs/ghost-sync.log` (rotated, max 10MB).

- Hook operations (background push, auto-pull) always log
- CLI commands log at `--verbose` level by default
- `ghost-sync log` — tail recent log entries (shortcut for viewing)

## Git Hooks

`ghost-sync hooks install` creates hooks in `.git/hooks/` of the working project (local, never committed).

### post-commit

1. Check if committed files match sync patterns
2. If yes — copy changed files to sync repo
3. Commit in sync repo: `"sync: my-app@<short-sha> — <commit-msg>"`
4. Push sync repo in background (non-blocking)
5. On push failure — log warning, retry on next sync

### post-merge

1. Run `ghost-sync pull` for current project
2. Update AI files from sync repo (team members may have pushed new skills)

**Hook safety:** Hook scripts check for `ghost-sync` binary availability. If binary is not found (uninstalled, new machine), hooks exit silently with code 0 and print a one-line warning to stderr. Hooks never block or fail git operations.

Hooks never block work in the working project.

## Zero Trace in Working Projects

### `.git/info/exclude` (per-project, local)

`ghost-sync add` writes ignore patterns into `.git/info/exclude` — a local gitignore that is never committed or pushed.

```gitignore
# ghost-sync: AI agent files (managed automatically, do not edit)
.claude/
.serena/
CLAUDE.md
GEMINI.md
AGENTS.md
docs/superpowers/
# ghost-sync: end
```

`ghost-sync remove` cleans up this block.

Only registered projects have AI files ignored. Unregistered projects are not affected.

## AI Agent Integration

### Claude Code session hook

In `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{
      "command": "ghost-sync check",
      "timeout": 5000
    }]
  }
}
```

`ghost-sync check` behavior:

1. Detect project by cwd
2. If not registered: `"Project 'my-app' is not synced. Run: ghost-sync add"`
3. If registered, updates available: `"ghost-sync: 3 files updated since last sync. Run: ghost-sync pull"`
4. If up-to-date: silence

Analogous hooks can be configured for Gemini and other agents that support session hooks.

## Conflict Resolution

Three strategies, configurable globally and per-pattern:

### `latest-wins` (default)

Compares files by content hash (SHA256). If content differs, the file with more recent modification time wins. If mtimes are equal (clock skew), the sync-repo commit timestamp is used as tiebreaker. This avoids false conflicts from file copies that change mtime but not content.

### `manual`

Stops and prompts the user:

```
CONFLICT: .claude/skills/my-skill.md
  Local modified:  2026-03-18 14:30
  Remote modified: 2026-03-18 15:10

  [l] Keep local
  [r] Keep remote
  [d] Show diff
  [m] Open in merge tool ($EDITOR)
```

In hook context (non-interactive): skips conflicting file, logs warning, defers to next manual `ghost-sync pull`.

### `backup-and-overwrite`

Takes remote version, saves local copy in `~/.ghost-sync/backups/<timestamp>/`.

### Backup Retention

Backups (from deletion policy and `backup-and-overwrite` strategy) are retained for 30 days. `ghost-sync` prunes expired backups automatically during any sync operation. Maximum total backup size: 500MB; oldest backups are pruned first when limit is exceeded.

## Error Handling and Edge Cases

| Scenario | Behavior |
|----------|----------|
| **Network failure on push** | Files committed locally in sync repo. Next push/hook retries. Pending state tracked in `~/.ghost-sync/state.json` |
| **Concurrent local access** | Local file lock (`~/.ghost-sync/ghost-sync.lock`). Wait up to 10s, then skip with warning. Cross-machine concurrency handled by git's own merge/push. Hooks never block working project |
| **Project without remote** | Warning on `ghost-sync add`. Uses directory name hash as ID. No team sync |
| **Missing sync repo path** | `ghost-sync check` suggests `ghost-sync init` |
| **Missing working project path** | `ghost-sync list` marks as `[missing]`. Pull errors with suggestion to remove |
| **Large files (>10MB default)** | Skipped with warning |
| **Interrupted sync (crash)** | Next run checks sync repo state, resets if dirty |
| **Empty project (no AI files)** | `ghost-sync add` works. `ghost-sync push` prints "Nothing to sync" |

### State File (`~/.ghost-sync/state.json`)

Tracks operational state between runs:

```json
{
  "pending_pushes": [
    {
      "project_id": "a1b2c3d4e5",
      "sync_repo_commit": "abc1234",
      "timestamp": "2026-03-18T15:30:00Z",
      "reason": "network error: connection refused"
    }
  ],
  "last_backup_prune": "2026-03-18T00:00:00Z"
}
```

Pending pushes are retried on next `ghost-sync push`, `ghost-sync sync`, or post-commit hook execution.

### `ghost-sync check` Behavior

`ghost-sync check` is **fully offline** — it only compares local working project files against the local sync repo clone. It does NOT run `git fetch`. This guarantees fast execution within the session hook timeout (5s). To get remote updates, `ghost-sync pull` must be run explicitly or via post-merge hook.

### `ghost-sync init` Modes

- `ghost-sync init --repo <url>` — clones existing remote sync repo to `sync_repo_path`
- `ghost-sync init --path <dir>` — creates a new local sync repo at the given path (for later adding a remote)
- If neither flag is given, prompts interactively

`sync_repo` is the remote URL, `sync_repo_path` is the local clone directory. Both are auto-populated in config after init.

### Authentication

Ghost-sync delegates authentication to git. Both SSH (`git@github.com:...`) and HTTPS (`https://github.com/...`) remote URLs are supported. Authentication errors are surfaced as-is from git with an additional hint: `"Check your SSH keys or git credential helper"`.

## Go Project Structure

```
ai-ghost-sync/
├── cmd/
│   └── ghost-sync/
│       └── main.go              # Entry point, root cobra command
├── internal/
│   ├── cli/                     # Command definitions
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── add.go
│   │   ├── remove.go
│   │   ├── push.go
│   │   ├── pull.go
│   │   ├── sync.go
│   │   ├── status.go
│   │   ├── check.go
│   │   ├── list.go
│   │   ├── config.go
│   │   └── hooks.go
│   ├── config/                  # Config loading/writing
│   │   ├── config.go            # Config struct, Load/Save
│   │   ├── defaults.go          # Default patterns
│   │   └── state.go             # State file (pending pushes, etc.)
│   ├── project/                 # Working project operations
│   │   ├── detect.go            # Detect project by cwd, remote URL → ID
│   │   ├── registry.go          # Register/unregister projects
│   │   └── exclude.go           # Manage .git/info/exclude
│   ├── sync/                    # Core sync engine
│   │   ├── copier.go            # Copy files by patterns
│   │   ├── diff.go              # Compare files (mtime + checksum)
│   │   ├── conflict.go          # Conflict resolution strategies
│   │   └── lock.go              # File locking
│   ├── repo/                    # Sync git repo operations
│   │   ├── repo.go              # Clone, commit, push, pull
│   │   └── background.go        # Background push
│   └── hooks/                   # Git hook install/remove
│       ├── install.go
│       └── templates.go         # Hook script templates
├── go.mod
├── go.sum
├── Makefile                     # Build, test, lint, release
└── .goreleaser.yaml             # Cross-compile: windows, darwin, linux
```

### Dependencies

- `cobra` — CLI framework
- `yaml.v3` — config parsing
- Standard library for everything else (`os/exec` for git, `filepath` for patterns, `crypto/sha256` for hashes)

### Build and Distribution

- `goreleaser` — builds binaries for windows/amd64, darwin/amd64, darwin/arm64, linux/amd64
- Install via GitHub Releases or `go install`
