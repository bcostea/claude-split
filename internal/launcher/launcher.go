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

// splitVars are set by claude-split for a split. They are always stripped
// first, so a value inherited from a parent split session never leaks through.
var splitVars = []string{"CLAUDE_CONFIG_DIR=", "CLAUDE_CODE_OAUTH_TOKEN="}

// authVars override the login stored for a config dir. They are stripped for a
// split so that claude uses the split's own `/login` credentials.
var authVars = []string{
	"CLAUDE_CODE_OAUTH_TOKEN=",
	"CLAUDE_CODE_OAUTH_REFRESH_TOKEN=",
	"ANTHROPIC_API_KEY=",
	"ANTHROPIC_AUTH_TOKEN=",
}

// BuildEnv returns the environment to launch with. For the home profile pass
// configDir == "": only the split vars are stripped, so the home profile runs
// as plain `claude` would. For a split, the split vars and all auth overrides
// are stripped and CLAUDE_CONFIG_DIR is set. Claude Code keeps a separate login
// per CLAUDE_CONFIG_DIR, so the split then authenticates as its own account.
func BuildEnv(base []string, configDir string) []string {
	strip := splitVars
	if configDir != "" {
		strip = append(append([]string{}, splitVars...), authVars...)
	}
	out := make([]string, 0, len(base)+1)
	for _, e := range base {
		if hasAnyPrefix(e, strip) {
			continue
		}
		out = append(out, e)
	}
	if configDir != "" {
		out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
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
