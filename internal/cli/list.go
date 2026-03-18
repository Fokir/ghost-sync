package cli

import (
	"fmt"
	"os"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	Long:  `Display all projects registered with ghost-sync, including their status, path, and remote.`,
	RunE:  runList,
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	if len(cfg.Projects) == 0 {
		fmt.Println("No projects registered. Run `ghost-sync add` inside a git repository.")
		return nil
	}

	fmt.Printf("%-20s  %-12s  %-8s  %-40s  %s\n", "NAME", "ID", "STATUS", "PATH", "REMOTE")
	fmt.Printf("%-20s  %-12s  %-8s  %-40s  %s\n",
		"--------------------", "------------", "--------",
		"----------------------------------------", "------")

	for _, proj := range cfg.Projects {
		status := "ok"
		if _, err := os.Stat(proj.Path); os.IsNotExist(err) {
			status = "missing"
		}

		remote := proj.Remote
		if remote == "" {
			remote = "(none)"
		}

		shortID := proj.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		fmt.Printf("%-20s  %-12s  %-8s  %-40s  %s\n",
			proj.Name, shortID, status, proj.Path, remote)
	}

	return nil
}
