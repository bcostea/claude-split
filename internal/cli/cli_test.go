package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bcostea/claude-split/internal/registry"
)

// withHome points $HOME at a temp dir and forces the file token store so tests
// never touch the real Keychain or real config.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_SPLIT_TOKEN_STORE", "file")
	return home
}

func TestCmdDefaultUnknownFails(t *testing.T) {
	withHome(t)
	if code := Run([]string{"--split-default", "ghost"}); code == 0 {
		t.Fatal("expected nonzero for unknown split")
	}
}

func TestCmdDefaultPersists(t *testing.T) {
	home := withHome(t)
	r, _ := registry.Load(filepath.Join(home, ".claude-splits"))
	_ = r.Add("work")
	_ = r.Save()

	if code := Run([]string{"--split-default", "work"}); code != 0 {
		t.Fatalf("got %d", code)
	}
	r2, _ := registry.Load(filepath.Join(home, ".claude-splits"))
	if r2.Default != "work" {
		t.Fatalf("default not persisted: %+v", r2)
	}
}

func TestCmdRemovePersists(t *testing.T) {
	home := withHome(t)
	t.Setenv("CLAUDE_SPLIT_ASSUME_YES", "1")
	base := filepath.Join(home, ".claude-splits")
	r, _ := registry.Load(base)
	_ = r.Add("work")
	_ = r.Save()

	if code := Run([]string{"--split-rm", "work"}); code != 0 {
		t.Fatalf("got %d", code)
	}
	r2, _ := registry.Load(base)
	if r2.Has("work") {
		t.Fatal("split not removed")
	}
}

func TestCmdWhichExplicit(t *testing.T) {
	withHome(t)
	if code := Run([]string{"--split-which", "--split", "default"}); code != 0 {
		t.Fatalf("got %d", code)
	}
}

func TestLaunchPromptsWhenAmbiguous(t *testing.T) {
	home := withHome(t)
	r, _ := registry.Load(filepath.Join(home, ".claude-splits"))
	_ = r.Add("work")
	_ = r.Save()
	// No default, no --split, splits exist → should refuse to launch.
	if code := Run([]string{}); code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	_ = os.Stdout
}
