package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExcludePathForRepo(t *testing.T) {
	got := ExcludePathForRepo("/some/repo")
	expected := filepath.Join("/some/repo", ".git", "info", "exclude")
	if got != expected {
		t.Errorf("ExcludePathForRepo() = %q, want %q", got, expected)
	}
}

func TestWriteExclude_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exclude")

	patterns := []string{".claude/", ".cursor/", ".ai-context/"}
	if err := WriteExclude(path, patterns); err != nil {
		t.Fatalf("WriteExclude error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, excludeMarkerBegin) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, excludeMarkerEnd) {
		t.Error("missing end marker")
	}
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			t.Errorf("pattern %q not found in exclude file", p)
		}
	}
}

func TestWriteExclude_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exclude")

	existing := "# existing git exclude rules\n*.log\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns := []string{".claude/"}
	if err := WriteExclude(path, patterns); err != nil {
		t.Fatalf("WriteExclude error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "# existing git exclude rules") {
		t.Error("existing comment was removed")
	}
	if !strings.Contains(content, "*.log") {
		t.Error("existing pattern *.log was removed")
	}
	if !strings.Contains(content, ".claude/") {
		t.Error("new pattern .claude/ not found")
	}
}

func TestWriteExclude_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exclude")

	patterns1 := []string{".claude/", ".cursor/"}
	if err := WriteExclude(path, patterns1); err != nil {
		t.Fatalf("first WriteExclude error: %v", err)
	}

	patterns2 := []string{".claude/", ".cursor/", ".ai-context/"}
	if err := WriteExclude(path, patterns2); err != nil {
		t.Fatalf("second WriteExclude error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// Should have exactly one begin marker
	beginCount := strings.Count(content, excludeMarkerBegin)
	if beginCount != 1 {
		t.Errorf("expected 1 begin marker, got %d\ncontent:\n%s", beginCount, content)
	}
	endCount := strings.Count(content, excludeMarkerEnd)
	if endCount != 1 {
		t.Errorf("expected 1 end marker, got %d\ncontent:\n%s", endCount, content)
	}

	// Latest patterns should be present
	if !strings.Contains(content, ".ai-context/") {
		t.Error("latest pattern .ai-context/ not found after second write")
	}
}

func TestWriteExclude_IdempotentWithExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exclude")

	// Start with some existing content
	existing := "# git exclude\n*.tmp\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns := []string{".claude/"}
	_ = WriteExclude(path, patterns)
	_ = WriteExclude(path, patterns)

	data, _ := os.ReadFile(path)
	content := string(data)

	beginCount := strings.Count(content, excludeMarkerBegin)
	if beginCount != 1 {
		t.Errorf("expected 1 begin marker, got %d\ncontent:\n%s", beginCount, content)
	}
	// Existing lines should still be present
	if !strings.Contains(content, "*.tmp") {
		t.Error("*.tmp was lost")
	}
}

func TestRemoveExclude(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exclude")

	existing := "# keep this\n*.log\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns := []string{".claude/", ".cursor/"}
	_ = WriteExclude(path, patterns)
	if err := RemoveExclude(path); err != nil {
		t.Fatalf("RemoveExclude error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if strings.Contains(content, excludeMarkerBegin) {
		t.Error("begin marker still present after removal")
	}
	if strings.Contains(content, excludeMarkerEnd) {
		t.Error("end marker still present after removal")
	}
	if strings.Contains(content, ".claude/") {
		t.Error("pattern .claude/ still present after removal")
	}
	if !strings.Contains(content, "# keep this") {
		t.Error("pre-existing comment was removed")
	}
	if !strings.Contains(content, "*.log") {
		t.Error("pre-existing pattern *.log was removed")
	}
}

func TestRemoveExclude_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")

	// Should not return an error
	if err := RemoveExclude(path); err != nil {
		t.Errorf("RemoveExclude on non-existent file returned error: %v", err)
	}
}

func TestWriteExclude_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git", "info", "exclude")

	if err := WriteExclude(path, []string{".claude/"}); err != nil {
		t.Fatalf("WriteExclude with nested path error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
