package frontend

import "testing"

func TestFrontendObjectPath(t *testing.T) {
	got := frontendObjectPath("acct1", "agent1", "backups", "file.zip")
	want := "acct1/agent1/backups/file.zip"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
