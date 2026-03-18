package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/repo"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise ghost-sync and configure the sync repository",
	Long: `Initialise ghost-sync configuration and set up the sync repository.

Use --repo to clone an existing remote sync repository, or --path to create
a new local sync repository.`,
	RunE: runInit,
}

var (
	initRepoURL   string
	initLocalPath string
)

func init() {
	initCmd.Flags().StringVar(&initRepoURL, "repo", "", "remote URL of the sync repository to clone")
	initCmd.Flags().StringVar(&initLocalPath, "path", "", "local directory for the sync repository")
	RootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return fmt.Errorf("resolving config path: %w", err)
	}

	cfg, err := config.EnsureDefaults(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	repoURL := initRepoURL
	localPath := initLocalPath

	switch {
	case repoURL != "":
		// Clone existing remote sync repo.
		if localPath == "" {
			if cfg.SyncRepoPath != "" {
				localPath = cfg.SyncRepoPath
			} else {
				// Default to ~/.ghost-sync/sync-repo
				dir, err := config.ConfigDir()
				if err != nil {
					return err
				}
				localPath = filepath.Join(dir, "sync-repo")
			}
		}

		if _, err := repo.CloneSyncRepo(repoURL, localPath); err != nil {
			return fmt.Errorf("cloning sync repo: %w", err)
		}

		cfg.SyncRepo = repoURL
		cfg.SyncRepoPath = localPath
		fmt.Printf("Cloned sync repo from %s into %s\n", repoURL, localPath)

	case localPath != "":
		// Create new local sync repo.
		if _, err := repo.InitSyncRepo(localPath); err != nil {
			return fmt.Errorf("initialising sync repo: %w", err)
		}

		cfg.SyncRepoPath = localPath
		fmt.Printf("Initialised new sync repo at %s\n", localPath)

	default:
		return fmt.Errorf("provide --repo <url> to clone an existing sync repo, or --path <dir> to create a new one")
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	_ = os.MkdirAll(filepath.Join(cfg.SyncRepoPath, "projects"), 0o755)

	fmt.Println("ghost-sync initialised successfully.")
	return nil
}
