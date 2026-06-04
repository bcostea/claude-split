# Interactive split selector — Design

**Date:** 2026-06-04
**Status:** Approved design, pre-implementation

## Summary

When split resolution is ambiguous (no `--split`, no folder memory, no global default, but splits exist) and the session is interactive, `claude-split` shows an arrow-key menu instead of printing the list and exiting. The menu lists the home profile, every split, and a trailing "new split" entry that creates and authenticates a split inline. The chosen split is launched and recorded as the folder's split (so the menu does not reappear there).

## Trigger and fallback

The selector replaces only the `PrintListExit` outcome, and only when both stdin and stderr are TTYs. In any non-interactive context (pipes, CI, `-p`), the existing behavior is kept: print the list plus the `--split` hint and exit 1. The home directory never reaches this path (the home guard converts the ambiguous case to the default profile first).

## Menu

```
Select a split (↑/↓, Enter, q to cancel):
❯ default (home)
  xogito        ok
  surepoint     ok
  + new split…
```

- Entries: `default (home)`, then each split with its token status (`ok` / `no token`), then `+ new split…`.
- A tokenless split is still selectable; the existing missing-token error applies on launch.
- Controls: ↑/↓ and `k`/`j` move; `Enter` confirms; `Ctrl-C`/`q`/`Esc` cancels (exit 1, nothing launched).

## On confirm

- **Split or default** → launch it and record `folder → choice` (same as an explicit `--split`).
- **`+ new split…`** → prompt for a name (cooked-mode line input), run the existing create+authenticate flow (`claude setup-token`), record `folder → name`, then launch it.

## Structure

- New package `internal/selector`:
  - `decodeKey(r *bufio.Reader) (key, error)` — pure key decoding (arrow escape sequences, `j`/`k`, Enter, Ctrl-C/`q`/Esc).
  - `Run(in io.Reader, out io.Writer, items []Item) (Result, error)` — render/read loop over plain streams; `Result.Kind` is `Pick` (with `Name`), `New`, or `Cancel`. Rendering is hand-rolled ANSI (cursor-up redraw, reverse-video highlight).
- `internal/cli`:
  - `interactiveTTY()` and raw-mode setup/restore (`golang.org/x/term` `IsTerminal`, `MakeRaw`, `Restore`) wrap `selector.Run`.
  - The `PrintListExit` branch becomes: interactive → `runSelector` → funnel `Pick`/`New` into the existing explicit-launch path (token load, folder-memory record, `exec`); else fallback.

## Dependency

Adds `golang.org/x/term` (small, semi-official; pulls `golang.org/x/sys`). No TUI framework.

## Testing

- Unit: `decodeKey` table; `Run` driven by byte sequences (`↓ Enter` → `Pick` second item; navigate to last + `Enter` → `New`; `Ctrl-C` → `Cancel`) with rendered-frame assertions.
- The raw-mode TTY loop and the create-then-launch wiring are verified manually (interactive terminal).
