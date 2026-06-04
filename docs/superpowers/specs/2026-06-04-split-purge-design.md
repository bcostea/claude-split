# `--split-purge` — Design

**Date:** 2026-06-04
**Status:** Approved design, pre-implementation

## Summary

`--split-purge` resets `claude-split` to a clean state: it removes every split and all `claude-split` configuration, leaving only the default/home profile. It never touches `~/.claude.json` or `~/.claude/`.

## Behavior

1. **Confirm** (destructive): `Purge N split(s), their tokens, and all claude-split config? The home profile is untouched. [y/N]`. Skipped when `CLAUDE_SPLIT_ASSUME_YES=1` or stdin is not a TTY, matching `--split-rm`.
2. **Delete each split's token** by iterating `registry.Splits` and calling `store.Delete(name)`, so macOS Keychain items are removed (a directory wipe alone would leave them).
3. **`os.RemoveAll(~/.claude-splits)`** removes all split directories and `registry.json`.
4. **Remove `${XDG_CONFIG_HOME:-~/.config}/claude-split/folders.json`** (per-folder memory); a missing file is not an error.
5. **Print a summary**: `Purged N split(s). Only the default profile remains.`

Idempotent: running it with nothing to purge succeeds.

## Wiring

- `args`: a `--split-purge` boolean flag (`Parsed.Purge`).
- `cli`: a new `cmdPurge(reg, store, baseDir)` dispatched from `Run` alongside the other `--split*` commands.

## Testing

- `args`: parse test for `--split-purge`.
- `cli`: integration test with the file token store and a temp `$HOME` seeded with a split directory, token, `registry.json`, and `folders.json`, plus a sentinel `~/.claude.json`. Asserts `~/.claude-splits` and `folders.json` are gone after purge while `~/.claude.json` is untouched.
