package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sokolovsky/ghost-sync/internal/backup"
	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull files from the sync repository to the local project",
	Long: `Pull synced files from the sync repository into the current project.

Fetches the latest changes from the remote (if configured), then copies
matching files from the sync repo into the working project.`,
	RunE: runPull,
}

var (
	pullGlobal   bool
	pullFromHook bool
)

func init() {
	pullCmd.Flags().BoolVar(&pullGlobal, "global", false, "pull global paths instead of project files")
	pullCmd.Flags().BoolVar(&pullFromHook, "from-hook", false, "called from a git hook")
	_ = pullCmd.Flags().MarkHidden("from-hook")
	RootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
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

	if pullGlobal {
		return doPullGlobal(cfg, pullFromHook, verbose)
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

	return doPull(cfg, proj, pullFromHook, verbose, false)
}

// doPull copies files from the sync repo to the working project.
// If skipLock is true, the caller is responsible for holding the lock.
func doPull(cfg *config.Config, proj *config.ProjectEntry, fromHook bool, verbose bool, skipLock bool) error {
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

	// Pull sync repo from remote first.
	syncRepo := repo.New(cfg.SyncRepoPath)
	if syncRepo.HasRemote() {
		if err := syncRepo.Pull(); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: git pull failed: %v\n", err)
			}
			// Continue anyway — we may have local changes.
		}
	}

	patterns := cfg.EffectivePatterns(proj)
	ignore := cfg.Ignore
	if len(ignore) == 0 {
		ignore = config.DefaultIgnore()
	}

	projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))

	// Check if projDir exists.
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		fmt.Printf("No synced files found for project %s\n", proj.Name)
		return nil
	}

	maxFileSize, err := config.ParseFileSize(cfg.MaxFileSize)
	if err != nil {
		maxFileSize, _ = config.ParseFileSize(config.DefaultMaxFileSize)
	}

	gitRoot := proj.Path

	// Copy files from sync repo to working project.
	count, err := gosync.CopyByPatterns(projDir, gitRoot, patterns, ignore, maxFileSize)
	if err != nil {
		return fmt.Errorf("copying files from sync repo: %w", err)
	}
	if verbose {
		fmt.Printf("Copied %d files from sync repo\n", count)
	}

	// Handle deletions: find files that match patterns, exist locally but NOT in sync repo projDir.
	deleted, err := deleteStaleLocalFiles(projDir, gitRoot, patterns, ignore)
	if err != nil {
		return fmt.Errorf("cleaning stale local files: %w", err)
	}
	if verbose && deleted > 0 {
		fmt.Printf("Deleted %d stale local files (backed up)\n", deleted)
	}

	// Prune old backups: keep last 30 days, max 500 MB.
	_ = backup.Prune(backup.DefaultDir(), 30*24*time.Hour, 500*1024*1024)

	fmt.Printf("Pull complete for project %s\n", proj.Name)
	return nil
}

// deleteStaleLocalFiles removes files from gitRoot that match patterns but no longer exist in projDir.
// Files are backed up before deletion.
func deleteStaleLocalFiles(projDir, gitRoot string, patterns, ignore []string) (int, error) {
	localFiles, err := gosync.CollectFiles(gitRoot)
	if err != nil {
		return 0, err
	}

	backupDir := backup.DefaultDir()
	deleted := 0

	for rel := range localFiles {
		// Only consider files that match our sync patterns.
		if !gosync.MatchesPatterns(rel, patterns) {
			continue
		}
		if gosync.IsIgnored(rel, ignore) {
			continue
		}

		// Check if the file exists in the sync repo.
		syncPath := filepath.Join(projDir, filepath.FromSlash(rel))
		if _, err := os.Stat(syncPath); os.IsNotExist(err) {
			// File was deleted from sync repo — back up and remove locally.
			localPath := filepath.Join(gitRoot, filepath.FromSlash(rel))
			_ = backup.Create(backupDir, localPath, rel)
			if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
				return deleted, fmt.Errorf("delete %s locally: %w", rel, err)
			}
			deleted++
		}
	}
	return deleted, nil
}

// doPullGlobal pulls global paths from <syncRepoPath>/global/<machineID>/.
func doPullGlobal(cfg *config.Config, fromHook bool, verbose bool) error {
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

	syncRepo := repo.New(cfg.SyncRepoPath)
	if syncRepo.HasRemote() {
		if err := syncRepo.Pull(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: git pull failed: %v\n", err)
		}
	}

	machineID := cfg.MachineID
	if machineID == "" {
		hostname, _ := os.Hostname()
		machineID = hostname
	}

	globalDir := filepath.Join(cfg.SyncRepoPath, "global", machineID)
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		fmt.Println("No global files found in sync repo.")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	maxFileSize, err := config.ParseFileSize(cfg.MaxFileSize)
	if err != nil {
		maxFileSize, _ = config.ParseFileSize(config.DefaultMaxFileSize)
	}

	// Normalize global patterns: strip ~/ prefix so they match relative paths under home/globalDir.
	globalPatterns := normalizeGlobalPatterns(cfg.GlobalSync.Patterns)

	count, err := gosync.CopyByPatterns(globalDir, home, globalPatterns, cfg.Ignore, maxFileSize)
	if err != nil {
		return fmt.Errorf("copying global files from sync repo: %w", err)
	}
	if verbose {
		fmt.Printf("Copied %d global files from sync repo\n", count)
	}

	fmt.Println("Global pull complete.")
	return nil
}
