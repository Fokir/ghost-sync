package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local files to the sync repository",
	Long: `Push synced files from the current project to the sync repository.

Copies matching files from the working project into the sync repo,
commits changes, and pushes to the remote (if configured).`,
	RunE: runPush,
}

var (
	pushGlobal   bool
	pushFromHook bool
)

func init() {
	pushCmd.Flags().BoolVar(&pushGlobal, "global", false, "push global paths instead of project files")
	pushCmd.Flags().BoolVar(&pushFromHook, "from-hook", false, "called from a git hook (background push)")
	_ = pushCmd.Flags().MarkHidden("from-hook")
	RootCmd.AddCommand(pushCmd)
}

// ghostSyncMeta is the metadata written to .ghost-sync.meta in the sync repo project dir.
type ghostSyncMeta struct {
	RemoteURL      string `yaml:"remote_url"`
	ProjectName    string `yaml:"project_name"`
	ProjectID      string `yaml:"project_id"`
	LastSyncAt     string `yaml:"last_sync_at"`
	LastSyncCommit string `yaml:"last_sync_commit"`
	SyncedBy       string `yaml:"synced_by"`
}

func runPush(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return fmt.Errorf("resolving config path: %w", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config not found — run `ghost-sync init` first")
		}
		return fmt.Errorf("loading config: %w", err)
	}

	if pushGlobal {
		return doPushGlobal(cfg, pushFromHook, verbose)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	proj := cfg.FindProjectByPath(gitRoot)
	if proj == nil {
		return fmt.Errorf("current directory is not a registered project — run `ghost-sync add` first")
	}

	return doPush(cfg, proj, pushFromHook, verbose, false)
}

// doPush copies files from the working project to the sync repo and commits/pushes.
// If skipLock is true, the caller is responsible for holding the lock.
func doPush(cfg *config.Config, proj *config.ProjectEntry, fromHook bool, verbose bool, skipLock bool) error {
	if cfg.SyncRepoPath == "" {
		return fmt.Errorf("sync_repo_path not configured — run `ghost-sync init` first")
	}

	// Acquire lock unless caller already holds it.
	if !skipLock {
		lock, err := gosync.AcquireLock(gosync.DefaultLockPath(), 10*time.Second)
		if err != nil {
			return fmt.Errorf("another ghost-sync operation in progress")
		}
		defer gosync.ReleaseLock(lock)
	}

	patterns := cfg.EffectivePatterns(proj)
	ignore := cfg.Ignore
	if len(ignore) == 0 {
		ignore = config.DefaultIgnore()
	}

	projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return fmt.Errorf("creating project dir in sync repo: %w", err)
	}

	maxFileSize, err := config.ParseFileSize(cfg.MaxFileSize)
	if err != nil {
		// Fall back to default.
		maxFileSize, _ = config.ParseFileSize(config.DefaultMaxFileSize)
	}

	gitRoot := proj.Path

	// Copy files from working project to sync repo.
	count, err := gosync.CopyByPatterns(gitRoot, projDir, patterns, ignore, maxFileSize)
	if err != nil {
		return fmt.Errorf("copying files to sync repo: %w", err)
	}
	if verbose {
		fmt.Printf("Copied %d files to sync repo\n", count)
	}

	// Handle deletions: find files in projDir that no longer exist in gitRoot (for pattern-matching files only).
	deleted, err := deleteStaleSyncFiles(projDir, gitRoot, patterns, ignore)
	if err != nil {
		return fmt.Errorf("cleaning stale files from sync repo: %w", err)
	}
	if verbose && deleted > 0 {
		fmt.Printf("Deleted %d stale files from sync repo\n", deleted)
	}

	// Write .ghost-sync.meta.
	syncRepo := repo.New(cfg.SyncRepoPath)
	commitSHA, metaErr := writeMetaAndCommit(syncRepo, projDir, proj, cfg)
	if metaErr != nil {
		// "nothing to commit" is not an error for push.
		if metaErr.Error() == "nothing to commit: working tree clean" {
			fmt.Println("Already up to date.")
			return nil
		}
		return fmt.Errorf("committing to sync repo: %w", metaErr)
	}

	if verbose {
		fmt.Printf("Committed: %s\n", commitSHA)
	}

	// Push to remote.
	if syncRepo.HasRemote() {
		if fromHook {
			if err := repo.BackgroundPush(syncRepo); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: background push failed: %v\n", err)
				}
			}
		} else {
			if err := repo.ForegroundPush(syncRepo); err != nil {
				return fmt.Errorf("pushing sync repo: %w", err)
			}
		}
	}

	fmt.Printf("Push complete for project %s\n", proj.Name)
	return nil
}

// doPushGlobal copies global paths to <syncRepoPath>/global/<machineID>/.
func doPushGlobal(cfg *config.Config, fromHook bool, verbose bool) error {
	if cfg.SyncRepoPath == "" {
		return fmt.Errorf("sync_repo_path not configured — run `ghost-sync init` first")
	}
	if cfg.GlobalSync == nil || !cfg.GlobalSync.Enabled {
		return fmt.Errorf("global sync is not enabled in config")
	}

	lock, err := gosync.AcquireLock(gosync.DefaultLockPath(), 10*time.Second)
	if err != nil {
		return fmt.Errorf("another ghost-sync operation in progress")
	}
	defer gosync.ReleaseLock(lock)

	machineID := cfg.MachineID
	if machineID == "" {
		hostname, _ := os.Hostname()
		machineID = hostname
	}

	globalDir := filepath.Join(cfg.SyncRepoPath, "global", machineID)
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return fmt.Errorf("creating global dir in sync repo: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	maxFileSize, err := config.ParseFileSize(cfg.MaxFileSize)
	if err != nil {
		maxFileSize, _ = config.ParseFileSize(config.DefaultMaxFileSize)
	}

	// Normalize global patterns: strip ~/ prefix so they match relative paths under home.
	globalPatterns := normalizeGlobalPatterns(cfg.GlobalSync.Patterns)

	count, err := gosync.CopyByPatterns(home, globalDir, globalPatterns, cfg.Ignore, maxFileSize)
	if err != nil {
		return fmt.Errorf("copying global files to sync repo: %w", err)
	}
	if verbose {
		fmt.Printf("Copied %d global files to sync repo\n", count)
	}

	syncRepo := repo.New(cfg.SyncRepoPath)
	commitSHA, commitErr := syncRepo.Commit(fmt.Sprintf("push global from %s", machineID))
	if commitErr != nil {
		if commitErr.Error() == "nothing to commit: working tree clean" {
			fmt.Println("Global files already up to date.")
			return nil
		}
		return fmt.Errorf("committing: %w", commitErr)
	}
	if verbose {
		fmt.Printf("Committed: %s\n", commitSHA)
	}

	if syncRepo.HasRemote() {
		if fromHook {
			_ = repo.BackgroundPush(syncRepo)
		} else {
			if err := repo.ForegroundPush(syncRepo); err != nil {
				return fmt.Errorf("pushing sync repo: %w", err)
			}
		}
	}

	fmt.Println("Global push complete.")
	return nil
}

// deleteStaleSyncFiles removes files from projDir that match patterns but no longer exist in gitRoot.
func deleteStaleSyncFiles(projDir, gitRoot string, patterns, ignore []string) (int, error) {
	syncFiles, err := gosync.CollectFiles(projDir)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for rel := range syncFiles {
		// Skip meta file.
		if rel == ".ghost-sync.meta" {
			continue
		}

		// Only consider files that match our patterns and are not ignored.
		if !gosync.MatchesPatterns(rel, patterns) {
			continue
		}
		if gosync.IsIgnored(rel, ignore) {
			continue
		}

		// Check if the file exists in the working project.
		localPath := filepath.Join(gitRoot, filepath.FromSlash(rel))
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			// File was deleted locally — remove from sync repo.
			syncPath := filepath.Join(projDir, filepath.FromSlash(rel))
			if err := os.Remove(syncPath); err != nil && !os.IsNotExist(err) {
				return deleted, fmt.Errorf("delete %s from sync repo: %w", rel, err)
			}
			deleted++
		}
	}
	return deleted, nil
}

// writeMetaAndCommit writes .ghost-sync.meta and commits.
func writeMetaAndCommit(syncRepo *repo.Repo, projDir string, proj *config.ProjectEntry, cfg *config.Config) (string, error) {
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	syncedBy := fmt.Sprintf("%s@%s", username, hostname)

	meta := ghostSyncMeta{
		RemoteURL:   proj.Remote,
		ProjectName: proj.Name,
		ProjectID:   proj.ID,
		LastSyncAt:  time.Now().UTC().Format(time.RFC3339),
		SyncedBy:    syncedBy,
	}

	metaData, err := yaml.Marshal(&meta)
	if err != nil {
		return "", fmt.Errorf("marshaling meta: %w", err)
	}

	metaPath := filepath.Join(projDir, ".ghost-sync.meta")
	if err := os.WriteFile(metaPath, metaData, 0o644); err != nil {
		return "", fmt.Errorf("writing meta file: %w", err)
	}

	commitSHA, err := syncRepo.Commit(fmt.Sprintf("push %s", proj.Name))
	if err != nil {
		return "", err
	}

	return commitSHA, nil
}

// normalizeGlobalPatterns strips the ~/ prefix from patterns so they work as
// relative paths when walking the home directory.
func normalizeGlobalPatterns(patterns []string) []string {
	result := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimPrefix(p, "~/")
		p = strings.TrimPrefix(p, "~\\")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
