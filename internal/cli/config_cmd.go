package cli

import (
	"fmt"
	"os"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current ghost-sync configuration",
	Long:  `Print the path and contents of the ghost-sync configuration file.`,
	RunE:  runConfig,
}

func init() {
	RootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return fmt.Errorf("resolving config path: %w", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config not found at %s — run `ghost-sync init` first", cfgPath)
		}
		return fmt.Errorf("reading config: %w", err)
	}

	fmt.Printf("# Config path: %s\n\n", cfgPath)
	fmt.Print(string(data))
	return nil
}
