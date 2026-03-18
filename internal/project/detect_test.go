package project

import (
	"testing"
)

func TestGenerateProjectID(t *testing.T) {
	t.Run("length is 10", func(t *testing.T) {
		id := GenerateProjectID("https://github.com/team/my-app.git")
		if len(id) != 10 {
			t.Errorf("expected length 10, got %d: %q", len(id), id)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := GenerateProjectID("https://github.com/team/my-app.git")
		id2 := GenerateProjectID("https://github.com/team/my-app.git")
		if id1 != id2 {
			t.Errorf("expected same ID, got %q and %q", id1, id2)
		}
	})

	t.Run("strips .git suffix", func(t *testing.T) {
		idWithGit := GenerateProjectID("https://github.com/team/my-app.git")
		idWithoutGit := GenerateProjectID("https://github.com/team/my-app")
		if idWithGit != idWithoutGit {
			t.Errorf("expected same ID after .git strip, got %q and %q", idWithGit, idWithoutGit)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		id1 := GenerateProjectID("https://GITHUB.COM/team/my-app.git")
		id2 := GenerateProjectID("https://github.com/team/my-app.git")
		if id1 != id2 {
			t.Errorf("expected same ID regardless of case, got %q and %q", id1, id2)
		}
	})

	t.Run("different URLs produce different IDs", func(t *testing.T) {
		id1 := GenerateProjectID("https://github.com/team/my-app.git")
		id2 := GenerateProjectID("https://github.com/team/other-app.git")
		if id1 == id2 {
			t.Errorf("expected different IDs for different URLs, both got %q", id1)
		}
	})
}

func TestExtractRepoName(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "SSH with path",
			url:      "git@github.com:team/my-app.git",
			expected: "my-app",
		},
		{
			name:     "HTTPS",
			url:      "https://github.com/team/my-app.git",
			expected: "my-app",
		},
		{
			name:     "SSH without .git",
			url:      "git@github.com:team/my-app",
			expected: "my-app",
		},
		{
			name:     "HTTPS without .git",
			url:      "https://github.com/team/my-app",
			expected: "my-app",
		},
		{
			name:     "simple name",
			url:      "git@github.com:org/simple.git",
			expected: "simple",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractRepoName(tc.url)
			if got != tc.expected {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", tc.url, got, tc.expected)
			}
		})
	}
}

func TestProjectDirName(t *testing.T) {
	got := ProjectDirName("my-app", "abc1234567")
	expected := "my-app--abc1234567"
	if got != expected {
		t.Errorf("ProjectDirName() = %q, want %q", got, expected)
	}
}
