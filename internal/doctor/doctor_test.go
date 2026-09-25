package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bcostea/claude-split/internal/auth"
)

type fixture struct {
	home, base string
	deleted    []string
}

func newFixture(t *testing.T, splits ...string) *fixture {
	t.Helper()
	f := &fixture{home: t.TempDir()}
	f.base = filepath.Join(f.home, ".claude-splits")
	for _, s := range splits {
		mkdir(t, filepath.Join(f.base, s))
	}
	mkdir(t, filepath.Join(f.home, ".claude"))
	return f
}

func (f *fixture) env(splits []string, legacy []string, loggedIn bool) Env {
	return Env{
		Home:    f.home,
		BaseDir: f.base,
		Splits:  splits,
		Status: func(string) (auth.Info, error) {
			return auth.Info{LoggedIn: loggedIn, Email: "a@b.c"}, nil
		},
		LegacyTokens: legacy,
		DeleteLegacy: func(n string) error { f.deleted = append(f.deleted, n); return nil },
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func fixAll(t *testing.T, fs []Finding) {
	t.Helper()
	for _, f := range fs {
		if f.Fix != nil {
			if err := f.Fix(); err != nil {
				t.Fatalf("fix %q: %v", f.Problem, err)
			}
		}
	}
}

func problems(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Split+": "+f.Problem)
	}
	return out
}

func TestCleanSplitHasNoFindings(t *testing.T) {
	f := newFixture(t, "work")
	if got := Check(f.env([]string{"work"}, nil, true)); len(got) != 0 {
		t.Fatalf("expected no findings, got %v", problems(got))
	}
}

func TestNotLoggedInNeedsUser(t *testing.T) {
	f := newFixture(t, "work")
	got := Check(f.env([]string{"work"}, []string{"work"}, false))
	if len(got) != 2 {
		t.Fatalf("got %v", problems(got))
	}
	for _, x := range got {
		if x.Fix != nil {
			t.Fatalf("%q must not be auto-fixable before login", x.Problem)
		}
	}
}

func TestLegacyTokensDeletedWhenLoggedIn(t *testing.T) {
	f := newFixture(t, "work")
	got := Check(f.env([]string{"work"}, []string{"ghost", "work"}, true))
	if len(got) != 2 {
		t.Fatalf("got %v", problems(got))
	}
	fixAll(t, got)
	if !reflect.DeepEqual(f.deleted, []string{"work", "ghost"}) {
		t.Fatalf("deleted %v", f.deleted)
	}
}

func TestHomePathsRewrittenWhenSplitHasCopy(t *testing.T) {
	f := newFixture(t, "work")
	dir := filepath.Join(f.base, "work")
	write(t, filepath.Join(dir, "statusline-command.sh"), "")
	write(t, filepath.Join(dir, "plugins", "cache", "p", "1.0"), "")
	settings := filepath.Join(dir, "settings.json")
	write(t, settings, `{"statusLine":{"command":"bash ~/.claude/statusline-command.sh"},"x":"~/.claude-splits/work/keep"}`)
	plugins := filepath.Join(dir, "plugins", "installed_plugins.json")
	write(t, plugins, `{"a":{"installPath":"`+f.home+`/.claude/plugins/cache/p/1.0"},"b":{"installPath":"$HOME/.claude/plugins/cache/gone/2.0"}}`)

	got := Check(f.env([]string{"work"}, nil, true))
	if len(got) != 3 {
		t.Fatalf("got %v", problems(got))
	}
	fixAll(t, got)

	if s := read(t, settings); !strings.Contains(s, "bash "+dir+"/statusline-command.sh") || !strings.Contains(s, "~/.claude-splits/work/keep") {
		t.Fatalf("settings not rewritten correctly: %s", s)
	}
	p := read(t, plugins)
	if !strings.Contains(p, `"installPath":"`+dir+`/plugins/cache/p/1.0"`) {
		t.Fatalf("plugin path not rewritten: %s", p)
	}
	if !strings.Contains(p, "$HOME/.claude/plugins/cache/gone/2.0") {
		t.Fatalf("path with no copy must stay: %s", p)
	}
	// The missing plugin remains as a manual finding.
	again := Check(f.env([]string{"work"}, nil, true))
	if len(again) != 1 || again[0].Fix != nil {
		t.Fatalf("expected one manual finding, got %v", problems(again))
	}
}

func TestClaudeMdExcludeAddedPreservingOrder(t *testing.T) {
	f := newFixture(t, "work")
	write(t, filepath.Join(f.home, ".claude", "CLAUDE.md"), "home rules")
	settings := filepath.Join(f.base, "work", "settings.json")
	write(t, settings, "{\n  \"zeta\": 1,\n  \"alpha\": {\"b\": [1, 2]}\n}\n")

	fixAll(t, Check(f.env([]string{"work"}, nil, true)))

	s := read(t, settings)
	want := "{\n  \"zeta\": 1,\n  \"alpha\": {\n    \"b\": [\n      1,\n      2\n    ]\n  },\n  \"claudeMdExcludes\": [\n    \"" + filepath.Join(f.home, ".claude", "CLAUDE.md") + "\"\n  ]\n}\n"
	if s != want {
		t.Fatalf("got:\n%s\nwant:\n%s", s, want)
	}
	if again := Check(f.env([]string{"work"}, nil, true)); len(again) != 0 {
		t.Fatalf("fix is not idempotent: %v", problems(again))
	}
}

func TestClaudeMdExcludeCreatesSettings(t *testing.T) {
	f := newFixture(t, "work")
	write(t, filepath.Join(f.home, ".claude", "CLAUDE.md"), "home rules")
	fixAll(t, Check(f.env([]string{"work"}, nil, true)))
	list, err := stringList([]byte(read(t, filepath.Join(f.base, "work", "settings.json"))), "claudeMdExcludes")
	if err != nil || len(list) != 1 {
		t.Fatalf("got %v, %v", list, err)
	}
}

func TestSymlinksIntoHomeRemoved(t *testing.T) {
	f := newFixture(t, "work")
	dir := filepath.Join(f.base, "work")
	write(t, filepath.Join(f.home, ".claude", "debug", "x.txt"), "")
	mkdir(t, filepath.Join(dir, "debug"))
	bad := filepath.Join(dir, "debug", "latest")
	if err := os.Symlink(filepath.Join(f.home, ".claude", "debug", "x.txt"), bad); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "debug", "own")
	if err := os.Symlink(filepath.Join(dir, "debug"), good); err != nil {
		t.Fatal(err)
	}

	fixAll(t, Check(f.env([]string{"work"}, nil, true)))
	if _, err := os.Lstat(bad); !os.IsNotExist(err) {
		t.Fatal("link into home profile not removed")
	}
	if _, err := os.Lstat(good); err != nil {
		t.Fatal("link inside the split must stay")
	}
}

func TestFixesOnSameFileDoNotOverwriteEachOther(t *testing.T) {
	f := newFixture(t, "work")
	dir := filepath.Join(f.base, "work")
	write(t, filepath.Join(f.home, ".claude", "CLAUDE.md"), "home rules")
	write(t, filepath.Join(dir, "statusline-command.sh"), "")
	settings := filepath.Join(dir, "settings.json")
	write(t, settings, `{"statusLine":{"command":"bash ~/.claude/statusline-command.sh"}}`)

	fixAll(t, Check(f.env([]string{"work"}, nil, true)))

	if again := Check(f.env([]string{"work"}, nil, true)); len(again) != 0 {
		t.Fatalf("a fix was lost: %v\n%s", problems(again), read(t, settings))
	}
}
