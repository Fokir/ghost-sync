package main

import (
	"os"

	"github.com/sokolovsky/ghost-sync/internal/cli"
)

// Build-time variables injected via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Wire ldflags values into the cli package variables.
	cli.Version = version
	cli.Commit = commit
	cli.Date = date

	if err := cli.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
