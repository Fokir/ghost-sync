package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sokolovsky/ghost-sync/internal/config"
	"github.com/sokolovsky/ghost-sync/internal/project"
	gosync "github.com/sokolovsky/ghost-sync/internal/sync"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status for the current project",
	Long: `Display the sync status between the local project and the sync repository.

Shows files that are local-only (will be pushed), remote-only (will be pulled),
conflicting (different content), and in-sync.`,
	RunE: runStatus,
}

func init() {
	RootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	if cfg.SyncRepoPath == "" {
		return fmt.Errorf("sync_repo_path not configured — run `ghost-sync init` first")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.DetectGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("detecting git root: %w", err)
	}

	proj := cfg.FindProjectByPath(gitRoot)
	if proj == nil {
		return fmt.Errorf("current directory is not a registered project — run `ghost-sync add` first")
	}

	patterns := cfg.EffectivePatterns(proj)
	ignore := cfg.Ignore
	if len(ignore) == 0 {
		ignore = config.DefaultIgnore()
	}

	projDir := filepath.Join(cfg.SyncRepoPath, "projects", project.ProjectDirName(proj.Name, proj.ID))

	// Check if projDir exists; if not, everything is local-only.
	projDirExists := true
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		projDirExists = false
	}

	fmt.Printf("Project: %s (ID: %s)\n", proj.Name, proj.ID)
	fmt.Printf("Path:    %s\n", proj.Path)
	fmt.Printf("Sync:    %s\n", projDir)
	fmt.Println()

	if !projDirExists {
		// Collect local files matching patterns.
		localFiles, err := collectMatchingFiles(gitRoot, patterns, ignore)
		if err != nil {
			return fmt.Errorf("collecting local files: %w", err)
		}
		if len(localFiles) == 0 {
			fmt.Println("No matching files found.")
		} else {
			fmt.Printf("Local only (will be pushed): %d files\n", len(localFiles))
			for _, f := range localFiles {
				fmt.Printf("  + %s\n", f)
			}
		}
		return nil
	}

	// Run DiffFiles between local (gitRoot) and remote (projDir).
	diff, err := gosync.DiffFiles(gitRoot, projDir)
	if err != nil {
		return fmt.Errorf("comparing files: %w", err)
	}

	// Filter results by sync patterns.
	localOnly := filterByPatterns(diff.LocalOnly, patterns, ignore)
	remoteOnly := filterByPatterns(diff.RemoteOnly, patterns, ignore)
	same := filterByPatterns(diff.Same, patterns, ignore)

	var conflicts []gosync.ConflictEntry
	for _, c := range diff.Conflicts {
		if gosync.MatchesPatterns(c.Path, patterns) && !gosync.IsIgnored(c.Path, ignore) {
			conflicts = append(conflicts, c)
		}
	}

	// Remove .ghost-sync.meta from remote-only (it's not a real sync file).
	remoteOnly = filterOutMeta(remoteOnly)

	sort.Strings(localOnly)
	sort.Strings(remoteOnly)
	sort.Strings(same)

	totalTracked := len(localOnly) + len(remoteOnly) + len(same) + len(conflicts)

	if totalTracked == 0 {
		fmt.Println("No matching files found.")
		return nil
	}

	if len(same) > 0 {
		fmt.Printf("In sync: %d files\n", len(same))
	}

	if len(localOnly) > 0 {
		fmt.Printf("\nLocal only (will be pushed): %d files\n", len(localOnly))
		for _, f := range localOnly {
			fmt.Printf("  + %s\n", f)
		}
	}

	if len(remoteOnly) > 0 {
		fmt.Printf("\nRemote only (will be pulled): %d files\n", len(remoteOnly))
		for _, f := range remoteOnly {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(conflicts) > 0 {
		fmt.Printf("\nConflicts: %d files\n", len(conflicts))
		for _, c := range conflicts {
			fmt.Printf("  ! %s (local: %s, remote: %s)\n", c.Path,
				c.LocalMtime.Format("2006-01-02 15:04:05"),
				c.RemoteMtime.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

// collectMatchingFiles collects files from dir that match patterns and are not ignored.
func collectMatchingFiles(dir string, patterns, ignore []string) ([]string, error) {
	allFiles, err := gosync.CollectFiles(dir)
	if err != nil {
		return nil, err
	}

	var result []string
	for rel := range allFiles {
		if gosync.MatchesPatterns(rel, patterns) && !gosync.IsIgnored(rel, ignore) {
			result = append(result, rel)
		}
	}
	sort.Strings(result)
	return result, nil
}

// filterByPatterns returns only paths that match patterns and are not ignored.
func filterByPatterns(paths []string, patterns, ignore []string) []string {
	var result []string
	for _, p := range paths {
		if gosync.MatchesPatterns(p, patterns) && !gosync.IsIgnored(p, ignore) {
			result = append(result, p)
		}
	}
	return result
}

// filterOutMeta removes .ghost-sync.meta from the list.
func filterOutMeta(paths []string) []string {
	var result []string
	for _, p := range paths {
		if p != ".ghost-sync.meta" {
			result = append(result, p)
		}
	}
	return result
}
