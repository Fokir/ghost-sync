package cli

import (
	"fmt"
	"os"

	"github.com/sokolovsky/ghost-sync/internal/hooks"
	"github.com/sokolovsky/ghost-sync/internal/project"
	"github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks for ghost-sync",
	Long:  `Install or remove ghost-sync git hooks in the current repository.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install ghost-sync git hooks",
	Long:  `Install post-commit and post-merge git hooks in the current repository.`,
	RunE:  runHooksInstall,
}

var hooksRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove ghost-sync git hooks",
	Long:  `Remove ghost-sync sections from post-commit and post-merge git hooks.`,
	RunE:  runHooksRemove,
}

func init() {
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksRemoveCmd)
	RootCmd.AddCommand(hooksCmd)
}

func runHooksInstall(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	if err := hooks.Install(gitRoot); err != nil {
		return fmt.Errorf("installing hooks: %w", err)
	}

	fmt.Printf("Ghost-sync hooks installed in %s\n", gitRoot)
	return nil
}

func runHooksRemove(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	if err := hooks.Remove(gitRoot); err != nil {
		return fmt.Errorf("removing hooks: %w", err)
	}

	fmt.Printf("Ghost-sync hooks removed from %s\n", gitRoot)
	return nil
}
