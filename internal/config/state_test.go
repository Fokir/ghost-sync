package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateSaveLoadRoundtrip(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "state.json")

	original := &State{
		LastBackupPrune: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		PendingPushes: []PendingPush{
			{
				ProjectID:      "proj-1",
				SyncRepoCommit: "abc123",
				Timestamp:      time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC),
				Reason:         "file changed",
			},
		},
	}

	if err := SaveState(original, path); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if len(loaded.PendingPushes) != 1 {
		t.Fatalf("PendingPushes length = %d, want 1", len(loaded.PendingPushes))
	}

	pp := loaded.PendingPushes[0]
	if pp.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q, want \"proj-1\"", pp.ProjectID)
	}
	if pp.SyncRepoCommit != "abc123" {
		t.Errorf("SyncRepoCommit = %q, want \"abc123\"", pp.SyncRepoCommit)
	}
	if pp.Reason != "file changed" {
		t.Errorf("Reason = %q, want \"file changed\"", pp.Reason)
	}
	if !loaded.LastBackupPrune.Equal(original.LastBackupPrune) {
		t.Errorf("LastBackupPrune = %v, want %v", loaded.LastBackupPrune, original.LastBackupPrune)
	}
}

func TestLoadStateNonexistent(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "nonexistent.json")

	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() on nonexistent file returned error: %v", err)
	}
	if state == nil {
		t.Fatal("LoadState() returned nil state")
	}
	if len(state.PendingPushes) != 0 {
		t.Errorf("PendingPushes = %v, want empty", state.PendingPushes)
	}
}

func TestAddPendingPush(t *testing.T) {
	s := &State{}

	before := time.Now().UTC()
	s.AddPendingPush("proj-a", "deadbeef", "test reason")
	after := time.Now().UTC()

	if len(s.PendingPushes) != 1 {
		t.Fatalf("PendingPushes length = %d, want 1", len(s.PendingPushes))
	}

	pp := s.PendingPushes[0]
	if pp.ProjectID != "proj-a" {
		t.Errorf("ProjectID = %q, want \"proj-a\"", pp.ProjectID)
	}
	if pp.SyncRepoCommit != "deadbeef" {
		t.Errorf("SyncRepoCommit = %q, want \"deadbeef\"", pp.SyncRepoCommit)
	}
	if pp.Reason != "test reason" {
		t.Errorf("Reason = %q, want \"test reason\"", pp.Reason)
	}
	if pp.Timestamp.Before(before) || pp.Timestamp.After(after) {
		t.Errorf("Timestamp %v not in expected range [%v, %v]", pp.Timestamp, before, after)
	}
}

func TestClearPendingPushes(t *testing.T) {
	s := &State{}
	s.AddPendingPush("proj-a", "", "r1")
	s.AddPendingPush("proj-b", "", "r2")
	s.AddPendingPush("proj-a", "", "r3")

	s.ClearPendingPushes("proj-a")

	if len(s.PendingPushes) != 1 {
		t.Fatalf("PendingPushes length = %d after clear, want 1", len(s.PendingPushes))
	}
	if s.PendingPushes[0].ProjectID != "proj-b" {
		t.Errorf("remaining push ProjectID = %q, want \"proj-b\"", s.PendingPushes[0].ProjectID)
	}
}

func TestClearPendingPushesNonexistent(t *testing.T) {
	s := &State{}
	s.AddPendingPush("proj-a", "", "r1")

	// Clearing a project that has no pushes should not panic or corrupt state.
	s.ClearPendingPushes("proj-z")

	if len(s.PendingPushes) != 1 {
		t.Errorf("PendingPushes length = %d, want 1", len(s.PendingPushes))
	}
}
