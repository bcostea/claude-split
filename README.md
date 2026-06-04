# claude-split

Run multiple isolated [Claude Code](https://docs.claude.com/claude-code) profiles ("splits") from a single Claude Code install — each with its own config and login. `claude-split` wraps the real `claude`: it selects a split, sets up its isolated config and authentication, and forwards every other argument through unchanged.

## Install

```sh
go install github.com/bcostea/claude-split@latest
```

Or build from source: clone the repo and run `make build` (the binary lands at `bin/claude-split`).

## Quick start

```sh
# Create a split and authenticate it (opens the login flow once)
claude-split --split-new work

# Use it inside a project — this also remembers the choice for this folder
cd ~/code/project && claude-split --split work

# Later, in the same folder, a bare invocation auto-loads the remembered split
cd ~/code/project && claude-split
```

## Commands

| Command | What it does |
|---|---|
| `claude-split [claude args...]` | Launch the resolved split and pass all arguments through to `claude` |
| `claude-split --split <name> ...` | Launch a specific split |
| `claude-split --split-list` | List splits, the default, and token status |
| `claude-split --split-new <name>` | Create a split and authenticate it |
| `claude-split --split-default <name>` | Set the global default (`default` = home profile) |
| `claude-split --split-rm <name>` | Remove a split |
| `claude-split --split-which` | Show which split would be launched |

Every argument other than the `--split*` flags is passed to `claude` untouched.

## How splits work

Each split is a self-contained directory at `~/.claude-splits/<name>/` holding its own `.claude.json`, `.claude/` state, and a long-lived auth token. `claude-split` selects one by pointing `CLAUDE_CONFIG_DIR` at its directory and injecting the token as `CLAUDE_CODE_OAUTH_TOKEN`, then `exec`s the real `claude`. The home profile (`~/.claude.json`, `~/.claude/`) is the implicit `default` split and is never modified, so running `claude` directly behaves exactly as before.

Because each split authenticates separately, you log in once per split — with whatever account you choose: the same subscription across all of them, or different accounts entirely. On macOS each split's token is stored as its own Keychain item; elsewhere it is a `0600` file inside the split directory.

## Choosing which split runs

When no `--split` is given, `claude-split` resolves the target in this order:

```
explicit --split  >  folder's last-used split  >  global default  >  prompt
```

- **Folder memory.** Launching a split explicitly (`--split <name>`) from a project folder records that the folder uses that split. A later bare `claude-split` in the same folder auto-loads it. The map lives in `${XDG_CONFIG_HOME:-~/.config}/claude-split/folders.json`, keyed by canonical absolute path. Only explicit choices are recorded — including `--split default`, which pins a folder to your home profile — and an entry self-prunes if its split is removed, falling back to the global default.
- **Global default.** Set with `--split-default <name>`; applies in any folder without its own remembered split.
- **Select.** With no selection and no default, an interactive menu appears — arrow keys (or `j`/`k`) to move, `Enter` to choose, `q`/`Ctrl-C` to cancel. It lists the home profile, every split with its token status, and a `+ new split…` entry that creates and authenticates a split on the spot. Your choice is remembered for the folder, so the menu does not reappear there. In a non-interactive session (pipe, CI, `-p`) it instead prints the list and exits.

## Don't run splits from your home directory

`CLAUDE_CONFIG_DIR` isolates only *user-scope* config. Claude Code also loads *project-scope* config from `<cwd>/.claude/` — settings, `statusLine`, hooks, skills, `CLAUDE.md` — and that is not affected by `CLAUDE_CONFIG_DIR`. When your working directory is your home directory, `<cwd>/.claude` *is* `~/.claude`, so the default profile's settings and skills leak into a split and defeat isolation.

For that reason the home directory is always the default profile: an explicit `--split <name>` from `$HOME` is refused (run it from a project directory instead), and a folder- or globally-configured split is downgraded to the home profile there. Set `CLAUDE_SPLIT_ALLOW_HOME=1` to override.

## Environment

| Variable | Effect |
|---|---|
| `CLAUDE_SPLIT_TOKEN_STORE=file` | Force the file-based token store instead of the macOS Keychain |
| `CLAUDE_SPLIT_ASSUME_YES=1` | Skip the confirmation prompt on `--split-rm` (scripting) |
| `CLAUDE_SPLIT_ALLOW_HOME=1` | Allow launching a split from your home directory (isolation is degraded — see above) |
