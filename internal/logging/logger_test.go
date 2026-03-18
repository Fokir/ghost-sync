package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAndTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	logger.Info("first message")
	logger.Error("second message")
	logger.Warn("warning message")

	lines, err := Tail(path, 10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[2], "warning") {
		t.Errorf("last line should contain 'warning', got: %s", lines[2])
	}
}

func TestTailWithLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	for i := 0; i < 100; i++ {
		logger.Info("line")
	}

	lines, err := Tail(path, 5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
}

func TestTailNonExistent(t *testing.T) {
	lines, err := Tail("/nonexistent/path/to/log.log", 10)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got: %v", err)
	}
	if lines != nil {
		t.Fatalf("expected nil lines for nonexistent file, got: %v", lines)
	}
}

func TestTailByProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	logger.Info("project-alpha: started")
	logger.Info("project-beta: started")
	logger.Info("project-alpha: done")

	lines, err := TailByProject(path, "project-alpha", 10)
	if err != nil {
		t.Fatalf("TailByProject: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestDefaultLogPath(t *testing.T) {
	path := DefaultLogPath()
	if path == "" {
		t.Fatal("DefaultLogPath returned empty string")
	}
	if !strings.HasSuffix(path, "ghost-sync.log") {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestLoggerClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "close.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewLoggerCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "test.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}
