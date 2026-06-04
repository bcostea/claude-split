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
	cmd := newCmd(bin, home, claudeDir, outFile, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func newCmd(bin, home, claudeDir, outFile string, args ...string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"CLAUDE_SPLIT_TOKEN_STORE=file",
		"PATH="+claudeDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CLAUDE_TEST_OUT="+outFile,
	)
	return cmd
}

func TestBareLaunchFromHomeUsesDefault(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir()
	base := filepath.Join(home, ".claude-splits")
	// A split exists but no default is configured. Bare launch from home must
	// run the default profile (no CLAUDE_CONFIG_DIR), not prompt.
	if err := os.MkdirAll(filepath.Join(base, "xogito"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "registry.json"),
		[]byte(`{"splits":["xogito"],"default":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "xogito", ".token"),
		[]byte("sk-ant-oat-X"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := newCmd(bin, home, claudeDir, out, "-p", "hi")
	cmd.Dir = home
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected default launch to succeed, got %v", err)
	}
	s := string(mustRead(t, out))
	if !strings.Contains(s, "ARGS:-p hi") {
		t.Fatalf("passthrough wrong: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CONFIG_DIR=\n") {
		t.Fatalf("home default should not set CLAUDE_CONFIG_DIR: %q", s)
	}
}

func TestExplicitSplitFromHomeIsRefused(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir()
	base := filepath.Join(home, ".claude-splits")
	if err := os.MkdirAll(filepath.Join(base, "work"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "registry.json"),
		[]byte(`{"splits":["work"],"default":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "work", ".token"),
		[]byte("sk-ant-oat-T"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := newCmd(bin, home, claudeDir, out, "--split", "work", "-p", "hi")
	cmd.Dir = home // run FROM the home directory
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when launching a split from home")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("fake claude was invoked; the split should have been refused")
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

// seedSplit creates a registry with a single split + token in the given home.
func seedSplit(t *testing.T, home, name, def string) string {
	t.Helper()
	base := filepath.Join(home, ".claude-splits")
	if err := os.MkdirAll(filepath.Join(base, name), 0o700); err != nil {
		t.Fatal(err)
	}
	reg := `{"splits":["` + name + `"],"default":"` + def + `"}`
	if err := os.WriteFile(filepath.Join(base, "registry.json"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, name, ".token"), []byte("sk-ant-oat-"+name), 0o600); err != nil {
		t.Fatal(err)
	}
	return base
}

func TestFolderMemoryAutoLoadsRememberedSplit(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir()
	base := seedSplit(t, home, "work", "") // no global default
	proj := t.TempDir()

	// 1) Explicit launch in the project records folder -> work.
	out1 := filepath.Join(t.TempDir(), "out1.txt")
	c1 := newCmd(bin, home, claudeDir, out1, "--split", "work", "-p", "one")
	c1.Dir = proj
	c1.Stderr = os.Stderr
	if err := c1.Run(); err != nil {
		t.Fatalf("explicit launch failed: %v", err)
	}

	// 2) Bare launch in the same project must auto-load work.
	out2 := filepath.Join(t.TempDir(), "out2.txt")
	c2 := newCmd(bin, home, claudeDir, out2, "-p", "two")
	c2.Dir = proj
	c2.Stderr = os.Stderr
	if err := c2.Run(); err != nil {
		t.Fatalf("bare launch failed: %v", err)
	}
	s := string(mustRead(t, out2))
	if !strings.Contains(s, "ARGS:-p two") {
		t.Fatalf("passthrough wrong: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CONFIG_DIR="+filepath.Join(base, "work")) {
		t.Fatalf("folder memory did not auto-load split: %q", s)
	}
	if !strings.Contains(s, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat-work") {
		t.Fatalf("token not injected: %q", s)
	}
}

func TestFolderMemoryStaleEntryFallsBackAndPrunes(t *testing.T) {
	bin := buildBinary(t)
	claudeDir := fakeClaudeDir(t)
	home := t.TempDir()
	base := seedSplit(t, home, "work", "work") // global default = work
	proj := t.TempDir()

	// Pre-seed folders.json pointing this project at a now-deleted split.
	fpath := filepath.Join(home, ".config", "claude-split", "folders.json")
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		t.Fatal(err)
	}
	canon, _ := filepath.EvalSymlinks(proj)
	if err := os.WriteFile(fpath, []byte(`{"`+canon+`":"ghost"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out.txt")
	c := newCmd(bin, home, claudeDir, out, "-p", "hi")
	c.Dir = proj
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		t.Fatalf("launch failed: %v", err)
	}
	// Fell back to the global default split.
	s := string(mustRead(t, out))
	if !strings.Contains(s, "CLAUDE_CONFIG_DIR="+filepath.Join(base, "work")) {
		t.Fatalf("expected fallback to global default work: %q", s)
	}
	// Stale ghost entry pruned.
	if data, _ := os.ReadFile(fpath); strings.Contains(string(data), "ghost") {
		t.Fatalf("stale entry not pruned: %s", data)
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
