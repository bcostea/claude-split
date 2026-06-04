# claude-split

Run multiple isolated [Claude Code](https://docs.claude.com/claude-code) profiles ("splits") from one install and one subscription. `claude-split` wraps the real `claude`, selects a split, and forwards every other argument unchanged.

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

All arguments other than the `--split*` flags above are passed through to `claude` untouched.

## Environment

| Variable | Effect |
|---|---|
| `CLAUDE_SPLIT_TOKEN_STORE=file` | Force the file-based token store instead of the macOS Keychain |
| `CLAUDE_SPLIT_ASSUME_YES=1` | Skip the confirmation prompt on `--split-rm` (scripting) |

## Install

```sh
go install github.com/bcostea/claude-split@latest
```

Or build from source: `git clone` then `make build` (binary lands at `bin/claude-split`).
