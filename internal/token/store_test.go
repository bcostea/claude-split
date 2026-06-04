package token

import "testing"

func TestFileStoreRoundTrip(t *testing.T) {
	s := FileStore{BaseDir: t.TempDir()}
	if err := s.Save("work", "sk-ant-oat-abc"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("work")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-ant-oat-abc" {
		t.Fatalf("got %q", got)
	}
}

func TestFileStoreLoadMissing(t *testing.T) {
	s := FileStore{BaseDir: t.TempDir()}
	if _, err := s.Load("nope"); err == nil {
		t.Fatal("expected error loading missing token")
	}
}

func TestFileStoreDeleteIdempotent(t *testing.T) {
	s := FileStore{BaseDir: t.TempDir()}
	_ = s.Save("work", "x")
	if err := s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatalf("delete of missing should be nil, got %v", err)
	}
}

func TestNewForceFile(t *testing.T) {
	t.Setenv("CLAUDE_SPLIT_TOKEN_STORE", "file")
	if _, ok := New(t.TempDir()).(FileStore); !ok {
		t.Fatal("expected FileStore when forced")
	}
}
