package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ghost-sync %s\n", Version)
		fmt.Printf("commit: %s\n", Commit)
		fmt.Printf("date:   %s\n", Date)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
