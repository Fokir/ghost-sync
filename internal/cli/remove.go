package cli

import (
	"fmt"
	"os"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/hooks"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Unregister the current project from ghost-sync",
	Long: `Unregister the current git project from ghost-sync.

Removes git exclude patterns, uninstalls git hooks, and removes the project
from the ghost-sync registry.`,
	RunE: runRemove,
}

func init() {
	RootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
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

	// Detect git root.
	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	// Find registered project by path.
	proj := cfg.FindProjectByPath(gitRoot)
	if proj == nil {
		return fmt.Errorf("no ghost-sync project registered for %s", gitRoot)
	}

	projName := proj.Name
	projID := proj.ID

	// Remove exclude patterns.
	excludePath := project.ExcludePathForRepo(gitRoot)
	if err := project.RemoveExclude(excludePath); err != nil {
		return fmt.Errorf("removing exclude patterns: %w", err)
	}

	// Remove git hooks (warn on error, do not abort).
	if err := hooks.Remove(gitRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove git hooks: %v\n", err)
	}

	// Remove from registry.
	if err := project.RemoveProject(cfg, projID); err != nil {
		return fmt.Errorf("removing project from registry: %w", err)
	}

	// Save config.
	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Removed project: %s (ID: %s)\n", projName, projID)
	return nil
}
