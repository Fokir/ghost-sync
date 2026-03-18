package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for remote updates (session hook, offline)",
	Long: `Fast offline check for remote-only or conflicting files.

Designed to be run as a git session hook. Does not perform any network
operations. Prints a message if remote files have changed since last sync.`,
	RunE:          runCheck,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	RootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		// Config not found or unreadable — exit silently.
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		// Not a git repo — exit silently.
		return nil
	}

	proj := cfg.FindProjectByPath(gitRoot)
	if proj == nil {
		fmt.Fprintf(os.Stderr, "ghost-sync: Project '%s' is not synced. Run: ghost-sync add\n",
			filepath.Base(gitRoot))
		return nil
	}

	if cfg.SyncRepoPath == "" {
		return nil
	}

	projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		// Sync dir doesn't exist yet — exit silently.
		return nil
	}

	diff, err := gosync.DiffFiles(gitRoot, projDir)
	if err != nil {
		return nil
	}

	patterns := cfg.EffectivePatterns(proj)
	ignore := cfg.Ignore
	if len(ignore) == 0 {
		ignore = config.DefaultIgnore()
	}

	remoteOnly := filterByPatterns(diff.RemoteOnly, patterns, ignore)
	remoteOnly = filterOutMeta(remoteOnly)

	var conflictCount int
	for _, c := range diff.Conflicts {
		if gosync.MatchesPatterns(c.Path, patterns) && !gosync.IsIgnored(c.Path, ignore) {
			conflictCount++
		}
	}

	total := len(remoteOnly) + conflictCount
	if total > 0 {
		fmt.Fprintf(os.Stderr, "ghost-sync: %d file(s) updated since last sync. Run: ghost-sync pull\n", total)
	}

	return nil
}
