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

func TestBuildEnvDefaultUnchanged(t *testing.T) {
	base := []string{"FOO=bar"}
	if got := BuildEnv(base, "", ""); !reflect.DeepEqual(got, base) {
		t.Fatalf("default env should be unchanged, got %v", got)
	}
}

func TestBuildEnvSplitSetsVarsAndStripsStale(t *testing.T) {
	base := []string{"FOO=bar", "CLAUDE_CONFIG_DIR=/old", "CLAUDE_CODE_OAUTH_TOKEN=old"}
	got := BuildEnv(base, "/splits/work", "tok123")
	assertHas(t, got, "FOO=bar")
	assertHas(t, got, "CLAUDE_CONFIG_DIR=/splits/work")
	assertHas(t, got, "CLAUDE_CODE_OAUTH_TOKEN=tok123")
	if count(got, "CLAUDE_CONFIG_DIR=") != 1 {
		t.Fatalf("stale CLAUDE_CONFIG_DIR not stripped: %v", got)
	}
}

func TestBuildEnvSplitWithoutToken(t *testing.T) {
	got := BuildEnv([]string{"FOO=bar"}, "/splits/work", "")
	assertHas(t, got, "CLAUDE_CONFIG_DIR=/splits/work")
	if count(got, "CLAUDE_CODE_OAUTH_TOKEN=") != 0 {
		t.Fatalf("should not set token when empty: %v", got)
	}
}

// helpers
func writeExec(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
func assertHas(t *testing.T, env []string, want string) {
	t.Helper()
	for _, e := range env {
		if e == want {
			return
		}
	}
	t.Fatalf("env missing %q: %v", want, env)
}
func count(env []string, prefix string) int {
	n := 0
	for _, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}
