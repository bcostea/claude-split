package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles claude-split once into a temp dir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude-split")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/bcostea/claude-split")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	return bin
}

// fakeClaudeDir installs the fake claude script into a fresh dir and returns it.
func fakeClaudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "fake-claude.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "claude")
	if err := os.WriteFile(dst, src, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func run(t *testing.T, bin, home, claudeDir, outFile string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_SPLIT_TOKEN_STORE=file",
		"PATH="+claudeDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CLAUDE_TEST_OUT="+outFile,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestLaunchSplitPassesEnvAndArgs(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir()
	base := filepath.Join(home, ".claude-splits")

	// Seed registry + token using the same on-disk layout the binary expects.
	if err := os.MkdirAll(filepath.Join(base, "work"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "registry.json"),
		[]byte(`{"splits":["work"],"default":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "work", ".token"),
		[]byte("sk-ant-oat-TESTTOKEN"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out.txt")
	run(t, bin, home, claudeDir, out, "--split", "work", "-p", "hello")

	got, _ := os.ReadFile(out)
	s := string(got)
	if !strings.Contains(s, "ARGS:-p hello") {
		t.Fatalf("passthrough wrong: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CONFIG_DIR="+filepath.Join(base, "work")) {
		t.Fatalf("config dir wrong: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat-TESTTOKEN") {
		t.Fatalf("token wrong: %q", s)
	}
}

func TestLaunchHomeProfileSetsNoSplitVars(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir() // no registry → no splits → home profile

	out := filepath.Join(t.TempDir(), "out.txt")
	run(t, bin, home, claudeDir, out, "-p", "hi")

	s := string(mustRead(t, out))
	if !strings.Contains(s, "ARGS:-p hi") {
		t.Fatalf("passthrough wrong: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CONFIG_DIR=\n") {
		t.Fatalf("home profile should not set CLAUDE_CONFIG_DIR: %q", s)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
