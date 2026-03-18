# ghost-sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cross-platform Go CLI tool that synchronizes AI-agent files across projects and machines via a private git repo, with zero trace in working projects.

**Architecture:** Copy-sync approach — a dedicated git repo stores canonical copies of AI files. CLI tool copies files bidirectionally between working projects and sync repo. Git hooks automate sync on commit/merge. `.git/info/exclude` hides AI files from working project git.

**Tech Stack:** Go 1.22+, cobra (CLI), yaml.v3 (config), goreleaser (cross-compile)

**Spec:** `docs/superpowers/specs/2026-03-18-ghost-sync-design.md`

---

## Known Issues & Corrections

These corrections MUST be applied during implementation. The code blocks below contain known issues that require fixes:

### Critical Fixes

1. **`add.go` missing import**: Add `"path/filepath"` to imports in `internal/cli/add.go`.

2. **Duplicate `ConflictRule` type**: Remove `ConflictRule` struct from `internal/sync/conflict.go`. Import and use `config.ConflictRule` from `internal/config/config.go` instead. Update `GetConflictStrategy` signature to accept `[]config.ConflictRule`.

3. **`sync_cmd.go` broken flag propagation**: Extract core push/pull logic into shared functions (`doPush(cfg, proj, fromHook)` and `doPull(cfg, proj, fromHook)`) in separate files. Both CLI commands and `sync_cmd.go` should call these shared functions instead of constructing new cobra commands.

4. **Symlink detection broken with `filepath.Walk`**: In both `diff.go:collectFiles()` and `copier.go:CopyByPatterns()`, replace `filepath.Walk` with `filepath.WalkDir` and use `d.Type()&os.ModeSymlink != 0` to detect symlinks. `filepath.Walk` resolves symlinks before calling the callback, making the current check dead code.

### Important Fixes

5. **DATA LOSS BUG in `pull.go`**: `DiffFiles(gitRoot, projDir)` compares ALL files in gitRoot (including source code) vs sync repo. The `LocalOnly` list then includes every non-AI file. The code iterates `diff.LocalOnly` and deletes them. **Fix**: Filter `DiffFiles` results by patterns before acting on deletions. Only delete files whose relative path matches sync patterns. OR pass patterns to `DiffFiles` so it only collects matching files.

6. **Global sync passes empty pattern `""`**: `CopyByPatterns(expanded, dstDir, []string{""}, ...)` matches nothing. **Fix**: For global sync, use a dedicated `CopyAll(src, dst, ignore, maxSize)` function that copies everything except ignored paths, or use pattern `[]string{"./"}` and adjust `matchesPatterns` to handle it.

7. **File locking never used**: `AcquireLock`/`ReleaseLock` are implemented but never called in CLI commands. **Fix**: Wrap push, pull, sync operations with lock acquisition in the CLI layer.

8. **`.ghost-sync.meta` never written**: The spec requires a meta file per project in sync repo. **Fix**: After each push, write/update `.ghost-sync.meta` in the project directory within sync repo.

9. **Push doesn't propagate deletions**: Files deleted from working project are not removed from sync repo. **Fix**: In push command, run `DiffFiles` to find `RemoteOnly` files (exist in sync repo but not locally) and delete them from sync repo before committing.

10. **`max_file_size` string never parsed**: Config stores `"10MB"` as string, but `CopyByPatterns` expects `int64`. **Fix**: Add `ParseFileSize(s string) (int64, error)` helper to config package. Parse in CLI commands before passing to copier.

11. **`TailByProject` has dead code**: The function calls `Tail(path, 0)`, stores result, then reads the file again separately and has `_ = lines`. **Fix**: Remove the initial `Tail` call and the unused variable.

12. **No `manual` conflict strategy prompt**: Only `latest-wins` and `backup-and-overwrite` are implemented. **Fix**: Add `ResolveManual(entry ConflictEntry) ConflictAction` with interactive stdin prompt supporting [l]/[r]/[d]/[m] options. When non-interactive (hook context, detected via `os.Stdin` isatty check), return `Skip`.

### Deferred (v2)

These spec features are intentionally deferred to keep v1 scope manageable:
- Log rotation (max 10MB)
- Interrupted sync recovery (dirty state detection)
- `--pattern <glob>` flag
- `--force` flag
- Pending push retry on startup
- `ghost-sync check` suggesting `ghost-sync init` when config missing

---

## File Structure

```
ai-ghost-sync/
├── cmd/ghost-sync/main.go           # Entry point, wire root command
├── internal/
│   ├── config/
│   │   ├── config.go                # Config struct, Load(), Save(), EnsureDefaults()
│   │   ├── config_test.go
│   │   ├── defaults.go              # DefaultPatterns, DefaultIgnore, DefaultConfig()
│   │   ├── defaults_test.go
│   │   ├── state.go                 # State struct, LoadState(), SaveState()
│   │   └── state_test.go
│   ├── project/
│   │   ├── detect.go                # DetectProject() — cwd → remote URL → project ID
│   │   ├── detect_test.go
│   │   ├── registry.go              # AddProject(), RemoveProject(), FindProject()
│   │   ├── registry_test.go
│   │   ├── exclude.go               # WriteExclude(), RemoveExclude()
│   │   └── exclude_test.go
│   ├── sync/
│   │   ├── copier.go                # CopyFiles() — pattern-based file copy
│   │   ├── copier_test.go
│   │   ├── diff.go                  # DiffFiles() — compare local vs sync repo
│   │   ├── diff_test.go
│   │   ├── conflict.go              # ResolveConflict() — strategy dispatch
│   │   ├── conflict_test.go
│   │   ├── lock.go                  # AcquireLock(), ReleaseLock()
│   │   └── lock_test.go
│   ├── repo/
│   │   ├── repo.go                  # Clone(), Pull(), Commit(), Push()
│   │   ├── repo_test.go
│   │   ├── background.go            # BackgroundPush()
│   │   └── background_test.go
│   ├── hooks/
│   │   ├── install.go               # Install(), Remove()
│   │   ├── install_test.go
│   │   ├── templates.go             # PostCommitScript(), PostMergeScript()
│   │   └── templates_test.go
│   ├── logging/
│   │   ├── logger.go                # Init(), Log(), Tail()
│   │   └── logger_test.go
│   ├── backup/
│   │   ├── backup.go                # Create(), Prune()
│   │   └── backup_test.go
│   └── cli/
│       ├── root.go                  # Root cobra command, version flag
│       ├── init.go                  # ghost-sync init
│       ├── add.go                   # ghost-sync add
│       ├── remove.go                # ghost-sync remove
│       ├── push.go                  # ghost-sync push
│       ├── pull.go                  # ghost-sync pull
│       ├── sync_cmd.go              # ghost-sync sync (sync_cmd to avoid Go keyword)
│       ├── status.go                # ghost-sync status
│       ├── check.go                 # ghost-sync check
│       ├── list.go                  # ghost-sync list
│       ├── config_cmd.go            # ghost-sync config
│       ├── hooks_cmd.go             # ghost-sync hooks install/remove
│       ├── log_cmd.go               # ghost-sync log
│       └── version.go               # ghost-sync version
├── go.mod
├── go.sum
├── Makefile
└── .goreleaser.yaml
```

---

### Task 1: Project Bootstrap

**Files:**
- Create: `go.mod`
- Create: `cmd/ghost-sync/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/version.go`
- Create: `Makefile`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd C:/Users/sokol/Documents/ai-ghost-sync
go mod init github.com/sokolovsky/ghost-sync
```

- [ ] **Step 2: Add cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Create root command**

Create `internal/cli/root.go`:

```go
package cli

import (
	"github.com/spf13/cobra"
)

var (
	verbose bool
	dryRun  bool
)

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ghost-sync",
		Short: "Sync AI-agent files across projects and machines",
		Long:  "ghost-sync synchronizes AI-generated files (.claude/, .serena/, CLAUDE.md, etc.) via a private git repository, leaving zero traces in working projects.",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Detailed output")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")

	return cmd
}
```

- [ ] **Step 4: Create version command**

Create `internal/cli/version.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVersionCmd(version, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show ghost-sync version and build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ghost-sync %s\n", version)
			if commit != "" {
				fmt.Printf("  commit: %s\n", commit)
			}
			if date != "" {
				fmt.Printf("  built:  %s\n", date)
			}
		},
	}
}
```

- [ ] **Step 5: Create main.go entry point**

Create `cmd/ghost-sync/main.go`:

```go
package main

import (
	"os"

	"github.com/sokolovsky/ghost-sync/internal/cli"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	rootCmd := cli.NewRootCmd(version)
	rootCmd.AddCommand(cli.NewVersionCmd(version, commit, date))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Create Makefile**

Create `Makefile`:

```makefile
BINARY := ghost-sync
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build test lint clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/ghost-sync

test:
	go test ./... -v

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
```

- [ ] **Step 7: Verify build**

Run: `cd C:/Users/sokol/Documents/ai-ghost-sync && go build ./cmd/ghost-sync`
Expected: Builds without errors.

Run: `go run ./cmd/ghost-sync version`
Expected: `ghost-sync dev`

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum cmd/ internal/cli/root.go internal/cli/version.go Makefile
git commit -m "feat: bootstrap project with cobra CLI and version command"
```

---

### Task 2: Config System

**Files:**
- Create: `internal/config/defaults.go`
- Create: `internal/config/defaults_test.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/state.go`
- Create: `internal/config/state_test.go`

- [ ] **Step 1: Add yaml.v3 dependency**

Run: `go get gopkg.in/yaml.v3@latest`

- [ ] **Step 2: Write test for default patterns**

Create `internal/config/defaults_test.go`:

```go
package config

import "testing"

func TestDefaultPatterns(t *testing.T) {
	patterns := DefaultPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected default patterns, got none")
	}

	expected := []string{".claude/", ".serena/", "docs/superpowers/", "CLAUDE.md", "GEMINI.md", "AGENTS.md"}
	for _, exp := range expected {
		found := false
		for _, p := range patterns {
			if p == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing default pattern: %s", exp)
		}
	}
}

func TestDefaultIgnore(t *testing.T) {
	ignore := DefaultIgnore()
	if len(ignore) == 0 {
		t.Fatal("expected default ignore patterns, got none")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestDefault -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 4: Implement defaults.go**

Create `internal/config/defaults.go`:

```go
package config

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

func DefaultIgnore() []string {
	return []string{
		"node_modules/",
		".claude/cache/",
	}
}

const (
	DefaultMaxFileSize     = "10MB"
	DefaultConflictStrategy = "latest-wins"
	ConfigVersion          = 1
)
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestDefault -v`
Expected: PASS

- [ ] **Step 6: Write test for config Load/Save**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Version:          ConfigVersion,
		SyncRepo:         "git@github.com:user/sync.git",
		SyncRepoPath:     "/tmp/sync",
		MachineID:        "test-machine",
		Patterns:         DefaultPatterns(),
		Ignore:           DefaultIgnore(),
		MaxFileSize:      DefaultMaxFileSize,
		ConflictStrategy: DefaultConflictStrategy,
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.SyncRepo != cfg.SyncRepo {
		t.Errorf("SyncRepo mismatch: got %q, want %q", loaded.SyncRepo, cfg.SyncRepo)
	}
	if loaded.Version != ConfigVersion {
		t.Errorf("Version mismatch: got %d, want %d", loaded.Version, ConfigVersion)
	}
	if loaded.MachineID != "test-machine" {
		t.Errorf("MachineID mismatch: got %q", loaded.MachineID)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestEnsureDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := EnsureDefaults(path)
	if err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	if cfg.Version != ConfigVersion {
		t.Errorf("expected version %d, got %d", ConfigVersion, cfg.Version)
	}
	if len(cfg.Patterns) == 0 {
		t.Error("expected default patterns")
	}
	if cfg.MachineID == "" {
		t.Error("expected auto-generated machine ID")
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestProjectPatternsOverride(t *testing.T) {
	cfg := &Config{
		Patterns: DefaultPatterns(),
	}
	proj := ProjectEntry{
		Name: "test",
		ID:   "abc123",
		PatternsOverride: &PatternsOverride{
			Add:     []string{".cursor/"},
			Exclude: []string{".serena/"},
		},
	}

	effective := cfg.EffectivePatterns(proj)

	// Should include .cursor/
	hasCursor := false
	for _, p := range effective {
		if p == ".cursor/" {
			hasCursor = true
		}
		if p == ".serena/" {
			t.Error(".serena/ should be excluded")
		}
	}
	if !hasCursor {
		t.Error("missing .cursor/ from override add")
	}
}
```

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestConfig -v`
Expected: FAIL — types not defined yet.

- [ ] **Step 8: Implement config.go**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type PatternsOverride struct {
	Add     []string `yaml:"add,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type ProjectEntry struct {
	Name             string            `yaml:"name"`
	Remote           string            `yaml:"remote"`
	ID               string            `yaml:"id"`
	Path             string            `yaml:"path"`
	PatternsOverride *PatternsOverride `yaml:"patterns_override,omitempty"`
}

type ConflictRule struct {
	Pattern  string `yaml:"pattern"`
	Strategy string `yaml:"strategy"`
}

type GlobalSync struct {
	Enabled bool     `yaml:"enabled"`
	Paths   []string `yaml:"paths,omitempty"`
}

type Config struct {
	Version          int            `yaml:"version"`
	SyncRepo         string         `yaml:"sync_repo"`
	SyncRepoPath     string         `yaml:"sync_repo_path"`
	MachineID        string         `yaml:"machine_id"`
	Patterns         []string       `yaml:"patterns"`
	Ignore           []string       `yaml:"ignore"`
	MaxFileSize      string         `yaml:"max_file_size"`
	ConflictStrategy string         `yaml:"conflict_strategy"`
	ConflictRules    []ConflictRule `yaml:"conflict_rules,omitempty"`
	Projects         []ProjectEntry `yaml:"projects,omitempty"`
	GlobalSync       GlobalSync     `yaml:"global_sync"`
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ghost-sync")
}

func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func EnsureDefaults(path string) (*Config, error) {
	if _, err := os.Stat(path); err == nil {
		return Load(path)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	cfg := &Config{
		Version:          ConfigVersion,
		MachineID:        hostname,
		Patterns:         DefaultPatterns(),
		Ignore:           DefaultIgnore(),
		MaxFileSize:      DefaultMaxFileSize,
		ConflictStrategy: DefaultConflictStrategy,
	}

	if err := Save(cfg, path); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) EffectivePatterns(proj ProjectEntry) []string {
	if proj.PatternsOverride == nil {
		return c.Patterns
	}

	excludeSet := make(map[string]bool)
	for _, e := range proj.PatternsOverride.Exclude {
		excludeSet[e] = true
	}

	var result []string
	for _, p := range c.Patterns {
		if !excludeSet[p] {
			result = append(result, p)
		}
	}

	result = append(result, proj.PatternsOverride.Add...)
	return result
}

func (c *Config) FindProject(id string) *ProjectEntry {
	for i := range c.Projects {
		if c.Projects[i].ID == id {
			return &c.Projects[i]
		}
	}
	return nil
}

func (c *Config) FindProjectByPath(path string) *ProjectEntry {
	for i := range c.Projects {
		if c.Projects[i].Path == path {
			return &c.Projects[i]
		}
	}
	return nil
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS.

- [ ] **Step 10: Write test for state file**

Create `internal/config/state_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &State{
		PendingPushes: []PendingPush{
			{
				ProjectID:     "a1b2c3d4e5",
				SyncRepoCommit: "abc1234",
				Timestamp:     time.Now().UTC(),
				Reason:        "network error",
			},
		},
		LastBackupPrune: time.Now().UTC(),
	}

	if err := SaveState(state, path); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if len(loaded.PendingPushes) != 1 {
		t.Fatalf("expected 1 pending push, got %d", len(loaded.PendingPushes))
	}
	if loaded.PendingPushes[0].ProjectID != "a1b2c3d4e5" {
		t.Errorf("ProjectID mismatch: %s", loaded.PendingPushes[0].ProjectID)
	}
}

func TestLoadStateNonExistent(t *testing.T) {
	state, err := LoadState("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("expected empty state for nonexistent file, got error: %v", err)
	}
	if len(state.PendingPushes) != 0 {
		t.Error("expected empty pending pushes")
	}
}
```

- [ ] **Step 11: Implement state.go**

Create `internal/config/state.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PendingPush struct {
	ProjectID      string    `json:"project_id"`
	SyncRepoCommit string    `json:"sync_repo_commit"`
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason"`
}

type State struct {
	PendingPushes   []PendingPush `json:"pending_pushes"`
	LastBackupPrune time.Time     `json:"last_backup_prune"`
}

func DefaultStatePath() string {
	return filepath.Join(ConfigDir(), "state.json")
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func SaveState(state *State, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *State) AddPendingPush(projectID, commit, reason string) {
	s.PendingPushes = append(s.PendingPushes, PendingPush{
		ProjectID:      projectID,
		SyncRepoCommit: commit,
		Timestamp:      time.Now().UTC(),
		Reason:         reason,
	})
}

func (s *State) ClearPendingPushes(projectID string) {
	var remaining []PendingPush
	for _, pp := range s.PendingPushes {
		if pp.ProjectID != projectID {
			remaining = append(remaining, pp)
		}
	}
	s.PendingPushes = remaining
}
```

- [ ] **Step 12: Run all config tests**

Run: `go test ./internal/config/ -v`
Expected: All PASS.

- [ ] **Step 13: Commit**

```bash
git add internal/config/ go.sum
git commit -m "feat: add config system with defaults, load/save, state tracking"
```

---

### Task 3: Project Detection and Registry

**Files:**
- Create: `internal/project/detect.go`
- Create: `internal/project/detect_test.go`
- Create: `internal/project/registry.go`
- Create: `internal/project/registry_test.go`
- Create: `internal/project/exclude.go`
- Create: `internal/project/exclude_test.go`

- [ ] **Step 1: Write test for project ID generation**

Create `internal/project/detect_test.go`:

```go
package project

import "testing"

func TestGenerateProjectID(t *testing.T) {
	tests := []struct {
		remoteURL string
		wantLen   int
	}{
		{"git@github.com:team/my-app.git", 10},
		{"https://github.com/team/my-app.git", 10},
		{"git@github.com:team/my-app", 10},
	}

	for _, tt := range tests {
		id := GenerateProjectID(tt.remoteURL)
		if len(id) != tt.wantLen {
			t.Errorf("GenerateProjectID(%q) len = %d, want %d", tt.remoteURL, len(id), tt.wantLen)
		}
	}
}

func TestGenerateProjectIDDeterministic(t *testing.T) {
	id1 := GenerateProjectID("git@github.com:team/my-app.git")
	id2 := GenerateProjectID("git@github.com:team/my-app.git")
	if id1 != id2 {
		t.Errorf("IDs not deterministic: %q vs %q", id1, id2)
	}
}

func TestGenerateProjectIDNormalization(t *testing.T) {
	// .git suffix should be stripped, so these produce the same ID
	id1 := GenerateProjectID("git@github.com:team/my-app.git")
	id2 := GenerateProjectID("git@github.com:team/my-app")
	if id1 != id2 {
		t.Errorf("IDs differ after .git strip: %q vs %q", id1, id2)
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		remoteURL string
		want      string
	}{
		{"git@github.com:team/my-app.git", "my-app"},
		{"https://github.com/team/my-app.git", "my-app"},
		{"git@github.com:team/my-app", "my-app"},
	}

	for _, tt := range tests {
		got := ExtractRepoName(tt.remoteURL)
		if got != tt.want {
			t.Errorf("ExtractRepoName(%q) = %q, want %q", tt.remoteURL, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestGenerate -v`
Expected: FAIL

- [ ] **Step 3: Implement detect.go**

Create `internal/project/detect.go`:

```go
package project

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func GenerateProjectID(remoteURL string) string {
	normalized := normalizeRemoteURL(remoteURL)
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", hash[:])[:10]
}

func normalizeRemoteURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.ToLower(url)
	url = strings.TrimSuffix(url, ".git")
	return url
}

func ExtractRepoName(remoteURL string) string {
	url := strings.TrimSuffix(remoteURL, ".git")
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	// Handle git@github.com:team/repo format
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[idx+1:]
	}
	return name
}

// DetectProject detects git remote URL and repo name from cwd.
// Returns remoteURL, repoName, error.
func DetectProject(dir string) (string, string, error) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("not a git repo or no remote origin: %w", err)
	}

	remoteURL := strings.TrimSpace(string(out))
	repoName := ExtractRepoName(remoteURL)

	return remoteURL, repoName, nil
}

// ProjectDirName returns the directory name used in sync repo: <name>--<id>
func ProjectDirName(name, id string) string {
	return fmt.Sprintf("%s--%s", name, id)
}

// DetectGitRoot finds the git root directory from the given path.
func DetectGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/ -run TestGenerate -v && go test ./internal/project/ -run TestExtract -v`
Expected: All PASS.

- [ ] **Step 5: Write test for exclude management**

Create `internal/project/exclude_test.go`:

```go
package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteExclude(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git", "info")
	os.MkdirAll(gitDir, 0755)
	excludePath := filepath.Join(gitDir, "exclude")

	// Write initial content
	os.WriteFile(excludePath, []byte("# existing rules\n*.log\n"), 0644)

	patterns := []string{".claude/", ".serena/", "CLAUDE.md"}
	if err := WriteExclude(excludePath, patterns); err != nil {
		t.Fatalf("WriteExclude failed: %v", err)
	}

	data, _ := os.ReadFile(excludePath)
	content := string(data)

	if !strings.Contains(content, "# ghost-sync: AI agent files") {
		t.Error("missing ghost-sync header marker")
	}
	if !strings.Contains(content, ".claude/") {
		t.Error("missing .claude/ pattern")
	}
	if !strings.Contains(content, "# existing rules") {
		t.Error("existing content was removed")
	}
	if !strings.Contains(content, "# ghost-sync: end") {
		t.Error("missing ghost-sync end marker")
	}
}

func TestRemoveExclude(t *testing.T) {
	dir := t.TempDir()
	excludePath := filepath.Join(dir, "exclude")

	content := "# existing\n# ghost-sync: AI agent files (managed automatically, do not edit)\n.claude/\n# ghost-sync: end\n# other\n"
	os.WriteFile(excludePath, []byte(content), 0644)

	if err := RemoveExclude(excludePath); err != nil {
		t.Fatalf("RemoveExclude failed: %v", err)
	}

	data, _ := os.ReadFile(excludePath)
	result := string(data)

	if strings.Contains(result, "ghost-sync") {
		t.Error("ghost-sync block was not removed")
	}
	if !strings.Contains(result, "# existing") {
		t.Error("existing content was removed")
	}
	if !strings.Contains(result, "# other") {
		t.Error("trailing content was removed")
	}
}

func TestWriteExcludeIdempotent(t *testing.T) {
	dir := t.TempDir()
	excludePath := filepath.Join(dir, "exclude")

	patterns := []string{".claude/"}
	WriteExclude(excludePath, patterns)
	WriteExclude(excludePath, append(patterns, ".serena/"))

	data, _ := os.ReadFile(excludePath)
	content := string(data)

	// Should have only one ghost-sync block
	count := strings.Count(content, "# ghost-sync: AI agent files")
	if count != 1 {
		t.Errorf("expected 1 ghost-sync block, got %d", count)
	}
	if !strings.Contains(content, ".serena/") {
		t.Error("updated pattern missing")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestExclude -v`
Expected: FAIL

- [ ] **Step 7: Implement exclude.go**

Create `internal/project/exclude.go`:

```go
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	markerStart = "# ghost-sync: AI agent files (managed automatically, do not edit)"
	markerEnd   = "# ghost-sync: end"
)

func WriteExclude(excludePath string, patterns []string) error {
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return fmt.Errorf("creating exclude dir: %w", err)
	}

	existing := ""
	if data, err := os.ReadFile(excludePath); err == nil {
		existing = string(data)
	}

	// Remove existing ghost-sync block if present
	existing = removeGhostSyncBlock(existing)

	// Build new block
	var block strings.Builder
	block.WriteString(markerStart + "\n")
	for _, p := range patterns {
		block.WriteString(p + "\n")
	}
	block.WriteString(markerEnd + "\n")

	// Append to existing content
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	result := existing + block.String()

	return os.WriteFile(excludePath, []byte(result), 0644)
}

func RemoveExclude(excludePath string) error {
	data, err := os.ReadFile(excludePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	result := removeGhostSyncBlock(string(data))
	return os.WriteFile(excludePath, []byte(result), 0644)
}

func removeGhostSyncBlock(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inBlock := false

	for _, line := range lines {
		if strings.TrimSpace(line) == markerStart {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == markerEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// ExcludePathForRepo returns the .git/info/exclude path for a git repo root.
func ExcludePathForRepo(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "info", "exclude")
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/project/ -v`
Expected: All PASS.

- [ ] **Step 9: Write test for registry**

Create `internal/project/registry_test.go`:

```go
package project

import (
	"testing"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

func TestAddProject(t *testing.T) {
	cfg := &config.Config{
		Version:  config.ConfigVersion,
		Patterns: config.DefaultPatterns(),
	}

	err := AddProject(cfg, "my-app", "git@github.com:team/my-app.git", "a1b2c3d4e5", "/path/to/my-app")
	if err != nil {
		t.Fatalf("AddProject failed: %v", err)
	}

	if len(cfg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "my-app" {
		t.Errorf("project name: got %q, want %q", cfg.Projects[0].Name, "my-app")
	}
}

func TestAddProjectDuplicate(t *testing.T) {
	cfg := &config.Config{}
	AddProject(cfg, "my-app", "git@github.com:team/my-app.git", "a1b2c3d4e5", "/path/1")

	err := AddProject(cfg, "my-app", "git@github.com:team/my-app.git", "a1b2c3d4e5", "/path/2")
	if err == nil {
		t.Fatal("expected error for duplicate project")
	}
}

func TestRemoveProject(t *testing.T) {
	cfg := &config.Config{}
	AddProject(cfg, "my-app", "git@github.com:team/my-app.git", "abc", "/path")

	err := RemoveProject(cfg, "abc")
	if err != nil {
		t.Fatalf("RemoveProject failed: %v", err)
	}
	if len(cfg.Projects) != 0 {
		t.Error("project was not removed")
	}
}
```

- [ ] **Step 10: Implement registry.go**

Create `internal/project/registry.go`:

```go
package project

import (
	"fmt"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

func AddProject(cfg *config.Config, name, remote, id, path string) error {
	if existing := cfg.FindProject(id); existing != nil {
		return fmt.Errorf("project %q (id: %s) already registered", existing.Name, id)
	}

	cfg.Projects = append(cfg.Projects, config.ProjectEntry{
		Name:   name,
		Remote: remote,
		ID:     id,
		Path:   path,
	})

	return nil
}

func RemoveProject(cfg *config.Config, id string) error {
	for i, p := range cfg.Projects {
		if p.ID == id {
			cfg.Projects = append(cfg.Projects[:i], cfg.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project with id %q not found", id)
}
```

- [ ] **Step 11: Run all project tests**

Run: `go test ./internal/project/ -v`
Expected: All PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/project/
git commit -m "feat: add project detection, registry, and git exclude management"
```

---

### Task 4: Sync Engine — File Locking

**Files:**
- Create: `internal/sync/lock.go`
- Create: `internal/sync/lock_test.go`

- [ ] **Step 1: Write test for file locking**

Create `internal/sync/lock_test.go`:

```go
package sync

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireAndReleaseLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "ghost-sync.lock")

	lock, err := AcquireLock(lockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	if err := ReleaseLock(lock); err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}
}

func TestAcquireLockTimeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "ghost-sync.lock")

	// Acquire first lock
	lock1, err := AcquireLock(lockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("first AcquireLock failed: %v", err)
	}
	defer ReleaseLock(lock1)

	// Try second lock with short timeout — should fail
	_, err = AcquireLock(lockPath, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for second lock")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sync/ -run TestAcquire -v`
Expected: FAIL

- [ ] **Step 3: Implement lock.go**

Create `internal/sync/lock.go`:

```go
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Lock struct {
	path string
	file *os.File
}

func AcquireLock(path string, timeout time.Duration) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating lock dir: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Fprintf(file, "%d", os.Getpid())
			return &Lock{path: path, file: file}, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock timeout after %v: %s", timeout, path)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func ReleaseLock(lock *Lock) error {
	if lock == nil {
		return nil
	}
	lock.file.Close()
	return os.Remove(lock.path)
}

func DefaultLockPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ghost-sync", "ghost-sync.lock")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sync/ -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sync/lock.go internal/sync/lock_test.go
git commit -m "feat: add file locking for concurrent access protection"
```

---

### Task 5: Sync Engine — Diff and Copy

**Files:**
- Create: `internal/sync/diff.go`
- Create: `internal/sync/diff_test.go`
- Create: `internal/sync/copier.go`
- Create: `internal/sync/copier_test.go`

- [ ] **Step 1: Write test for file diff**

Create `internal/sync/diff_test.go`:

```go
package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	hash, err := FileHash(path)
	if err != nil {
		t.Fatalf("FileHash failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same content = same hash
	path2 := filepath.Join(dir, "test2.txt")
	os.WriteFile(path2, []byte("hello"), 0644)
	hash2, _ := FileHash(path2)
	if hash != hash2 {
		t.Error("same content should produce same hash")
	}
}

func TestDiffFiles(t *testing.T) {
	localDir := t.TempDir()
	remoteDir := t.TempDir()

	// File only in local
	os.WriteFile(filepath.Join(localDir, "local-only.md"), []byte("local"), 0644)

	// File only in remote
	os.WriteFile(filepath.Join(remoteDir, "remote-only.md"), []byte("remote"), 0644)

	// Same file, same content
	os.WriteFile(filepath.Join(localDir, "same.md"), []byte("same"), 0644)
	os.WriteFile(filepath.Join(remoteDir, "same.md"), []byte("same"), 0644)

	// Same file, different content
	os.WriteFile(filepath.Join(localDir, "changed.md"), []byte("local-version"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(remoteDir, "changed.md"), []byte("remote-version"), 0644)

	result, err := DiffFiles(localDir, remoteDir)
	if err != nil {
		t.Fatalf("DiffFiles failed: %v", err)
	}

	if len(result.LocalOnly) != 1 || result.LocalOnly[0] != "local-only.md" {
		t.Errorf("LocalOnly: got %v", result.LocalOnly)
	}
	if len(result.RemoteOnly) != 1 || result.RemoteOnly[0] != "remote-only.md" {
		t.Errorf("RemoteOnly: got %v", result.RemoteOnly)
	}
	if len(result.Same) != 1 || result.Same[0] != "same.md" {
		t.Errorf("Same: got %v", result.Same)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Path != "changed.md" {
		t.Errorf("Conflicts: got %v", result.Conflicts)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sync/ -run TestFile -v && go test ./internal/sync/ -run TestDiff -v`
Expected: FAIL

- [ ] **Step 3: Implement diff.go**

Create `internal/sync/diff.go`:

```go
package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ConflictEntry struct {
	Path       string
	LocalMtime time.Time
	RemoteMtime time.Time
}

type DiffResult struct {
	LocalOnly  []string        // Files only in local
	RemoteOnly []string        // Files only in remote
	Same       []string        // Files with identical content
	Conflicts  []ConflictEntry // Files with different content
}

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

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func DiffFiles(localDir, remoteDir string) (*DiffResult, error) {
	localFiles, err := collectFiles(localDir)
	if err != nil {
		return nil, fmt.Errorf("scanning local: %w", err)
	}

	remoteFiles, err := collectFiles(remoteDir)
	if err != nil {
		return nil, fmt.Errorf("scanning remote: %w", err)
	}

	result := &DiffResult{}

	// Check all local files
	for relPath := range localFiles {
		if _, exists := remoteFiles[relPath]; !exists {
			result.LocalOnly = append(result.LocalOnly, relPath)
			continue
		}

		localHash, err := FileHash(filepath.Join(localDir, relPath))
		if err != nil {
			return nil, err
		}
		remoteHash, err := FileHash(filepath.Join(remoteDir, relPath))
		if err != nil {
			return nil, err
		}

		if localHash == remoteHash {
			result.Same = append(result.Same, relPath)
		} else {
			localInfo, _ := os.Stat(filepath.Join(localDir, relPath))
			remoteInfo, _ := os.Stat(filepath.Join(remoteDir, relPath))
			result.Conflicts = append(result.Conflicts, ConflictEntry{
				Path:        relPath,
				LocalMtime:  localInfo.ModTime(),
				RemoteMtime: remoteInfo.ModTime(),
			})
		}
	}

	// Check remote-only files
	for relPath := range remoteFiles {
		if _, exists := localFiles[relPath]; !exists {
			result.RemoteOnly = append(result.RemoteOnly, relPath)
		}
	}

	return result, nil
}

func collectFiles(dir string) (map[string]struct{}, error) {
	files := make(map[string]struct{})

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		files[relPath] = struct{}{}
		return nil
	})

	return files, err
}

// IsIgnored checks if a relative path matches any ignore pattern.
func IsIgnored(relPath string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(relPath, pattern) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run diff tests**

Run: `go test ./internal/sync/ -run TestDiff -v && go test ./internal/sync/ -run TestFileHash -v`
Expected: All PASS.

- [ ] **Step 5: Write test for copier**

Create `internal/sync/copier_test.go`:

```go
package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFilesBySinglePattern(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure
	os.MkdirAll(filepath.Join(srcDir, ".claude", "skills"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".claude", "skills", "test.md"), []byte("skill content"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".claude", "config.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(srcDir, "unrelated.txt"), []byte("skip"), 0644)

	patterns := []string{".claude/"}
	copied, err := CopyByPatterns(srcDir, dstDir, patterns, nil, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns failed: %v", err)
	}

	if copied != 2 {
		t.Errorf("expected 2 copied files, got %d", copied)
	}

	// Verify files exist in dst
	if _, err := os.Stat(filepath.Join(dstDir, ".claude", "skills", "test.md")); err != nil {
		t.Error("skill file not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".claude", "config.json")); err != nil {
		t.Error("config file not copied")
	}
	// unrelated.txt should NOT exist
	if _, err := os.Stat(filepath.Join(dstDir, "unrelated.txt")); err == nil {
		t.Error("unrelated file should not be copied")
	}
}

func TestCopyByPatternsWithSingleFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Readme"), 0644)

	patterns := []string{"CLAUDE.md"}
	copied, err := CopyByPatterns(srcDir, dstDir, patterns, nil, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns failed: %v", err)
	}

	if copied != 1 {
		t.Errorf("expected 1 copied file, got %d", copied)
	}
}

func TestCopyByPatternsWithIgnore(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, ".claude", "cache"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".claude", "config.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".claude", "cache", "big.dat"), []byte("cached"), 0644)

	patterns := []string{".claude/"}
	ignore := []string{".claude/cache/"}
	copied, err := CopyByPatterns(srcDir, dstDir, patterns, ignore, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns failed: %v", err)
	}

	if copied != 1 {
		t.Errorf("expected 1 copied file, got %d", copied)
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".claude", "cache", "big.dat")); err == nil {
		t.Error("cached file should be ignored")
	}
}

func TestCopyByPatternsMaxFileSize(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("small"), 0644)

	// Max 3 bytes — "small" is 5 bytes, should be skipped
	copied, err := CopyByPatterns(srcDir, dstDir, []string{"CLAUDE.md"}, nil, 3)
	if err != nil {
		t.Fatalf("CopyByPatterns failed: %v", err)
	}

	if copied != 0 {
		t.Errorf("expected 0 copied (file too large), got %d", copied)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/sync/ -run TestCopy -v`
Expected: FAIL

- [ ] **Step 7: Implement copier.go**

Create `internal/sync/copier.go`:

```go
package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CopyByPatterns copies files matching patterns from src to dst.
// Returns count of copied files.
// maxFileSize of 0 means no limit.
func CopyByPatterns(srcDir, dstDir string, patterns, ignore []string, maxFileSize int64) (int, error) {
	copied := 0

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil // skip symlinks
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if !matchesPatterns(relPath, patterns) {
			return nil
		}
		if IsIgnored(relPath, ignore) {
			return nil
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil // skip large files
		}

		dstPath := filepath.Join(dstDir, relPath)
		if err := copyFile(path, dstPath); err != nil {
			return fmt.Errorf("copying %s: %w", relPath, err)
		}

		copied++
		return nil
	})

	return copied, err
}

func matchesPatterns(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		// Directory pattern: ".claude/"
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, pattern) || strings.HasPrefix(relPath, strings.TrimSuffix(pattern, "/")) {
				return true
			}
		} else {
			// File pattern: "CLAUDE.md"
			if relPath == pattern {
				return true
			}
		}
	}
	return false
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve mtime
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

// DeleteFiles removes files from dir that are listed in paths.
func DeleteFiles(dir string, paths []string) error {
	for _, relPath := range paths {
		fullPath := filepath.Join(dir, relPath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting %s: %w", relPath, err)
		}
	}
	return nil
}
```

- [ ] **Step 8: Run all sync tests**

Run: `go test ./internal/sync/ -v`
Expected: All PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/sync/diff.go internal/sync/diff_test.go internal/sync/copier.go internal/sync/copier_test.go
git commit -m "feat: add file diff engine and pattern-based copier"
```

---

### Task 6: Sync Engine — Conflict Resolution

**Files:**
- Create: `internal/sync/conflict.go`
- Create: `internal/sync/conflict_test.go`

- [ ] **Step 1: Write test for conflict resolution**

Create `internal/sync/conflict_test.go`:

```go
package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveLatestWins_RemoteNewer(t *testing.T) {
	entry := ConflictEntry{
		Path:        "test.md",
		LocalMtime:  time.Now().Add(-1 * time.Hour),
		RemoteMtime: time.Now(),
	}

	action := ResolveLatestWins(entry)
	if action != TakeRemote {
		t.Errorf("expected TakeRemote, got %v", action)
	}
}

func TestResolveLatestWins_LocalNewer(t *testing.T) {
	entry := ConflictEntry{
		Path:        "test.md",
		LocalMtime:  time.Now(),
		RemoteMtime: time.Now().Add(-1 * time.Hour),
	}

	action := ResolveLatestWins(entry)
	if action != TakeLocal {
		t.Errorf("expected TakeLocal, got %v", action)
	}
}

func TestResolveBackupAndOverwrite(t *testing.T) {
	localDir := t.TempDir()
	backupDir := t.TempDir()

	localFile := filepath.Join(localDir, "test.md")
	os.WriteFile(localFile, []byte("local content"), 0644)

	err := BackupFile(localFile, backupDir, "test.md")
	if err != nil {
		t.Fatalf("BackupFile failed: %v", err)
	}

	// Verify backup exists
	entries, _ := os.ReadDir(backupDir)
	if len(entries) == 0 {
		t.Fatal("backup was not created")
	}
}

func TestGetConflictStrategy(t *testing.T) {
	rules := []ConflictRule{
		{Pattern: "CLAUDE.md", Strategy: "manual"},
		{Pattern: ".claude/skills/", Strategy: "latest-wins"},
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"CLAUDE.md", "manual"},
		{".claude/skills/test.md", "latest-wins"},
		{"unknown.md", "latest-wins"}, // default
	}

	for _, tt := range tests {
		got := GetConflictStrategy(tt.path, rules, "latest-wins")
		if got != tt.expected {
			t.Errorf("GetConflictStrategy(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sync/ -run TestResolve -v && go test ./internal/sync/ -run TestGetConflict -v`
Expected: FAIL

- [ ] **Step 3: Implement conflict.go**

Create `internal/sync/conflict.go`:

```go
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ConflictAction int

const (
	TakeLocal  ConflictAction = iota
	TakeRemote
	Skip
)

type ConflictRule struct {
	Pattern  string
	Strategy string
}

func ResolveLatestWins(entry ConflictEntry) ConflictAction {
	if entry.RemoteMtime.After(entry.LocalMtime) {
		return TakeRemote
	}
	return TakeLocal
}

func BackupFile(srcPath, backupDir, relPath string) error {
	timestamp := time.Now().UTC().Format("2006-01-02T150405")
	backupPath := filepath.Join(backupDir, timestamp, relPath)

	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return err
	}

	return copyFile(srcPath, backupPath)
}

func GetConflictStrategy(relPath string, rules []ConflictRule, defaultStrategy string) string {
	for _, rule := range rules {
		if strings.HasPrefix(relPath, rule.Pattern) || relPath == rule.Pattern {
			return rule.Strategy
		}
	}
	return defaultStrategy
}

func DefaultBackupDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ghost-sync", "backups")
}

// PruneBackups removes backups older than maxAge and respects maxSize.
func PruneBackups(backupDir string, maxAge time.Duration, maxSizeBytes int64) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse timestamp from dir name: 2006-01-02T150405
		ts, err := time.Parse("2006-01-02T150405", entry.Name())
		if err != nil {
			continue // skip non-timestamp dirs
		}

		if ts.Before(cutoff) {
			path := filepath.Join(backupDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("pruning backup %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sync/ -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sync/conflict.go internal/sync/conflict_test.go
git commit -m "feat: add conflict resolution strategies and backup management"
```

---

### Task 7: Repo Operations

**Files:**
- Create: `internal/repo/repo.go`
- Create: `internal/repo/repo_test.go`
- Create: `internal/repo/background.go`
- Create: `internal/repo/background_test.go`

- [ ] **Step 1: Write test for repo operations**

Create `internal/repo/repo_test.go`:

```go
package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v failed: %s %v", args, out, err)
		}
	}
	// Create initial commit
	os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("* text=auto\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	return dir
}

func TestCommitAndGetHEAD(t *testing.T) {
	dir := setupTestRepo(t)
	r := New(dir)

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	sha, err := r.Commit("test commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if sha == "" {
		t.Fatal("expected non-empty SHA")
	}

	head, err := r.HEAD()
	if err != nil {
		t.Fatalf("HEAD failed: %v", err)
	}
	if head != sha {
		t.Errorf("HEAD %q != commit SHA %q", head, sha)
	}
}

func TestCommitNoChanges(t *testing.T) {
	dir := setupTestRepo(t)
	r := New(dir)

	_, err := r.Commit("empty commit")
	if err == nil {
		t.Fatal("expected error for commit with no changes")
	}
}

func TestInitSyncRepo(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "sync-repo")

	r, err := InitSyncRepo(repoPath)
	if err != nil {
		t.Fatalf("InitSyncRepo failed: %v", err)
	}

	// Verify .gitattributes exists
	if _, err := os.Stat(filepath.Join(repoPath, ".gitattributes")); err != nil {
		t.Error(".gitattributes not created")
	}

	// Verify it's a git repo
	if _, err := r.HEAD(); err != nil {
		t.Error("not a valid git repo after init")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repo/ -v`
Expected: FAIL

- [ ] **Step 3: Implement repo.go**

Create `internal/repo/repo.go`:

```go
package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Path string
}

func New(path string) *Repo {
	return &Repo{Path: path}
}

func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Path}, args...)...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("git %s: %s (%w)", strings.Join(args, " "), output, err)
	}
	return output, nil
}

func (r *Repo) Commit(message string) (string, error) {
	if _, err := r.git("add", "-A"); err != nil {
		return "", err
	}

	// Check if there are staged changes
	status, err := r.git("status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status == "" {
		return "", fmt.Errorf("nothing to commit")
	}

	if _, err := r.git("commit", "-m", message); err != nil {
		return "", err
	}

	return r.HEAD()
}

func (r *Repo) HEAD() (string, error) {
	return r.git("rev-parse", "--short", "HEAD")
}

func (r *Repo) Pull() error {
	_, err := r.git("pull", "--rebase")
	return err
}

func (r *Repo) Push() error {
	_, err := r.git("push")
	return err
}

func (r *Repo) HasRemote() bool {
	_, err := r.git("remote", "get-url", "origin")
	return err == nil
}

func CloneSyncRepo(remoteURL, localPath string) (*Repo, error) {
	cmd := exec.Command("git", "clone", remoteURL, localPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return New(localPath), nil
}

func InitSyncRepo(localPath string) (*Repo, error) {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return nil, err
	}

	r := New(localPath)

	if _, err := r.git("init"); err != nil {
		// git init needs to run in the directory
		cmd := exec.Command("git", "init", localPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git init: %s (%w)", string(out), err)
		}
	}

	// Configure user if not set
	r.git("config", "user.email", "ghost-sync@local")
	r.git("config", "user.name", "ghost-sync")

	// Create .gitattributes
	attrPath := filepath.Join(localPath, ".gitattributes")
	if err := os.WriteFile(attrPath, []byte("* text=auto\n"), 0644); err != nil {
		return nil, err
	}

	// Create project and global dirs
	os.MkdirAll(filepath.Join(localPath, "projects"), 0755)
	os.MkdirAll(filepath.Join(localPath, "global"), 0755)

	// Initial commit
	if _, err := r.Commit("init: ghost-sync repository"); err != nil {
		return nil, err
	}

	return r, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/repo/ -v`
Expected: All PASS.

- [ ] **Step 5: Write test for background push**

Create `internal/repo/background_test.go`:

```go
package repo

import (
	"testing"
)

func TestBackgroundPushNoRemote(t *testing.T) {
	dir := setupTestRepo(t)
	r := New(dir)

	err := BackgroundPush(r)
	if err == nil {
		t.Fatal("expected error for repo without remote")
	}
}
```

- [ ] **Step 6: Implement background.go**

Create `internal/repo/background.go`:

```go
package repo

import (
	"fmt"
	"os/exec"
)

// BackgroundPush starts git push in a background process.
// Returns immediately. Errors are logged, not returned to caller.
func BackgroundPush(r *Repo) error {
	if !r.HasRemote() {
		return fmt.Errorf("no remote configured, skipping push")
	}

	cmd := exec.Command("git", "-C", r.Path, "push")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting background push: %w", err)
	}

	// Don't wait — let it run in background
	go cmd.Wait()

	return nil
}

// ForegroundPush runs git push and waits for completion.
func ForegroundPush(r *Repo) error {
	return r.Push()
}
```

- [ ] **Step 7: Run all repo tests**

Run: `go test ./internal/repo/ -v`
Expected: All PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/repo/
git commit -m "feat: add sync repo operations with clone, init, commit, push"
```

---

### Task 8: Logging and Backup

**Files:**
- Create: `internal/logging/logger.go`
- Create: `internal/logging/logger_test.go`
- Create: `internal/backup/backup.go`
- Create: `internal/backup/backup_test.go`

- [ ] **Step 1: Write test for logger**

Create `internal/logging/logger_test.go`:

```go
package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAndTail(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer logger.Close()

	logger.Info("test message 1")
	logger.Info("test message 2")
	logger.Warn("warning message")

	lines, err := Tail(logPath, 10)
	if err != nil {
		t.Fatalf("Tail failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[2], "warning message") {
		t.Errorf("unexpected last line: %s", lines[2])
	}
}

func TestTailWithLimit(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, _ := NewLogger(logPath)
	for i := 0; i < 100; i++ {
		logger.Info("line")
	}
	logger.Close()

	lines, _ := Tail(logPath, 5)
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
}

func TestTailNonExistent(t *testing.T) {
	lines, err := Tail("/nonexistent.log", 10)
	if err != nil {
		t.Fatalf("expected empty result, got error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}
```

- [ ] **Step 2: Implement logger.go**

Create `internal/logging/logger.go`:

```go
package logging

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Logger struct {
	file *os.File
}

func DefaultLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ghost-sync", "logs", "ghost-sync.log")
}

func NewLogger(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{file: file}, nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) log(level, msg string) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.file, "[%s] %s: %s\n", ts, level, msg)
}

func (l *Logger) Info(msg string)  { l.log("INFO", msg) }
func (l *Logger) Warn(msg string)  { l.log("WARN", msg) }
func (l *Logger) Error(msg string) { l.log("ERROR", msg) }

// Tail returns the last n lines from the log file.
func Tail(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if len(allLines) <= n {
		return allLines, nil
	}
	return allLines[len(allLines)-n:], nil
}

// TailByProject returns last n lines matching a project name.
func TailByProject(path string, project string, n int) ([]string, error) {
	lines, err := Tail(path, 0) // get all
	if err != nil {
		return nil, err
	}

	// Read all lines (Tail with 0 returns empty — use different approach)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var filtered []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, project) {
			filtered = append(filtered, line)
		}
	}

	if n > 0 && len(filtered) > n {
		filtered = filtered[len(filtered)-n:]
	}
	_ = lines
	return filtered, nil
}
```

- [ ] **Step 3: Run logger tests**

Run: `go test ./internal/logging/ -v`
Expected: All PASS.

- [ ] **Step 4: Write test for backup**

Create `internal/backup/backup_test.go`:

```go
package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateAndPrune(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "test.md")
	os.WriteFile(srcFile, []byte("content"), 0644)

	err := Create(backupDir, srcFile, "test.md")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify backup exists
	entries, _ := os.ReadDir(backupDir)
	if len(entries) == 0 {
		t.Fatal("no backup created")
	}

	// Prune with 0 age — should remove everything
	err = Prune(backupDir, 0, 500*1024*1024)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	entries, _ = os.ReadDir(backupDir)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after prune, got %d", len(entries))
	}
}

func TestPruneKeepsRecent(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "test.md")
	os.WriteFile(srcFile, []byte("content"), 0644)

	Create(backupDir, srcFile, "test.md")

	// Prune with 30 days — should keep recent backup
	err := Prune(backupDir, 30*24*time.Hour, 500*1024*1024)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) == 0 {
		t.Error("recent backup should not be pruned")
	}
}
```

- [ ] **Step 5: Implement backup.go**

Create `internal/backup/backup.go`:

```go
package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ghost-sync", "backups")
}

// Create saves a backup of srcPath under backupDir/<timestamp>/<relPath>.
func Create(backupDir, srcPath, relPath string) error {
	timestamp := time.Now().UTC().Format("2006-01-02T150405")
	dstPath := filepath.Join(backupDir, timestamp, relPath)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// Prune removes backup directories older than maxAge.
// If total size exceeds maxSize, removes oldest first.
func Prune(backupDir string, maxAge time.Duration, maxSize int64) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ts, err := time.Parse("2006-01-02T150405", entry.Name())
		if err != nil {
			continue
		}

		if ts.Before(cutoff) {
			path := filepath.Join(backupDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("pruning %s: %w", entry.Name(), err)
			}
		}
	}

	// Check total size
	totalSize, err := dirSize(backupDir)
	if err != nil {
		return err
	}

	if totalSize > maxSize {
		entries, _ = os.ReadDir(backupDir)
		for _, entry := range entries {
			if totalSize <= maxSize {
				break
			}
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(backupDir, entry.Name())
			size, _ := dirSize(path)
			os.RemoveAll(path)
			totalSize -= size
		}
	}

	return nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/logging/ -v && go test ./internal/backup/ -v`
Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/logging/ internal/backup/
git commit -m "feat: add logging system and backup management with pruning"
```

---

### Task 9: Git Hooks

**Files:**
- Create: `internal/hooks/templates.go`
- Create: `internal/hooks/templates_test.go`
- Create: `internal/hooks/install.go`
- Create: `internal/hooks/install_test.go`

- [ ] **Step 1: Write test for hook templates**

Create `internal/hooks/templates_test.go`:

```go
package hooks

import (
	"strings"
	"testing"
)

func TestPostCommitScript(t *testing.T) {
	script := PostCommitScript()

	if !strings.Contains(script, "ghost-sync") {
		t.Error("script should reference ghost-sync")
	}
	if !strings.Contains(script, "command -v ghost-sync") {
		t.Error("script should check for ghost-sync binary")
	}
	if !strings.Contains(script, "exit 0") {
		t.Error("script should exit 0 when binary not found")
	}
}

func TestPostMergeScript(t *testing.T) {
	script := PostMergeScript()

	if !strings.Contains(script, "ghost-sync pull") {
		t.Error("post-merge should run ghost-sync pull")
	}
	if !strings.Contains(script, "command -v ghost-sync") {
		t.Error("script should check for ghost-sync binary")
	}
}
```

- [ ] **Step 2: Implement templates.go**

Create `internal/hooks/templates.go`:

```go
package hooks

func PostCommitScript() string {
	return `#!/bin/sh
# ghost-sync post-commit hook (managed by ghost-sync, do not edit)
if ! command -v ghost-sync >/dev/null 2>&1; then
    echo "ghost-sync: binary not found, skipping sync" >&2
    exit 0
fi
ghost-sync push --from-hook 2>/dev/null &
exit 0
`
}

func PostMergeScript() string {
	return `#!/bin/sh
# ghost-sync post-merge hook (managed by ghost-sync, do not edit)
if ! command -v ghost-sync >/dev/null 2>&1; then
    echo "ghost-sync: binary not found, skipping sync" >&2
    exit 0
fi
ghost-sync pull --from-hook 2>/dev/null
exit 0
`
}
```

- [ ] **Step 3: Write test for hook installation**

Create `internal/hooks/install_test.go`:

```go
package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	return dir
}

func TestInstallHooks(t *testing.T) {
	dir := setupGitRepo(t)

	err := Install(dir)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify post-commit exists
	postCommit := filepath.Join(dir, ".git", "hooks", "post-commit")
	data, err := os.ReadFile(postCommit)
	if err != nil {
		t.Fatalf("post-commit not found: %v", err)
	}
	if !strings.Contains(string(data), "ghost-sync") {
		t.Error("post-commit doesn't contain ghost-sync")
	}

	// Verify post-merge exists
	postMerge := filepath.Join(dir, ".git", "hooks", "post-merge")
	if _, err := os.Stat(postMerge); err != nil {
		t.Error("post-merge not found")
	}
}

func TestInstallPreservesExistingHooks(t *testing.T) {
	dir := setupGitRepo(t)

	// Create existing post-commit hook
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	existing := "#!/bin/sh\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(existing), 0755)

	err := Install(dir)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
	content := string(data)

	// Should contain both existing and ghost-sync
	if !strings.Contains(content, "existing hook") {
		t.Error("existing hook content was removed")
	}
	if !strings.Contains(content, "ghost-sync") {
		t.Error("ghost-sync hook was not appended")
	}
}

func TestRemoveHooks(t *testing.T) {
	dir := setupGitRepo(t)

	Install(dir)
	err := Remove(dir)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	postCommit := filepath.Join(dir, ".git", "hooks", "post-commit")
	data, _ := os.ReadFile(postCommit)
	if strings.Contains(string(data), "ghost-sync") {
		t.Error("ghost-sync hook was not removed")
	}
}
```

- [ ] **Step 4: Implement install.go**

Create `internal/hooks/install.go`:

```go
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	hookMarkerStart = "# ghost-sync post-"
	hookMarkerEnd   = "exit 0\n"
	ghostSyncMarker = "# ghost-sync"
)

func Install(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("creating hooks dir: %w", err)
	}

	hooks := map[string]string{
		"post-commit": PostCommitScript(),
		"post-merge":  PostMergeScript(),
	}

	for name, script := range hooks {
		path := filepath.Join(hooksDir, name)
		if err := installHook(path, script); err != nil {
			return fmt.Errorf("installing %s: %w", name, err)
		}
	}

	return nil
}

func Remove(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")

	for _, name := range []string{"post-commit", "post-merge"} {
		path := filepath.Join(hooksDir, name)
		if err := removeHookContent(path); err != nil {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}

	return nil
}

func installHook(path, script string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
		// Remove existing ghost-sync content
		existing = removeGhostSyncContent(existing)
	}

	var content string
	if existing == "" {
		content = script
	} else {
		// Append ghost-sync script to existing hook
		if !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		content = existing + "\n" + script
	}

	return os.WriteFile(path, []byte(content), 0755)
}

func removeHookContent(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := removeGhostSyncContent(string(data))

	if strings.TrimSpace(content) == "" {
		return os.Remove(path)
	}

	return os.WriteFile(path, []byte(content), 0755)
}

func removeGhostSyncContent(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inGhostSync := false

	for _, line := range lines {
		if strings.Contains(line, "ghost-sync") && strings.HasPrefix(strings.TrimSpace(line), "#") {
			inGhostSync = true
			continue
		}
		if inGhostSync {
			if strings.TrimSpace(line) == "exit 0" {
				inGhostSync = false
				continue
			}
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
```

- [ ] **Step 5: Run all hook tests**

Run: `go test ./internal/hooks/ -v`
Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/
git commit -m "feat: add git hook installation with templates for post-commit and post-merge"
```

---

### Task 10: CLI Commands — init, add, remove

**Files:**
- Create: `internal/cli/init.go`
- Create: `internal/cli/add.go`
- Create: `internal/cli/remove.go`
- Modify: `cmd/ghost-sync/main.go`

- [ ] **Step 1: Implement init command**

Create `internal/cli/init.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/repo"
)

func NewInitCmd() *cobra.Command {
	var repoURL string
	var localPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize sync repository",
		Long:  "Clone an existing sync repo or create a new one locally.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.EnsureDefaults(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if repoURL != "" {
				if localPath == "" {
					localPath = cfg.SyncRepoPath
					if localPath == "" {
						return fmt.Errorf("specify --path or set sync_repo_path in config")
					}
				}

				fmt.Printf("Cloning sync repo from %s...\n", repoURL)
				_, err := repo.CloneSyncRepo(repoURL, localPath)
				if err != nil {
					return err
				}

				cfg.SyncRepo = repoURL
				cfg.SyncRepoPath = localPath
			} else if localPath != "" {
				fmt.Printf("Creating new sync repo at %s...\n", localPath)
				_, err := repo.InitSyncRepo(localPath)
				if err != nil {
					return err
				}

				cfg.SyncRepoPath = localPath
			} else {
				return fmt.Errorf("specify --repo <url> to clone or --path <dir> to create new")
			}

			if err := config.Save(cfg, cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Println("Sync repo initialized successfully.")
			fmt.Printf("Config saved to %s\n", cfgPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Remote URL to clone")
	cmd.Flags().StringVar(&localPath, "path", "", "Local path for sync repo")

	return cmd
}
```

- [ ] **Step 2: Implement add command**

Create `internal/cli/add.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
)

func NewAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Register current project for sync",
		Long:  "Detects git remote and registers this project in ghost-sync. Adds AI file patterns to .git/info/exclude.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found. Run 'ghost-sync init' first: %w", err)
			}

			remoteURL, repoName, err := project.DetectProject(cwd)
			if err != nil {
				fmt.Println("WARNING: No git remote found. Using directory name for project ID.")
				repoName = filepath.Base(cwd)
				remoteURL = ""
			}

			var projectID string
			if remoteURL != "" {
				projectID = project.GenerateProjectID(remoteURL)
			} else {
				projectID = project.GenerateProjectID(cwd)
			}

			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository: %w", err)
			}

			if err := project.AddProject(cfg, repoName, remoteURL, projectID, gitRoot); err != nil {
				return err
			}

			// Write exclude patterns
			excludePath := project.ExcludePathForRepo(gitRoot)
			if err := project.WriteExclude(excludePath, cfg.Patterns); err != nil {
				return fmt.Errorf("writing exclude: %w", err)
			}

			// Create project dir in sync repo
			if cfg.SyncRepoPath != "" {
				projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(repoName, projectID))
				os.MkdirAll(projDir, 0755)
			}

			if err := config.Save(cfg, cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Project '%s' registered (id: %s)\n", repoName, projectID)
			fmt.Printf("AI files added to .git/info/exclude\n")
			fmt.Println("Run 'ghost-sync hooks install' to enable automatic sync.")
			return nil
		},
	}
}
```

Note: Add `"path/filepath"` to imports in add.go.

- [ ] **Step 3: Implement remove command**

Create `internal/cli/remove.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/hooks"
	"github.com/sokolovsky/ghost-sync/internal/project"
)

func NewRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Unregister current project from sync",
		Long:  "Removes project from ghost-sync registry and cleans up .git/info/exclude.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found: %w", err)
			}

			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository: %w", err)
			}

			proj := cfg.FindProjectByPath(gitRoot)
			if proj == nil {
				return fmt.Errorf("project at %s is not registered", gitRoot)
			}

			projectName := proj.Name

			// Remove exclude patterns
			excludePath := project.ExcludePathForRepo(gitRoot)
			if err := project.RemoveExclude(excludePath); err != nil {
				fmt.Printf("Warning: could not clean exclude file: %v\n", err)
			}

			// Remove hooks
			if err := hooks.Remove(gitRoot); err != nil {
				fmt.Printf("Warning: could not remove hooks: %v\n", err)
			}

			if err := project.RemoveProject(cfg, proj.ID); err != nil {
				return err
			}

			if err := config.Save(cfg, cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Project '%s' removed from ghost-sync.\n", projectName)
			return nil
		},
	}
}
```

- [ ] **Step 4: Update main.go to wire all commands**

Update `cmd/ghost-sync/main.go` — add init, add, remove commands to rootCmd:

```go
package main

import (
	"os"

	"github.com/sokolovsky/ghost-sync/internal/cli"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	rootCmd := cli.NewRootCmd(version)
	rootCmd.AddCommand(
		cli.NewVersionCmd(version, commit, date),
		cli.NewInitCmd(),
		cli.NewAddCmd(),
		cli.NewRemoveCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Fix any import issues and build**

Run: `cd C:/Users/sokol/Documents/ai-ghost-sync && go build ./cmd/ghost-sync`
Expected: Builds without errors. Fix any import issues if needed.

- [ ] **Step 6: Verify commands are registered**

Run: `go run ./cmd/ghost-sync --help`
Expected: Shows init, add, remove, version in available commands.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/init.go internal/cli/add.go internal/cli/remove.go cmd/ghost-sync/main.go
git commit -m "feat: add init, add, remove CLI commands"
```

---

### Task 11: CLI Commands — push, pull, sync, status

**Files:**
- Create: `internal/cli/push.go`
- Create: `internal/cli/pull.go`
- Create: `internal/cli/sync_cmd.go`
- Create: `internal/cli/status.go`
- Modify: `cmd/ghost-sync/main.go`

- [ ] **Step 1: Implement push command**

Create `internal/cli/push.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

func NewPushCmd() *cobra.Command {
	var global bool
	var fromHook bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push AI files from working project to sync repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found. Run 'ghost-sync init' first")
			}
			if cfg.SyncRepoPath == "" {
				return fmt.Errorf("sync_repo_path not configured. Run 'ghost-sync init'")
			}

			if global {
				return pushGlobal(cfg)
			}

			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository")
			}

			proj := cfg.FindProjectByPath(gitRoot)
			if proj == nil {
				return fmt.Errorf("project not registered. Run 'ghost-sync add'")
			}

			patterns := cfg.EffectivePatterns(*proj)
			projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))
			os.MkdirAll(projDir, 0755)

			copied, err := gosync.CopyByPatterns(gitRoot, projDir, patterns, cfg.Ignore, 0)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}

			if copied == 0 {
				if !fromHook {
					fmt.Println("Nothing to sync.")
				}
				return nil
			}

			r := repo.New(cfg.SyncRepoPath)
			commitMsg := fmt.Sprintf("sync: %s — push %d files", proj.Name, copied)
			if _, err := r.Commit(commitMsg); err != nil {
				return fmt.Errorf("committing: %w", err)
			}

			if fromHook {
				repo.BackgroundPush(r)
			} else {
				if r.HasRemote() {
					fmt.Println("Pushing to remote...")
					if err := r.Push(); err != nil {
						fmt.Printf("Warning: push failed: %v\n", err)
					}
				}
				fmt.Printf("Synced %d files from '%s'.\n", copied, proj.Name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Push global configs")
	cmd.Flags().BoolVar(&fromHook, "from-hook", false, "Called from git hook (background push)")
	cmd.Flags().MarkHidden("from-hook")

	return cmd
}

func pushGlobal(cfg *config.Config) error {
	if !cfg.GlobalSync.Enabled {
		return fmt.Errorf("global sync is not enabled")
	}

	globalDir := filepath.Join(cfg.SyncRepoPath, "global", cfg.MachineID)
	os.MkdirAll(globalDir, 0755)

	home, _ := os.UserHomeDir()
	totalCopied := 0

	for _, syncPath := range cfg.GlobalSync.Paths {
		expanded := os.ExpandEnv(syncPath)
		if len(expanded) > 0 && expanded[0] == '~' {
			expanded = filepath.Join(home, expanded[1:])
		}

		// Copy entire directory to global/<machine-id>/<dirname>
		dirName := filepath.Base(expanded)
		dstDir := filepath.Join(globalDir, dirName)

		copied, err := gosync.CopyByPatterns(expanded, dstDir, []string{""}, cfg.Ignore, 0)
		if err != nil {
			fmt.Printf("Warning: could not sync %s: %v\n", syncPath, err)
			continue
		}
		totalCopied += copied
	}

	if totalCopied == 0 {
		fmt.Println("Nothing to sync globally.")
		return nil
	}

	r := repo.New(cfg.SyncRepoPath)
	commitMsg := fmt.Sprintf("sync: global/%s — %d files", cfg.MachineID, totalCopied)
	if _, err := r.Commit(commitMsg); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	if r.HasRemote() {
		fmt.Println("Pushing to remote...")
		if err := r.Push(); err != nil {
			fmt.Printf("Warning: push failed: %v\n", err)
		}
	}

	fmt.Printf("Synced %d global files.\n", totalCopied)
	return nil
}
```

- [ ] **Step 2: Implement pull command**

Create `internal/cli/pull.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/backup"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

func NewPullCmd() *cobra.Command {
	var global bool
	var fromHook bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull AI files from sync repo to working project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found. Run 'ghost-sync init' first")
			}

			r := repo.New(cfg.SyncRepoPath)

			// Pull remote changes
			if r.HasRemote() {
				if !fromHook {
					fmt.Println("Pulling sync repo...")
				}
				if err := r.Pull(); err != nil {
					if !fromHook {
						fmt.Printf("Warning: pull failed: %v\n", err)
					}
				}
			}

			if global {
				return pullGlobal(cfg)
			}

			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository")
			}

			proj := cfg.FindProjectByPath(gitRoot)
			if proj == nil {
				return fmt.Errorf("project not registered. Run 'ghost-sync add'")
			}

			projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))
			if _, err := os.Stat(projDir); os.IsNotExist(err) {
				if !fromHook {
					fmt.Println("No synced files found for this project.")
				}
				return nil
			}

			// Diff to find what changed
			diff, err := gosync.DiffFiles(gitRoot, projDir)
			if err != nil {
				return fmt.Errorf("comparing files: %w", err)
			}

			// Copy remote-only files (new from sync repo)
			patterns := cfg.EffectivePatterns(*proj)
			copied, err := gosync.CopyByPatterns(projDir, gitRoot, patterns, cfg.Ignore, 0)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}

			// Handle deletions: files in local but deleted from sync repo
			// (files in diff.LocalOnly that match patterns — they were removed remotely)
			for _, relPath := range diff.LocalOnly {
				if gosync.IsIgnored(relPath, cfg.Ignore) {
					continue
				}
				localPath := filepath.Join(gitRoot, relPath)
				backup.Create(backup.DefaultDir(), localPath, relPath)
				os.Remove(localPath)
			}

			if !fromHook {
				fmt.Printf("Pulled %d files for '%s'.\n", copied, proj.Name)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Pull global configs")
	cmd.Flags().BoolVar(&fromHook, "from-hook", false, "Called from git hook")
	cmd.Flags().MarkHidden("from-hook")

	return cmd
}

func pullGlobal(cfg *config.Config) error {
	if !cfg.GlobalSync.Enabled {
		return fmt.Errorf("global sync is not enabled")
	}

	globalDir := filepath.Join(cfg.SyncRepoPath, "global", cfg.MachineID)
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		fmt.Println("No global files synced yet.")
		return nil
	}

	home, _ := os.UserHomeDir()
	totalCopied := 0

	for _, syncPath := range cfg.GlobalSync.Paths {
		expanded := os.ExpandEnv(syncPath)
		if len(expanded) > 0 && expanded[0] == '~' {
			expanded = filepath.Join(home, expanded[1:])
		}

		dirName := filepath.Base(expanded)
		srcDir := filepath.Join(globalDir, dirName)

		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		copied, err := gosync.CopyByPatterns(srcDir, expanded, []string{""}, cfg.Ignore, 0)
		if err != nil {
			fmt.Printf("Warning: could not pull %s: %v\n", syncPath, err)
			continue
		}
		totalCopied += copied
	}

	fmt.Printf("Pulled %d global files.\n", totalCopied)
	return nil
}
```

- [ ] **Step 3: Implement sync command**

Create `internal/cli/sync_cmd.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Pull then push (full bidirectional sync)",
		Long:  "Pulls remote changes first, resolves conflicts, then pushes local changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Step 1/2: Pulling...")
			pullCmd := NewPullCmd()
			if err := pullCmd.RunE(pullCmd, nil); err != nil {
				fmt.Printf("Pull warning: %v\n", err)
			}

			fmt.Println("Step 2/2: Pushing...")
			pushCmd := NewPushCmd()
			if err := pushCmd.RunE(pushCmd, nil); err != nil {
				return err
			}

			return nil
		},
	}
}
```

- [ ] **Step 4: Implement status command**

Create `internal/cli/status.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync status for current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found. Run 'ghost-sync init' first")
			}

			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository")
			}

			proj := cfg.FindProjectByPath(gitRoot)
			if proj == nil {
				return fmt.Errorf("project not registered. Run 'ghost-sync add'")
			}

			projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))
			if _, err := os.Stat(projDir); os.IsNotExist(err) {
				fmt.Printf("Project '%s' (id: %s) — no synced files yet.\n", proj.Name, proj.ID)
				return nil
			}

			diff, err := gosync.DiffFiles(gitRoot, projDir)
			if err != nil {
				return fmt.Errorf("comparing: %w", err)
			}

			fmt.Printf("Project: %s (id: %s)\n", proj.Name, proj.ID)
			fmt.Printf("Sync repo: %s\n\n", projDir)

			if len(diff.LocalOnly) == 0 && len(diff.RemoteOnly) == 0 && len(diff.Conflicts) == 0 {
				fmt.Println("Everything is in sync.")
				return nil
			}

			if len(diff.LocalOnly) > 0 {
				fmt.Printf("Local only (%d files — will be pushed):\n", len(diff.LocalOnly))
				for _, f := range diff.LocalOnly {
					fmt.Printf("  + %s\n", f)
				}
			}

			if len(diff.RemoteOnly) > 0 {
				fmt.Printf("Remote only (%d files — will be pulled):\n", len(diff.RemoteOnly))
				for _, f := range diff.RemoteOnly {
					fmt.Printf("  ↓ %s\n", f)
				}
			}

			if len(diff.Conflicts) > 0 {
				fmt.Printf("Modified (%d files — conflict resolution needed):\n", len(diff.Conflicts))
				for _, c := range diff.Conflicts {
					fmt.Printf("  ~ %s (local: %s, remote: %s)\n", c.Path,
						c.LocalMtime.Format("2006-01-02 15:04"),
						c.RemoteMtime.Format("2006-01-02 15:04"))
				}
			}

			if len(diff.Same) > 0 {
				fmt.Printf("\nIn sync: %d files\n", len(diff.Same))
			}

			return nil
		},
	}
}
```

- [ ] **Step 5: Update main.go**

Add push, pull, sync, status commands to `cmd/ghost-sync/main.go`:

```go
rootCmd.AddCommand(
    cli.NewVersionCmd(version, commit, date),
    cli.NewInitCmd(),
    cli.NewAddCmd(),
    cli.NewRemoveCmd(),
    cli.NewPushCmd(),
    cli.NewPullCmd(),
    cli.NewSyncCmd(),
    cli.NewStatusCmd(),
)
```

- [ ] **Step 6: Build and verify**

Run: `cd C:/Users/sokol/Documents/ai-ghost-sync && go build ./cmd/ghost-sync`
Expected: Builds without errors.

Run: `go run ./cmd/ghost-sync --help`
Expected: Shows all commands including push, pull, sync, status.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/push.go internal/cli/pull.go internal/cli/sync_cmd.go internal/cli/status.go cmd/ghost-sync/main.go
git commit -m "feat: add push, pull, sync, status CLI commands"
```

---

### Task 12: CLI Commands — check, list, hooks, config, log

**Files:**
- Create: `internal/cli/check.go`
- Create: `internal/cli/list.go`
- Create: `internal/cli/hooks_cmd.go`
- Create: `internal/cli/config_cmd.go`
- Create: `internal/cli/log_cmd.go`
- Modify: `cmd/ghost-sync/main.go`

- [ ] **Step 1: Implement check command**

Create `internal/cli/check.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

func NewCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check sync status (for session hooks)",
		Long:  "Offline check of sync status. Used by AI agent session hooks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				// No config — ghost-sync not initialized
				return nil
			}

			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return nil // Not a git repo, silently exit
			}

			proj := cfg.FindProjectByPath(gitRoot)
			if proj == nil {
				fmt.Printf("ghost-sync: Project '%s' is not synced. Run: ghost-sync add\n",
					filepath.Base(gitRoot))
				return nil
			}

			projDir := filepath.Join(cfg.SyncRepoPath, "projects",
				project.ProjectDirName(proj.Name, proj.ID))
			if _, err := os.Stat(projDir); os.IsNotExist(err) {
				return nil
			}

			diff, err := gosync.DiffFiles(gitRoot, projDir)
			if err != nil {
				return nil // Don't fail session start on diff errors
			}

			changes := len(diff.RemoteOnly) + len(diff.Conflicts)
			if changes > 0 {
				fmt.Printf("ghost-sync: %d files updated since last sync. Run: ghost-sync pull\n", changes)
			}

			return nil
		},
	}
}
```

- [ ] **Step 2: Implement list command**

Create `internal/cli/list.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
)

func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found. Run 'ghost-sync init' first")
			}

			if len(cfg.Projects) == 0 {
				fmt.Println("No projects registered. Run 'ghost-sync add' in a git project.")
				return nil
			}

			fmt.Printf("Registered projects (%d):\n\n", len(cfg.Projects))
			for _, p := range cfg.Projects {
				status := "ok"
				if _, err := os.Stat(p.Path); os.IsNotExist(err) {
					status = "missing"
				}

				fmt.Printf("  %s (id: %s) [%s]\n", p.Name, p.ID, status)
				fmt.Printf("    Path:   %s\n", p.Path)
				if p.Remote != "" {
					fmt.Printf("    Remote: %s\n", p.Remote)
				}
				fmt.Println()
			}

			return nil
		},
	}
}
```

- [ ] **Step 3: Implement hooks command**

Create `internal/cli/hooks_cmd.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/hooks"
	"github.com/sokolovsky/ghost-sync/internal/project"
)

func NewHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage git hooks",
	}

	cmd.AddCommand(newHooksInstallCmd())
	cmd.AddCommand(newHooksRemoveCmd())

	return cmd
}

func newHooksInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install git hooks in current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository")
			}

			if err := hooks.Install(gitRoot); err != nil {
				return err
			}

			fmt.Println("Git hooks installed (post-commit, post-merge).")
			return nil
		},
	}
}

func newHooksRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove git hooks from current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			gitRoot, err := project.DetectGitRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a git repository")
			}

			if err := hooks.Remove(gitRoot); err != nil {
				return err
			}

			fmt.Println("Git hooks removed.")
			return nil
		},
	}
}
```

- [ ] **Step 4: Implement config command**

Create `internal/cli/config_cmd.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/config"
)

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.DefaultConfigPath()

			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return fmt.Errorf("config not found at %s. Run 'ghost-sync init'", cfgPath)
			}

			fmt.Printf("Config: %s\n\n", cfgPath)
			fmt.Println(string(data))
			return nil
		},
	}
}
```

- [ ] **Step 5: Implement log command**

Create `internal/cli/log_cmd.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sokolovsky/ghost-sync/internal/logging"
)

func NewLogCmd() *cobra.Command {
	var lines int
	var projectFilter string

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show recent sync operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			logPath := logging.DefaultLogPath()

			var entries []string
			var err error

			if projectFilter != "" {
				entries, err = logging.TailByProject(logPath, projectFilter, lines)
			} else {
				entries, err = logging.Tail(logPath, lines)
			}

			if err != nil {
				return fmt.Errorf("reading log: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No log entries found.")
				return nil
			}

			for _, entry := range entries {
				fmt.Println(entry)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&lines, "lines", 50, "Number of lines to show")
	cmd.Flags().StringVar(&projectFilter, "project", "", "Filter by project name")

	return cmd
}
```

- [ ] **Step 6: Update main.go with all remaining commands**

```go
rootCmd.AddCommand(
    cli.NewVersionCmd(version, commit, date),
    cli.NewInitCmd(),
    cli.NewAddCmd(),
    cli.NewRemoveCmd(),
    cli.NewPushCmd(),
    cli.NewPullCmd(),
    cli.NewSyncCmd(),
    cli.NewStatusCmd(),
    cli.NewCheckCmd(),
    cli.NewListCmd(),
    cli.NewHooksCmd(),
    cli.NewConfigCmd(),
    cli.NewLogCmd(),
)
```

- [ ] **Step 7: Build and verify all commands**

Run: `cd C:/Users/sokol/Documents/ai-ghost-sync && go build ./cmd/ghost-sync`
Expected: Builds without errors.

Run: `go run ./cmd/ghost-sync --help`
Expected: All 13 commands listed.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/check.go internal/cli/list.go internal/cli/hooks_cmd.go internal/cli/config_cmd.go internal/cli/log_cmd.go cmd/ghost-sync/main.go
git commit -m "feat: add check, list, hooks, config, log CLI commands"
```

---

### Task 13: Build and Distribution

**Files:**
- Create: `.goreleaser.yaml`
- Modify: `Makefile`

- [ ] **Step 1: Create goreleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2

builds:
  - main: ./cmd/ghost-sync
    binary: ghost-sync
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}
    goos:
      - windows
      - darwin
      - linux
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

- [ ] **Step 2: Verify build for current platform**

Run: `cd C:/Users/sokol/Documents/ai-ghost-sync && go build -o bin/ghost-sync.exe ./cmd/ghost-sync`
Expected: Binary created in `bin/`.

- [ ] **Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml Makefile
git commit -m "feat: add goreleaser config for cross-platform builds"
```

---

### Task 14: Integration Test

**Files:**
- Create: `internal/integration_test.go`

- [ ] **Step 1: Write end-to-end integration test**

Create `internal/integration_test.go`:

```go
package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
)

func TestFullSyncCycle(t *testing.T) {
	// Setup: create a sync repo and a "working project"
	syncRepoDir := t.TempDir()
	workProjectDir := t.TempDir()

	// Init sync repo
	syncRepo, err := repo.InitSyncRepo(syncRepoDir)
	if err != nil {
		t.Fatalf("InitSyncRepo: %v", err)
	}

	// Init working project as git repo
	exec.Command("git", "init", workProjectDir).Run()
	exec.Command("git", "-C", workProjectDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", workProjectDir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(workProjectDir, "README.md"), []byte("# Test"), 0644)
	exec.Command("git", "-C", workProjectDir, "add", ".").Run()
	exec.Command("git", "-C", workProjectDir, "commit", "-m", "init").Run()

	// Create AI files in working project
	os.MkdirAll(filepath.Join(workProjectDir, ".claude", "skills"), 0755)
	os.WriteFile(filepath.Join(workProjectDir, ".claude", "skills", "test.md"), []byte("# Test Skill"), 0644)
	os.WriteFile(filepath.Join(workProjectDir, "CLAUDE.md"), []byte("# Instructions"), 0644)

	// Setup config
	cfg := &config.Config{
		Version:          config.ConfigVersion,
		SyncRepoPath:     syncRepoDir,
		MachineID:        "test",
		Patterns:         config.DefaultPatterns(),
		Ignore:           config.DefaultIgnore(),
		ConflictStrategy: config.DefaultConflictStrategy,
	}

	projID := project.GenerateProjectID("test-project")
	project.AddProject(cfg, "test-project", "", projID, workProjectDir)

	// PUSH: Copy files from working project to sync repo
	projDir := filepath.Join(syncRepoDir, "projects", project.ProjectDirName("test-project", projID))
	os.MkdirAll(projDir, 0755)

	copied, err := gosync.CopyByPatterns(workProjectDir, projDir, cfg.Patterns, cfg.Ignore, 0)
	if err != nil {
		t.Fatalf("Push copy: %v", err)
	}
	if copied != 2 {
		t.Errorf("expected 2 files pushed, got %d", copied)
	}

	// Commit in sync repo
	if _, err := syncRepo.Commit("sync: test push"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// PULL: Simulate pulling to a "clean" working project
	cleanDir := t.TempDir()
	exec.Command("git", "init", cleanDir).Run()

	pulled, err := gosync.CopyByPatterns(projDir, cleanDir, cfg.Patterns, cfg.Ignore, 0)
	if err != nil {
		t.Fatalf("Pull copy: %v", err)
	}
	if pulled != 2 {
		t.Errorf("expected 2 files pulled, got %d", pulled)
	}

	// Verify files exist in clean dir
	if _, err := os.Stat(filepath.Join(cleanDir, ".claude", "skills", "test.md")); err != nil {
		t.Error("skill file not pulled")
	}
	if _, err := os.Stat(filepath.Join(cleanDir, "CLAUDE.md")); err != nil {
		t.Error("CLAUDE.md not pulled")
	}

	// DIFF: Should show everything in sync
	diff, err := gosync.DiffFiles(cleanDir, projDir)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(diff.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(diff.Conflicts))
	}
	if len(diff.Same) != 2 {
		t.Errorf("expected 2 same files, got %d", len(diff.Same))
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./internal/ -run TestFullSyncCycle -v`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/integration_test.go
git commit -m "test: add full sync cycle integration test"
```
