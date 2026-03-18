package cli

import (
	"github.com/spf13/cobra"
)

var (
	verbose bool
	dryRun  bool
)

// RootCmd is the root cobra command for ghost-sync.
var RootCmd = &cobra.Command{
	Use:   "ghost-sync",
	Short: "Synchronize AI-agent files across projects via a private git repo",
	Long: `ghost-sync is a CLI tool that synchronizes AI-agent configuration
and context files across multiple projects through a private git repository.`,
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	RootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "simulate actions without making changes")
}
