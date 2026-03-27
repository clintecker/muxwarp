# Changelog

All notable changes to muxwarp are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-03-27

### Added
- **Debug logging**: `--log <path>` writes structured JSON logs to a file for
  post-mortem diagnosis of scan failures, ghost session creation, and SSH args
- **`--help` flag**: prints usage summary with all available flags and modes

### Fixed
- **Ghost session cmd bug**: sessions with a `cmd` (e.g. `echo "READY"`) would
  die instantly because the command replaced the shell. Now uses `tmux send-keys`
  to type the command into the session's shell, keeping it alive after the
  command exits
- **Session name validation too strict**: names containing brackets, spaces,
  quotes, or other non-alphanumeric characters (e.g. `bllooop[`) were rejected.
  Validation now accepts any printable character except `:` (tmux's separator)

### Changed
- `--log` supports both `--log <path>` and `--log=<path>` forms

## [0.3.0] - 2026-03-27

### Added
- **In-TUI config editor**: press `a` to add a host, `e` to edit the selected
  host, `d` to delete — no need to hand-edit YAML
- **First-run wizard**: when no config file exists, an interactive wizard walks
  you through adding your first host and optional desired session
- **SSH config autocomplete**: host input reads `~/.ssh/config` and offers ghost
  text suggestions, a metadata preview line, and a Ctrl+Space dropdown picker
- **YAML preview panel**: live syntax-highlighted preview of the config entry
  being edited (visible at terminal width >= 100)

## [0.2.0] - 2026-03-27

### Changed
- Warp animation reverted to clean single-line growing gradient bar

### Added
- Goreleaser GitHub Actions workflow for automated cross-platform releases

## [0.1.0] - 2026-03-26

### Added
- **Desired sessions**: declare per-host tmux sessions in config; ghost entries
  (◌ NEW) appear in the TUI for sessions that don't exist yet and are created
  on-the-fly when you warp into them
- Mixed config format: hosts can be plain strings or objects with `sessions` list
- Creation animation ("materializing lane") plays before warp for ghost sessions
- Brochure site with feature cards, terminal mockup, and install instructions

### Changed
- Config `hosts` now supports both `- user@host` strings and
  `- target: user@host` objects with optional `sessions` list

## [0.0.0] - 2026-03-25

Initial implementation.

### Added
- TUI with Bubble Tea v2: session list, header with gradient rule, adaptive
  column layout, fuzzy filter mode, keyboard navigation
- Parallel SSH scanning with configurable timeout and BatchMode=yes
- Direct warp mode (`muxwarp <name>`) with fuzzy matching
- Warp animation with cyan→purple gradient block bar
- Session name validation against `[A-Za-z0-9._-]`
- `syscall.Exec` handoff for clean TTY transfer to ssh
- YAML config at `~/.muxwarp.config.yaml`
- Pre-commit hooks enforcing formatting, lint, tests, and complexity thresholds

[0.4.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.4.0
[0.3.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.3.0
[0.2.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.2.0
[0.1.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.1.0
[0.0.0]: https://github.com/clintecker/muxwarp/commits/bfc0afe
