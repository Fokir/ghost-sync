# ghost-sync

ghost-sync is a cross-platform CLI tool written in Go that synchronizes AI-agent generated files (`.claude/`, `.serena/`, `CLAUDE.md`, `docs/superpowers/`, etc.) across projects and machines via a private git repository, leaving zero traces in working project repositories.

## The Problem

AI agents generate configuration and artifact files inside working projects. These files:

- Should not appear in project git history (no traces of AI usage)
- Need to be shared across team members and machines
- Need to survive project cloning and re-setup

## Solution

A dedicated private git repository stores canonical copies of all AI files. ghost-sync copies files between working projects and the sync repo, using git hooks for automation. Working projects stay clean: AI files are excluded via `.git/info/exclude`, which is local and never committed.

## Features

- **Zero trace** — uses `.git/info/exclude` (local, never committed) to hide AI files from working project git history
- **Copy-sync approach** — private git repo stores canonical copies; no submodules, no symlinks
- **Automatic sync** — git hooks (`post-commit`, `post-merge`) trigger sync without manual intervention
- **AI agent session hooks** — `ghost-sync check` integrates as a Claude Code (or other agent) session hook
- **Cross-platform** — Windows, macOS, Linux
- **Conflict resolution** — `latest-wins` (default), `manual`, or `backup-and-overwrite` strategies, configurable per file pattern
- **Per-project pattern overrides** — add or exclude patterns on a per-project basis
- **Global config sync** — optionally sync `~/.claude/`, `~/.serena/`, etc. across machines
- **File locking** — protects against concurrent access
- **Backup with retention** — automatic backups on deletion and overwrite; 30-day retention, 500MB max

## Installation

**One-liner** (Linux / macOS / Git Bash on Windows):

```bash
curl -fsSL https://raw.githubusercontent.com/Fokir/ghost-sync/master/scripts/install.sh | bash
```

**Specific version:**

```bash
GHOST_SYNC_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/Fokir/ghost-sync/master/scripts/install.sh | bash
```

**Custom install directory:**

```bash
INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/Fokir/ghost-sync/master/scripts/install.sh | bash
```

**From source:**

```bash
go install github.com/sokolovsky/ghost-sync/cmd/ghost-sync@latest
```

## Quick Start

```bash
# 1. Initialize (clone existing or create a new sync repo)
ghost-sync init --repo git@github.com:you/ai-sync.git

# 2. Register a working project
cd ~/projects/my-app
ghost-sync add
ghost-sync hooks install

# 3. Files sync automatically via git hooks.
# Or sync manually:
ghost-sync push    # push local AI files to sync repo
ghost-sync pull    # pull AI files from sync repo
ghost-sync sync    # pull then push (full bidirectional sync)
ghost-sync status  # show diff between local and sync repo
```

## Commands

```
ghost-sync init        Initialise ghost-sync and configure the sync repository
ghost-sync add         Register the current project with ghost-sync
ghost-sync remove      Unregister the current project from ghost-sync
ghost-sync push        Push local AI files to the sync repository
ghost-sync pull        Pull AI files from the sync repository to the local project
ghost-sync sync        Pull then push — full bidirectional sync
ghost-sync status      Show sync status for the current project
ghost-sync check       Check for remote updates (session hook, offline)
ghost-sync hooks       Manage git hooks (install / remove)
ghost-sync list        List all registered projects
ghost-sync config      Show current ghost-sync configuration
ghost-sync log         Show ghost-sync operation log
ghost-sync version     Print version information
```

Global flags available on all commands:

```
-n, --dry-run    Simulate actions without making changes
-v, --verbose    Enable verbose output
```

The `log` command supports `--lines N` and `--project <name>` for filtering.

## How It Works

1. `ghost-sync init` — clones or creates a private git repo that acts as sync storage. Creates `~/.ghost-sync/config.yaml` with default settings.

2. `ghost-sync add` — registers the current project in config, writes AI file patterns to `.git/info/exclude` so they are never committed in the working repo.

3. `ghost-sync hooks install` — installs `post-commit` and `post-merge` hooks in `.git/hooks/` (local, never committed):
   - `post-commit` — copies changed AI files to sync repo, commits, and pushes in the background
   - `post-merge` — runs `ghost-sync pull` to pick up changes from team members

4. On `ghost-sync push` — AI files are copied to the sync repo under `projects/<name>--<id>/`, committed, and pushed.

5. On `ghost-sync pull` — sync repo is pulled, AI files are copied back to the working project with conflict resolution applied.

6. **Project ID** — derived from the SHA256 hash of the git remote URL (first 10 hex chars). The same remote always produces the same ID on any machine, so all team members share the same sync directory for a given project.

Hook scripts check for the `ghost-sync` binary before running. If the binary is absent (e.g., new machine), hooks exit silently with code 0 and print a one-time warning to stderr. Git operations are never blocked.

## Configuration

Default config location: `~/.ghost-sync/config.yaml`

```yaml
version: 1

sync_repo: "git@github.com:you/ai-sync.git"
sync_repo_path: "/home/you/ai-sync"

machine_id: "your-hostname"

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

projects:
  - name: "my-app"
    remote: "git@github.com:team/my-app.git"
    id: "a1b2c3d4e5"
    path: "/home/you/projects/my-app"
    patterns_override:
      add:
        - ".cursor/"
      exclude:
        - ".serena/"

global_sync:
  enabled: false
  paths:
    - "~/.claude/"
    - "~/.serena/"
```

Config is auto-created with defaults on first `ghost-sync init`. The `version` field enables automatic migration when the schema changes.

### Conflict Resolution

Three strategies, configurable globally and per file pattern:

- **`latest-wins`** (default) — the file with the more recent modification time wins; sync repo commit timestamp is used as a tiebreaker on clock skew
- **`manual`** — prompts the user to keep local, keep remote, show diff, or open a merge tool; in non-interactive (hook) context, skips the conflicting file and defers to the next manual `ghost-sync pull`
- **`backup-and-overwrite`** — takes the remote version and saves the local copy in `~/.ghost-sync/backups/<timestamp>/`

Backups from any strategy are retained for 30 days (max 500MB; oldest pruned first).

### File Deletion

- Remote file deleted → local file is deleted on `ghost-sync pull` (backup saved automatically)
- Local file deleted → file is removed from the sync repo on `ghost-sync push`

### Sync Repository Structure

```
<sync-repo>/
├── .gitattributes
├── global/
│   └── <machine-id>/
│       ├── claude/
│       └── serena/
└── projects/
    └── my-app--a1b2c3d4e5/
        ├── .claude/
        ├── .serena/
        ├── docs/superpowers/
        └── .ghost-sync.meta
```

## AI Agent Integration

Add `ghost-sync check` as a session start hook in your AI agent. For Claude Code, add to `~/.claude/settings.json`:

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

`ghost-sync check` is fully offline — it compares local project files against the local sync repo clone without any network access, ensuring it completes within the hook timeout. Output:

- Unregistered project: `"Project 'my-app' is not synced. Run: ghost-sync add"`
- Updates available: `"ghost-sync: 3 files updated since last sync. Run: ghost-sync pull"`
- Up to date: silent

Analogous hooks can be configured for Gemini and other agents that support session hooks.

## Building

```bash
make build        # Build for current platform
make test         # Run tests
go test ./... -v  # Run all tests with verbose output
```

Cross-platform releases (Windows/amd64, macOS/amd64, macOS/arm64, Linux/amd64, Linux/arm64) are built via goreleaser.

## Releasing

```bash
# Auto-detect version bump from conventional commits (feat → minor, fix → patch)
./scripts/release.sh

# Force a specific bump
./scripts/release.sh minor

# Dry run — show what would happen
DRY_RUN=1 ./scripts/release.sh
```

The script determines the next semver tag from commit history, creates an annotated git tag, and pushes it. GitHub Actions then builds and publishes binaries via goreleaser.

## License

MIT
