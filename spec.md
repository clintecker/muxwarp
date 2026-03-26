# muxwarp spec

> Warp into tmux sessions on remote machines.

## Overview

muxwarp is a single Go binary that discovers tmux sessions running on remote machines and lets you warp into them. It SSHes to your configured hosts in parallel, collects all running tmux sessions, and presents them in a fun, whimsical TUI. Pick a session, hit enter, and you're in.

No session creation. No local tmux management. No SSH config duplication. Just fast, beautiful remote tmux attachment.

## Install

```
go install github.com/clintecker/muxwarp@latest
```

Single static binary. No runtime dependencies beyond `ssh` on your machine and `tmux` on the remote hosts.

## Usage

### TUI mode (default)

```
muxwarp
```

Scans all configured hosts in parallel, presents a flat interleaved list of every tmux session found. Navigate with keyboard, hit enter to warp.

### Direct warp

```
muxwarp <name>
```

Fuzzy-matches `<name>` against session names (and host names) across all hosts.
- **Single match** -> warp immediately, no TUI
- **Multiple matches** -> open TUI pre-filtered to matches
- **Zero matches** -> print error and exit: `no sessions matching "<name>"`

## Config

`~/.muxwarp.config.yaml`

### Minimal (most users)

```yaml
hosts:
  - alice@atlas
  - alice@forge
```

That's it. Each entry is an SSH target string passed directly to your system `ssh` binary. Leverages your existing `~/.ssh/config`, ssh-agent, keys, ProxyCommand — everything.

### With options

```yaml
defaults:
  timeout: 3s             # per-host SSH timeout (default: 3s)
  term: xterm-256color    # TERM to set on attach (default: xterm-256color)

hosts:
  - alice@atlas
  - alice@forge
```

### Config resolution

1. `~/.muxwarp.config.yaml`
2. If not found, print friendly error with example config and exit

No env var overrides, no XDG, no merging. One file, one location.

## TUI Design

### Layout

```
▲ muxwarp ─────────────────────────────────── 2 hosts · 4 sessions

▸  api-server     ◇ IDLE  ▪▪▪▪▪                           atlas
   web-dev        ◇ IDLE  ▪▪                               atlas
   build-main     ◇ IDLE  ▪▪▪                              forge
   monitoring     ◆ LIVE  ▪                                forge

↑/↓ navigate │ enter warp │ / filter │ r rescan │ q quit
```

### Header

A single-line header: `▲ muxwarp ──gradient── status`. The `─` rule uses a neon-blue-to-electric-purple gradient. Right-aligned status:

- While scanning: `Spooling drives… {done}/{total}`
- After scan: `{N} host(s) · {N} session(s)` (properly pluralized)

### Session list

A single flat list. Every session from every host, interleaved. Each row:

```
{selector} {session}  {badge}  {dots}  ···padding···  {host}
```

- **selector**: `▸` (bright cyan) or blank
- **session**: tmux session name, left-aligned. Color: light text
- **badge**:
  - `◆ LIVE` (green) — session has attached clients
  - `◇ IDLE` (cyan) — session is detached
- **dots**: `▪` repeated per window count. Color: dim slate
- **host**: short name extracted from the SSH target (e.g., `alice@atlas` -> `atlas`), right-aligned, very dim. Color: #4A4A5E with faint

The selected row gets a subtle background tint.

### Adaptive column layout

As terminal width shrinks, columns adapt:

- Width >= 80: `▸  name  ◇ IDLE  ▪▪  ···padding···  host`
- Width >= 60: `▸  name  ◇  ▪▪  ···padding···  host` (badge text drops)
- Width >= 45: `▸  name  ◇  ···padding···  host` (dots drop)
- Width < 45: `▸  name  ◇  ···padding···  hos` (host truncates to 3 chars)

### Sorting

1. IDLE (detached) sessions first — these are the ones you probably want
2. Then LIVE (attached)
3. Within each group: alphabetical by host, then session name

### Filter mode

Press `/` to activate inline filter at the bottom of the list. Fuzzy-matches against host name + session name. List filters in real-time as you type. **Matched characters are highlighted** (bold/bright) in both host and session columns for visual feedback. A match count appears in the footer: `3 matches`.

Press `Esc` to clear filter, `Enter` to warp into selected match.

### Footer

Single line, dim, context-sensitive:
- **Normal**: `↑/↓ navigate │ enter warp │ / filter │ r rescan │ q quit`
- **Filtering**: `type to filter │ enter warp │ esc clear` + right-aligned match count
- **Empty**: `r rescan │ q quit`

### Color palette

| Role         | Color   | Hex     |
|-------------|---------|---------|
| Primary     | Cyan    | #8BE9FD |
| Accent      | Lavender| #BD93F9 |
| Success     | Green   | #2EE6A6 |
| Error       | Red     | #FF5555 |
| Dim         | Slate   | #6B7280 |
| Text        | Light   | #E6E6E6 |

Dracula-inspired. Use `lipgloss.AdaptiveColor` to provide 256-color fallbacks for each, ensuring the tool looks good on any terminal.

## Warp Sequence

When the user presses Enter on a session:

1. Bubble Tea model sets the pending warp target and returns `tea.Quit`
2. `program.Run()` returns, terminal state is fully restored (alt screen off, cooked mode)
3. Brief animation (~200ms total, 4 frames at ~50ms each) rendered with plain fmt.Print:

```
engaging jumpgate: atlas/api-server █
engaging jumpgate: atlas/api-server ████
engaging jumpgate: atlas/api-server ████████
engaging jumpgate: atlas/api-server ██████████████
```

Block bar uses a cyan-to-lavender gradient, width capped to `terminal_width - label_length`.

4. `syscall.Exec` replaces the process with ssh (see SSH Exec section)

### Why two-phase (quit then exec)?

Bubble Tea owns the terminal (raw mode, alt screen). If we `syscall.Exec` while Bubble Tea is still running, ssh inherits a broken TTY — no echo, hidden cursor, mangled input. By letting `Run()` return first, the terminal is cleanly restored before ssh takes over.

### Direct warp mode

`muxwarp api-server` skips the TUI entirely on single match:
1. Scan all hosts (same parallel scan, with a CLI spinner)
2. Fuzzy match "api-server" against all session names
3. Single match -> warp animation + exec
4. Multiple matches -> open TUI pre-filtered
5. Zero matches -> `error: no sessions matching "api-server"` and exit 1

## SSH Exec — Command Construction and Security

### The problem

Session names come from the remote host's tmux output. A malicious or accidental session name like `foo; rm -rf ~` would be dangerous if interpolated into a shell command string.

### The solution: no shell interpolation

Instead of passing a shell command string to ssh, construct the remote command to avoid shell interpretation of the session name:

```
ssh -t <target> -- tmux attach-session -t <session_name>
```

The `TERM` is set via ssh's `SendEnv` or by prepending `env TERM=xterm-256color` as a separate argument. The session name is never passed through a shell.

Additionally, **validate session names** when parsing `tmux list-sessions` output:
- Allow only `[A-Za-z0-9._-]` (tmux's own default allowed characters)
- Reject/skip any session with names outside this charset
- Max length: 256 characters

### syscall.Exec

```go
sshPath, _ := exec.LookPath("ssh")
syscall.Exec(sshPath, []string{
    "ssh", "-t", target, "--",
    "env", "TERM=" + term, "tmux", "attach-session", "-t", sessionName,
}, os.Environ())
```

This replaces the muxwarp process entirely — clean TTY handoff, no orphaned parent, no signal forwarding needed. If `exec.LookPath` fails, print error and exit 1.

### Platform notes

`syscall.Exec` works identically on macOS (darwin) and Linux. It's a direct `execve(2)` call. No platform-specific concerns.

## Scanning

### How it works

1. Parse config, build list of SSH targets
2. Launch one goroutine per host, bounded by semaphore (max 8 concurrent)
3. Each goroutine runs via `exec.CommandContext` with a timeout:
   ```
   ssh -o ConnectTimeout=3 -o BatchMode=yes <target> \
     tmux list-sessions -F '#{session_name}\t#{session_attached}\t#{session_windows}'
   ```
4. Parse stdout: split lines on `\t` -> session name, attached count, window count
5. Validate each session name against allowed charset, skip invalid ones
6. Send results to TUI via Bubble Tea messages as they arrive

### Timeout and cancellation

Each SSH scan command is wrapped in `exec.CommandContext` with a `context.WithTimeout`. This provides a hard timeout that works even when SSH's own `ConnectTimeout` doesn't fire (e.g., DNS hangs, TCP blackholes). The context is derived from a parent context that gets cancelled if the user quits the TUI mid-scan.

```go
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()
cmd := exec.CommandContext(ctx, "ssh", args...)
```

### Incremental rendering

Sessions appear in the TUI as each host responds. No waiting for the slowest host. As results stream in, the list grows and re-sorts in place.

**Selection stability**: when the list re-sorts, the currently selected item is tracked by identity (`host + "/" + session_name`), not by index. The cursor follows the logical item, not the position.

### Error handling

| Condition | Behavior |
|-----------|----------|
| SSH `ConnectTimeout` / context timeout | Host silently omitted |
| `BatchMode=yes` auth failure | Host silently omitted |
| `tmux list-sessions` exit 1 ("no server running") | Treated as 0 sessions |
| `tmux` not found on remote | Host silently omitted |
| All hosts fail | Show empty state |

Silent omission is deliberate — this is a personal tool scanning known hosts. If a host is down, you don't need an error banner, you just don't see its sessions.

### Why system ssh?

Using the system `ssh` binary (not Go's `crypto/ssh`) means:
- Respects `~/.ssh/config` (aliases, ProxyCommand, ControlMaster, etc.)
- Works with ssh-agent, FIDO/U2F keys, hardware tokens
- No SSH implementation bugs to worry about
- Familiar behavior — it's literally ssh

## Empty State

When all hosts return zero sessions:

```
▲ muxwarp ─────────────────────────────────── 2 hosts · 0 sessions

All gates are calm — no active lanes detected.

Start a session:  ssh <host> -t tmux new -s <name>

r rescan │ q quit
```

## Architecture

### Project structure

```
muxwarp/
  cmd/
    muxwarp/
      main.go           # entry point, arg parsing, config loading
  internal/
    config/
      config.go         # YAML parsing, defaults, validation
    scanner/
      scanner.go        # parallel SSH scanning, result parsing
    tui/
      model.go          # Bubble Tea model, update, view
      styles.go         # Lip Gloss styles, palette, adaptive colors
      header.go         # banner rendering
      warp.go           # warp animation (runs after TUI exits)
    ssh/
      exec.go           # build SSH command args, syscall.Exec handoff
      validate.go       # session name validation
  go.mod
  go.sum
```

### Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — list, spinner, textinput components
- `github.com/charmbracelet/lipgloss` — styling
- `gopkg.in/yaml.v3` — config parsing
- `github.com/sahilm/fuzzy` — fuzzy matching (used by bubbles list, and for direct warp mode)

No SSH libraries. No network libraries. Just TUI + YAML + the system ssh binary.

### Key types

```go
// Config
type Config struct {
    Defaults Defaults `yaml:"defaults"`
    Hosts    []string `yaml:"hosts"`
}

type Defaults struct {
    Timeout string `yaml:"timeout"` // e.g. "3s", default "3s"
    Term    string `yaml:"term"`    // e.g. "xterm-256color"
}

// Scanner results
type Session struct {
    Host      string // SSH target (alice@atlas)
    HostShort string // display name (atlas)
    Name      string // tmux session name (validated)
    Attached  int    // number of attached clients
    Windows   int    // number of windows
}

// Stable identity for selection tracking
func (s Session) Key() string {
    return s.Host + "/" + s.Name
}

// TUI state
type model struct {
    sessions    []Session
    list        list.Model
    scanning    bool
    scanDone    int
    scanTotal   int
    quitting    bool
    warpTarget  *Session  // set on Enter, triggers tea.Quit
}
```

### Flow

```
main()
  -> loadConfig()
  -> if arg provided: directWarp(arg)
  -> else: runTUI()

runTUI()
  -> p := tea.NewProgram(initialModel, tea.WithAltScreen())
  -> model.Init() starts scanner goroutines
  -> scanner sends SessionMsg per host via tea.Program.Send
  -> model.Update merges results, re-sorts list (preserving selection by key)
  -> user selects session, presses Enter
  -> model.Update sets warpTarget, returns tea.Quit
  -> p.Run() returns (terminal restored)
  -> if warpTarget != nil:
       -> playWarpAnimation(warpTarget)    // plain prints, ~200ms
       -> sshExec(warpTarget)              // syscall.Exec, never returns

directWarp(pattern)
  -> scanAll() (blocking, with CLI spinner)
  -> fuzzyMatch(pattern, sessions)
  -> 0 matches: print error, exit 1
  -> 1 match: playWarpAnimation + sshExec
  -> N matches: runTUI(prefiltered)
```

## Keybindings

| Key       | Action                    |
|-----------|---------------------------|
| `↑` / `k` | Move selection up         |
| `↓` / `j` | Move selection down       |
| `Enter`   | Warp into selected session|
| `/`        | Toggle filter mode        |
| `Esc`      | Clear filter              |
| `r`        | Rescan all hosts          |
| `q`        | Quit                      |

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Config file missing | Print friendly error with example config, exit 1 |
| Config file malformed | Print parse error with line number, exit 1 |
| Empty hosts list | Print "no hosts configured", exit 1 |
| All hosts unreachable | Show empty state with rescan option |
| Some hosts unreachable | Show sessions from reachable hosts only |
| Host has no tmux | Silently omit (same as 0 sessions) |
| Host has no tmux server | Silently omit (same as 0 sessions) |
| SSH key issues (BatchMode) | Host silently omitted; no password prompts |
| Terminal too narrow (<40) | Compact header + adaptive column hiding |
| Terminal resize during scan | Bubble Tea handles SIGWINCH natively |
| Direct warp, 0 matches | Print error, exit 1 |
| Direct warp, 1 match | Warp animation + exec, no TUI |
| Direct warp, N matches | Open TUI pre-filtered |
| Malicious session name | Validated against `[A-Za-z0-9._-]`, rejected if invalid |
| Session name with special chars | Rejected during scan parsing, never reaches exec |
| ssh binary not found | Print error, exit 1 |
| Very long session list | Bubbles list handles scrolling natively |

## What's NOT in v1

- Session creation (`tmux new`)
- Mosh transport
- Per-host labels/nicknames
- SSH ControlMaster management
- Local tmux session management
- Session preview / window list drill-down
- Config hot-reload
- Shell completions
- Themes / customizable colors
