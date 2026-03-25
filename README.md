# muxwarp

Warp into tmux sessions on remote machines.

muxwarp scans your configured SSH hosts in parallel, finds every running tmux
session, and presents them in a TUI. Pick one, hit enter, and you're in. No
session creation, no local tmux management, no SSH config duplication -- just
fast remote tmux attachment.

## Quick start

```
go install github.com/clint/muxwarp@latest
```

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

## What it looks like

```
╭──── muxwarp ────╮    2 hosts · 4 sessions
│ muxwarp — warp  │
│   to tmux       │
╰─────────────────╯

  > [server1] dev            ○ FREE    w3
    [server1] build-farm     ○ FREE    w2
    [server2] hacking        ○ FREE    w3
    [server2] monitoring     ● DOCKED  w1

  ↑/↓ navigate · enter warp · / filter · r rescan · q quit
```

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
  - devbox
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
6. `syscall.Exec` replaces the process with:
   ```
   ssh -t <target> -- env TERM=xterm-256color tmux attach-session -t <session>
   ```

The process replacement means clean TTY handoff -- no orphaned parent, no
signal forwarding needed. You're just in ssh+tmux.

Hosts that are down, fail auth (BatchMode=yes), or don't have tmux are silently
skipped. This is deliberate -- you're scanning known hosts, and missing ones
just mean fewer sessions in the list.

## Requirements

- **Go 1.23+** to build
- **ssh** on your local machine (system binary, not a Go library)
- **tmux** on the remote hosts you're scanning

## Building from source

### go install

```
go install github.com/clint/muxwarp@latest
```

### make

```
git clone https://github.com/clint/muxwarp.git
cd muxwarp
make build
```

Binary goes to `bin/muxwarp`.

### goreleaser

```
goreleaser release --snapshot --clean
```

Builds for linux/darwin on amd64/arm64. See `.goreleaser.yml`.

## Contributing

```
make hooks    # install pre-commit hooks
make check    # run lint + tests
```

See [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) for the full guide.

## License

MIT
