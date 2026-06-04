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
