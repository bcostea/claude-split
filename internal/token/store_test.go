package token

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func seed(t *testing.T, base, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, name, ".token"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFileStoreListAndDelete(t *testing.T) {
	base := t.TempDir()
	seed(t, base, "work")
	seed(t, base, "alt")
	s := FileStore{BaseDir: base}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"alt", "work"}) {
		t.Fatalf("got %v", got)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatalf("delete of missing should be nil, got %v", err)
	}
	if got, _ := s.List(); !reflect.DeepEqual(got, []string{"alt"}) {
		t.Fatalf("got %v", got)
	}
}

func TestNewForceFile(t *testing.T) {
	t.Setenv("CLAUDE_SPLIT_TOKEN_STORE", "file")
	if _, ok := New(t.TempDir()).(FileStore); !ok {
		t.Fatal("expected FileStore when forced")
	}
}

func TestParseKeychainDump(t *testing.T) {
	dump := []byte(`keychain: "/k.keychain-db"
class: "genp"
attributes:
    "acct"<blob>="work"
    "svce"<blob>="claude-split"
keychain: "/k.keychain-db"
class: "genp"
attributes:
    "acct"<blob>="user"
    "svce"<blob>="Claude Code-credentials-0a1b2c3d"
keychain: "/k.keychain-db"
class: "genp"
attributes:
    "acct"<blob>="alt"
    "svce"<blob>="claude-split"
`)
	if got := parseKeychainDump(dump); !reflect.DeepEqual(got, []string{"alt", "work"}) {
		t.Fatalf("got %v", got)
	}
}
