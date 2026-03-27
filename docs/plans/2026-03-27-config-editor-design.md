# Config Editor: In-TUI Config Management

## Overview

A full-screen config editor inside the muxwarp TUI that lets users add, edit,
and delete hosts and desired sessions without ever opening a YAML file. Includes
a first-run wizard for onboarding and SSH config autocomplete for host entry.

## Design Decisions

- **Full-screen editor** — replaces the session list when active. Clean, focused,
  no overlay compositing needed.
- **bubbles v2** — use `charm.land/bubbles/v2` (textinput, etc.) for form
  components. Compatible with Bubble Tea v2.
- **No YAML comment preservation** — the TUI owns the config. `config.Save()`
  does a clean typed marshal. Simple and correct.
- **SSH config autocomplete** — parse `~/.ssh/config` for Host aliases, offer
  inline ghost-text completion and a Ctrl+Space dropdown picker with metadata
  preview (HostName, User, Port).
- **First-run wizard** — 2-step guided flow when no config exists. Same editor
  components, step-based layout.

## Modes

The TUI Model gains a `Mode` enum:

```
ModeList    — existing session list (filter, warp, rescan)
ModeEdit    — full-screen config editor
ModeWizard  — first-run guided setup
```

### List Mode (existing, new keybindings)

| Key | Action |
|-----|--------|
| `a` | Add new host (opens editor) |
| `e` | Edit host of selected session (opens editor) |
| `n` | Add new session to selected host (opens editor, session fields focused) |
| `d` | Delete selected host (inline y/n confirm) |

Footer updates to show these hints when sessions are present.

### Editor Mode

Full-screen form with these regions:

```
▲ muxwarp ──────────────────────── Edit Host

  Host target
  ┌──────────────────────────────────────────┐
  │ alice@atlas                              │
  └──────────────────────────────────────────┘
    SSH target string (e.g. user@host, hostname, IP)

  Sessions                          ┌─ Config Preview ─────────┐
  ┌──────────────────────────────┐  │ - target: alice@atlas    │
  │ ▸ api-server  ~/code/api     │  │   sessions:              │
  │   web-dev     ~/code/web     │  │     - name: api-server   │
  │                              │  │       dir: ~/code/api    │
  │   ctrl+n add │ ctrl+d delete │  │     - name: web-dev      │
  └──────────────────────────────┘  │       dir: ~/code/web    │
                                    └──────────────────────────┘
  Session: api-server
  ┌──────────────────────────────────────────┐
  │ api-server                               │  name
  └──────────────────────────────────────────┘
  ┌──────────────────────────────────────────┐
  │ ~/code/api                               │  dir (optional)
  └──────────────────────────────────────────┘
  ┌──────────────────────────────────────────┐
  │ nvim                                     │  cmd (optional)
  └──────────────────────────────────────────┘

ctrl+s save │ esc cancel │ tab next field
```

#### Form fields

Each field is a `textinput.Model` from bubbles v2 with:
- Placeholder text (dim, shown when empty)
- Helper text below (dim, context-specific)
- Validation callback
- Focused state: cyan border + left bar. Unfocused: slate border.
- Valid: dim `✓` after border. Invalid: red error text replaces helper.

#### Focus cycling

Tab / Shift-Tab rotates: host → session list → name → dir → cmd.
Delegated to the focused component. Global keys (Ctrl+S, Esc) intercepted first.

#### Session list

Mini navigable list within the editor:
- Arrow keys navigate when focused
- Ctrl+N adds a new empty session
- Ctrl+D deletes with inline "y/n" confirm
- Enter selects a session for editing (populates name/dir/cmd fields)

#### YAML preview

Read-only, live-updating, syntax-highlighted panel. Only shown at width >= 100.
Structured rendering (not generic parsing) since we know the output shape:
- Keys (`target:`, `name:`) in cyan
- String values in text white
- Punctuation (`-`, `:`) in slate dim
- Structure keywords (`sessions:`) in lavender
- Box border in dim green

#### Editor keybindings

| Context | Keys |
|---------|------|
| Host field | `tab` next, `ctrl+space` SSH hosts picker, `ctrl+s` save, `esc` cancel |
| Session list | `ctrl+n` add, `ctrl+d` delete, `enter` edit, `tab` next, `esc` cancel |
| Session fields | `tab` next, `shift+tab` prev, `ctrl+s` save, `esc` back to list |

### Wizard Mode

2-step guided flow when no config file exists.

**Step 1 — Add your first host:**
```
▲ muxwarp ──────────────────────── Welcome

  Welcome to muxwarp! Let's set up your first host.

  Step 1 of 2 ─────────────────────────────── ●○

  Host target
  ┌──────────────────────────────────────────┐
  │                                          │
  └──────────────────────────────────────────┘
    e.g. user@hostname, 192.168.1.50, or an SSH config alias

  Your ~/.ssh/config aliases, keys, and ProxyCommand all work.

enter continue │ q quit
```

**Step 2 — Add a session (optional):**
```
▲ muxwarp ──────────────────────── Welcome

  Step 2 of 2 ─────────────────────────────── ●●

  Add a desired session for alice@atlas? (optional)

  Session name     Working directory     Command
  ┌────────────┐   ┌────────────────┐   ┌──────────┐
  │            │   │                │   │          │
  └────────────┘   └────────────────┘   └──────────┘

enter save │ tab next field │ esc skip sessions
```

- Step 2 is skippable with Esc (saves host only, no sessions)
- Same textinput components as the editor
- SSH autocomplete active in step 1
- After save: config file created, normal TUI starts with scan

## SSH Config Autocomplete

### Parsing

New `internal/sshconfig/` package:

```go
type SSHHost struct {
    Alias    string // Host directive value (e.g. "atlas")
    HostName string // resolved HostName
    User     string // User if set
    Port     string // Port if non-default
}

func ParseHosts() ([]SSHHost, error)
```

Line-by-line parser of `~/.ssh/config`:
- Finds `Host` directives (skips wildcards `*`, `?`)
- Collects `HostName`, `User`, `Port` until the next `Host` block
- Split into `parseLine()` + `finishBlock()` to keep complexity under 5

### Inline ghost-text completion

As you type in the host field, the best fuzzy match from SSH hosts appears as
dim ghost text after the cursor (like fish shell). Tab accepts it.

```
  │ atl░as                                   │  ← "as" is ghost text
    atlas → alice@192.168.1.50:22              ← metadata preview
```

### Ctrl+Space dropdown picker

Opens a filtered dropdown below the host field:

```
  │ a░                                       │
  ┌──────────────────────────────────────────┐
  │ ▸ atlas        alice@192.168.1.50        │
  │   api-gateway  deploy@10.0.0.5:2222      │
  │   ansible-ctrl root@172.16.0.1           │
  └──────────────────────────────────────────┘
```

- Fuzzy matching against alias, hostname, and user
- Arrow keys navigate, Enter selects, Esc dismisses
- Hosts already in muxwarp config get dim "(added)" tag

## Config Persistence

### config.Save()

```go
func Save(cfg *Config, path string) error
```

1. Custom `MarshalYAML` on Config: writes HostEntry as plain string when no
   sessions, as mapping object when sessions exist
2. `yaml.Marshal` the typed struct
3. Write to temp file (same dir, 0600 permissions)
4. `os.Rename` over original (atomic on POSIX)

### Output format

```yaml
defaults:
    timeout: 3s
    term: xterm-256color
hosts:
    - alice@atlas
    - target: alice@forge
      sessions:
        - name: api-server
          dir: ~/code/api
        - name: web-dev
          dir: ~/code/web
          cmd: nvim
```

### Editor save flow

1. Ctrl+S → validate all fields → error on first invalid field
2. EditorModel sends `EditorSavedMsg{Entry: config.HostEntry{...}}`
3. Parent updates in-memory Config, calls `config.Save()`
4. Parent closes editor, triggers rescan
5. Toast "Saved" appears in header (green, 1.5s)

### Delete flow

1. `d` in session list → inline "Delete atlas? y/n"
2. On `y`: remove HostEntry from Config, save, rescan
3. Toast "Deleted" in header (lavender, 1.5s)

### First-run detection

`main.go` catches config load failure (missing file or empty hosts). Instead
of printing error and exiting, opens wizard mode. After wizard save, proceeds
to normal TUI with scan.

## Wow Details

- **Transitions**: 2-frame crossfade (~30ms) between list and editor screens
- **Toast notifications**: header-bar messages ("Saved", "Deleted", "Config created")
  that fade after 1.5s via `tea.Tick`
- **Smart prefills**: `e` on a session loads that host + selects that session;
  `n` pre-selects the host, focuses name field
- **Live validation**: `✓` on valid fields, red flash on invalid border (single
  frame, 100ms reset), save button brightens when all valid
- **Contextual footers**: always show exactly what you can do in current context
- **Empty session state**: helpful message with ctrl+n hint
- **Live YAML preview**: updates on every keystroke with syntax highlighting

## Files

### New

| File | Purpose |
|------|---------|
| `internal/sshconfig/parse.go` | Parse ~/.ssh/config for Host aliases + metadata |
| `internal/sshconfig/parse_test.go` | Tests with sample SSH config content |
| `internal/tui/editor/editor.go` | EditorModel: textinputs, focus cycling, save/cancel |
| `internal/tui/editor/editor_test.go` | Editor state transitions, validation, save msg |
| `internal/tui/editor/preview.go` | Syntax-highlighted YAML preview renderer |
| `internal/tui/editor/preview_test.go` | Preview output tests |
| `internal/tui/editor/complete.go` | SSH host autocomplete: ghost text, dropdown |
| `internal/tui/editor/complete_test.go` | Completion filtering, fuzzy matching |
| `internal/tui/editor/wizard.go` | Wizard step flow (step 1: host, step 2: sessions) |
| `internal/tui/editor/wizard_test.go` | Wizard step transitions |

### Modified

| File | Changes |
|------|---------|
| `internal/config/config.go` | `Save()`, custom `MarshalYAML` for clean output |
| `internal/config/config_test.go` | Save round-trip, marshal format tests |
| `internal/tui/model.go` | `Mode` enum, `editor` field, toast fields |
| `internal/tui/update.go` | Editor routing, `a`/`e`/`n`/`d` keys, saved/canceled msgs |
| `internal/tui/view.go` | Full-screen editor rendering, toast in header |
| `internal/tui/styles.go` | Editor field styles, validation states, preview styles |
| `cmd/muxwarp/main.go` | Missing config → wizard mode, pass config for editor saves |
| `go.mod` | Add `charm.land/bubbles/v2` |

## Complexity Budget

All functions must stay under cognitive/cyclomatic complexity 5:

- `EditorModel.Update`: switch on focus target, delegate to component
- Focus cycling: `cycleFocus(delta)` helper
- Validation: one-liner callbacks (`ssh.ValidSessionName`, non-empty check)
- SSH config parser: `parseLine()` + `finishBlock()` split
- `config.Save`: marshal → write temp → rename (3 steps, no branching)
- `MarshalYAML`: check sessions empty → scalar or mapping (2 branches)

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Duplicate host target | Validation error: "host already configured" |
| Duplicate session name on same host | Validation error on name field |
| Empty host target | Validation error: "host target required" |
| Invalid session name | Validation error with allowed chars hint |
| SSH config missing or unreadable | Autocomplete silently unavailable, manual entry works |
| SSH config with wildcards (Host *) | Skipped during parsing |
| Terminal too narrow for preview (<100) | Preview panel hidden, form takes full width |
| Save fails (permissions, disk full) | Error toast in header, editor stays open |
| Rescan during editor open | Messages queued, applied when editor closes |
| Wizard quit (q in step 1) | Exit muxwarp entirely (no config = can't run) |
