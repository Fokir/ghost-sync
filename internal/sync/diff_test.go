package sync

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFileHash_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(p, []byte("hello world"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := FileHash(p)
	if err != nil {
		t.Fatalf("FileHash error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestFileHash_SameContent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("consistent content")

	p1 := filepath.Join(dir, "a.txt")
	p2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(p1, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, content, 0600); err != nil {
		t.Fatal(err)
	}

	h1, err := FileHash(p1)
	if err != nil {
		t.Fatalf("FileHash a: %v", err)
	}
	h2, err := FileHash(p2)
	if err != nil {
		t.Fatalf("FileHash b: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected same hash, got %q vs %q", h1, h2)
	}
}

func TestDiffFiles(t *testing.T) {
	localDir := t.TempDir()
	remoteDir := t.TempDir()

	writeFile := func(dir, name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	// local-only
	writeFile(localDir, "local-only.txt", "local only")
	// remote-only
	writeFile(remoteDir, "remote-only.txt", "remote only")
	// same content in both
	writeFile(localDir, "same.txt", "identical content")
	writeFile(remoteDir, "same.txt", "identical content")
	// conflict: different content
	writeFile(localDir, "conflict.txt", "local version")
	writeFile(remoteDir, "conflict.txt", "remote version")

	result, err := DiffFiles(localDir, remoteDir)
	if err != nil {
		t.Fatalf("DiffFiles error: %v", err)
	}

	sort.Strings(result.LocalOnly)
	sort.Strings(result.RemoteOnly)
	sort.Strings(result.Same)

	if len(result.LocalOnly) != 1 || result.LocalOnly[0] != "local-only.txt" {
		t.Errorf("LocalOnly = %v, want [local-only.txt]", result.LocalOnly)
	}
	if len(result.RemoteOnly) != 1 || result.RemoteOnly[0] != "remote-only.txt" {
		t.Errorf("RemoteOnly = %v, want [remote-only.txt]", result.RemoteOnly)
	}
	if len(result.Same) != 1 || result.Same[0] != "same.txt" {
		t.Errorf("Same = %v, want [same.txt]", result.Same)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Path != "conflict.txt" {
		t.Errorf("Conflicts = %v, want [{conflict.txt ...}]", result.Conflicts)
	}
}

func TestDiffFiles_Subdirectory(t *testing.T) {
	localDir := t.TempDir()
	remoteDir := t.TempDir()

	writeNested := func(dir, rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	writeNested(localDir, ".claude/settings.json", `{"key":"value"}`)
	writeNested(remoteDir, ".claude/settings.json", `{"key":"value"}`)

	result, err := DiffFiles(localDir, remoteDir)
	if err != nil {
		t.Fatalf("DiffFiles error: %v", err)
	}
	if len(result.Same) != 1 || result.Same[0] != ".claude/settings.json" {
		t.Errorf("Same = %v, want [.claude/settings.json]", result.Same)
	}
}
