package cli

import (
	"fmt"

	"github.com/sokolovsky/ghost-sync/internal/logging"
	"github.com/spf13/cobra"
)

var (
	logLines   int
	logProject string
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show ghost-sync operation log",
	Long: `Display recent ghost-sync log entries.

Use --project to filter by project name, and --lines to control how many entries are shown.`,
	RunE: runLog,
}

func init() {
	logCmd.Flags().IntVar(&logLines, "lines", 50, "number of log lines to show")
	logCmd.Flags().StringVar(&logProject, "project", "", "filter log entries by project name")
	RootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	logPath := logging.DefaultLogPath()

	var entries []string
	var err error

	if logProject != "" {
		entries, err = logging.TailByProject(logPath, logProject, logLines)
	} else {
		entries, err = logging.Tail(logPath, logLines)
	}

	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No log entries found.")
		return nil
	}

	for _, line := range entries {
		fmt.Println(line)
	}

	return nil
}
