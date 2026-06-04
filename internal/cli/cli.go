package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/bcostea/claude-split/internal/args"
	"github.com/bcostea/claude-split/internal/folders"
	"github.com/bcostea/claude-split/internal/launcher"
	"github.com/bcostea/claude-split/internal/registry"
	"github.com/bcostea/claude-split/internal/resolve"
	"github.com/bcostea/claude-split/internal/token"
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
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	canonCwd := canonDir(cwd)

	// Per-folder memory: a previously chosen split for this folder acts as the
	// effective default, overriding the global default. Stale entries are pruned.
	fstore, ferr := folders.Load(folders.DefaultPath())
	if ferr != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", ferr)
		return 1
	}
	effDefault, pruned := effectiveDefault(fstore, reg, canonCwd)
	if pruned {
		_ = fstore.Save()
	}

	d := resolve.Resolve(p.Split, p.SplitSet, effDefault, reg.Splits)

	// In the home directory, project-scope config (~/.claude) would leak into a
	// split and defeat isolation, so home always resolves to the default profile.
	allowHome := os.Getenv("CLAUDE_SPLIT_ALLOW_HOME") != ""
	d, note, gErr := applyHomeGuard(d, p.SplitSet, cwd, home, allowHome)
	if gErr != nil {
		fmt.Fprintln(os.Stderr, "claude-split:", gErr)
		return 1
	}
	if note != "" {
		fmt.Fprintln(os.Stderr, "claude-split:", note)
	}

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

	// Remember an explicit choice for this folder (never in home). The split's
	// name, including "default", is recorded so a later bare launch reuses it.
	if p.SplitSet && canonCwd != "" && !sameDir(cwd, home) {
		if cur, ok := fstore.Get(canonCwd); !ok || cur != p.Split {
			fstore.Set(canonCwd, p.Split)
			if err := fstore.Save(); err != nil {
				fmt.Fprintln(os.Stderr, "claude-split: warning: could not record folder split:", err)
			}
		}
	}

	env := launcher.BuildEnv(os.Environ(), configDir, tok)
	if err := launcher.Exec(claudePath, p.Passthrough, env); err != nil {
		fmt.Fprintln(os.Stderr, "claude-split: exec failed:", err)
		return 1
	}
	return 0 // unreachable on success (process replaced)
}

// effectiveDefault returns the default split to feed into resolution for the
// given folder: the folder's remembered split when present and still valid,
// otherwise the global default. A remembered split that no longer exists is
// deleted from the store and reported via pruned (caller persists the store).
func effectiveDefault(store *folders.Store, reg *registry.Registry, folder string) (def string, pruned bool) {
	v, ok := store.Get(folder)
	if !ok {
		return reg.Default, false
	}
	if v == "default" || reg.Has(v) {
		return v, false
	}
	store.Delete(folder)
	return reg.Default, true
}

// applyHomeGuard enforces "home is always the default profile". When launching
// from the home directory, the project-scope config at ~/.claude (settings,
// statusLine, hooks, skills, CLAUDE.md) is loaded regardless of
// CLAUDE_CONFIG_DIR, so a split cannot actually be isolated there.
//
// It returns the (possibly adjusted) decision, an optional notice to print, and
// an error if the launch should be refused. An explicit --split is refused; a
// split coming only from the configured default is downgraded to the home
// profile with a notice. allow bypasses the guard entirely.
func applyHomeGuard(d resolve.Decision, explicit bool, cwd, home string, allow bool) (resolve.Decision, string, error) {
	if allow || cwd == "" || home == "" || !sameDir(cwd, home) {
		return d, "", nil
	}
	// In the home directory the profile is always the default.
	switch d.Action {
	case resolve.LaunchDefault:
		return d, "", nil
	case resolve.LaunchSplit:
		if explicit {
			return d, "", fmt.Errorf("refusing to launch split %q from your home directory: project config in ~/.claude would leak in and defeat isolation. cd into a project directory, or set CLAUDE_SPLIT_ALLOW_HOME=1 to override", d.Split)
		}
		return resolve.Decision{Action: resolve.LaunchDefault},
			fmt.Sprintf("in home directory: using the default profile (configured default split %q skipped)", d.Split),
			nil
	default: // PrintListExit — no ambiguity in home, just use default
		return resolve.Decision{Action: resolve.LaunchDefault}, "", nil
	}
}

// sameDir reports whether two paths refer to the same directory, resolving
// relative paths and symlinks where possible.
func sameDir(a, b string) bool {
	return canonDir(a) == canonDir(b)
}

func canonDir(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if ev, err := filepath.EvalSymlinks(p); err == nil {
		return ev
	}
	return filepath.Clean(p)
}

// confirm prompts for y/N on an interactive terminal. It auto-confirms when
// CLAUDE_SPLIT_ASSUME_YES is set (scripting) or when stdin is not a TTY.
func confirm(prompt string) bool {
	if os.Getenv("CLAUDE_SPLIT_ASSUME_YES") != "" {
		return true
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	fmt.Fprint(os.Stderr, prompt)
	var resp string
	_, _ = fmt.Scanln(&resp)
	return resp == "y" || resp == "Y" || resp == "yes"
}
