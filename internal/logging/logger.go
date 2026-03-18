package logging

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger writes timestamped log entries to a file.
type Logger struct {
	file *os.File
}

// DefaultLogPath returns the default log file path.
func DefaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ghost-sync", "logs", "ghost-sync.log")
	}
	return filepath.Join(home, ".ghost-sync", "logs", "ghost-sync.log")
}

// NewLogger creates the necessary directories and opens the log file in append mode.
func NewLogger(path string) (*Logger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{file: f}, nil
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	return l.file.Close()
}

// log writes a single formatted log entry.
func (l *Logger) log(level, msg string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.file, "[%s] %s: %s\n", ts, level, msg)
}

// Info logs an informational message.
func (l *Logger) Info(msg string) { l.log("INFO", msg) }

// Warn logs a warning message.
func (l *Logger) Warn(msg string) { l.log("WARN", msg) }

// Error logs an error message.
func (l *Logger) Error(msg string) { l.log("ERROR", msg) }

// Tail returns the last n lines from the log file at path.
// If the file does not exist it returns nil, nil.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if n >= len(lines) {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

// TailByProject reads the log file, filters lines containing the project name,
// and returns the last n matching lines.
func TailByProject(path, project string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var matched []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, project) {
			matched = append(matched, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if n >= len(matched) {
		return matched, nil
	}
	return matched[len(matched)-n:], nil
}
