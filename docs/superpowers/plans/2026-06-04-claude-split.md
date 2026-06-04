# claude-split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A thin Go launcher that wraps the real `claude` CLI to manage multiple isolated Claude Code profiles ("splits"), selecting one and `exec`ing `claude` with the right env while passing all other args through.

**Architecture:** Pure, independently-testable packages (`args`, `registry`, `resolve`, `token`, `launcher`) wired by a thin `cli` dispatcher and a one-line `main`. A split is a directory under `~/.claude-splits/<name>/` selected via `CLAUDE_CONFIG_DIR`; auth is isolated per-split via a long-lived token injected as `CLAUDE_CODE_OAUTH_TOKEN`. The home profile (`~/.claude.json`, `~/.claude/`) is the implicit `default` and is never touched.

**Tech Stack:** Go 1.22 (stdlib only), `syscall.Exec`, macOS `security` CLI for Keychain, `go test`.

**Conventions:** Module path is `claude-split` (local) until first release, when it becomes the GitHub path. Errors print to stderr prefixed `claude-split:`. Exit code 2 = usage error, 1 = runtime error, 0 = success.

---

### Task 1: Scaffold the Go module

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing test**

`internal/cli/cli_test.go`:
```go
package cli

import "testing"

func TestRunNoArgsReturnsInt(t *testing.T) {
	// With no splits and no args, Run should resolve to launching the default
	// profile. There is no real `claude` on PATH in CI, so it returns 1, not a panic.
	code := Run([]string{})
	if code != 1 {
		t.Fatalf("expected exit code 1 (no claude on PATH), got %d", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL — `go.mod` / package does not exist yet (compile error).

- [ ] **Step 3: Create the module and minimal implementation**

`go.mod`:
```
module claude-split

go 1.22
```

`internal/cli/cli.go`:
```go
package cli

// Run is the entrypoint. It returns a process exit code.
// Fleshed out in later tasks; for now it always reports "no claude found".
func Run(argv []string) int {
	return 1
}
```

`main.go`:
```go
package main

import (
	"os"

	"claude-split/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...` then `go build ./...`
Expected: PASS, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add go.mod main.go internal/cli/
git commit -m "chore: scaffold claude-split go module"
```

---

### Task 2: Argument partitioning (`args` package)

Splits `argv` into wrapper-owned `--split*` flags and everything else (passthrough). Supports both `--split work` and `--split=work` forms.

**Files:**
- Create: `internal/args/args.go`
- Test: `internal/args/args_test.go`

- [ ] **Step 1: Write the failing test**

`internal/args/args_test.go`:
```go
package args

import (
	"reflect"
	"testing"
)

func TestParsePassthroughOnly(t *testing.T) {
	p, err := Parse([]string{"--dangerously-skip-permissions", "-p", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SplitSet || p.List || p.NewSet {
		t.Fatalf("no wrapper flags expected, got %+v", p)
	}
	want := []string{"--dangerously-skip-permissions", "-p", "hi"}
	if !reflect.DeepEqual(p.Passthrough, want) {
		t.Fatalf("passthrough = %v, want %v", p.Passthrough, want)
	}
}

func TestParseSplitSpaceForm(t *testing.T) {
	p, err := Parse([]string{"--split", "work", "-p", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.SplitSet || p.Split != "work" {
		t.Fatalf("Split=%q set=%v", p.Split, p.SplitSet)
	}
	if !reflect.DeepEqual(p.Passthrough, []string{"-p", "hi"}) {
		t.Fatalf("passthrough = %v", p.Passthrough)
	}
}

func TestParseSplitEqualsForm(t *testing.T) {
	p, _ := Parse([]string{"--split=work"})
	if !p.SplitSet || p.Split != "work" {
		t.Fatalf("Split=%q set=%v", p.Split, p.SplitSet)
	}
}

func TestParseBoolFlags(t *testing.T) {
	p, _ := Parse([]string{"--split-list"})
	if !p.List {
		t.Fatal("expected List=true")
	}
	if _, err := Parse([]string{"--split-list=x"}); err == nil {
		t.Fatal("expected error for value on bool flag")
	}
}

func TestParseValueFlagMissingValue(t *testing.T) {
	if _, err := Parse([]string{"--split"}); err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestParseAllValueFlags(t *testing.T) {
	p, _ := Parse([]string{"--split-new", "a"})
	if !p.NewSet || p.New != "a" {
		t.Fatalf("New=%q set=%v", p.New, p.NewSet)
	}
	p, _ = Parse([]string{"--split-default", "b"})
	if !p.DefaultSet || p.Default != "b" {
		t.Fatalf("Default=%q set=%v", p.Default, p.DefaultSet)
	}
	p, _ = Parse([]string{"--split-rm", "c"})
	if !p.RmSet || p.Rm != "c" {
		t.Fatalf("Rm=%q set=%v", p.Rm, p.RmSet)
	}
	p, _ = Parse([]string{"--split-which"})
	if !p.Which {
		t.Fatal("expected Which=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/args/`
Expected: FAIL — package `args` does not exist.

- [ ] **Step 3: Write the implementation**

`internal/args/args.go`:
```go
package args

import (
	"fmt"
	"strings"
)

// Parsed holds the wrapper-owned flags plus everything to forward to claude.
type Parsed struct {
	Split      string
	SplitSet   bool
	List       bool
	New        string
	NewSet     bool
	Default    string
	DefaultSet bool
	Rm         string
	RmSet      bool
	Which      bool

	Passthrough []string
}

var valueFlags = map[string]bool{
	"--split":         true,
	"--split-new":     true,
	"--split-default": true,
	"--split-rm":      true,
}

var boolFlags = map[string]bool{
	"--split-list":  true,
	"--split-which": true,
}

// Parse partitions argv. Any argument that is not a recognized wrapper flag
// (and not the value consumed by one) is appended verbatim to Passthrough.
func Parse(argv []string) (Parsed, error) {
	var p Parsed
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, inlineVal, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--split") {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name, inlineVal, hasInline = arg[:eq], arg[eq+1:], true
			}
		}

		switch {
		case valueFlags[name]:
			val := inlineVal
			if !hasInline {
				if i+1 >= len(argv) {
					return p, fmt.Errorf("flag %s requires a value", name)
				}
				i++
				val = argv[i]
			}
			switch name {
			case "--split":
				p.Split, p.SplitSet = val, true
			case "--split-new":
				p.New, p.NewSet = val, true
			case "--split-default":
				p.Default, p.DefaultSet = val, true
			case "--split-rm":
				p.Rm, p.RmSet = val, true
			}
		case boolFlags[name]:
			if hasInline {
				return p, fmt.Errorf("flag %s does not take a value", name)
			}
			switch name {
			case "--split-list":
				p.List = true
			case "--split-which":
				p.Which = true
			}
		default:
			p.Passthrough = append(p.Passthrough, arg)
		}
	}
	return p, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/args/ -v`
Expected: PASS (all cases).

- [ ] **Step 5: Commit**

```bash
git add internal/args/
git commit -m "feat: argument partitioning for wrapper flags vs passthrough"
```

---

### Task 3: Split registry (`registry` package)

JSON-backed list of splits + the auto-default, stored at `<baseDir>/registry.json`.

**Files:**
- Create: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: Write the failing test**

`internal/registry/registry_test.go`:
```go
package registry

import "testing"

func TestLoadMissingReturnsEmpty(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Splits) != 0 || r.Default != "" {
		t.Fatalf("expected empty registry, got %+v", r)
	}
}

func TestAddSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r, _ := Load(dir)
	if err := r.Add("work"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault("work"); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}

	r2, _ := Load(dir)
	if !r2.Has("work") || r2.Default != "work" {
		t.Fatalf("round-trip lost data: %+v", r2)
	}
}

func TestAddDuplicateFails(t *testing.T) {
	r, _ := Load(t.TempDir())
	_ = r.Add("work")
	if err := r.Add("work"); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRemoveClearsDefaultAndRefusesDefault(t *testing.T) {
	r, _ := Load(t.TempDir())
	_ = r.Add("work")
	_ = r.SetDefault("work")
	if err := r.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if r.Has("work") || r.Default != "" {
		t.Fatalf("remove failed: %+v", r)
	}
	if err := r.Remove("default"); err == nil {
		t.Fatal("expected refusal to remove 'default'")
	}
}

func TestSetDefaultUnknownFails(t *testing.T) {
	r, _ := Load(t.TempDir())
	if err := r.SetDefault("nope"); err == nil {
		t.Fatal("expected error for unknown split")
	}
	if err := r.SetDefault("default"); err != nil {
		t.Fatal("'default' should always be allowed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/registry/registry.go`:
```go
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Registry is the persisted set of splits and the configured auto-default.
// "default" is a reserved name meaning the home profile; it is never listed
// in Splits.
type Registry struct {
	Splits  []string `json:"splits"`
	Default string   `json:"default"`

	path string
}

// Load reads registry.json from baseDir. A missing file yields an empty,
// saveable registry (no error).
func Load(baseDir string) (*Registry, error) {
	r := &Registry{path: filepath.Join(baseDir, "registry.json")}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", r.path, err)
	}
	return r, nil
}

func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

func (r *Registry) Has(name string) bool {
	for _, s := range r.Splits {
		if s == name {
			return true
		}
	}
	return false
}

func (r *Registry) Add(name string) error {
	if r.Has(name) {
		return fmt.Errorf("split %q already exists", name)
	}
	r.Splits = append(r.Splits, name)
	return nil
}

func (r *Registry) Remove(name string) error {
	if name == "default" {
		return errors.New("cannot remove the default (home) profile")
	}
	if !r.Has(name) {
		return fmt.Errorf("split %q does not exist", name)
	}
	out := make([]string, 0, len(r.Splits)-1)
	for _, s := range r.Splits {
		if s != name {
			out = append(out, s)
		}
	}
	r.Splits = out
	if r.Default == name {
		r.Default = ""
	}
	return nil
}

func (r *Registry) SetDefault(name string) error {
	if name != "default" && !r.Has(name) {
		return fmt.Errorf("split %q does not exist", name)
	}
	r.Default = name
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/registry/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat: JSON-backed split registry with default management"
```

---

### Task 4: Split resolution (`resolve` package)

Decides what to do given the explicit flag, configured default, and known splits.

**Files:**
- Create: `internal/resolve/resolve.go`
- Test: `internal/resolve/resolve_test.go`

- [ ] **Step 1: Write the failing test**

`internal/resolve/resolve_test.go`:
```go
package resolve

import "testing"

func TestExplicitDefaultKeyword(t *testing.T) {
	d := Resolve("default", true, "work", []string{"work"})
	if d.Action != LaunchDefault {
		t.Fatalf("got %v", d.Action)
	}
}

func TestExplicitNamed(t *testing.T) {
	d := Resolve("work", true, "", []string{"work"})
	if d.Action != LaunchSplit || d.Split != "work" {
		t.Fatalf("got %+v", d)
	}
}

func TestFallsBackToConfiguredDefault(t *testing.T) {
	d := Resolve("", false, "work", []string{"work"})
	if d.Action != LaunchSplit || d.Split != "work" {
		t.Fatalf("got %+v", d)
	}
	d = Resolve("", false, "default", []string{"work"})
	if d.Action != LaunchDefault {
		t.Fatalf("got %+v", d)
	}
}

func TestNoDefaultWithSplitsPromptsList(t *testing.T) {
	d := Resolve("", false, "", []string{"work"})
	if d.Action != PrintListExit {
		t.Fatalf("got %v", d.Action)
	}
}

func TestNoDefaultNoSplitsUsesHome(t *testing.T) {
	d := Resolve("", false, "", nil)
	if d.Action != LaunchDefault {
		t.Fatalf("got %v", d.Action)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resolve/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/resolve/resolve.go`:
```go
package resolve

// Action is what the launcher should do after resolution.
type Action int

const (
	LaunchDefault Action = iota // home profile, no CLAUDE_CONFIG_DIR
	LaunchSplit                 // a named split
	PrintListExit               // ambiguous — show the list and exit
)

type Decision struct {
	Action Action
	Split  string // set only when Action == LaunchSplit
}

// Resolve applies the order: explicit flag > configured default > prompt.
// When there are no splits at all and nothing chosen, the home profile is the
// only option, so launch it directly.
func Resolve(explicit string, explicitSet bool, configuredDefault string, splits []string) Decision {
	if explicitSet {
		if explicit == "default" {
			return Decision{Action: LaunchDefault}
		}
		return Decision{Action: LaunchSplit, Split: explicit}
	}
	if configuredDefault != "" {
		if configuredDefault == "default" {
			return Decision{Action: LaunchDefault}
		}
		return Decision{Action: LaunchSplit, Split: configuredDefault}
	}
	if len(splits) == 0 {
		return Decision{Action: LaunchDefault}
	}
	return Decision{Action: PrintListExit}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/resolve/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resolve/
git commit -m "feat: split resolution order (explicit > default > prompt)"
```

---

### Task 5: Token store (`token` package — storage)

`Store` interface with a file backend (all platforms) and a macOS Keychain backend. A `CLAUDE_SPLIT_TOKEN_STORE=file` env var forces the file backend (used by tests and by users who prefer it on macOS).

**Files:**
- Create: `internal/token/store.go`
- Test: `internal/token/store_test.go`

- [ ] **Step 1: Write the failing test**

`internal/token/store_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/token/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/token/store.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/token/ -v`
Expected: PASS (Keychain backend is not exercised by these tests).

- [ ] **Step 5: Commit**

```bash
git add internal/token/store.go internal/token/store_test.go
git commit -m "feat: token store (file + macOS keychain backends)"
```

---

### Task 6: Token minting (`token` package — acquisition)

Runs `claude setup-token` with `CLAUDE_CONFIG_DIR` pointed at the split dir, parses the long-lived token from stdout, and falls back to prompting the user to paste it. Parsing is the testable unit; the minter itself is behind an interface.

**Files:**
- Create: `internal/token/mint.go`
- Test: `internal/token/mint_test.go`

- [ ] **Step 1: Write the failing test**

`internal/token/mint_test.go`:
```go
package token

import "testing"

func TestParseTokenFound(t *testing.T) {
	out := "Visit the URL...\nYour token:\nsk-ant-oat01-AbC_123-xyz\nDone.\n"
	if got := ParseToken(out); got != "sk-ant-oat01-AbC_123-xyz" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTokenAbsent(t *testing.T) {
	if got := ParseToken("no token here"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/token/ -run ParseToken`
Expected: FAIL — `ParseToken` undefined.

- [ ] **Step 3: Write the implementation**

`internal/token/mint.go`:
```go
package token

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Minter obtains a fresh long-lived OAuth token for a split's config dir.
type Minter interface {
	Mint(configDir string) (string, error)
}

var tokenRe = regexp.MustCompile(`sk-ant-oat[0-9A-Za-z_-]+`)

// ParseToken extracts the first long-lived OAuth token from text, or "".
func ParseToken(s string) string {
	return tokenRe.FindString(s)
}

// ClaudeMinter shells out to `claude setup-token`.
type ClaudeMinter struct {
	ClaudePath string

	// In/Out/Err default to the process std streams; overridable for tests.
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

func (m ClaudeMinter) Mint(configDir string) (string, error) {
	in := m.In
	if in == nil {
		in = os.Stdin
	}
	out := m.Out
	if out == nil {
		out = os.Stdout
	}
	errw := m.Err
	if errw == nil {
		errw = os.Stderr
	}

	cmd := exec.Command(m.ClaudePath, "setup-token")
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+configDir)
	cmd.Stdin = in
	cmd.Stderr = errw
	// Capture stdout while still echoing it so the user sees the flow.
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(out, &buf)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running `claude setup-token`: %w", err)
	}

	if tok := ParseToken(buf.String()); tok != "" {
		return tok, nil
	}

	// Fallback: ask the user to paste it.
	fmt.Fprint(errw, "Could not auto-detect the token. Paste it here: ")
	sc := bufio.NewScanner(in)
	if sc.Scan() {
		if tok := ParseToken(sc.Text()); tok != "" {
			return tok, nil
		}
		if t := strings.TrimSpace(sc.Text()); t != "" {
			return t, nil
		}
	}
	return "", errors.New("no token obtained")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/token/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/token/mint.go internal/token/mint_test.go
git commit -m "feat: token minting via claude setup-token with paste fallback"
```

---

### Task 7: Launcher core (`launcher` package)

Locate the real `claude` on `$PATH` (skipping ourselves), build the env for a split, and `exec`.

**Files:**
- Create: `internal/launcher/launcher.go`
- Test: `internal/launcher/launcher_test.go`

- [ ] **Step 1: Write the failing test**

`internal/launcher/launcher_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/launcher/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/launcher/launcher.go`:
```go
package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// FindClaude returns the path to the real `claude` on pathEnv, skipping the
// entry that resolves to selfPath (so a wrapper installed as `claude` does not
// re-invoke itself).
func FindClaude(pathEnv, selfPath string) (string, error) {
	self, _ := filepath.Abs(selfPath)
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, "claude")
		info, err := os.Stat(cand)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue // not executable
		}
		if abs, _ := filepath.Abs(cand); abs == self {
			continue
		}
		return cand, nil
	}
	return "", errors.New("real `claude` binary not found on PATH")
}

// BuildEnv returns the environment to launch with. For the home profile pass
// configDir == "" (env returned unchanged). For a split, stale copies of the
// managed vars are stripped and the new values appended.
func BuildEnv(base []string, configDir, token string) []string {
	if configDir == "" {
		return base
	}
	managed := []string{"CLAUDE_CONFIG_DIR=", "CLAUDE_CODE_OAUTH_TOKEN="}
	out := make([]string, 0, len(base)+2)
	for _, e := range base {
		if hasAnyPrefix(e, managed) {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
	if token != "" {
		out = append(out, "CLAUDE_CODE_OAUTH_TOKEN="+token)
	}
	return out
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// Exec replaces the current process with claude. It only returns on error.
func Exec(claudePath string, args, env []string) error {
	argv := append([]string{claudePath}, args...)
	return syscall.Exec(claudePath, argv, env)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/launcher/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/launcher/
git commit -m "feat: launcher core (find real claude, build env, exec)"
```

---

### Task 8: CLI dispatch (`cli` package)

Wire the packages into the commands. Replaces the stub `Run` from Task 1.

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Replace the contents of `internal/cli/cli_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"claude-split/internal/registry"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL — `--split-default` etc. are no-ops in the stub `Run`.

- [ ] **Step 3: Write the implementation**

Replace `internal/cli/cli.go`:
```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"claude-split/internal/args"
	"claude-split/internal/launcher"
	"claude-split/internal/registry"
	"claude-split/internal/resolve"
	"claude-split/internal/token"
)

func Run(argv []string) int {
	p, err := args.Parse(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 2
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	baseDir := filepath.Join(home, ".claude-splits")
	reg, err := registry.Load(baseDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	store := token.New(baseDir)

	switch {
	case p.List:
		return cmdList(reg, store)
	case p.NewSet:
		return cmdNew(reg, store, baseDir, p.New)
	case p.DefaultSet:
		return cmdDefault(reg, p.Default)
	case p.RmSet:
		return cmdRemove(reg, store, p.Rm)
	case p.Which:
		return cmdWhich(reg, p)
	default:
		return cmdLaunch(reg, store, baseDir, p)
	}
}

func cmdList(reg *registry.Registry, store token.Store) int {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SPLIT\tDEFAULT\tTOKEN")
	marker := func(name string) string {
		if reg.Default == name {
			return "*"
		}
		return ""
	}
	fmt.Fprintf(w, "default (home)\t%s\tn/a\n", marker("default"))
	for _, s := range reg.Splits {
		tokState := "missing"
		if _, err := store.Load(s); err == nil {
			tokState = "ok"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s, marker(s), tokState)
	}
	w.Flush()
	return 0
}

func cmdDefault(reg *registry.Registry, name string) int {
	if err := reg.SetDefault(name); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	if err := reg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	fmt.Printf("Default split set to %q.\n", name)
	return 0
}

func cmdRemove(reg *registry.Registry, store token.Store, name string) int {
	if !reg.Has(name) {
		fmt.Fprintf(os.Stderr, "claude-split: split %q does not exist\n", name)
		return 1
	}
	if !confirm(fmt.Sprintf("Remove split %q and its stored token? [y/N] ", name)) {
		fmt.Println("Aborted.")
		return 0
	}
	if err := reg.Remove(name); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	_ = store.Delete(name)
	if err := reg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	fmt.Printf("Removed split %q. (Its directory under ~/.claude-splits is left in place.)\n", name)
	return 0
}

func cmdWhich(reg *registry.Registry, p args.Parsed) int {
	d := resolve.Resolve(p.Split, p.SplitSet, reg.Default, reg.Splits)
	switch d.Action {
	case resolve.LaunchDefault:
		fmt.Println("default (home profile)")
	case resolve.LaunchSplit:
		fmt.Println(d.Split)
	case resolve.PrintListExit:
		fmt.Println("(none — would prompt to choose)")
	}
	return 0
}

func cmdNew(reg *registry.Registry, store token.Store, baseDir, name string) int {
	if name == "default" {
		fmt.Fprintln(os.Stderr, "claude-split: 'default' is reserved for the home profile")
		return 1
	}
	if reg.Has(name) {
		fmt.Fprintf(os.Stderr, "claude-split: split %q already exists\n", name)
		return 1
	}
	configDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	self, _ := os.Executable()
	claudePath, err := launcher.FindClaude(os.Getenv("PATH"), self)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Authenticating split %q — complete the login when prompted...\n", name)
	tok, err := token.ClaudeMinter{ClaudePath: claudePath}.Mint(configDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	if err := store.Save(name, tok); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	_ = reg.Add(name)
	if err := reg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	fmt.Printf("Created split %q.\n", name)
	if reg.Default == "" {
		fmt.Printf("Tip: make it the default with: claude-split --split-default %s\n", name)
	}
	return 0
}

func cmdLaunch(reg *registry.Registry, store token.Store, baseDir string, p args.Parsed) int {
	d := resolve.Resolve(p.Split, p.SplitSet, reg.Default, reg.Splits)

	if d.Action == resolve.PrintListExit {
		fmt.Fprintln(os.Stderr, "No split selected. Available:")
		cmdList(reg, store)
		fmt.Fprintln(os.Stderr, "\nRe-run with --split <name> ('default' = home profile), or set one: --split-default <name>.")
		return 1
	}

	var configDir, tok string
	if d.Action == resolve.LaunchSplit {
		if !reg.Has(d.Split) {
			fmt.Fprintf(os.Stderr, "claude-split: unknown split %q\n", d.Split)
			return 1
		}
		configDir = filepath.Join(baseDir, d.Split)
		t, err := store.Load(d.Split)
		if err != nil {
			fmt.Fprintf(os.Stderr, "claude-split: no token for split %q. Run: claude-split --split-new %s\n", d.Split, d.Split)
			return 1
		}
		tok = t
	}

	self, _ := os.Executable()
	claudePath, err := launcher.FindClaude(os.Getenv("PATH"), self)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", err)
		return 1
	}
	env := launcher.BuildEnv(os.Environ(), configDir, tok)
	if err := launcher.Exec(claudePath, p.Passthrough, env); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split: exec failed:", err)
		return 1
	}
	return 0 // unreachable on success (process replaced)
}

func confirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	var resp string
	_, _ = fmt.Scanln(&resp)
	return resp == "y" || resp == "Y" || resp == "yes"
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -v`
Expected: PASS.

Note: `TestCmdRemovePersists` reaches `confirm`, which reads stdin; under `go test` stdin is empty so `fmt.Scanln` returns an error and `resp` stays "" → aborts → split NOT removed → test fails. **Fix before implementing:** make removal confirmation skippable. Add a `--yes`-style bypass is overkill here; instead, in `cmdRemove`, treat a non-terminal stdin as auto-confirm is wrong too. Simplest correct approach: have `confirm` return true when stdin is not a TTY (non-interactive), matching common CLI behavior. Replace `confirm` with:

```go
import "golang.org/x/term" // add dependency

func confirm(prompt string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true // non-interactive: proceed
	}
	fmt.Fprint(os.Stderr, prompt)
	var resp string
	_, _ = fmt.Scanln(&resp)
	return resp == "y" || resp == "Y" || resp == "yes"
}
```

Run `go get golang.org/x/term` first. Re-run the test; `TestCmdRemovePersists` now passes (test stdin is a pipe, not a TTY → auto-confirm).

- [ ] **Step 5: Commit**

```bash
go mod tidy
git add internal/cli/ go.mod go.sum
git commit -m "feat: wire CLI dispatch for all split commands"
```

---

### Task 9: End-to-end integration test (real binary + fake claude)

Builds the binary, places a fake `claude` on `$PATH` that records its argv and the managed env vars, and asserts passthrough + env are correct for both a named split and the home profile.

**Files:**
- Create: `internal/e2e/e2e_test.go`
- Create: `internal/e2e/testdata/fake-claude.sh`

- [ ] **Step 1: Write the failing test**

`internal/e2e/testdata/fake-claude.sh`:
```sh
#!/bin/sh
{
  echo "ARGS:$@"
  echo "CLAUDE_CONFIG_DIR=$CLAUDE_CONFIG_DIR"
  echo "CLAUDE_CODE_OAUTH_TOKEN=$CLAUDE_CODE_OAUTH_TOKEN"
} > "$CLAUDE_TEST_OUT"
```

`internal/e2e/e2e_test.go`:
```go
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
	cmd := exec.Command("go", "build", "-o", bin, "claude-split")
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/e2e/`
Expected: FAIL initially if any wiring is off; this is the full-stack check.

- [ ] **Step 3: Fix any wiring revealed**

No new production code is expected. If a test fails, debug the specific package (most likely `BuildEnv` env formatting or PATH self-skip). Make the minimal fix in the relevant package and re-run.

- [ ] **Step 4: Run the whole suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 5: Commit**

```bash
git add internal/e2e/
git commit -m "test: end-to-end launch passes env and args to real claude"
```

---

### Task 10: Distribution & docs

**Files:**
- Create: `README.md`
- Create: `Makefile`
- Create: `.goreleaser.yaml`
- Create: `LICENSE` (MIT)

- [ ] **Step 1: Write the Makefile**

`Makefile`:
```make
.PHONY: build test install
build:
	go build -o bin/claude-split .
test:
	go test ./...
install:
	go install .
```

- [ ] **Step 2: Write `.goreleaser.yaml`**

`.goreleaser.yaml`:
```yaml
version: 2
builds:
  - id: claude-split
    main: .
    binary: claude-split
    goos: [darwin, linux]
    goarch: [amd64, arm64]
brews:
  - name: claude-split
    repository:
      owner: REPLACE_WITH_GH_OWNER
      name: homebrew-tap
    description: "Manage multiple isolated Claude Code profiles"
    license: MIT
```

Note: set the brew tap owner/repo and update `go.mod`'s module path to the real GitHub URL before the first tagged release. Until then `goreleaser` is build-only.

- [ ] **Step 3: Write the README**

`README.md` (single logical line per paragraph; do not hard-wrap prose):
```markdown
# claude-split

Run multiple isolated [Claude Code](https://docs.claude.com/claude-code) profiles ("splits") from a single Claude Code install, each with its own config and login. `claude-split` wraps the real `claude`, selects a split, and forwards every other argument unchanged.

## How it works

Each split is a self-contained directory at `~/.claude-splits/<name>/` selected via `CLAUDE_CONFIG_DIR`, with its own long-lived auth token injected as `CLAUDE_CODE_OAUTH_TOKEN`. The home profile (`~/.claude.json`, `~/.claude/`) is the implicit `default` split and is never modified.

## Usage

| Command | What it does |
|---|---|
| `claude-split [claude args...]` | Launch the resolved split (explicit `--split` > configured default > prompt) |
| `claude-split --split <name> ...` | Launch a specific split |
| `claude-split --split-list` | List splits, the default, and token status |
| `claude-split --split-new <name>` | Create a split and authenticate it |
| `claude-split --split-default <name>` | Set the auto-default (`default` = home profile) |
| `claude-split --split-rm <name>` | Remove a split |
| `claude-split --split-which` | Show which split would be launched |

## Install

`brew install <owner>/tap/claude-split` or `go install` from source.
```

- [ ] **Step 4: Add LICENSE and verify the build**

Add an MIT `LICENSE` file (year 2026, author per the repo owner). Then:

Run: `make build && make test`
Expected: binary built at `bin/claude-split`, all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md Makefile .goreleaser.yaml LICENSE
git commit -m "docs: add README, build tooling, and release config"
```

---

## Self-Review Notes

- **Spec coverage:** core model (Tasks 3,4,7,8), CLI surface all six flags (Tasks 2,8), launch flow (Tasks 7,8,9), create flow with token minting (Tasks 5,6,8), errors & safety — never touch home, missing token, claude-not-found, rm-confirm/refuse-default (Tasks 3,7,8), testing unit+integration (all tasks + Task 9), distribution (Task 10), Go rationale (header). Covered.
- **Open verification item from spec** (`CLAUDE_CONFIG_DIR` relocates `.claude.json`?): not blocking — the wrapper only *sets* the var; whatever Claude does with it is correct by construction. Confirm behavior manually after Task 9 by launching a real `claude` in a split and checking where it writes.
- **Resolution refinement:** spec said "more than one split"; this plan prompts when **≥1** split exists and nothing is chosen (Task 4), which is the sensible behavior (home vs the split is still a choice). Intentional clarification.
- **Type consistency:** `Parsed` fields, `registry.Registry` methods, `resolve.Decision`/`Action`, `token.Store`/`Minter`/`ParseToken`, and `launcher.FindClaude`/`BuildEnv`/`Exec` signatures are used consistently across Tasks 2–9.
