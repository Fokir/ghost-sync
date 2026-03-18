package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PendingPush records a sync push that has been queued but not yet completed.
type PendingPush struct {
	ProjectID      string    `json:"project_id"`
	SyncRepoCommit string    `json:"sync_repo_commit,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason,omitempty"`
}

// State holds operational data persisted between ghost-sync runs.
type State struct {
	PendingPushes   []PendingPush `json:"pending_pushes,omitempty"`
	LastBackupPrune time.Time     `json:"last_backup_prune,omitzero"`
}

// LoadState reads State from the given JSON file path.
// If the file does not exist an empty State is returned (no error).
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("reading state file %q: %w", path, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file %q: %w", path, err)
	}

	return &s, nil
}

// SaveState writes State to the given JSON file path, creating directories as needed.
func SaveState(state *State, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing state file %q: %w", path, err)
	}

	return nil
}

// AddPendingPush appends a new PendingPush to the state.
func (s *State) AddPendingPush(projectID, syncRepoCommit, reason string) {
	s.PendingPushes = append(s.PendingPushes, PendingPush{
		ProjectID:      projectID,
		SyncRepoCommit: syncRepoCommit,
		Timestamp:      time.Now().UTC(),
		Reason:         reason,
	})
}

// ClearPendingPushes removes all pending pushes for the given project ID.
func (s *State) ClearPendingPushes(projectID string) {
	filtered := s.PendingPushes[:0]
	for _, p := range s.PendingPushes {
		if p.ProjectID != projectID {
			filtered = append(filtered, p)
		}
	}
	s.PendingPushes = filtered
}
