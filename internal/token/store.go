// Package token manages the legacy per-split tokens that older claude-split
// versions minted with `claude setup-token`. Those tokens only have the
// inference scope, so they break features that need the profile scope (for
// example remote managed settings). Splits now use Claude Code's own login;
// this package only finds and deletes the old tokens.
package token

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Store lists and deletes legacy tokens, one per split name.
type Store interface {
	List() ([]string, error)
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

// FileStore keeps a token at <BaseDir>/<name>/.token.
type FileStore struct{ BaseDir string }

func (f FileStore) path(name string) string {
	return filepath.Join(f.BaseDir, name, ".token")
}

func (f FileStore) List() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(f.BaseDir, "*", ".token"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(filepath.Dir(m)))
	}
	sort.Strings(names)
	return names, nil
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

// List reads item attributes only; `dump-keychain` without -d never prints
// secrets.
func (KeychainStore) List() ([]string, error) {
	out, err := exec.Command("security", "dump-keychain").Output()
	if err != nil {
		return nil, err
	}
	return parseKeychainDump(out), nil
}

var (
	acctRe = regexp.MustCompile(`^\s*"acct"<blob>="(.*)"\s*$`)
	svceRe = regexp.MustCompile(`^\s*"svce"<blob>="(.*)"\s*$`)
)

// parseKeychainDump returns the account names of all items whose service is
// claude-split.
func parseKeychainDump(dump []byte) []string {
	var names []string
	var acct, svce string
	flush := func() {
		if svce == keychainService && acct != "" {
			names = append(names, acct)
		}
		acct, svce = "", ""
	}
	sc := bufio.NewScanner(bytes.NewReader(dump))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "keychain: ") {
			flush()
			continue
		}
		if m := acctRe.FindStringSubmatch(line); m != nil {
			acct = m[1]
		} else if m := svceRe.FindStringSubmatch(line); m != nil {
			svce = m[1]
		}
	}
	flush()
	sort.Strings(names)
	return names
}

func (KeychainStore) Delete(name string) error {
	// Ignore "item not found" so delete is idempotent.
	_ = exec.Command("security", "delete-generic-password",
		"-a", name, "-s", keychainService).Run()
	return nil
}
