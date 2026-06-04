package token

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Store persists one auth token per split name.
type Store interface {
	Save(name, token string) error
	Load(name string) (string, error)
	Delete(name string) error
}

// New picks the platform-appropriate store. macOS uses the Keychain unless
// CLAUDE_SPLIT_TOKEN_STORE=file forces the file backend.
func New(baseDir string) Store {
	if os.Getenv("CLAUDE_SPLIT_TOKEN_STORE") == "file" {
		return FileStore{BaseDir: baseDir}
	}
	if runtime.GOOS == "darwin" {
		return KeychainStore{}
	}
	return FileStore{BaseDir: baseDir}
}

// FileStore writes <BaseDir>/<name>/.token with 0600 perms.
type FileStore struct{ BaseDir string }

func (f FileStore) path(name string) string {
	return filepath.Join(f.BaseDir, name, ".token")
}

func (f FileStore) Save(name, token string) error {
	p := f.path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(token), 0o600)
}

func (f FileStore) Load(name string) (string, error) {
	b, err := os.ReadFile(f.path(name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (f FileStore) Delete(name string) error {
	err := os.Remove(f.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

const keychainService = "claude-split"

// KeychainStore uses the macOS `security` CLI. Each split is a distinct
// generic-password item (service=claude-split, account=<name>).
type KeychainStore struct{}

func (KeychainStore) Save(name, token string) error {
	// -U updates the item if it already exists.
	return exec.Command("security", "add-generic-password",
		"-U", "-a", name, "-s", keychainService, "-w", token).Run()
}

func (KeychainStore) Load(name string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-a", name, "-s", keychainService, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (KeychainStore) Delete(name string) error {
	// Ignore "item not found" so delete is idempotent.
	_ = exec.Command("security", "delete-generic-password",
		"-a", name, "-s", keychainService).Run()
	return nil
}
