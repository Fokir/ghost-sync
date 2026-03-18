package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull then push — full bidirectional sync",
	Long: `Perform a full sync: pull from the sync repo, then push local changes.

This is equivalent to running 'ghost-sync pull' followed by 'ghost-sync push'.`,
	RunE: runSync,
}

var (
	syncGlobal   bool
	syncFromHook bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncGlobal, "global", false, "sync global paths instead of project files")
	syncCmd.Flags().BoolVar(&syncFromHook, "from-hook", false, "called from a git hook")
	_ = syncCmd.Flags().MarkHidden("from-hook")
	RootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
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

	if syncGlobal {
		// Pull global first, then push global.
		if err := doPullGlobal(cfg, syncFromHook, verbose); err != nil {
			return fmt.Errorf("global pull: %w", err)
		}
		if err := doPushGlobal(cfg, syncFromHook, verbose); err != nil {
			return fmt.Errorf("global push: %w", err)
		}
		return nil
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

	// Acquire lock once for the entire sync operation.
	lock, err := gosync.AcquireLock(gosync.DefaultLockPath(), 10*time.Second)
	if err != nil {
		return fmt.Errorf("another ghost-sync operation in progress")
	}
	defer gosync.ReleaseLock(lock)

	// Pull first, then push — skip lock since we already hold it.
	if err := doPull(cfg, proj, syncFromHook, verbose, true); err != nil {
		return fmt.Errorf("pull: %w", err)
	}
	if err := doPush(cfg, proj, syncFromHook, verbose, true); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	return nil
}
