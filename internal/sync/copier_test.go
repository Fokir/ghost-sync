package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to create a file with given content in a temp dir layout.
func createTestFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func fileExists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

func TestCopyByPatterns_DirPattern(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	createTestFile(t, src, ".claude/settings.json", `{"key":"val"}`)
	createTestFile(t, src, ".claude/cache/big.json", `{}`)
	createTestFile(t, src, "unrelated.txt", "skip me")

	n, err := CopyByPatterns(src, dst, []string{".claude/"}, nil, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns error: %v", err)
	}
	if n != 2 {
		t.Errorf("copied %d files, want 2", n)
	}
	if !fileExists(dst, ".claude/settings.json") {
		t.Error(".claude/settings.json should be copied")
	}
	if !fileExists(dst, ".claude/cache/big.json") {
		t.Error(".claude/cache/big.json should be copied")
	}
	if fileExists(dst, "unrelated.txt") {
		t.Error("unrelated.txt should NOT be copied")
	}
}

func TestCopyByPatterns_FilePattern(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	createTestFile(t, src, "CLAUDE.md", "# Claude")
	createTestFile(t, src, "README.md", "# Readme")
	createTestFile(t, src, ".claude/settings.json", "{}")

	n, err := CopyByPatterns(src, dst, []string{"CLAUDE.md"}, nil, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns error: %v", err)
	}
	if n != 1 {
		t.Errorf("copied %d files, want 1", n)
	}
	if !fileExists(dst, "CLAUDE.md") {
		t.Error("CLAUDE.md should be copied")
	}
	if fileExists(dst, "README.md") {
		t.Error("README.md should NOT be copied")
	}
}

func TestCopyByPatterns_IgnorePattern(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	createTestFile(t, src, ".claude/settings.json", `{"key":"val"}`)
	createTestFile(t, src, ".claude/cache/cached.json", `{}`)

	n, err := CopyByPatterns(src, dst, []string{".claude/"}, []string{".claude/cache/"}, 0)
	if err != nil {
		t.Fatalf("CopyByPatterns error: %v", err)
	}
	if n != 1 {
		t.Errorf("copied %d files, want 1", n)
	}
	if !fileExists(dst, ".claude/settings.json") {
		t.Error(".claude/settings.json should be copied")
	}
	if fileExists(dst, ".claude/cache/cached.json") {
		t.Error(".claude/cache/cached.json should NOT be copied (ignored)")
	}
}

func TestCopyByPatterns_MaxFileSize(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	createTestFile(t, src, "small.txt", "hi")         // 2 bytes
	createTestFile(t, src, "large.txt", "hello world") // 11 bytes

	// Allow up to 5 bytes.
	n, err := CopyByPatterns(src, dst, nil, nil, 5)
	if err != nil {
		t.Fatalf("CopyByPatterns error: %v", err)
	}
	if n != 1 {
		t.Errorf("copied %d files, want 1", n)
	}
	if !fileExists(dst, "small.txt") {
		t.Error("small.txt should be copied")
	}
	if fileExists(dst, "large.txt") {
		t.Error("large.txt should NOT be copied (exceeds size limit)")
	}
}

func TestDeleteFiles(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "a.txt", "aaa")
	createTestFile(t, dir, "b.txt", "bbb")

	if err := DeleteFiles(dir, []string{"a.txt"}); err != nil {
		t.Fatalf("DeleteFiles error: %v", err)
	}
	if fileExists(dir, "a.txt") {
		t.Error("a.txt should have been deleted")
	}
	if !fileExists(dir, "b.txt") {
		t.Error("b.txt should still exist")
	}
}

func TestDeleteFiles_NonExistentIsOK(t *testing.T) {
	dir := t.TempDir()
	// Deleting a non-existent file should not return an error.
	if err := DeleteFiles(dir, []string{"ghost.txt"}); err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}
