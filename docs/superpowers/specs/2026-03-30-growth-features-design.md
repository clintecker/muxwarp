# muxwarp v0.5.0 Growth Features Design

Five features to improve adoption, onboarding, and daily utility.

## 1. Session Metadata in TUI

### Scanner Changes

Expand the `tmux list-sessions -F` format string to include `#{session_created}` and `#{session_activity}` (tab-separated, appended to existing fields).

Add to `scanner.Session`:

```go
Created      int64  // Unix timestamp from #{session_created}
LastActivity int64  // Unix timestamp from #{session_activity}
```

Parse these as `int64` from the tab-separated output. If missing or unparseable (old tmux versions), default to `0` and hide the columns for that session.

### TUI Display

Full width (>=80 cols):

```text
▸  api-server   ◆ LIVE 2↗  ▪▪▪▪  3d  5m ago   atlas
   web-dev      ◇ IDLE     ▪▪    2w  1d ago    atlas
   monitoring   ◆ LIVE     ▪     6h  now       forge
   deploy       ◌ NEW                           forge
```

New columns:
- **Attached count**: shown as `2↗` only when `Attached > 1` (single-attach is already indicated by `LIVE` badge). Placed right after the badge.
- **Age**: time since `Created`. Compact format: `now`, `5m`, `2h`, `3d`, `2w`, `3mo`, `1y` — always the single largest applicable unit.
- **Last active**: time since `LastActivity`. Same compact format, suffixed with `ago`. Shows `now` if activity within last 60 seconds.

Ghost/NEW sessions show no metadata columns (no data available).

### Responsive Column Dropping

Columns drop in this order as terminal width shrinks (first listed = first to go):

1. Last-active (hidden below 80)
2. Age (hidden below 70)
3. Attached count (hidden below 65)
4. Window dots (hidden below 60 — existing behavior)
5. Badge text (hidden below 45 — existing behavior, keeps symbol only)

### Time Formatting

A small `internal/tui/timeformat.go` (or similar) with a function:

```go
func formatAge(t time.Time) string
```

Thresholds:
- < 60s: `now`
- < 60m: `Xm` (minutes)
- < 24h: `Xh` (hours)
- < 14d: `Xd` (days)
- < 60d: `Xw` (weeks)
- < 365d: `Xmo` (months)
- else: `Xy` (years)

## 2. Host Tags

### Config Changes

Add `Tags []string` to `HostEntry`:

```yaml
hosts:
  - target: clint@indigo
    tags: [prod, api]
    sessions:
      - name: api-server
  - target: deploy@atlas
    tags: [staging]
  - target: admin@forge
    tags: [prod, monitoring]
```

Tags are optional. An empty or missing `tags` field means the host has no tags. Tag names are freeform strings (lowercase recommended, no validation beyond non-empty).

### Data Flow

Tags flow from config through to TUI session items:
- `config.HostEntry.Tags` is set during config load
- When the scanner returns sessions for a host, the TUI model attaches the host's tags to each session item
- The TUI `Session` struct gains a `Tags []string` field

### TUI Tag Filtering

- **`t` keybinding**: opens an inline tag picker — a small overlay or footer list showing all unique tags discovered across all hosts
- Selecting a tag filters the session list to only sessions on hosts with that tag
- Active tag filter shown in footer: `tag: prod (3 sessions)`
- `t` again or `Esc` clears the tag filter
- Tag filter **composes** with the existing `/` fuzzy filter (both applied simultaneously)
- When no tags exist in config, `t` does nothing (or shows a hint)

No grouped display (section headers). Flat list with filtering only.

### Config Editor

The config editor (`internal/tui/editor/`) gains a tags field for host entries. Free-text comma-separated input. Autocomplete from existing tags in config.

## 3. `muxwarp init`

### Command

New subcommand recognized in `main.go` flag parsing:

```text
muxwarp init [--force]
```

### Behavior

1. Check if `~/.muxwarp.config.yaml` exists. If yes and `--force` not set, print message and exit 1.
2. Read `~/.ssh/config` via existing `sshconfig.Parse()`.
3. Filter out:
   - Wildcard hosts (already filtered by parser)
   - Common non-server hosts: `github.com`, `gitlab.com`, `bitbucket.org`, `bitbucket.com`, `ssh.dev.azure.com`, and any host containing `git` in the alias (configurable skip list not needed for v1)
4. Build a `config.Config` with default timeout/term and all remaining hosts (no sessions).
5. Marshal to YAML and write to `~/.muxwarp.config.yaml`.
6. Print summary: `Created ~/.muxwarp.config.yaml with N hosts from ~/.ssh/config`
7. Print hint: `Run 'muxwarp' to start scanning. Press 'e' in the TUI to edit config.`

### No Interactive Mode in This PR

The `--interactive` flag (Bubble Tea host checklist) is deferred to a future PR. Non-interactive only for now.

### Edge Cases

- No `~/.ssh/config`: print helpful message suggesting manual config creation, exit 1.
- `~/.ssh/config` exists but yields 0 valid hosts after filtering: print message explaining all hosts were filtered, exit 1. An empty config would fail `Load()` validation, so we don't write one.
- `--force` overwrites existing config without prompt.

## 4. Homebrew Tap + Shell Completions

### Homebrew Tap

Create a new GitHub repository: `clintecker/homebrew-tap`.

Add to `.goreleaser.yml`:

```yaml
brews:
  - name: muxwarp
    repository:
      owner: clintecker
      name: homebrew-tap
    homepage: https://github.com/clintecker/muxwarp
    description: "Warp into tmux sessions on remote machines"
    license: "MIT"
    install: |
      bin.install "muxwarp"
      bash_completion.install "completions/muxwarp.bash" => "muxwarp"
      zsh_completion.install "completions/muxwarp.zsh" => "_muxwarp"
      fish_completion.install "completions/muxwarp.fish" => "muxwarp.fish"
```

GoReleaser auto-generates and pushes the formula on each tagged release.

Users install via: `brew install clintecker/tap/muxwarp`

### Shell Completions

New flag: `muxwarp --completions bash|zsh|fish` — outputs the completion script to stdout.

Static completion scripts stored in `completions/` directory:
- `completions/muxwarp.bash`
- `completions/muxwarp.zsh`
- `completions/muxwarp.fish`

Completions cover:
- Flags: `--version`, `--help`, `--log`, `--completions`, `--force`
- Subcommands: `init`
- `--completions` completes with `bash`, `zsh`, `fish`
- `--log` expects a file path (use default file completion)
- `init` completes `--force`

No dynamic session/host completion in this version.

The `--completions` flag reads the corresponding file from an embedded `embed.FS` and prints to stdout. This keeps the scripts maintainable as standalone files while shipping them inside the binary.

### Goreleaser Extra Files

Add completions directory to the archive so Homebrew formula can reference them:

```yaml
archives:
  - id: tgz
    builds: [muxwarp]
    format: tar.gz
    files:
      - completions/*
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
```

## 5. VHS Demo Environment

### Docker Setup (`demo/`)

```text
demo/
  Dockerfile          # Alpine + openssh-server + tmux
  docker-compose.yml  # 4 containers on ports 2201-2204
  entrypoint.sh       # Creates tmux sessions, starts sshd
  id_ed25519          # Demo-only SSH key (not a secret)
  id_ed25519.pub      # Public key
  muxwarp.config.yaml # Config pointing at localhost:2201-2204
  demo.tape           # VHS recording script
```

### Containers

| Container | Hostname | Port | Pre-created Sessions |
|-----------|----------|------|---------------------|
| atlas     | atlas    | 2201 | `api-server` (3 windows), `web-dev` (2 windows) |
| forge     | forge    | 2202 | `monitoring` (1 window), `build-main` (3 windows) |
| nebula    | nebula   | 2203 | `data-pipeline` (2 windows) |
| comet     | comet    | 2204 | (no sessions — shows host with nothing running) |

### Dockerfile

Based on `alpine:latest`:
- Install `openssh-server`, `tmux`, `bash`
- Create user `demo` with the baked-in SSH public key in `~/.ssh/authorized_keys`
- Copy `entrypoint.sh`

### entrypoint.sh

1. Generate host keys if missing
2. Start tmux server as `demo` user
3. Create pre-defined sessions with specified window counts (sessions defined via environment variables)
4. Start sshd in foreground

### Demo Config

```yaml
defaults:
  timeout: "2s"
hosts:
  - target: demo@localhost:2201
    tags: [prod]
  - target: demo@localhost:2202
    tags: [prod, infra]
  - target: demo@localhost:2203
    tags: [staging]
  - target: demo@localhost:2204
    tags: [dev]
```

### VHS Tape (`demo/demo.tape`)

Records a ~30s demo:
1. Launch `muxwarp` with demo config
2. Wait for scan results
3. Navigate with `j`/`k`
4. Filter with `/api`
5. Clear filter
6. Filter by tag with `t`
7. Warp into a session

Output: `demo/demo.gif` (gitignored, generated on demand).

### Makefile Targets

```makefile
demo-up:      docker compose -f demo/docker-compose.yml up -d
demo-down:    docker compose -f demo/docker-compose.yml down
demo-record:  vhs demo/demo.tape
```

## Implementation Order

Within the single PR, implement in this order (each step builds on the previous):

1. **Session metadata** — scanner + TUI changes, no config changes
2. **Host tags** — config struct + TUI tag filter
3. **`muxwarp init`** — new subcommand, uses existing sshconfig parser
4. **Shell completions** — new flag + embedded scripts
5. **Homebrew tap** — goreleaser config (+ separate repo creation)
6. **VHS demo** — Docker environment + tape file (exercises all features above)
