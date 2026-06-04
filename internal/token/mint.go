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
