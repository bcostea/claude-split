package folders

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "folders.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("/anything"); ok {
		t.Fatal("expected empty store")
	}
}

func TestSetSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "folders.json")
	s, _ := Load(path)
	s.Set("/Users/bogdan/code/xogito", "xogito")
	s.Set("/Users/bogdan/code/foo", "default")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	s2, _ := Load(path)
	if v, ok := s2.Get("/Users/bogdan/code/xogito"); !ok || v != "xogito" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
	if v, ok := s2.Get("/Users/bogdan/code/foo"); !ok || v != "default" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
}

func TestDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "folders.json")
	s, _ := Load(path)
	s.Set("/p", "work")
	s.Delete("/p")
	if _, ok := s.Get("/p"); ok {
		t.Fatal("expected entry deleted")
	}
}

func TestDefaultPathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if got := DefaultPath(); got != "/tmp/xdg/claude-split/folders.json" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "claude-split", "folders.json")
	if got := DefaultPath(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
