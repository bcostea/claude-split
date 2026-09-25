package launcher

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindClaudeSkipsSelf(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "claude") // pretend we are installed as `claude`
	writeExec(t, self)
	other := filepath.Join(t.TempDir(), "claude")
	writeExec(t, other)

	// PATH lists self's dir first, then other's dir.
	pathEnv := filepath.Dir(self) + string(os.PathListSeparator) + filepath.Dir(other)
	got, err := FindClaude(pathEnv, self)
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Fatalf("expected %s, got %s", other, got)
	}
}

func TestFindClaudeNotFound(t *testing.T) {
	if _, err := FindClaude(t.TempDir(), "/nope/claude"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestBuildEnvDefaultStripsOnlySplitVars(t *testing.T) {
	base := []string{"FOO=bar", "ANTHROPIC_API_KEY=k", "CLAUDE_CONFIG_DIR=/splits/old", "CLAUDE_CODE_OAUTH_TOKEN=old"}
	got := BuildEnv(base, "")
	want := []string{"FOO=bar", "ANTHROPIC_API_KEY=k"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildEnvSplitSetsDirAndStripsAuth(t *testing.T) {
	base := []string{
		"FOO=bar", "CLAUDE_CONFIG_DIR=/old", "CLAUDE_CODE_OAUTH_TOKEN=old",
		"CLAUDE_CODE_OAUTH_REFRESH_TOKEN=r", "ANTHROPIC_API_KEY=k", "ANTHROPIC_AUTH_TOKEN=a",
	}
	got := BuildEnv(base, "/splits/work")
	want := []string{"FOO=bar", "CLAUDE_CONFIG_DIR=/splits/work"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// helpers
func writeExec(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
