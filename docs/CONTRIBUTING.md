# Contributing to muxwarp

## Prerequisites

- **Go 1.23+**
- **golangci-lint** ([install](https://golangci-lint.run/usage/install/))

## Setup

```
git clone https://github.com/clint/muxwarp.git
cd muxwarp
make hooks    # points git hooks to .githooks/pre-commit
make build    # builds bin/muxwarp
```

## Running

Create a test config at `~/.muxwarp.config.yaml` with at least one reachable
SSH host that has tmux running:

```yaml
hosts:
  - user@your-server
```

Then:

```
make run
```

## Testing

```
make test     # go test -race -count=1 ./...
make lint     # golangci-lint run ./...
make check    # both of the above
```

The pre-commit hook runs `gofmt`, `go vet`, `golangci-lint`, tests with race
detector, and a coverage check (minimum 70%).

## Code style

The golangci-lint config enforces:

- **Cyclomatic complexity**: max 5 per function
- **Function length**: keep functions short and focused
- Standard Go formatting (`gofmt`)

If a function is getting complex, break it into smaller named functions.

## Project layout

```
cmd/muxwarp/main.go          Entry point
internal/config/              Config loading
internal/scanner/             Parallel SSH scanning
internal/tui/                 Bubble Tea v2 TUI
internal/ssh/                 SSH command construction, validation
```

See [architecture.md](architecture.md) for design details.

## PR checklist

- [ ] `make check` passes (lint + tests)
- [ ] New code has tests
- [ ] No `crypto/ssh` -- we use system ssh
- [ ] Session names validated against `[A-Za-z0-9._-]`
- [ ] No shell interpolation in SSH command construction
- [ ] Functions stay under complexity limit (5)
