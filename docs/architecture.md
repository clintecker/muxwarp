# Architecture

Technical overview of muxwarp for contributors and the curious.

## Overview

muxwarp is a single Go binary with four internal packages. Config is loaded,
hosts are scanned over SSH in parallel, results are displayed in a Bubble Tea
v2 TUI, and selection triggers a `syscall.Exec` into ssh+tmux.

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│  config  │────>│ scanner │────>│   tui   │────>│   ssh   │
│  (YAML)  │     │(parallel│     │(Bubble  │     │(syscall │
│          │     │  SSH)   │     │  Tea v2)│     │  .Exec) │
└─────────┘     └─────────┘     └─────────┘     └─────────┘
```

## Package structure

```
cmd/muxwarp/main.go        Entry point, arg parsing, orchestration
internal/
  config/config.go         YAML parsing, defaults, validation
  scanner/scanner.go       Parallel SSH scanning, result parsing
  tui/
    model.go               Bubble Tea model, session types, sorting
    update.go              Key handling, message routing
    view.go                Rendering: header, list, footer, empty state
    filter.go              Fuzzy filter logic, match highlighting
    styles.go              Lip Gloss styles, Dracula-inspired palette
    warp.go                Warp animation (runs after TUI exits)
  ssh/
    exec.go                SSH argv construction, syscall.Exec handoff
    validate.go            Session name validation (security)
```

### config

Loads `~/.muxwarp.config.yaml`, applies defaults (`timeout: 3s`,
`term: xterm-256color`), validates that at least one host is configured.
Returns a `Config` struct consumed by the rest of the app.

### scanner

`ScanAll` fans out across hosts with bounded concurrency (semaphore channel,
max 8). Each goroutine runs `ssh ... tmux list-sessions` via
`exec.CommandContext` with a timeout. Results are parsed line-by-line, validated
via `ssh.ValidSessionName`, and delivered to the caller via a callback. Any
failure (timeout, auth, no tmux) returns zero sessions -- never an error.

### tui

The Bubble Tea v2 model. `NewModel` starts in scanning state;
`SessionBatchMsg` messages arrive as hosts respond, growing the list
incrementally. The list is re-sorted on each batch with selection stability
(cursor tracks by `host/session` key, not index).

Filter mode uses `github.com/sahilm/fuzzy` to match against
`hostShort/sessionName`, with matched character positions highlighted in the
rendered output.

### ssh

`BuildAttachArgs` constructs the argv for `syscall.Exec`. `BuildScanArgs`
constructs the argv for the scanner's `exec.CommandContext`. `ValidSessionName`
rejects anything outside `[A-Za-z0-9._-]` (max 256 chars). `ExecReplace`
does the actual `syscall.Exec` to hand off the process.

## Data flow

### TUI mode (`muxwarp`)

```
main()
  ├─ config.Load(~/.muxwarp.config.yaml)
  ├─ tui.NewModel(hostCount)
  ├─ tea.NewProgram(model)
  ├─ goroutine: scanner.ScanAll(hosts, callback)
  │    ├─ per host: ssh ... tmux list-sessions
  │    ├─ parse + validate session names
  │    └─ p.Send(SessionBatchMsg{...})  ← streams into TUI
  │
  ├─ p.Run()  ← blocks until user quits or warps
  │    ├─ Update: merges batches, re-sorts, maintains cursor
  │    ├─ View: renders header, list, footer
  │    └─ user presses Enter → sets warpTarget, returns tea.Quit
  │
  ├─ p.Run() returns  ← terminal fully restored
  ├─ playWarpAnimation()  ← plain fmt.Print, ~200ms
  └─ ssh.ExecReplace()  ← syscall.Exec, never returns
```

### Direct mode (`muxwarp <name>`)

```
main()
  ├─ config.Load()
  ├─ scanner.ScanAll() (blocking, with stderr spinner)
  ├─ fuzzyMatch(pattern, allSessions)
  ├─ 0 matches → error, exit 1
  ├─ 1 match → warp animation + ExecReplace
  └─ N matches → tui.NewModelWithSessions(prefiltered) → normal TUI flow
```

## Key design decisions

### System ssh, not crypto/ssh

muxwarp shells out to your system `ssh` binary rather than using Go's
`crypto/ssh` library. This means:

- Full `~/.ssh/config` support (aliases, ProxyCommand, ControlMaster)
- ssh-agent, FIDO/U2F keys, hardware tokens all work
- No SSH implementation to maintain or debug
- Behavior matches what you'd get typing `ssh` yourself

The tradeoff is a runtime dependency on `ssh` being in PATH, which is
effectively universal on the target platforms (macOS, Linux).

### Custom TUI components (no bubbles)

Bubble Tea v2 (`charm.land/bubbletea/v2`) uses a new `tea.View` return type
and different message types (`tea.KeyPressMsg` instead of `tea.KeyMsg`). The
existing bubbles v1 components (`spinner`, `list`, `textinput`) import
Bubble Tea v1 and are not type-compatible with v2.

Rather than wrapping v1 components in adapters, muxwarp implements its own
list rendering, cursor management, scrolling viewport, and filter input
directly. This keeps the code simple and avoids fighting type mismatches.

Key v2 differences documented in `v2_smoke_test.go`:
- `View()` returns `tea.View`, not `string`
- Alt screen is set via `view.AltScreen = true`, not a program option
- `tea.KeyPressMsg` replaces `tea.KeyMsg` for key press events
- Space bar: `msg.String()` returns `"space"`, not `" "`
- `tea.Msg` is `uv.Event` (ultraviolet event interface)

### Two-phase quit-then-exec

When the user presses Enter to warp:

1. The model sets `warpTarget` and returns `tea.Quit`
2. `p.Run()` returns -- Bubble Tea restores the terminal (exits alt screen,
   re-enables cooked mode, shows cursor)
3. The warp animation prints to stdout (plain `fmt.Print`)
4. `syscall.Exec` replaces the process with ssh

This ordering is critical. If we called `syscall.Exec` while Bubble Tea still
owned the terminal (raw mode, alt screen), ssh would inherit a broken TTY --
no echo, hidden cursor, mangled input. Letting `Run()` return first ensures a
clean terminal state for the ssh session.

### Session name validation

Session names come from parsing `tmux list-sessions` output on remote hosts.
A malicious or accidental name like `foo; rm -rf ~` would be dangerous if
it reached a shell. Two defenses:

1. **Validation**: names are checked against `^[A-Za-z0-9._-]{1,256}$` during
   parsing. Invalid names are silently dropped.
2. **No shell interpolation**: SSH commands are constructed as separate argv
   elements, never assembled into a shell command string. The `--` separator
   prevents the session name from being interpreted as ssh flags.

### No shell interpolation in SSH commands

Both scanning and attaching build argument arrays, never command strings:

```go
// Scanning
[]string{"ssh", "-o", "ConnectTimeout=3", "-o", "BatchMode=yes", target,
    "tmux", "list-sessions", "-F", format}

// Attaching
[]string{"ssh", "-t", target, "--",
    "env", "TERM=xterm-256color", "tmux", "attach-session", "-t", sessionName}
```

Each element is a distinct argument to `execve(2)`. No shell is involved.

## Bubble Tea v2 patterns

For anyone maintaining the TUI code, here are the v2 patterns used:

### Model interface

```go
type Model struct { ... }
func (m Model) Init() tea.Cmd        { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
func (m Model) View() tea.View       { ... }
```

`View()` returns `tea.View`, not `string`. Create one with:

```go
v := tea.NewView(content)
v.AltScreen = true
return v
```

### Message types

- `tea.WindowSizeMsg{Width, Height}` -- terminal resize
- `tea.KeyPressMsg{Code, Text, Mod}` -- key press (use `msg.String()` for key name)
- Custom messages: define a struct, send via `p.Send(msg)` from goroutines

### Sending from goroutines

```go
p := tea.NewProgram(model)
go func() {
    // do work...
    p.Send(MyResultMsg{data})
}()
p.Run()
```

### Quit flow

Return `tea.Quit` (which is `func() tea.Msg`) as a command from `Update`:

```go
return m, tea.Quit
```

## Testing strategy

### Unit test coverage

Each package has focused tests:

- **config**: load valid/invalid YAML, defaults, missing file, empty hosts
- **ssh/validate**: valid names, every category of invalid character
- **ssh/exec**: argv construction, no-shell-interpolation verification,
  `--` separator presence
- **scanner**: full scan pipeline, timeout, invalid names filtered out
- **tui/model**: model initialization, sorting, session key identity
- **tui/update**: key handling (navigation, filter, enter, quit, rescan),
  message processing (SessionBatchMsg, ScanDoneMsg, WindowSizeMsg)
- **tui/view**: header rendering during/after scan, footer modes,
  empty state, content presence
- **tui/warp**: frame generation, growing bar, animation output

### Fake SSH for scanner tests

Scanner tests can't hit real hosts. Instead, `withFakeSSH` creates a shell
script in a temp directory and prepends it to `PATH`:

```go
func withFakeSSH(t *testing.T, script string) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755)
    t.Setenv("PATH", dir + ":" + os.Getenv("PATH"))
}
```

The fake script can simulate success (print tmux output), failure (exit 1),
timeout (sleep), or mixed valid/invalid session names. This tests the real
`exec.CommandContext` code path without network access.

### Bubble Tea v2 smoke tests

`v2_smoke_test.go` validates the BT v2 API surface: `tea.NewView`,
`tea.KeyPressMsg`, `tea.Quit` type, `p.Send`, program options. It also
documents the bubbles v1 incompatibility finding. These tests serve as both
validation and living documentation of the v2 API.

### What's NOT tested

- `syscall.Exec` (replaces the process -- can't test in-process)
- Full interactive TUI flow (no teatest in v2 yet)
- Real SSH connections (by design -- this is a personal tool)
