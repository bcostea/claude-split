# claude-split on Windows (PowerShell)

The Go binary does not run on Windows today, for two independent reasons:

- `internal/launcher/launcher.go` calls `syscall.Exec`, which is Unix-only, so the
  package does not compile for `GOOS=windows`.
- `FindClaude` looks for an extensionless `claude` carrying a Unix executable bit.
  Windows has `claude.exe`, and Go never sets the `0111` bits on a regular file
  there, so the candidate would be rejected even if the name matched.

`claude-split.ps1` in this directory is a stopgap: an independent PowerShell
implementation of the same mechanism, **not** a port of the Go code. It writes the
same `registry.json` and `folders.json`, in the same locations, with the same
shape, so switching to the Go binary once it builds for Windows needs no
migration.

## Install

Dot-source it from your `$PROFILE`:

```powershell
# add to $PROFILE
. "$env:USERPROFILE\.claude-splits\claude-split.ps1"
```

That defines `cs` and `claude-split` as aliases. Reload with `. $PROFILE`.

## Commands

| Flag | Go CLI | Here | Notes |
|---|:--:|:--:|---|
| `--split <name>` | yes | yes | Launch a specific split |
| `--split-list` | yes | yes | Lists splits, the default, and whether each has signed in |
| `--split-new <name>` | yes | yes | Creates and seeds a split. Does **not** authenticate: the split signs in on its own first launch |
| `--split-default <name>` | yes | yes | `default` means the home profile |
| `--split-rm <name>` | yes | yes | Registry only. The split's directory and sessions are left on disk |
| `--split-purge` | yes | yes | Deletes registered splits and the pin config. Leaves unregistered directories and reports them |
| `--split-which` | yes | yes | Shows which split would launch, and why |
| `--split-pin <name>` | no | yes | Addition: pin the current folder without launching |

Every other argument is forwarded to `claude` untouched.

## What differs from the Go CLI, and why

- **No minted `CLAUDE_CODE_OAUTH_TOKEN`.** On Windows credentials live per config
  dir in `<CLAUDE_CONFIG_DIR>\.credentials.json`, so each split signs in
  independently and there is nothing to mint or store. `CLAUDE_SPLIT_TOKEN_STORE`
  therefore has no meaning here.

  There is also an argument for not sharing one credential set even where you can:
  the token chain rotates and is re-issued on use, so two profiles sharing one
  copied credentials file means whichever refreshes first invalidates the other,
  surfacing as an unexplained sign-out days later.

- **Spawn, not exec.** Windows has no `execve`, so the wrapper runs `claude` as a
  child process and restores the environment afterwards.

- **Folder pins match by longest prefix, not exact path.** One pin covers
  everything beneath it, and a deeper pin overrides a broader one. The Go CLI keys
  `folders.json` by exact path, so every subdirectory needs its own entry.

- **Pins are normalised through `subst` mappings.** A `subst` drive gives one
  directory two valid absolute paths and `Resolve-Path` canonicalises neither.
  Without this, a pin on each form both match and which profile you land in
  depends on the path you typed -- which silently splits one project's history
  across two profiles.

- **A new split gets an `ide` junction to `~/.claude/ide`.** The editor's Claude
  Code extension advertises itself there and has no `CLAUDE_CONFIG_DIR` of its
  own, so without the junction a split reads a directory the extension never
  writes to and loses IDE attachment entirely. Creating a junction does not need
  elevation. See the separate issue -- this affects splits on every platform, not
  just Windows.

- **Plugins are copied, not shared.** Junctioning would give every profile one
  install, but plugins auto-update, so sharing risks two profiles racing on the
  same marketplace clone. The seeded manifest's `installPath` is rewritten to the
  split's own cache; otherwise the split stays silently dependent on the home
  profile's copy surviving.

- **`CLAUDE.md` is seeded.** User-scope instructions are read from the profile's
  own config dir, so a split without a copy silently drops all of them.

## Environment

| Variable | Effect |
|---|---|
| `CLAUDE_SPLIT_ALLOW_HOME=1` | Allow launching a split from your home directory (isolation is degraded -- `<cwd>\.claude` *is* the home profile there) |
| `CLAUDE_SPLIT_ASSUME_YES=1` | Skip confirmation on `--split-rm` and `--split-purge` |
| `CLAUDE_SPLIT_SEED_EXCLUDE` | Comma- or semicolon-separated name fragments to leave out when seeding a new split -- matched against marketplace and plugin names. Empty by default, so a new split is seeded with everything the home profile has |

## What has been verified, and how

Manual verification on Windows 11 with PowerShell 7. The full lifecycle
(`--split-new`, `--split-pin`, `--split-which`, `--split-list`, `--split-rm`,
`--split-purge`) was exercised against a temporary root so no real profile was
touched, checking each time that:

- the seeded split contains plugins, commands, settings and `CLAUDE.md`, and
  **no** `.credentials.json`
- `installPath` in the seeded manifest points inside the split
- `CLAUDE_SPLIT_SEED_EXCLUDE` drops exactly the named marketplace and keeps the rest
- a pin on a parent folder resolves from a nested subdirectory
- `--split-rm` prunes that split's pins and leaves its directory
- `--split-purge` removes the `ide` junction **without** deleting through it --
  `~/.claude/ide` and its lock file survive intact
- claude's own flags (`-p`, `-c`, `-v`, `--model`, `--verbose`) reach `claude`
  rather than being swallowed as PowerShell common parameters

Beyond that it has been in daily use driving three profiles.

## Known gaps

- No interactive selector. With no pin and no default it prints a numbered list
  and reads a number, instead of the arrow-key menu the Go CLI has, and it has no
  `+ new split` entry.
- Not a compiled binary, so nothing ships through goreleaser and there is no
  install story beyond dot-sourcing.
- No automated tests. The Go test suite does not cover this file, and nothing in
  CI runs PowerShell.
