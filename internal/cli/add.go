package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Register the current project with ghost-sync",
	Long: `Register the current git project with ghost-sync.

Detects the remote URL and project name from the current directory,
registers the project in the config, and writes git exclude patterns.`,
	RunE: runAdd,
}

func init() {
	RootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
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

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Detect remote URL and project name.
	remoteURL, repoName, detectErr := project.DetectProject(cwd)
	if detectErr != nil {
		// No remote — warn and fall back to directory name.
		fmt.Fprintf(os.Stderr, "Warning: could not detect git remote: %v\n", detectErr)
		fmt.Fprintln(os.Stderr, "Using directory name as project name.")
		repoName = filepath.Base(cwd)
		remoteURL = ""
	}

	// Generate project ID.
	idSource := remoteURL
	if idSource == "" {
		idSource = cwd
	}
	projectID := project.GenerateProjectID(idSource)

	// Detect git root.
	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	// Register project in config.
	if err := project.AddProject(cfg, repoName, remoteURL, projectID, gitRoot); err != nil {
		return fmt.Errorf("registering project: %w", err)
	}

	// Write exclude patterns.
	excludePath := project.ExcludePathForRepo(gitRoot)
	if err := project.WriteExclude(excludePath, cfg.Patterns); err != nil {
		return fmt.Errorf("writing exclude patterns: %w", err)
	}

	// Create project directory in sync repo.
	if cfg.SyncRepoPath != "" {
		projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(repoName, projectID))
		if err := os.MkdirAll(projDir, 0o755); err != nil {
			return fmt.Errorf("creating project directory in sync repo: %w", err)
		}
	}

	// Save config.
	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Registered project: %s (ID: %s)\n", repoName, projectID)
	fmt.Println("Run `ghost-sync hooks install` to install git hooks.")
	return nil
}
