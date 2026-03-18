package repo

import (
	"errors"
	"os/exec"
)

// BackgroundPush starts a git push in the background.
// It returns immediately after starting the process.
// Returns an error if the repository has no remote configured.
func BackgroundPush(r *Repo) error {
	if !r.HasRemote() {
		return errors.New("no remote configured for repository")
	}

	cmd := exec.Command("git", "-C", r.Path, "push")
	if err := cmd.Start(); err != nil {
		return err
	}
	// Wait in a goroutine so we don't leave a zombie process.
	go func() { _ = cmd.Wait() }()
	return nil
}

// ForegroundPush performs a blocking git push.
func ForegroundPush(r *Repo) error {
	return r.Push()
}
