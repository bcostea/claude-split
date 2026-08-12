# claude-split for Windows -- a PowerShell port of github.com/bcostea/claude-split.
#
# The Go binary does not run here: internal/launcher/launcher.go calls
# syscall.Exec (Unix-only, so the package does not compile for GOOS=windows) and
# FindClaude looks for an extensionless `claude` carrying a Unix executable bit,
# while Windows has `claude.exe` and Go never sets 0111 on a regular file there.
#
# Same mechanism as upstream: point CLAUDE_CONFIG_DIR at ~/.claude-splits/<name>
# and run the real claude. Same on-disk layout too -- registry.json and
# folders.json in the same places with the same shape -- so switching to the Go
# binary later needs no migration.
#
# Deliberate differences from upstream, all Windows-driven:
#
#   * No minted CLAUDE_CODE_OAUTH_TOKEN. Upstream mints one so splits can share a
#     login; on Windows credentials live per config dir in
#     <CLAUDE_CONFIG_DIR>\.credentials.json, so each split simply signs in on its
#     own first launch. Nothing to mint, store, or keep in an OS secret store.
#   * No exec replacement. Windows has no execve, so the wrapper spawns claude as
#     a child and restores the environment afterwards.
#   * Folder pins match by longest path prefix, not exact path, so one pin covers
#     everything beneath it and a deeper pin can override a broader one.
#   * Pins are normalised through `subst` mappings. A subst drive gives one
#     directory two absolute paths and Resolve-Path canonicalises neither, so
#     without this a pin on each form both match and which profile you land in
#     depends on the path you typed -- silently splitting one project's history
#     across two profiles.
#   * A new split gets an `ide` junction to ~/.claude/ide, because the editor
#     extension advertises itself there and has no CLAUDE_CONFIG_DIR of its own.
#     Without it a split loses IDE attachment entirely.
#
# Install: dot-source from $PROFILE.
#     . "$env:USERPROFILE\.claude-splits\claude-split.ps1"
# Usage: cs [--split <name>] [claude args...]

$script:CsRoot = Join-Path $env:USERPROFILE '.claude-splits'
$script:CsFolders = if ($env:XDG_CONFIG_HOME) {
  Join-Path $env:XDG_CONFIG_HOME 'claude-split\folders.json'
} else {
  Join-Path $env:USERPROFILE '.config\claude-split\folders.json'
}

# ---- helpers ---------------------------------------------------------------

# `subst` drive mappings, as printed by subst.exe: "P:\: => C:\some\folder".
# Cached because CsNorm runs once per pin per launch; a subst added mid-shell needs
# a new shell to be picked up.
function script:CsSubstMap {
  if ($null -ne $script:CsSubst) { return $script:CsSubst }
  $map = @{}
  try {
    foreach ($line in (& "$env:SystemRoot\System32\subst.exe" 2>$null)) {
      $m = [regex]::Match($line, '^([A-Za-z]):\\?:\s+=>\s+(.+)$')
      if ($m.Success) {
        $map[($m.Groups[1].Value + ':').ToLowerInvariant()] = $m.Groups[2].Value.TrimEnd('\')
      }
    }
  } catch { }
  $script:CsSubst = $map
  return $map
}

function script:CsNorm([string]$p) {
  if (-not $p) { return '' }
  try { $p = (Resolve-Path -LiteralPath $p -ErrorAction Stop).Path } catch { }
  $p = $p.TrimEnd('\')
  if ($p.Length -ge 2 -and $p[1] -eq ':') {
    $sub = script:CsSubstMap
    $drive = $p.Substring(0, 2).ToLowerInvariant()
    if ($sub.ContainsKey($drive)) { $p = $sub[$drive] + $p.Substring(2) }
  }
  return $p.ToLowerInvariant()
}

# Honours CLAUDE_SPLIT_ASSUME_YES=1 and auto-confirms when stdin is not a console,
# matching upstream so scripted use does not hang.
function script:CsConfirm([string]$prompt) {
  if ($env:CLAUDE_SPLIT_ASSUME_YES) { return $true }
  if ([Console]::IsInputRedirected) { return $true }
  $ans = Read-Host $prompt
  return $ans -match '^(y|yes)$'
}

# Name fragments to leave out when seeding a new split, from
# CLAUDE_SPLIT_SEED_EXCLUDE (comma- or semicolon-separated). Empty by default:
# a new split is seeded with everything the home profile has.
function script:CsSeedExclusions {
  if (-not $env:CLAUDE_SPLIT_SEED_EXCLUDE) { return @() }
  return @($env:CLAUDE_SPLIT_SEED_EXCLUDE -split '[;,]' |
             ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

function script:CsExcluded([string]$name, [string[]]$exclusions) {
  foreach ($e in $exclusions) { if ($name -like "*$e*") { return $true } }
  return $false
}

# Removes a junction without following it. Remove-Item -Recurse can delete
# THROUGH a junction and take the target's contents with it, which for the `ide`
# junction means wiping ~/.claude/ide.
function script:CsRemoveJunction([string]$path) {
  if (-not (Test-Path $path)) { return }
  $i = Get-Item $path -Force
  if ($i.Attributes -band [IO.FileAttributes]::ReparsePoint) {
    [System.IO.Directory]::Delete($path, $false)
  }
}

function script:CsLoadRegistry {
  $f = Join-Path $script:CsRoot 'registry.json'
  if (Test-Path $f) {
    $r = Get-Content $f -Raw | ConvertFrom-Json
    return [pscustomobject]@{ splits = @($r.splits); default = [string]$r.default }
  }
  return [pscustomobject]@{ splits = @(); default = '' }
}

function script:CsSaveRegistry($reg) {
  New-Item -ItemType Directory -Force $script:CsRoot | Out-Null
  [ordered]@{ splits = @($reg.splits); default = $reg.default } |
    ConvertTo-Json -Depth 5 | Set-Content (Join-Path $script:CsRoot 'registry.json') -Encoding utf8
}

function script:CsLoadFolders {
  $map = [ordered]@{}
  if (Test-Path $script:CsFolders) {
    $o = Get-Content $script:CsFolders -Raw | ConvertFrom-Json
    foreach ($p in $o.PSObject.Properties) { $map[$p.Name] = [string]$p.Value }
  }
  return $map
}

function script:CsSaveFolders($map) {
  New-Item -ItemType Directory -Force (Split-Path $script:CsFolders) | Out-Null
  $map | ConvertTo-Json -Depth 5 | Set-Content $script:CsFolders -Encoding utf8
}

# Longest matching prefix wins, so a deeper pin can override a broader one.
function script:CsPinFor([string]$cwd, $map) {
  $c = script:CsNorm $cwd
  $best = $null; $bestLen = -1; $bestKey = $null
  foreach ($k in $map.Keys) {
    $kn = script:CsNorm $k
    if ($c -eq $kn -or $c.StartsWith($kn + '\')) {
      if ($kn.Length -gt $bestLen) { $best = $map[$k]; $bestLen = $kn.Length; $bestKey = $k }
    }
  }
  if ($null -eq $best) { return $null }
  return [pscustomobject]@{ split = $best; pin = $bestKey }
}

# Upstream errors on a value flag with no value, and on a bool flag handed one.
# Match that instead of silently taking $null or eating the next argument.
function script:CsFlagValue([string[]]$argv, [ref]$idx, [string]$flag, [string]$inline) {
  if ($inline) { return $inline }
  if ($idx.Value + 1 -ge $argv.Count) { throw "flag $flag requires a value" }
  $idx.Value = $idx.Value + 1
  return $argv[$idx.Value]
}

function script:CsNoValue([string]$flag, [string]$inline) {
  if ($inline) { throw "flag $flag does not take a value" }
}

function script:CsClaudeExe {
  $c = Get-Command claude -CommandType Application -ErrorAction SilentlyContinue |
       Select-Object -First 1
  if (-not $c) { throw 'claude.exe not found on PATH' }
  return $c.Source
}

# ---- seeding ---------------------------------------------------------------

function script:CsSeedSplit([string]$name) {
  # Not $home: PowerShell's $HOME is ReadOnly+AllScope, so assigning to it is a
  # non-terminating WriteError that leaves the variable at the user profile root
  # and sends every path below it one level too high.
  $homeDir = Join-Path $env:USERPROFILE '.claude'
  $dir     = Join-Path $script:CsRoot $name
  $excl    = script:CsSeedExclusions
  New-Item -ItemType Directory -Force $dir | Out-Null

  # Plugins are copied rather than shared: they auto-update, so sharing one
  # install risks two profiles racing on the same marketplace clone.
  $src = Join-Path $homeDir 'plugins'
  if (Test-Path $src) {
    $xd = @(); foreach ($e in $excl) { $xd += '/XD'; $xd += $e }
    robocopy $src (Join-Path $dir 'plugins') /E @xd /NFL /NDL /NJH /NJS /NP | Out-Null
    if ($LASTEXITCODE -ge 8) { throw "robocopy failed seeding plugins (exit $LASTEXITCODE)" }
    $global:LASTEXITCODE = 0
  }
  Copy-Item (Join-Path $homeDir 'commands') (Join-Path $dir 'commands') `
    -Recurse -Force -ErrorAction SilentlyContinue

  $ip = Join-Path $dir 'plugins\installed_plugins.json'
  $km = Join-Path $dir 'plugins\known_marketplaces.json'

  if ($excl.Count -and (Test-Path $ip)) {
    $j = Get-Content $ip -Raw | ConvertFrom-Json -AsHashtable
    if ($j['plugins']) {
      foreach ($k in @($j['plugins'].Keys)) {
        if (script:CsExcluded $k $excl) { $j['plugins'].Remove($k) }
      }
    }
    $j | ConvertTo-Json -Depth 10 | Set-Content $ip -Encoding utf8
  }
  if ($excl.Count -and (Test-Path $km)) {
    $j = Get-Content $km -Raw | ConvertFrom-Json -AsHashtable
    foreach ($k in @($j.Keys)) { if (script:CsExcluded $k $excl) { $j.Remove($k) } }
    $j | ConvertTo-Json -Depth 10 | Set-Content $km -Encoding utf8
  }

  # Repoint installPath at this split's own cache. Seeding copies the plugin trees
  # in, but the manifest still records the home profile's paths, leaving the split
  # silently dependent on ~/.claude/plugins surviving. The escaped form is what is
  # actually in the JSON; the plain replace catches any unescaped copy.
  if (Test-Path $ip) {
    $from = Join-Path $homeDir 'plugins\cache'
    $to   = Join-Path $dir 'plugins\cache'
    $raw = Get-Content $ip -Raw
    $raw = $raw.Replace($from.Replace('\', '\\'), $to.Replace('\', '\\'))
    $raw = $raw.Replace($from, $to)
    Set-Content $ip -Value $raw -Encoding utf8
  }

  # The editor extension advertises itself in ~/.claude/ide and has no
  # CLAUDE_CONFIG_DIR of its own, so without this junction the split's CLI reads a
  # directory the extension never writes to and loses IDE attachment.
  $ide = Join-Path $dir 'ide'
  if (-not (Test-Path $ide)) {
    New-Item -ItemType Directory -Force (Join-Path $homeDir 'ide') | Out-Null
    New-Item -ItemType Junction -Path $ide -Target (Join-Path $homeDir 'ide') | Out-Null
  }

  # User-scope rules are read from the profile's own config dir, so a split
  # without this copy silently drops every CLAUDE.md instruction.
  $md = Join-Path $homeDir 'CLAUDE.md'
  if ((Test-Path $md) -and -not (Test-Path (Join-Path $dir 'CLAUDE.md'))) {
    Copy-Item $md (Join-Path $dir 'CLAUDE.md') -Force
  }

  # Carry settings over, dropping only what the exclusion list names. Deliberately
  # NOT copied: .credentials.json -- each split signs in on its own.
  $settings = Join-Path $homeDir 'settings.json'
  if (Test-Path $settings) {
    $s = Get-Content $settings -Raw | ConvertFrom-Json -AsHashtable
    foreach ($key in @('enabledPlugins', 'extraKnownMarketplaces')) {
      if (-not $s.ContainsKey($key)) { continue }
      foreach ($k in @($s[$key].Keys)) {
        if (script:CsExcluded $k $excl) { $s[$key].Remove($k) }
      }
      if ($s[$key].Count -eq 0) { $s.Remove($key) }
    }
    $s | ConvertTo-Json -Depth 10 | Set-Content (Join-Path $dir 'settings.json') -Encoding utf8
  }
}

# ---- entry point -----------------------------------------------------------

# Deliberately a plain function with no param block. Any [Parameter()] attribute
# (or [CmdletBinding()]) makes this an advanced function, and PowerShell then binds
# claude's own flags to its common parameters: `cs -p "prompt"` fails with
# "parameter name 'p' is ambiguous. Possible matches include: -ProgressAction,
# -PipelineVariable" and never reaches claude. $args keeps every token verbatim,
# which is what a passthrough wrapper requires.
function Invoke-ClaudeSplit {
  $Arguments = @($args)

  $reg  = script:CsLoadRegistry
  $pass = [System.Collections.Generic.List[string]]::new()
  $split = $null; $splitSet = $false
  $cmd = $null; $cmdArg = $null

  for ($i = 0; $i -lt $Arguments.Count; $i++) {
    $a = $Arguments[$i]
    $name = $a; $inline = $null
    if ($a -like '--split*' -and $a.Contains('=')) {
      $name = $a.Substring(0, $a.IndexOf('=')); $inline = $a.Substring($a.IndexOf('=') + 1)
    }
    switch ($name) {
      '--split'         { $split = script:CsFlagValue $Arguments ([ref]$i) $name $inline; $splitSet = $true }
      '--split-new'     { $cmd = 'new';     $cmdArg = script:CsFlagValue $Arguments ([ref]$i) $name $inline }
      '--split-default' { $cmd = 'default'; $cmdArg = script:CsFlagValue $Arguments ([ref]$i) $name $inline }
      '--split-rm'      { $cmd = 'rm';      $cmdArg = script:CsFlagValue $Arguments ([ref]$i) $name $inline }
      '--split-pin'     { $cmd = 'pin';     $cmdArg = script:CsFlagValue $Arguments ([ref]$i) $name $inline }
      '--split-list'    { script:CsNoValue $name $inline; $cmd = 'list' }
      '--split-which'   { script:CsNoValue $name $inline; $cmd = 'which' }
      '--split-purge'   { script:CsNoValue $name $inline; $cmd = 'purge' }
      default           { $pass.Add($a) }
    }
  }

  $cwd = (Get-Location).Path
  $map = script:CsLoadFolders

  switch ($cmd) {
    'list' {
      $rows = @([pscustomobject]@{
        SPLIT = 'default (home)'
        DEFAULT = $(if ($reg.default -eq 'default') { '*' } else { '' })
        'SIGNED IN' = 'yes'
      })
      foreach ($s in $reg.splits) {
        $rows += [pscustomobject]@{
          SPLIT     = $s
          DEFAULT   = $(if ($reg.default -eq $s) { '*' } else { '' })
          'SIGNED IN' = $(if (Test-Path (Join-Path $script:CsRoot "$s\.credentials.json")) { 'yes' }
                          else { 'no -- signs in on first launch' })
        }
      }
      $rows | Format-Table -AutoSize
      if ($map.Count) {
        'Folder pins (longest prefix wins):'
        $map.GetEnumerator() | ForEach-Object { "  {0,-40} -> {1}" -f $_.Key, $_.Value }
      } else { 'No folder pins.' }
      return
    }
    'new' {
      if ($cmdArg -eq 'default') { Write-Error "'default' is reserved for the home profile"; return }
      if ($reg.splits -contains $cmdArg) { Write-Error "split '$cmdArg' already exists"; return }
      script:CsSeedSplit $cmdArg
      $reg.splits = @($reg.splits + $cmdArg)
      script:CsSaveRegistry $reg
      $excl = script:CsSeedExclusions
      $seeded = if ($excl.Count) { "plugins, commands, settings and CLAUDE.md (excluding: $($excl -join ', '))" }
                else { 'plugins, commands, settings and CLAUDE.md' }
      "Created split '$cmdArg' -- seeded with $seeded."
      "It signs in on first launch: cs --split $cmdArg"
      return
    }
    'default' {
      if ($cmdArg -ne 'default' -and -not ($reg.splits -contains $cmdArg)) { Write-Error "unknown split '$cmdArg'"; return }
      $reg.default = $cmdArg; script:CsSaveRegistry $reg
      "Global default split set to '$cmdArg'."
      return
    }
    'rm' {
      if (-not ($reg.splits -contains $cmdArg)) { Write-Error "unknown split '$cmdArg'"; return }
      if (-not (script:CsConfirm "Remove split '$cmdArg' from the registry? Its directory and sessions are LEFT ON DISK. [y/N]")) {
        'Aborted.'; return
      }
      $reg.splits = @($reg.splits | Where-Object { $_ -ne $cmdArg })
      if ($reg.default -eq $cmdArg) { $reg.default = '' }
      script:CsSaveRegistry $reg
      foreach ($k in @($map.Keys)) { if ($map[$k] -eq $cmdArg) { $map.Remove($k) } }
      script:CsSaveFolders $map
      "Removed '$cmdArg' from the registry. Directory kept at $(Join-Path $script:CsRoot $cmdArg)."
      return
    }
    'purge' {
      if (-not $reg.splits.Count -and -not (Test-Path $script:CsFolders)) { 'Nothing to purge.'; return }
      if (-not (script:CsConfirm "DELETE all splits under $($script:CsRoot) and the folder-pin config? The home profile is untouched. [y/N]")) {
        'Aborted.'; return
      }
      foreach ($s in $reg.splits) {
        $d = Join-Path $script:CsRoot $s
        # Drop the ide junction first, or -Recurse deletes through it.
        script:CsRemoveJunction (Join-Path $d 'ide')
        if (Test-Path $d) { Remove-Item $d -Recurse -Force -ErrorAction SilentlyContinue }
      }
      Remove-Item (Join-Path $script:CsRoot 'registry.json') -Force -ErrorAction SilentlyContinue
      Remove-Item $script:CsFolders -Force -ErrorAction SilentlyContinue
      'Purged every registered split and the claude-split config. The home profile was not touched.'
      # Only registered splits are deleted, and the root itself is left alone --
      # --split-rm keeps a removed split's directory on disk by design, and the
      # root may hold unrelated files. Say what was left rather than imply a
      # clean sweep.
      $left = @(Get-ChildItem $script:CsRoot -Directory -Force -ErrorAction SilentlyContinue)
      if ($left.Count) {
        "Left in place (not registered as splits): {0}" -f (($left | ForEach-Object Name) -join ', ')
        "Delete by hand if you want them gone: $($script:CsRoot)"
      }
      return
    }
    'pin' {
      if ($cmdArg -ne 'default' -and -not ($reg.splits -contains $cmdArg)) { Write-Error "unknown split '$cmdArg'"; return }
      $map[$cwd] = $cmdArg
      script:CsSaveFolders $map
      "Pinned $cwd (and everything under it) -> $cmdArg"
      return
    }
  }

  # ---- resolve: explicit --split > folder pin > global default > ask --------
  $reason = ''
  if ($splitSet) { $reason = 'explicit --split' }
  else {
    $hit = script:CsPinFor $cwd $map
    if ($hit) { $split = $hit.split; $splitSet = $true; $reason = "folder pin $($hit.pin)" }
    elseif ($reg.default) { $split = $reg.default; $splitSet = $true; $reason = 'global default' }
  }

  if (-not $splitSet) {
    if ($cmd -eq 'which') { '(none -- would prompt to choose)'; return }
    $choices = @('default') + $reg.splits
    "No split pinned for $cwd. Choose one:"
    for ($i = 0; $i -lt $choices.Count; $i++) {
      $label = if ($choices[$i] -eq 'default') { 'default (home profile)' } else { $choices[$i] }
      "  [{0}] {1}" -f ($i + 1), $label
    }
    $sel = Read-Host 'Number (or Enter to cancel)'
    if (-not $sel) { 'Cancelled.'; return }
    $idx = 0
    if (-not [int]::TryParse($sel, [ref]$idx) -or $idx -lt 1 -or $idx -gt $choices.Count) {
      Write-Error 'Invalid choice'; return
    }
    $split = $choices[$idx - 1]; $splitSet = $true; $reason = 'chosen interactively'
  }

  # ---- home-directory guard ------------------------------------------------
  # In $HOME, <cwd>\.claude *is* the home profile's config dir, and Claude Code
  # loads it as project scope regardless of CLAUDE_CONFIG_DIR, so a split there is
  # not actually isolated.
  if ($split -ne 'default' -and (script:CsNorm $cwd) -eq (script:CsNorm $env:USERPROFILE) -and
      -not $env:CLAUDE_SPLIT_ALLOW_HOME) {
    Write-Error "refusing to launch split '$split' from your home directory: ~/.claude would load as project config and defeat isolation. cd into a project, or set CLAUDE_SPLIT_ALLOW_HOME=1."
    return
  }

  if ($cmd -eq 'which') {
    if ($split -eq 'default') { "default (home profile)   [$reason]" } else { "$split   [$reason]" }
    return
  }

  $configDir = $null
  if ($split -ne 'default') {
    if (-not ($reg.splits -contains $split)) { Write-Error "unknown split '$split'"; return }
    $configDir = Join-Path $script:CsRoot $split
    if (-not (Test-Path $configDir)) { Write-Error "split directory missing: $configDir"; return }
  }

  # Remember an explicit choice for this folder, unless a pin already resolves it
  # to the same split (prefix matching makes a redundant child pin useless).
  if ($reason -in @('explicit --split', 'chosen interactively') -and
      (script:CsNorm $cwd) -ne (script:CsNorm $env:USERPROFILE)) {
    $existing = script:CsPinFor $cwd $map
    if (-not $existing -or $existing.split -ne $split) {
      $map[$cwd] = $split
      script:CsSaveFolders $map
      Write-Host "claude-split: pinned $cwd -> $split" -ForegroundColor DarkGray
    }
  }

  # No execve on Windows, so spawn as a child and restore the environment after.
  $exe = script:CsClaudeExe
  $had = Test-Path Env:\CLAUDE_CONFIG_DIR
  $old = $env:CLAUDE_CONFIG_DIR
  try {
    if ($configDir) { $env:CLAUDE_CONFIG_DIR = $configDir }
    elseif ($had)   { Remove-Item Env:\CLAUDE_CONFIG_DIR }
    & $exe @pass
  } finally {
    if ($had) { $env:CLAUDE_CONFIG_DIR = $old }
    elseif (Test-Path Env:\CLAUDE_CONFIG_DIR) { Remove-Item Env:\CLAUDE_CONFIG_DIR }
  }
}

Set-Alias cs Invoke-ClaudeSplit
Set-Alias claude-split Invoke-ClaudeSplit
