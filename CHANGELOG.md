# Changelog

All notable changes to muxwarp are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.2.0
[0.1.0]: https://github.com/clintecker/muxwarp/releases/tag/v0.1.0
[0.0.0]: https://github.com/clintecker/muxwarp/commits/bfc0afe
