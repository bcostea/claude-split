package cli

import (
	"path/filepath"
	"testing"

	"github.com/bcostea/claude-split/internal/folders"
	"github.com/bcostea/claude-split/internal/registry"
)

func newStore(t *testing.T) *folders.Store {
	t.Helper()
	s, err := folders.Load(filepath.Join(t.TempDir(), "folders.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEffectiveDefaultNoMemoryUsesGlobal(t *testing.T) {
	reg := &registry.Registry{Splits: []string{"work"}, Default: "work"}
	def, pruned := effectiveDefault(newStore(t), reg, "/some/folder")
	if def != "work" || pruned {
		t.Fatalf("def=%q pruned=%v", def, pruned)
	}
}

func TestEffectiveDefaultFolderOverridesGlobal(t *testing.T) {
	reg := &registry.Registry{Splits: []string{"work", "xogito"}, Default: "work"}
	s := newStore(t)
	s.Set("/code/xogito", "xogito")
	def, pruned := effectiveDefault(s, reg, "/code/xogito")
	if def != "xogito" || pruned {
		t.Fatalf("def=%q pruned=%v", def, pruned)
	}
}

func TestEffectiveDefaultFolderDefaultKeyword(t *testing.T) {
	reg := &registry.Registry{Splits: []string{"work"}, Default: "work"}
	s := newStore(t)
	s.Set("/code/foo", "default")
	def, _ := effectiveDefault(s, reg, "/code/foo")
	if def != "default" {
		t.Fatalf("def=%q", def)
	}
}

func TestEffectiveDefaultStaleEntryPrunedFallsBackToGlobal(t *testing.T) {
	reg := &registry.Registry{Splits: []string{"work"}, Default: "work"}
	s := newStore(t)
	s.Set("/code/gone", "ghost") // ghost no longer exists in the registry
	def, pruned := effectiveDefault(s, reg, "/code/gone")
	if def != "work" || !pruned {
		t.Fatalf("expected fallback+prune, def=%q pruned=%v", def, pruned)
	}
	if _, ok := s.Get("/code/gone"); ok {
		t.Fatal("stale entry should have been deleted")
	}
}
