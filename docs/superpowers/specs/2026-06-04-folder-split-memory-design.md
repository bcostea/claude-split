# Per-folder split memory — Design

**Date:** 2026-06-04
**Status:** Approved design, pre-implementation

## Summary

When a split is launched *explicitly* (`--split <name>`) from a project folder, `claude-split` records a `folder → split` association. On a later bare launch (no `--split`) in that same folder, the remembered split is loaded automatically. This removes the need to repeatedly type `--split <name>` in a project you always use with the same split.

## Resolution order

```
explicit --split  >  folder memory  >  global default  >  prompt
```

Implemented by computing an *effective default* = the folder's remembered split (if present and still valid) else the global default, then feeding that into the existing `resolve.Resolve`. Folder memory therefore overrides the global default, while the home guard, missing-token handling, and the ambiguous-prompt behavior are unchanged.

## Storage

Path: `${XDG_CONFIG_HOME:-~/.config}/claude-split/folders.json` — a flat JSON map of canonical absolute folder path to split name:

```json
{
  "/Users/bogdan/code/xogito": "xogito",
  "/Users/bogdan/code/foo": "default"
}
```

Folder keys are canonicalized (absolute + symlink-resolved) so `.`, symlinks, and trailing slashes collapse to one key. The value `default` pins a folder to the home profile.

## Recording rules

- Records **only on an explicit `--split <name>`** (including `--split default`). This is deliberate: a launch that merely falls through to the global default is *not* recorded, otherwise the global default would silently pollute every folder and then permanently shadow itself.
- A bare launch that resolves *via* folder memory re-affirms the same value (a no-op write that is skipped when unchanged).
- Never records when the working directory is the home directory (the home guard means splits do not run there).

## Self-healing

When a folder's remembered split no longer exists (it was `--split-rm`'d), the lookup ignores it, falls back to the global default / prompt, and prunes the stale entry from `folders.json`.

## Structure

- New package `internal/folders`: load/get/set/delete/save over the JSON map, `XDG_CONFIG_HOME`-aware, with an injectable path for tests (`Load(path)`, `DefaultPath()`).
- `internal/cli`: canonicalizes cwd (reusing the existing `canonDir` helper), computes the effective default via a small pure `effectiveDefault(store, reg, folder)` function (unit-tested, including the prune case), passes it to the resolver, and records the folder→split mapping after an explicit choice resolves to a launch and before `exec`.

## Testing

- **Unit:** `folders` round-trip, missing file, `DefaultPath` honoring `XDG_CONFIG_HOME` and falling back to `~/.config`; `effectiveDefault` covering present/absent/stale-prune cases.
- **E2E:** explicit `--split work` from a temp project folder, then a bare launch in the same folder invokes the fake `claude` with that split's `CLAUDE_CONFIG_DIR` + token; a stale-entry launch falls back to the global default and prunes the entry.

## Out of scope (YAGNI)

No dedicated "forget this folder" command — re-pin with `--split default` or another name. Trivial to add later if wanted.
