// Package auth drives Claude Code's own per-config-dir login. Claude Code
// stores the credentials of each CLAUDE_CONFIG_DIR separately (on macOS, one
// Keychain item per config dir), so a split needs no token of its own.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/bcostea/claude-split/internal/launcher"
)

// Info is the subset of `claude auth status --json` that claude-split uses.
type Info struct {
	LoggedIn   bool   `json:"loggedIn"`
	AuthMethod string `json:"authMethod"`
	Email      string `json:"email"`
	OrgName    string `json:"orgName"`
}

// Account returns a short label for the logged-in account.
func (i Info) Account() string {
	switch {
	case !i.LoggedIn:
		return "not logged in"
	case i.Email != "" && i.OrgName != "":
		return i.Email + " (" + i.OrgName + ")"
	case i.Email != "":
		return i.Email
	default:
		return "logged in (" + i.AuthMethod + ")"
	}
}

// Claude runs `claude auth` subcommands against one config dir.
type Claude struct{ Path string }

func (c Claude) cmd(configDir string, args ...string) *exec.Cmd {
	cmd := exec.Command(c.Path, append([]string{"auth"}, args...)...)
	cmd.Env = launcher.BuildEnv(os.Environ(), configDir)
	return cmd
}

// Status reports the login stored for configDir.
func (c Claude) Status(configDir string) (Info, error) {
	// `auth status` exits non-zero when logged out, so parse stdout first.
	out, runErr := c.cmd(configDir, "status", "--json").Output()
	var info Info
	if err := json.Unmarshal(out, &info); err != nil {
		if runErr != nil {
			return Info{}, fmt.Errorf("claude auth status: %w", runErr)
		}
		return Info{}, fmt.Errorf("claude auth status: %w", err)
	}
	return info, nil
}

// Login runs the interactive claude.ai login for configDir.
func (c Claude) Login(configDir string) error {
	cmd := c.cmd(configDir, "login", "--claudeai")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude auth login: %w", err)
	}
	return nil
}

// Logout removes the login stored for configDir.
func (c Claude) Logout(configDir string) error {
	if out, err := c.cmd(configDir, "logout").CombinedOutput(); err != nil {
		return fmt.Errorf("claude auth logout: %w: %s", err, out)
	}
	return nil
}
