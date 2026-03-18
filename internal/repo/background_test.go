package repo

import (
	"testing"
)

func TestBackgroundPushNoRemote(t *testing.T) {
	r := setupTestRepo(t)

	err := BackgroundPush(r)
	if err == nil {
		t.Fatal("expected error from BackgroundPush when no remote configured, got nil")
	}
}
