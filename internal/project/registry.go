package project

import (
	"fmt"

	"github.com/sokolovsky/ghost-sync/internal/config"
)

// AddProject adds a new ProjectEntry to the config. Returns an error if a project
// with the same ID already exists.
func AddProject(cfg *config.Config, name, remote, id, path string) error {
	for _, p := range cfg.Projects {
		if p.ID == id {
			return fmt.Errorf("project with ID %q already registered (name: %q)", id, p.Name)
		}
	}
	cfg.Projects = append(cfg.Projects, config.ProjectEntry{
		Name:   name,
		Remote: remote,
		ID:     id,
		Path:   path,
	})
	return nil
}

// RemoveProject removes the project with the given ID from the config. Returns an
// error if no project with that ID exists.
func RemoveProject(cfg *config.Config, id string) error {
	for i, p := range cfg.Projects {
		if p.ID == id {
			cfg.Projects = append(cfg.Projects[:i], cfg.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project with ID %q not found", id)
}
