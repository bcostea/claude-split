# claude-split

Run multiple isolated [Claude Code](https://docs.claude.com/claude-code) profiles ("splits") from a single Claude Code install — each with its own config and login. `claude-split` wraps the real `claude`: it selects a split, sets up its isolated config and authentication, and forwards every other argument through unchanged.

## Install

Homebrew (macOS):

```sh
brew install bcostea/claude-tools/claude-split
```

Go (any platform):

```sh
go install github.com/bcostea/claude-split@latest
```

Or build from source: clone the repo and run `make build` (the binary lands at `bin/claude-split`).

### Tip: alias `claude`

To route your normal `claude` usage through claude-split, add an alias to your shell rc:

```sh
alias claude="claude-split"
```

It only affects interactive use, and `claude-split` still execs the real `claude` binary by `PATH` lookup, so there is no recursion.

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
| `claude-split --split-list` | List splits, the default, and the account each split uses |
| `claude-split --split-new <name>` | Create a split and log it in |
| `claude-split --split-login <name>` | Log an existing split in again (change its account, or move it off a legacy token) |
| `claude-split --split-default <name>` | Set the global default (`default` = home profile) |
| `claude-split --split-rm <name>` | Remove a split and log it out |
| `claude-split --split-purge` | Remove all splits, their logins, and config (the home profile is untouched) |
| `claude-split --split-which` | Show which split would be launched |
| `claude-split --split-doctor` | Report isolation and login problems (exit code 1 if there are problems) |
| `claude-split --split-fix` | Report the problems, then apply the automatic fixes after confirmation |

Every argument other than the `--split*` flags is passed to `claude` untouched.

## How splits work

Each split is a self-contained directory at `~/.claude-splits/<name>/` holding its own `.claude.json` and state. `claude-split` selects one by pointing `CLAUDE_CONFIG_DIR` at its directory, then `exec`s the real `claude`. The home profile (`~/.claude.json`, `~/.claude/`) is the implicit `default` split and is never modified, so running `claude` directly behaves exactly as before.

Claude Code stores a separate login for each `CLAUDE_CONFIG_DIR` (on macOS, one Keychain item per config dir; elsewhere a file in the config dir). `--split-new` and `--split-login` run `claude auth login` for the split's directory, so you log in once per split with whatever account you choose. `--split-list` shows the account each split uses.

When it launches a split, `claude-split` removes `CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_CODE_OAUTH_REFRESH_TOKEN`, `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` from the environment. These variables override the stored login, so a value from your shell or from a parent session would make the split run as a different account. The home profile keeps them.

### Legacy tokens

Versions up to 0.1.0 created each split's credentials with `claude setup-token` and passed them as `CLAUDE_CODE_OAUTH_TOKEN`. Those tokens only have the inference scope. Features that need the profile scope fail, for example remote managed settings (`authentication rejected (401)`). The token also comes from whichever account the browser was logged into, which can differ from the account you expect. `--split-doctor` finds these tokens and `--split-fix` deletes them for each split that has its own login.

## Doctor

`--split-doctor` checks each split for:

- **Login.** The split has no login of its own, or still has a legacy token.
- **Home-profile paths.** `settings.json`, `plugins/installed_plugins.json` or `plugins/known_marketplaces.json` refers to a path in `~/.claude/`. This happens when a split starts as a copy of the home profile. `--split-fix` rewrites the path to the split's own copy. If the split has no copy, reinstall the plugin inside the split.
- **Home-profile CLAUDE.md.** Claude Code loads `<dir>/.claude/CLAUDE.md` for every parent folder of the project. For any project under `$HOME`, it therefore loads `~/.claude/CLAUDE.md`. `--split-fix` adds that file to the split's `claudeMdExcludes` setting.
- **Symlinks** into `~/.claude/` in the split directory.

## Choosing which split runs

When no `--split` is given, `claude-split` resolves the target in this order:

```
explicit --split  >  folder's last-used split  >  global default  >  prompt
```

- **Folder memory.** Launching a split explicitly (`--split <name>`) from a project folder records that the folder uses that split. A later bare `claude-split` in the same folder auto-loads it. The map lives in `${XDG_CONFIG_HOME:-~/.config}/claude-split/folders.json`, keyed by canonical absolute path. Only explicit choices are recorded — including `--split default`, which pins a folder to your home profile — and an entry self-prunes if its split is removed, falling back to the global default.
- **Global default.** Set with `--split-default <name>`; applies in any folder without its own remembered split.
- **Select.** With no selection and no default, an interactive menu appears — arrow keys (or `j`/`k`) to move, `Enter` to choose, `q`/`Ctrl-C` to cancel. It lists the home profile, every split with its account, and a `+ new split…` entry that creates and logs in a split on the spot. Your choice is remembered for the folder, so the menu does not reappear there. In a non-interactive session (pipe, CI, `-p`) it instead prints the list and exits.

## Don't run splits from your home directory

`CLAUDE_CONFIG_DIR` isolates only *user-scope* config. Claude Code also loads *project-scope* config from `<cwd>/.claude/` — settings, `statusLine`, hooks, skills, `CLAUDE.md` — and that is not affected by `CLAUDE_CONFIG_DIR`. When your working directory is your home directory, `<cwd>/.claude` *is* `~/.claude`, so the default profile's settings and skills leak into a split and defeat isolation.

For that reason the home directory is always the default profile: an explicit `--split <name>` from `$HOME` is refused (run it from a project directory instead), and a folder- or globally-configured split is downgraded to the home profile there. Set `CLAUDE_SPLIT_ALLOW_HOME=1` to override.

## Environment

| Variable | Effect |
|---|---|
| `CLAUDE_SPLIT_TOKEN_STORE=file` | Look for legacy tokens in files instead of the macOS Keychain |
| `CLAUDE_SPLIT_ASSUME_YES=1` | Skip the confirmation prompt on `--split-rm`, `--split-purge` and `--split-fix` (scripting) |
| `CLAUDE_SPLIT_ALLOW_HOME=1` | Allow launching a split from your home directory (isolation is degraded — see above) |
