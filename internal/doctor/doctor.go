// Package doctor finds places where a split is not isolated from the home
// profile (~/.claude) or still uses a legacy token, and fixes them.
package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bcostea/claude-split/internal/auth"
)

// Finding is one problem. Fix is nil when the user must act; Hint then says
// what to do.
type Finding struct {
	Split   string // "" for a problem that is not tied to one split
	Problem string
	Details []string
	Fix     func() error
	Hint    string
}

// Env is everything the checks read. Status and the legacy token functions
// are injected so tests never run claude or touch the Keychain.
type Env struct {
	Home         string
	BaseDir      string
	Splits       []string
	Status       func(configDir string) (auth.Info, error)
	LegacyTokens []string
	DeleteLegacy func(name string) error
}

// Check runs every check and returns the findings in a stable order.
func Check(e Env) []Finding {
	var out []Finding
	registered := map[string]bool{}
	legacy := map[string]bool{}
	for _, n := range e.LegacyTokens {
		legacy[n] = true
	}
	for _, s := range e.Splits {
		registered[s] = true
		dir := filepath.Join(e.BaseDir, s)
		out = append(out, checkLogin(e, s, dir, legacy[s])...)
		for _, rel := range pathFiles {
			out = append(out, checkHomePaths(e.Home, s, dir, rel)...)
		}
		out = append(out, checkClaudeMdExclude(e.Home, s, dir)...)
		out = append(out, checkSymlinks(e.Home, s, dir)...)
	}
	for _, n := range e.LegacyTokens {
		if !registered[n] {
			n := n
			out = append(out, Finding{
				Problem: fmt.Sprintf("legacy token for unknown split %q", n),
				Fix:     func() error { return e.DeleteLegacy(n) },
			})
		}
	}
	return out
}

func checkLogin(e Env, split, dir string, hasLegacy bool) []Finding {
	info, err := e.Status(dir)
	if err != nil {
		return []Finding{{Split: split, Problem: "cannot read login status: " + err.Error()}}
	}
	var out []Finding
	if !info.LoggedIn {
		out = append(out, Finding{
			Split:   split,
			Problem: "not logged in",
			Hint:    "run: claude-split --split-login " + split,
		})
	}
	if hasLegacy {
		f := Finding{Split: split, Problem: "legacy setup-token is still stored"}
		if info.LoggedIn {
			f.Fix = func() error { return e.DeleteLegacy(split) }
		} else {
			f.Hint = "log in first, then run: claude-split --split-fix"
		}
		out = append(out, f)
	}
	return out
}

// pathFiles are the split files whose paths claude follows. settings.local.json
// is left out: its home paths are permission rules, not paths claude loads.
var pathFiles = []string{
	"settings.json",
	filepath.Join("plugins", "installed_plugins.json"),
	filepath.Join("plugins", "known_marketplaces.json"),
}

// homeVarRe matches a path into the home profile written with ~, $HOME or
// ${HOME}. The absolute form is matched separately. It requires "/.claude/"
// so ".claude-splits" never matches.
const homePathTail = `/\.claude/([^"'\s)]*)`

var homeVarRe = regexp.MustCompile(`(?:~|\$HOME|\$\{HOME\})` + homePathTail)

func homePathMatches(home string, data string) [][]int {
	abs := regexp.MustCompile(regexp.QuoteMeta(home) + homePathTail)
	m := append(homeVarRe.FindAllStringSubmatchIndex(data, -1), abs.FindAllStringSubmatchIndex(data, -1)...)
	sort.Slice(m, func(i, j int) bool { return m[i][0] < m[j][0] })
	return m
}

// rewriteHomePaths replaces each path into ~/.claude in data with the split's
// own copy of it. Paths with no copy in the split are left and returned in
// missing.
func rewriteHomePaths(home, dir, data string) (out string, fixed, missing []string) {
	var b strings.Builder
	last := 0
	for _, m := range homePathMatches(home, data) {
		from := data[m[0]:m[1]]
		if from == homeMemoryFile(home) {
			continue // the claudeMdExcludes entry, which must name the home file
		}
		to := filepath.Join(dir, data[m[2]:m[3]])
		if _, err := os.Stat(to); err != nil {
			missing = append(missing, from)
			continue
		}
		fixed = append(fixed, from+" -> "+to)
		b.WriteString(data[last:m[0]])
		b.WriteString(to)
		last = m[1]
	}
	b.WriteString(data[last:])
	return b.String(), fixed, missing
}

// checkHomePaths reports paths into ~/.claude in one split file. A path whose
// equivalent exists inside the split is rewritten to it; others need the user.
// Fixes re-read the file, because another fix can change it first.
func checkHomePaths(home, split, dir, rel string) []Finding {
	file := filepath.Join(dir, rel)
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	_, fixed, missing := rewriteHomePaths(home, dir, string(raw))
	var out []Finding
	if len(fixed) > 0 {
		out = append(out, Finding{
			Split:   split,
			Problem: fmt.Sprintf("%s uses %d path(s) from the home profile", rel, len(fixed)),
			Details: fixed,
			Fix: func() error {
				raw, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				updated, _, _ := rewriteHomePaths(home, dir, string(raw))
				return writeKeepMode(file, []byte(updated))
			},
		})
	}
	if len(missing) > 0 {
		out = append(out, Finding{
			Split:   split,
			Problem: fmt.Sprintf("%s uses %d home-profile path(s) with no copy in the split", rel, len(missing)),
			Details: missing,
			Hint:    "reinstall the plugin (or copy the file) inside this split, then run: claude-split --split-doctor",
		})
	}
	return out
}

// homeMemoryFile is the home profile's user memory. Claude Code loads
// <dir>/.claude/CLAUDE.md for every parent dir of the project, so it loads this
// file as project memory in any project under $HOME.
func homeMemoryFile(home string) string {
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

// checkClaudeMdExclude adds the home memory file to the split's
// claudeMdExcludes setting.
func checkClaudeMdExclude(home, split, dir string) []Finding {
	target := homeMemoryFile(home)
	if _, err := os.Stat(target); err != nil {
		return nil
	}
	settings := filepath.Join(dir, "settings.json")
	raw, err := os.ReadFile(settings)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return []Finding{{Split: split, Problem: "cannot read settings.json: " + err.Error()}}
	}
	excludes, err := stringList(raw, "claudeMdExcludes")
	if err != nil {
		return []Finding{{Split: split, Problem: "cannot parse settings.json: " + err.Error()}}
	}
	for _, x := range excludes {
		if x == target {
			return nil
		}
	}
	return []Finding{{
		Split:   split,
		Problem: "home-profile CLAUDE.md loads in every project under " + home,
		Details: []string{"add " + target + " to claudeMdExcludes in settings.json"},
		Fix: func() error {
			raw, err := os.ReadFile(settings)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			excludes, err := stringList(raw, "claudeMdExcludes")
			if err != nil {
				return err
			}
			updated, err := setTopLevel(raw, "claudeMdExcludes", append(excludes, target))
			if err != nil {
				return err
			}
			return writeKeepMode(settings, updated)
		},
	}}
}

// checkSymlinks finds links into ~/.claude in the split's top two levels. They
// come from copying the home profile. projects/ is skipped: it is large and
// claude writes no such links there.
func checkSymlinks(home, split, dir string) []Finding {
	prefix := filepath.Join(home, ".claude") + string(filepath.Separator)
	var links []string
	visit := func(p string) {
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			return
		}
		dest, err := os.Readlink(p)
		if err == nil && strings.HasPrefix(dest, prefix) {
			links = append(links, p)
		}
	}
	top, _ := os.ReadDir(dir)
	for _, t := range top {
		p := filepath.Join(dir, t.Name())
		visit(p)
		if !t.IsDir() || t.Name() == "projects" {
			continue
		}
		sub, _ := os.ReadDir(p)
		for _, s := range sub {
			visit(filepath.Join(p, s.Name()))
		}
	}
	if len(links) == 0 {
		return nil
	}
	return []Finding{{
		Split:   split,
		Problem: fmt.Sprintf("%d symlink(s) point into the home profile", len(links)),
		Details: links,
		Fix: func() error {
			for _, l := range links {
				if err := os.Remove(l); err != nil {
					return err
				}
			}
			return nil
		},
	}}
}

func writeKeepMode(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}
