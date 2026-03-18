package project

import (
	"testing"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

func newTestConfig() *config.Config {
	return &config.Config{}
}

func TestAddProject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := newTestConfig()
		err := AddProject(cfg, "my-app", "https://github.com/team/my-app.git", "abc1234567", "/path/to/my-app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(cfg.Projects))
		}
		p := cfg.Projects[0]
		if p.Name != "my-app" {
			t.Errorf("Name = %q, want %q", p.Name, "my-app")
		}
		if p.ID != "abc1234567" {
			t.Errorf("ID = %q, want %q", p.ID, "abc1234567")
		}
		if p.Remote != "https://github.com/team/my-app.git" {
			t.Errorf("Remote = %q", p.Remote)
		}
		if p.Path != "/path/to/my-app" {
			t.Errorf("Path = %q", p.Path)
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		cfg := newTestConfig()
		_ = AddProject(cfg, "my-app", "https://github.com/team/my-app.git", "abc1234567", "/path/to/my-app")
		err := AddProject(cfg, "my-app-2", "https://github.com/team/my-app-2.git", "abc1234567", "/path/to/my-app-2")
		if err == nil {
			t.Fatal("expected error for duplicate ID, got nil")
		}
	})

	t.Run("multiple distinct projects", func(t *testing.T) {
		cfg := newTestConfig()
		_ = AddProject(cfg, "app1", "remote1", "id1111111a", "/path/1")
		_ = AddProject(cfg, "app2", "remote2", "id2222222b", "/path/2")
		if len(cfg.Projects) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
		}
	})
}

func TestRemoveProject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := newTestConfig()
		_ = AddProject(cfg, "my-app", "remote", "abc1234567", "/path")
		err := RemoveProject(cfg, "abc1234567")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Projects) != 0 {
			t.Errorf("expected 0 projects after removal, got %d", len(cfg.Projects))
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		cfg := newTestConfig()
		err := RemoveProject(cfg, "nonexistent")
		if err == nil {
			t.Fatal("expected error for not found, got nil")
		}
	})

	t.Run("removes correct project from multiple", func(t *testing.T) {
		cfg := newTestConfig()
		_ = AddProject(cfg, "app1", "remote1", "id1111111a", "/path/1")
		_ = AddProject(cfg, "app2", "remote2", "id2222222b", "/path/2")
		_ = AddProject(cfg, "app3", "remote3", "id3333333c", "/path/3")

		err := RemoveProject(cfg, "id2222222b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Projects) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
		}
		for _, p := range cfg.Projects {
			if p.ID == "id2222222b" {
				t.Error("removed project still present")
			}
		}
	})
}
