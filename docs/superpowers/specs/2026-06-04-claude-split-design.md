# claude-split — Design

**Date:** 2026-06-04
**Status:** Approved design, pre-implementation

## Summary

`claude-split` is a thin launcher (written in Go) that wraps the real `claude` CLI to manage multiple isolated Claude Code profiles ("splits") on one machine. It does not manage the Claude Code installation — it manages the *profile* (config directory and auth). It selects a split, sets the appropriate environment, and `exec`s the real `claude` binary passing **every** argument through untouched. It consumes only its own small set of `--split*` flags; everything else belongs to Claude.

The goal: one Claude install, multiple fully-isolated profiles — each logging in separately (the same subscription across all of them, or different accounts entirely) — without the profiles stomping each other.

## Key technical facts (basis for the design)

- Claude Code supports the `CLAUDE_CONFIG_DIR` environment variable, which relocates the config directory (`.claude/`) and, in current versions, `.claude.json` to a custom location. This lets a split be a self-contained directory rather than requiring files to be swapped in and out of `$HOME`.
- On macOS, Claude Code stores credentials in the system **Keychain as a single shared item**, *not* inside `.claude.json`. Therefore `CLAUDE_CONFIG_DIR` alone does **not** isolate auth on macOS — a second browser login overwrites the first split's token. Per-split long-lived OAuth tokens (`claude setup-token`) injected via `CLAUDE_CODE_OAUTH_TOKEN` solve this and deliver true auth isolation.
- **Open verification item:** confirm empirically whether `CLAUDE_CONFIG_DIR` relocates `.claude.json` as well as `.claude/` in the installed version (set the var, launch `claude`, observe where it reads/writes). The design holds either way; worst case a split's `.claude.json` lands at a slightly different path that the wrapper accounts for.

## Core model

- A **split** is a self-contained directory `~/.claude-splits/<name>/` holding that profile's `.claude.json`, `.claude/`, and its own auth token.
- The **default profile** is the real `~/.claude.json` + `~/.claude/`, used when no `CLAUDE_CONFIG_DIR` is set. It is the implicit `default` split. Plain `claude` run outside the wrapper is completely unaffected — the wrapper never writes to the home files.
- A **registry** at `~/.claude-splits/registry.json` lists the known splits and records which one is the auto-default.

### Split resolution order on launch

1. Explicit `--split <name>` flag.
2. Otherwise, the configured auto-default from the registry.
3. Otherwise, if no default is set and more than one split exists, **print the split list and exit** — never guess.

| Profile | `CLAUDE_CONFIG_DIR` | `.claude.json` used | `.claude/` used |
|---|---|---|---|
| default (home) | *(unset)* | `~/.claude.json` | `~/.claude/` |
| split `work` | `~/.claude-splits/work` | `~/.claude-splits/work/.claude.json` | `~/.claude-splits/work/.claude/` |

Nothing is ever swapped or moved; each split is a sealed directory.

## CLI surface

The binary is named `claude-split`. Wrapper-owned flags are consumed and never forwarded; all remaining arguments pass through verbatim to `claude`.

| Flag | Action |
|---|---|
| `--split <name>` | Launch claude in that split |
| `--split-list` | List splits, marking the default and which have a stored token |
| `--split-new <name>` | Create the split directory, run `claude setup-token`, store the token |
| `--split-default <name>` | Set the auto-default (use `default` for the home profile) |
| `--split-rm <name>` | Delete a split (requires confirmation; refuses `default`/home) |
| `--split-which` | Print which split the current resolution would pick, then exit |

**Collision handling:** the wrapper strips a known, fixed set of `--split*` flags from `argv`; every remaining argument passes through unchanged. If Claude ever ships a real `--split`, the wrapper's flag is renamed — low risk given the fixed, namespaced list.

## Launch flow

1. Parse `argv`, extract `--split*` flags, keep the remainder as `passthrough`.
2. Resolve the target split using the resolution order above.
3. If a named split: set `CLAUDE_CONFIG_DIR=~/.claude-splits/<name>`, read its stored token, and export `CLAUDE_CODE_OAUTH_TOKEN`. If default/home: set nothing.
4. Locate the real `claude` binary by resolving `$PATH` while skipping the wrapper itself, then `exec` it with `passthrough`. Using `exec` (replacing the process, not spawning a subprocess) adds zero latency and lets Claude own the TTY and signals directly.

## Create flow (`--split-new <name>`)

1. Create `~/.claude-splits/<name>/` and register it.
2. Run `claude setup-token` with `CLAUDE_CONFIG_DIR` pointed at the new directory; capture the long-lived OAuth token.
3. Store the token: a per-split **Keychain item on macOS** (service `claude-split`, account `<name>` — distinct items cannot stomp each other and nothing sensitive is written in plaintext); a `0600` file in the split directory on Linux.
4. Confirm success; if it's the first split, suggest `--split-default <name>`.

## Errors & safety

- Never write to `~/.claude.json` or `~/.claude/`.
- A split missing its token produces a clear message plus how to re-run setup.
- If the real `claude` is not found, or `$PATH` resolution would re-invoke the wrapper itself, fail with an explicit error.
- `--split-rm` requires confirmation and refuses to remove `default`/home.

## Testing

- **Unit:** argument partition (wrapper flags vs passthrough), registry read/write, split-resolution order.
- **Integration:** a fake `claude` stub placed on `$PATH` that asserts it receives the exact passthrough arguments and the expected environment; token-store round-trip (Keychain on macOS, file on Linux).

## Distribution

Single static Go binary, no runtime dependencies. Shipped via GitHub releases and a Homebrew tap; `go install` for contributors.

## Language rationale

Go: single static binary, instant startup (the wrapper runs on every `claude` launch, so Node's startup latency is unacceptable), trivial `exec`/env handling, simple cross-compilation, and easy distribution. Rejected alternatives — Bash (config/arg-parsing/registry management get unwieldy), Node/TS (per-launch latency despite frictionless `npm` distribution), Rust (ceremony without payoff for a thin launcher).
