package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- helpers ----

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ghost-sync-config-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// ---- Load / Save roundtrip ----

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		Version:          ConfigVersion,
		MachineID:        "testhost",
		Patterns:         []string{".claude/", "CLAUDE.md"},
		Ignore:           []string{"node_modules/"},
		MaxFileSize:      "5MB",
		ConflictStrategy: "latest-wins",
		Projects: []ProjectEntry{
			{Name: "proj1", ID: "abc123", Path: "/home/user/proj1"},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, original.Version)
	}
	if loaded.MachineID != original.MachineID {
		t.Errorf("MachineID = %q, want %q", loaded.MachineID, original.MachineID)
	}
	if loaded.MaxFileSize != original.MaxFileSize {
		t.Errorf("MaxFileSize = %q, want %q", loaded.MaxFileSize, original.MaxFileSize)
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].ID != "abc123" {
		t.Errorf("Projects not preserved correctly")
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
}

// ---- EnsureDefaults ----

func TestEnsureDefaultsCreatesFile(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.yaml")

	cfg, err := EnsureDefaults(path)
	if err != nil {
		t.Fatalf("EnsureDefaults() error: %v", err)
	}

	if cfg.Version != ConfigVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, ConfigVersion)
	}
	if cfg.MaxFileSize != DefaultMaxFileSize {
		t.Errorf("MaxFileSize = %q, want %q", cfg.MaxFileSize, DefaultMaxFileSize)
	}
	if cfg.ConflictStrategy != DefaultConflictStrategy {
		t.Errorf("ConflictStrategy = %q, want %q", cfg.ConflictStrategy, DefaultConflictStrategy)
	}
	if len(cfg.Patterns) == 0 {
		t.Error("Patterns should not be empty after EnsureDefaults")
	}
	if len(cfg.Ignore) == 0 {
		t.Error("Ignore should not be empty after EnsureDefaults")
	}

	// File must have been created.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("EnsureDefaults() did not create the config file")
	}
}

func TestEnsureDefaultsLoadsExisting(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.yaml")

	existing := &Config{
		Version:   ConfigVersion,
		MachineID: "myhost",
		Patterns:  []string{"custom/"},
	}
	if err := Save(existing, path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg, err := EnsureDefaults(path)
	if err != nil {
		t.Fatalf("EnsureDefaults() error: %v", err)
	}

	// Should load existing, not overwrite.
	if cfg.MachineID != "myhost" {
		t.Errorf("MachineID = %q, want \"myhost\"", cfg.MachineID)
	}
	if len(cfg.Patterns) != 1 || cfg.Patterns[0] != "custom/" {
		t.Errorf("Patterns not preserved: %v", cfg.Patterns)
	}
}

// ---- EffectivePatterns ----

func TestEffectivePatternsNoOverride(t *testing.T) {
	cfg := &Config{
		Patterns: DefaultPatterns(),
	}

	patterns := cfg.EffectivePatterns(nil)
	if len(patterns) != len(DefaultPatterns()) {
		t.Errorf("EffectivePatterns(nil) = %d patterns, want %d", len(patterns), len(DefaultPatterns()))
	}
}

func TestEffectivePatternsAdd(t *testing.T) {
	cfg := &Config{
		Patterns: DefaultPatterns(),
	}
	proj := &ProjectEntry{
		ID: "p1",
		PatternsOverride: &PatternsOverride{
			Add: []string{"custom/"},
		},
	}

	patterns := cfg.EffectivePatterns(proj)

	found := false
	for _, p := range patterns {
		if p == "custom/" {
			found = true
		}
	}
	if !found {
		t.Error("EffectivePatterns() did not include added pattern \"custom/\"")
	}
	if len(patterns) != len(DefaultPatterns())+1 {
		t.Errorf("EffectivePatterns() = %d patterns, want %d", len(patterns), len(DefaultPatterns())+1)
	}
}

func TestEffectivePatternsExclude(t *testing.T) {
	cfg := &Config{
		Patterns: DefaultPatterns(),
	}
	proj := &ProjectEntry{
		ID: "p1",
		PatternsOverride: &PatternsOverride{
			Exclude: []string{".serena/"},
		},
	}

	patterns := cfg.EffectivePatterns(proj)

	for _, p := range patterns {
		if p == ".serena/" {
			t.Error("EffectivePatterns() should have excluded \".serena/\"")
		}
	}
	if len(patterns) != len(DefaultPatterns())-1 {
		t.Errorf("EffectivePatterns() = %d patterns, want %d", len(patterns), len(DefaultPatterns())-1)
	}
}

func TestEffectivePatternsAddAndExclude(t *testing.T) {
	cfg := &Config{
		Patterns: DefaultPatterns(),
	}
	proj := &ProjectEntry{
		ID: "p1",
		PatternsOverride: &PatternsOverride{
			Add:     []string{"extra/"},
			Exclude: []string{"GEMINI.md", "AGENTS.md"},
		},
	}

	patterns := cfg.EffectivePatterns(proj)

	foundExtra := false
	for _, p := range patterns {
		if p == "GEMINI.md" || p == "AGENTS.md" {
			t.Errorf("excluded pattern %q still present", p)
		}
		if p == "extra/" {
			foundExtra = true
		}
	}
	if !foundExtra {
		t.Error("added pattern \"extra/\" not present")
	}
	// default(6) + 1 added - 2 excluded = 5
	if len(patterns) != 5 {
		t.Errorf("EffectivePatterns() = %d, want 5; patterns: %v", len(patterns), patterns)
	}
}

// ---- FindProject / FindProjectByPath ----

func TestFindProject(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectEntry{
			{Name: "a", ID: "id-a", Path: "/path/a"},
			{Name: "b", ID: "id-b", Path: "/path/b"},
		},
	}

	p := cfg.FindProject("id-a")
	if p == nil || p.Name != "a" {
		t.Error("FindProject(\"id-a\") did not return correct project")
	}

	if cfg.FindProject("nonexistent") != nil {
		t.Error("FindProject() should return nil for unknown ID")
	}
}

func TestFindProjectByPath(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectEntry{
			{Name: "a", ID: "id-a", Path: "/path/a"},
			{Name: "b", ID: "id-b", Path: "/path/b"},
		},
	}

	p := cfg.FindProjectByPath("/path/b")
	if p == nil || p.ID != "id-b" {
		t.Error("FindProjectByPath(\"/path/b\") did not return correct project")
	}

	if cfg.FindProjectByPath("/nonexistent") != nil {
		t.Error("FindProjectByPath() should return nil for unknown path")
	}
}

// ---- ParseFileSize ----

func TestParseFileSizeMB(t *testing.T) {
	n, err := ParseFileSize("10MB")
	if err != nil {
		t.Fatalf("ParseFileSize(\"10MB\") error: %v", err)
	}
	if n != 10*1024*1024 {
		t.Errorf("ParseFileSize(\"10MB\") = %d, want %d", n, 10*1024*1024)
	}
}

func TestParseFileSizeKB(t *testing.T) {
	n, err := ParseFileSize("500KB")
	if err != nil {
		t.Fatalf("ParseFileSize(\"500KB\") error: %v", err)
	}
	if n != 500*1024 {
		t.Errorf("ParseFileSize(\"500KB\") = %d, want %d", n, 500*1024)
	}
}

func TestParseFileSizeGB(t *testing.T) {
	n, err := ParseFileSize("2GB")
	if err != nil {
		t.Fatalf("ParseFileSize(\"2GB\") error: %v", err)
	}
	if n != 2*1024*1024*1024 {
		t.Errorf("ParseFileSize(\"2GB\") = %d, want %d", n, 2*1024*1024*1024)
	}
}

func TestParseFileSizeCaseInsensitive(t *testing.T) {
	cases := []string{"10mb", "10Mb", "10mB", "10MB"}
	for _, s := range cases {
		n, err := ParseFileSize(s)
		if err != nil {
			t.Errorf("ParseFileSize(%q) error: %v", s, err)
			continue
		}
		if n != 10*1024*1024 {
			t.Errorf("ParseFileSize(%q) = %d, want %d", s, n, 10*1024*1024)
		}
	}
}

func TestParseFileSizePlainBytes(t *testing.T) {
	n, err := ParseFileSize("1024")
	if err != nil {
		t.Fatalf("ParseFileSize(\"1024\") error: %v", err)
	}
	if n != 1024 {
		t.Errorf("ParseFileSize(\"1024\") = %d, want 1024", n)
	}
}

func TestParseFileSizeInvalid(t *testing.T) {
	invalids := []string{"", "abc", "10XB", "-5MB"}
	for _, s := range invalids {
		_, err := ParseFileSize(s)
		if err == nil {
			t.Errorf("ParseFileSize(%q) expected error, got nil", s)
		}
	}
}
