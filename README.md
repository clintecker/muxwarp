# muxwarp

[![CI](https://github.com/clintecker/muxwarp/actions/workflows/ci.yml/badge.svg)](https://github.com/clintecker/muxwarp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/clintecker/muxwarp)](https://goreportcard.com/report/github.com/clintecker/muxwarp)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Warp into tmux sessions on remote machines.

muxwarp scans your configured SSH hosts in parallel, finds every running tmux
session, and presents them in a TUI. Pick one, hit enter, and you're in. No
session creation, no local tmux management, no SSH config duplication -- just
fast remote tmux attachment.

## What it looks like

```
▲ muxwarp ──────────────────────── 2 hosts · 5 sessions

▸  chatalpha     ◇ IDLE  ▪      indigo
   scenarios     ◇ IDLE  ▪      indigo
   cjdos         ◆ LIVE  ▪▪     indigo
   build-farm    ◇ IDLE  ▪▪▪    devbox
   monitoring    ◆ LIVE  ▪      devbox

↑/↓ navigate │ enter warp │ / filter │ r rescan │ q quit
```

## Install

### go install

```
go install github.com/clintecker/muxwarp/cmd/muxwarp@latest
```

### From source

```
git clone https://github.com/clintecker/muxwarp.git
cd muxwarp
make build
```

Binary goes to `bin/muxwarp`.

### Releases

Pre-built binaries for Linux and macOS (amd64/arm64) are available on the
[releases page](https://github.com/clintecker/muxwarp/releases).

## Quick start

Create `~/.muxwarp.config.yaml`:

```yaml
hosts:
  - user@server1
  - user@server2
```

Run it:

```
muxwarp
```

That's it. muxwarp scans both hosts, finds every tmux session, and shows them
in a navigable list. Pick one and press Enter to warp in.

## Config

Config lives at `~/.muxwarp.config.yaml`. No XDG, no env vars, no merging.

### Minimal

```yaml
hosts:
  - user@server1
  - user@server2
```

Each entry is an SSH target string passed directly to your system `ssh` binary.
Your `~/.ssh/config` aliases, ProxyCommand, agent forwarding -- it all works.

### Full (with defaults shown)

```yaml
defaults:
  timeout: 3s               # per-host SSH connect timeout
  term: xterm-256color       # TERM to set when attaching

hosts:
  - user@server1
  - user@server2
  - devbox                   # SSH config aliases work too
```

See [`examples/muxwarp.config.yaml`](examples/muxwarp.config.yaml) for an
annotated example.

## Usage

### TUI mode

```
muxwarp
```

Scans all hosts, shows every tmux session in a navigable list. Sessions stream
in as each host responds -- no waiting for the slowest one.

After ssh exits (e.g. you detach from tmux), you're returned to the TUI with a
fresh scan. Pick another session or press `q` to quit.

### Direct warp

```
muxwarp <name>
```

Fuzzy-matches `<name>` against session and host names across all hosts:

- **1 match** -- warp immediately, no TUI
- **Multiple matches** -- open TUI pre-filtered
- **0 matches** -- print error and exit

### Version

```
muxwarp --version
```

## Keybindings

| Key         | Action                     |
|-------------|----------------------------|
| `Up` / `k`  | Move selection up          |
| `Down` / `j`| Move selection down        |
| `Enter`     | Warp into selected session |
| `/`         | Toggle filter mode         |
| `Esc`       | Clear filter               |
| `r`         | Rescan all hosts           |
| `q`         | Quit                       |
| `Ctrl+C`    | Quit (works in any mode)   |

In filter mode, type to fuzzy-match against host and session names. Matched
characters are highlighted in the list.

## How it works

1. Reads `~/.muxwarp.config.yaml` for your list of SSH targets
2. Spawns one goroutine per host (up to 8 concurrent), each running:
   ```
   ssh -o ConnectTimeout=3 -o BatchMode=yes <target> \
     tmux list-sessions -F '#{session_name}\t#{session_attached}\t#{session_windows}'
   ```
3. Parses results, validates session names, streams them into the TUI
4. When you pick a session and press Enter, the TUI exits cleanly
5. A brief warp animation plays
6. `ssh -t <target> -- env TERM=xterm-256color tmux attach-session -t <session>`
   runs as a child process

After ssh exits, the TUI relaunches with a fresh scan so you can pick another
session. Single-match direct warp (`muxwarp <name>` with exactly one hit) uses
`syscall.Exec` for a clean process replacement instead.

Hosts that are down, fail auth (BatchMode=yes), or don't have tmux are silently
skipped. This is deliberate -- you're scanning known hosts, and missing ones
just mean fewer sessions in the list.

## Security

muxwarp is careful about what it executes:

- **No shell interpolation** -- SSH commands are constructed as argument arrays
  passed directly to `execve(2)`. No shell is involved.
- **Session name validation** -- names from remote hosts are checked against
  `[A-Za-z0-9._-]` (max 256 chars). Invalid names are silently dropped.
- **`--` separator** -- prevents session names from being interpreted as SSH
  flags.
- **BatchMode=yes** -- scanning uses non-interactive SSH to prevent password
  prompts from appearing inside the TUI.

## Requirements

- **Go 1.23+** to build
- **ssh** on your local machine (system binary, not a Go library)
- **tmux** on the remote hosts you're scanning

## Architecture

muxwarp is a single Go binary with four internal packages:

```
cmd/muxwarp/main.go        Entry point, arg parsing, orchestration
internal/config/            YAML config loading, defaults, validation
internal/scanner/           Parallel SSH scanning, result parsing
internal/tui/               Bubble Tea v2 TUI (model, view, update, styles)
internal/ssh/               SSH argv construction, validation, exec
```

See [`docs/architecture.md`](docs/architecture.md) for the full technical
design, data flow diagrams, and design decisions.

## Contributing

```
make hooks    # install pre-commit hooks
make check    # run lint + tests
```

The pre-commit hook enforces formatting, static analysis, linting (cyclomatic
complexity max 5), tests with race detector, and 70% minimum coverage.

See [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) for the full contributor
guide.

## License

[MIT](LICENSE)
