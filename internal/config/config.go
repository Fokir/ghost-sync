package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// PatternsOverride allows per-project additions and exclusions to the default patterns.
type PatternsOverride struct {
	Add     []string `yaml:"add,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// ConflictRule defines a conflict resolution rule for a specific pattern.
type ConflictRule struct {
	Pattern  string `yaml:"pattern"`
	Strategy string `yaml:"strategy"`
}

// GlobalSync contains settings for syncing to all projects.
type GlobalSync struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns,omitempty"`
}

// ProjectEntry represents a registered project in the config.
type ProjectEntry struct {
	Name             string            `yaml:"name"`
	Remote           string            `yaml:"remote,omitempty"`
	ID               string            `yaml:"id"`
	Path             string            `yaml:"path"`
	PatternsOverride *PatternsOverride `yaml:"patterns_override,omitempty"`
}

// Config is the main configuration structure for ghost-sync.
type Config struct {
	Version          int            `yaml:"version"`
	SyncRepo         string         `yaml:"sync_repo,omitempty"`
	SyncRepoPath     string         `yaml:"sync_repo_path,omitempty"`
	MachineID        string         `yaml:"machine_id,omitempty"`
	Patterns         []string       `yaml:"patterns,omitempty"`
	Ignore           []string       `yaml:"ignore,omitempty"`
	MaxFileSize      string         `yaml:"max_file_size,omitempty"`
	ConflictStrategy string         `yaml:"conflict_strategy,omitempty"`
	ConflictRules    []ConflictRule `yaml:"conflict_rules,omitempty"`
	Projects         []ProjectEntry `yaml:"projects,omitempty"`
	GlobalSync       *GlobalSync    `yaml:"global_sync,omitempty"`
}

// ConfigDir returns the default ghost-sync config directory (~/.ghost-sync).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".ghost-sync"), nil
}

// DefaultConfigPath returns the default path to the config file (~/.ghost-sync/config.yaml).
func DefaultConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads a Config from the given YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return &cfg, nil
}

// Save writes the Config to the given YAML file path, creating directories as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config file %q: %w", path, err)
	}

	return nil
}

// EnsureDefaults creates a config file with default values if it doesn't exist.
// It auto-detects the hostname for MachineID.
func EnsureDefaults(path string) (*Config, error) {
	// If the file already exists, just load it.
	if _, err := os.Stat(path); err == nil {
		return Load(path)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	cfg := &Config{
		Version:          ConfigVersion,
		MachineID:        hostname,
		Patterns:         DefaultPatterns(),
		Ignore:           DefaultIgnore(),
		MaxFileSize:      DefaultMaxFileSize,
		ConflictStrategy: DefaultConflictStrategy,
	}

	if err := Save(cfg, path); err != nil {
		return nil, fmt.Errorf("saving default config: %w", err)
	}

	return cfg, nil
}

// EffectivePatterns returns the resolved list of sync patterns for the given project,
// applying any per-project add/exclude overrides on top of the global patterns.
func (c *Config) EffectivePatterns(proj *ProjectEntry) []string {
	base := c.Patterns
	if len(base) == 0 {
		base = DefaultPatterns()
	}

	if proj == nil || proj.PatternsOverride == nil {
		result := make([]string, len(base))
		copy(result, base)
		return result
	}

	override := proj.PatternsOverride

	// Build a set from base patterns.
	patSet := make(map[string]bool, len(base)+len(override.Add))
	ordered := make([]string, 0, len(base)+len(override.Add))

	for _, p := range base {
		if !patSet[p] {
			patSet[p] = true
			ordered = append(ordered, p)
		}
	}

	// Add extra patterns.
	for _, p := range override.Add {
		if !patSet[p] {
			patSet[p] = true
			ordered = append(ordered, p)
		}
	}

	// Remove excluded patterns.
	excludeSet := make(map[string]bool, len(override.Exclude))
	for _, e := range override.Exclude {
		excludeSet[e] = true
	}

	result := make([]string, 0, len(ordered))
	for _, p := range ordered {
		if !excludeSet[p] {
			result = append(result, p)
		}
	}

	return result
}

// FindProject returns the ProjectEntry with the given ID, or nil if not found.
func (c *Config) FindProject(id string) *ProjectEntry {
	for i := range c.Projects {
		if c.Projects[i].ID == id {
			return &c.Projects[i]
		}
	}
	return nil
}

// FindProjectByPath returns the ProjectEntry with the given path, or nil if not found.
// Paths are normalized with filepath.Clean and compared case-insensitively on Windows.
func (c *Config) FindProjectByPath(path string) *ProjectEntry {
	cleanPath := filepath.Clean(path)
	for i := range c.Projects {
		if pathsEqual(filepath.Clean(c.Projects[i].Path), cleanPath) {
			return &c.Projects[i]
		}
	}
	return nil
}

// ParseFileSize parses a human-readable file size string (e.g. "10MB", "500KB", "1GB")
// and returns the size in bytes. Supported suffixes: KB, MB, GB (case-insensitive).
func ParseFileSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty file size string")
	}

	upper := strings.ToUpper(s)

	var multiplier int64
	var numStr string

	switch {
	case strings.HasSuffix(upper, "GB"):
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(upper, "GB")
	case strings.HasSuffix(upper, "MB"):
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "KB"):
		multiplier = 1024
		numStr = strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "B"):
		multiplier = 1
		numStr = strings.TrimSuffix(upper, "B")
	default:
		// Try to parse as plain bytes.
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("unrecognised file size %q: no known suffix (KB, MB, GB) and not a plain integer", s)
		}
		return n, nil
	}

	numStr = strings.TrimSpace(numStr)
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing numeric part of file size %q: %w", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("file size must not be negative: %q", s)
	}

	return n * multiplier, nil
}

// pathsEqual compares two cleaned file paths. On Windows the comparison is
// case-insensitive because the file system is case-insensitive.
func pathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
