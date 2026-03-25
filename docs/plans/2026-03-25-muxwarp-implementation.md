# muxwarp Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go TUI tool that scans remote hosts for tmux sessions and lets you warp into them with a beautiful Bubble Tea interface.

**Architecture:** Single Go binary using system `ssh` for all remote operations. Bubble Tea v2 (`charm.land/bubbletea/v2`) for TUI with alt-screen. Two-phase quit-then-exec handoff for clean TTY transfer to ssh. Parallel scanning with `exec.CommandContext` and semaphore-bounded goroutines.

**Tech Stack:** Go 1.23+, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2 (`charm.land/lipgloss/v2`), Bubbles (v2-compatible), `gopkg.in/yaml.v3`, `github.com/sahilm/fuzzy`

**Spec:** `/Users/clint/code/muxwarp/spec.md`

---

## CRITICAL: Bubble Tea v2 API Changes

The plan code samples use v1-style patterns in some places. The executing agent MUST apply these v2 corrections:

### Import paths changed
```go
// WRONG (v1)
import tea "github.com/charmbracelet/bubbletea"
import "github.com/charmbracelet/lipgloss"

// RIGHT (v2)
import tea "charm.land/bubbletea/v2"
import "charm.land/lipgloss/v2"
```

### View() returns tea.View, not string
```go
// WRONG (v1)
func (m Model) View() string { return "hello" }

// RIGHT (v2)
func (m Model) View() tea.View { return tea.NewView("hello") }
```

### Alt screen is declarative in View, not a program option
```go
// WRONG (v1)
p := tea.NewProgram(m, tea.WithAltScreen())

// RIGHT (v2) — set in View:
func (m Model) View() tea.View {
    v := tea.NewView(m.renderContent())
    v.AltScreen = true
    return v
}
p := tea.NewProgram(m)
```

### KeyMsg is now KeyPressMsg
```go
// WRONG (v1)
case tea.KeyMsg:

// RIGHT (v2)
case tea.KeyPressMsg:
```

### Session name validation — remove colon
Colon is a tmux command separator (session:window syntax). The plan's regex `[A-Za-z0-9._-]` should be `[A-Za-z0-9._-]` — no colon.

### Model file split (recommended)
Split the large model.go into:
- `model.go` — types and state
- `update.go` — Update switch and transitions
- `view.go` — View and render helpers
- `delegate.go` — list.ItemDelegate
- `filter.go` — fuzzy match pipeline and highlight

## CRITICAL: Build Order Adjustment (from PAL review)

The plan below shows phases in their original order. The recommended EXECUTION order is:

1. **Phase 0**: Bootstrap (as written)
2. **Phase 1**: Config (as written)
3. **Phase 0.5** (NEW): Bubble Tea v2 smoke test — build a tiny program that proves `tea.NewProgram`, `tea.NewView`, `p.Send` from goroutines, and `tea.KeyPressMsg` all work. This catches v2 API issues before building the real TUI.
4. **Phase 4**: TUI Skeleton with fake data (move UP — get the WOW moment early)
5. **Phase 3**: Scanner (now you have a TUI to feed results into)
6. **Phase 6**: Wire Main (get an executable path early for smoke testing)
7. **Phase 2**: SSH Validate/Exec (now you have context for what exec needs)
8. **Phase 5**: Warp Animation
9. **Phase 7**: Direct Warp
10. **Phase 8**: Polish
11. **Phase 9**: Test Hardening
12. **Phase 10**: Packaging

## CRITICAL: Additional Testing Requirements (from PAL review)

The executing agent should add these tests that the plan doesn't explicitly cover:

- **Send-after-quit**: Scanner goroutines calling p.Send after program exits. Use context cancellation to stop scanners before/during quit.
- **Resize during scan**: Send tea.WindowSizeMsg while scanning is active. Verify no panic.
- **Non-TTY detection**: When stdout is not a TTY, skip TUI and either list sessions as text or error clearly.
- **Unicode session names**: Test with CJK and emoji in display (even though validation rejects them — test the rejection).
- **Running inside tmux**: When TMUX env var is set, the tool should still work (attaching to a remote tmux from within a local tmux is valid).

## CRITICAL: Ralph Loop Usage

At each Quality Gate, use Ralph Loops for iterative polish. The prompts are embedded at each gate below. Key principles:
- Each Ralph Loop has a clear `<promise>` completion signal
- Max iterations prevent infinite loops
- The loop sees its own previous work in git history and file state
- Use PAL tools (`codereview`, `secaudit`, `testgen`) as gut-checks between Ralph Loop iterations

---

## Phase 0: Project Bootstrap

### Task 0.1: Initialize Go module and project structure

**Files:**
- Create: `go.mod`
- Create: `cmd/muxwarp/main.go`
- Create: `internal/config/config.go`
- Create: `internal/scanner/scanner.go`
- Create: `internal/tui/model.go`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/header.go`
- Create: `internal/tui/warp.go`
- Create: `internal/ssh/exec.go`
- Create: `internal/ssh/validate.go`
- Create: `Makefile`
- Create: `.golangci.yml`

**Step 1: Initialize module and install deps**

```bash
cd /Users/clint/code/muxwarp
go mod init github.com/clint/muxwarp
go get charm.land/bubbletea/v2@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get gopkg.in/yaml.v3@latest
go get github.com/sahilm/fuzzy@latest
go mod tidy
```

**Step 2: Create directory structure**

```bash
mkdir -p cmd/muxwarp internal/config internal/scanner internal/tui internal/ssh
```

**Step 3: Create minimal main.go**

```go
// cmd/muxwarp/main.go
package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("muxwarp %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}
	fmt.Println("muxwarp: not yet implemented")
}
```

**Step 4: Create Makefile**

```makefile
BINARY := muxwarp

.PHONY: all build lint test clean run

all: lint test build

build:
	go build -trimpath -ldflags "-s -w \
		-X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
		-X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo none) \
		-X main.date=$$(date -u +%FT%TZ)" \
		-o bin/$(BINARY) ./cmd/muxwarp

lint:
	golangci-lint run

test:
	go test ./... -race -cover

clean:
	rm -rf bin/

run: build
	./bin/$(BINARY)
```

**Step 5: Create .golangci.yml**

```yaml
run:
  timeout: 4m
  tests: true

linters:
  enable:
    - govet
    - staticcheck
    - gosimple
    - errcheck
    - ineffassign
    - unused
    - gocritic
    - revive
    - gosec
    - misspell

linters-settings:
  errcheck:
    exclude-functions:
      - (*os.File).Close
  gosec:
    excludes:
      - G304

issues:
  exclude-rules:
    - path: internal/ssh/exec\.go
      linters: [gosec]
      text: "G204"
    - path: internal/scanner/.*\.go
      linters: [gosec]
      text: "G204"
```

**Step 6: Verify build**

Run: `cd /Users/clint/code/muxwarp && go build ./cmd/muxwarp`
Expected: Clean build, no errors

**Step 7: Init git and commit**

```bash
git init
git add -A
git commit -m "feat: bootstrap project structure and tooling"
```

---

## Phase 1: Config Loader

### Task 1.1: Write config types and loader with tests

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clint/muxwarp/internal/config"
)

func TestLoad_Minimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("hosts:\n  - clint@indigo\n  - clint@devboi\n"), 0o644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0] != "clint@indigo" {
		t.Fatalf("expected clint@indigo, got %s", cfg.Hosts[0])
	}
	// Defaults applied
	if cfg.Defaults.Timeout != "3s" {
		t.Fatalf("expected default timeout 3s, got %s", cfg.Defaults.Timeout)
	}
	if cfg.Defaults.Term != "xterm-256color" {
		t.Fatalf("expected default term xterm-256color, got %s", cfg.Defaults.Term)
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("defaults:\n  timeout: 5s\n  term: screen-256color\nhosts:\n  - clint@indigo\n"), 0o644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Defaults.Timeout != "5s" {
		t.Fatalf("expected timeout 5s, got %s", cfg.Defaults.Timeout)
	}
	if cfg.Defaults.Term != "screen-256color" {
		t.Fatalf("expected term screen-256color, got %s", cfg.Defaults.Term)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_EmptyHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("hosts: []\n"), 0o644)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty hosts")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("{{{{not yaml"), 0o644)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for malformed yaml")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `config.Load` not defined

**Step 3: Write the implementation**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Hosts    []string `yaml:"hosts"`
}

type Defaults struct {
	Timeout string `yaml:"timeout"`
	Term    string `yaml:"term"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no hosts configured in %s", path)
	}

	// Apply defaults
	if cfg.Defaults.Timeout == "" {
		cfg.Defaults.Timeout = "3s"
	}
	if cfg.Defaults.Term == "" {
		cfg.Defaults.Term = "xterm-256color"
	}

	return &cfg, nil
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.muxwarp.config.yaml"
}

// ExampleConfig returns a string showing example config for friendly errors.
func ExampleConfig() string {
	return `# ~/.muxwarp.config.yaml
hosts:
  - user@hostname
  - user@another-host
`
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config loader with defaults and validation"
```

### Task 1.2: Wire config into main.go

**Files:**
- Modify: `cmd/muxwarp/main.go`

**Step 1: Update main.go to load config and print hosts**

```go
// cmd/muxwarp/main.go
package main

import (
	"fmt"
	"os"

	"github.com/clint/muxwarp/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("muxwarp %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\nExample config:\n%s", err, config.ExampleConfig())
		os.Exit(1)
	}

	fmt.Printf("Loaded %d hosts\n", len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		fmt.Printf("  - %s\n", h)
	}
}
```

**Step 2: Test manually**

Run: `go run ./cmd/muxwarp` (without config file — expect friendly error)
Run: Create `~/.muxwarp.config.yaml` with test hosts, run again — expect host list

**Step 3: Commit**

```bash
git add cmd/muxwarp/main.go
git commit -m "feat: wire config loading into main"
```

---

## QUALITY GATE 1: Config Foundation

**Checklist:**
- [ ] `muxwarp` with no config shows friendly error with example
- [ ] `muxwarp` with valid config prints hosts
- [ ] Malformed YAML shows parse error
- [ ] Empty hosts list rejected
- [ ] All tests pass: `go test ./... -race`
- [ ] Lint clean: `golangci-lint run`

**PAL gut-check:** Use `mcp__pal__codereview` on `internal/config/config.go` — verify YAML parsing is robust, defaults are sensible, error messages are helpful.

---

## Phase 2: Session Name Validation + SSH Exec Builder

### Task 2.1: Session name validator

**Files:**
- Create: `internal/ssh/validate.go`
- Create: `internal/ssh/validate_test.go`

**Step 1: Write the failing tests**

```go
// internal/ssh/validate_test.go
package ssh_test

import (
	"testing"

	"github.com/clint/muxwarp/internal/ssh"
)

func TestValidSessionName(t *testing.T) {
	valid := []string{
		"cjdos",
		"build-farm",
		"my.session",
		"test_session",
		"session:1",
		"a",
	}
	for _, name := range valid {
		if !ssh.ValidSessionName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{
		"",
		"foo bar",
		"foo;rm -rf ~",
		"foo\nbar",
		"foo\tbar",
		"foo`whoami`",
		"foo$(cmd)",
		"foo|bar",
		"foo&bar",
		"foo'bar",
		"foo\"bar",
		string(make([]byte, 257)), // too long
	}
	for _, name := range invalid {
		if ssh.ValidSessionName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/ssh/ -v`
Expected: FAIL

**Step 3: Implement**

```go
// internal/ssh/validate.go
package ssh

import "regexp"

var validSession = regexp.MustCompile(`^[A-Za-z0-9._-]{1,256}$`)

// ValidSessionName checks that a tmux session name contains only safe characters.
func ValidSessionName(name string) bool {
	return validSession.MatchString(name)
}
```

**Step 4: Run tests**

Run: `go test ./internal/ssh/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/ssh/validate.go internal/ssh/validate_test.go
git commit -m "feat: session name validation against safe charset"
```

### Task 2.2: SSH exec command builder

**Files:**
- Create: `internal/ssh/exec.go`
- Create: `internal/ssh/exec_test.go`

**Step 1: Write the failing tests**

```go
// internal/ssh/exec_test.go
package ssh_test

import (
	"testing"

	"github.com/clint/muxwarp/internal/ssh"
)

func TestBuildAttachArgs(t *testing.T) {
	args := ssh.BuildAttachArgs("clint@indigo", "xterm-256color", "cjdos")
	expected := []string{"ssh", "-t", "clint@indigo", "--", "env", "TERM=xterm-256color", "tmux", "attach-session", "-t", "cjdos"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], arg)
		}
	}
}

func TestBuildScanArgs(t *testing.T) {
	args := ssh.BuildScanArgs("clint@indigo", "3")
	// Should contain BatchMode=yes and the tmux list-sessions command
	found := map[string]bool{}
	for _, a := range args {
		found[a] = true
	}
	if !found["clint@indigo"] {
		t.Error("missing target in scan args")
	}
}

func TestHostShort(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"clint@indigo", "indigo"},
		{"clint@clint-devboi", "clint-devboi"},
		{"indigo", "indigo"},
		{"user@host.example.com", "host.example.com"},
	}
	for _, tt := range tests {
		got := ssh.HostShort(tt.target)
		if got != tt.want {
			t.Errorf("HostShort(%q) = %q, want %q", tt.target, got, tt.want)
		}
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/ssh/ -v`
Expected: FAIL

**Step 3: Implement**

```go
// internal/ssh/exec.go
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// BuildAttachArgs constructs the ssh argv for attaching to a tmux session.
// No shell interpolation — each argument is passed directly.
func BuildAttachArgs(target, term, sessionName string) []string {
	return []string{
		"ssh", "-t", target, "--",
		"env", "TERM=" + term, "tmux", "attach-session", "-t", sessionName,
	}
}

// BuildScanArgs constructs the ssh argv for listing tmux sessions on a host.
func BuildScanArgs(target, timeoutSec string) []string {
	return []string{
		"ssh",
		"-o", "ConnectTimeout=" + timeoutSec,
		"-o", "BatchMode=yes",
		target,
		"tmux", "list-sessions", "-F",
		"#{session_name}\t#{session_attached}\t#{session_windows}",
	}
}

// HostShort extracts the hostname from a user@host SSH target.
func HostShort(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[i+1:]
	}
	return target
}

// ExecReplace replaces the current process with ssh attaching to a tmux session.
// This never returns on success.
func ExecReplace(target, term, sessionName string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found: %w", err)
	}
	args := BuildAttachArgs(target, term, sessionName)
	return syscall.Exec(sshPath, args, os.Environ())
}
```

**Step 4: Run tests**

Run: `go test ./internal/ssh/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/ssh/exec.go internal/ssh/exec_test.go
git commit -m "feat: SSH command builders with no shell interpolation"
```

---

## QUALITY GATE 2: Security Foundation

**Checklist:**
- [ ] Session names validated against `[A-Za-z0-9._-]{1,256}`
- [ ] SSH attach args have no shell — each arg is a separate argv element
- [ ] `--` separator prevents session name from being interpreted as ssh flags
- [ ] HostShort extraction handles all target formats
- [ ] All tests pass with `-race`

**PAL gut-check:** Use `mcp__pal__secaudit` on `internal/ssh/` — verify no injection vectors remain. Specifically check that `BuildAttachArgs` and `BuildScanArgs` cannot be exploited via crafted hostnames or session names.

---

## Phase 3: Parallel Scanner

### Task 3.1: Scanner types and single-host scan

**Files:**
- Create: `internal/scanner/scanner.go`
- Create: `internal/scanner/scanner_test.go`
- Create: `internal/scanner/testdata/` (for fake ssh scripts)

**Step 1: Write the failing tests**

```go
// internal/scanner/scanner_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/clint/muxwarp/internal/scanner"
)

// withFakeSSH injects a fake ssh script into PATH for testing.
func withFakeSSH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	sshPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(sshPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
}

func TestScanHost_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	withFakeSSH(t, `#!/bin/sh
printf "cjdos\t0\t5\n"
printf "build-farm\t1\t2\n"
exit 0
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessions, err := scanner.ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].Name != "cjdos" {
		t.Errorf("expected cjdos, got %s", sessions[0].Name)
	}
	if sessions[0].Attached != 0 {
		t.Errorf("expected 0 attached, got %d", sessions[0].Attached)
	}
	if sessions[0].Windows != 5 {
		t.Errorf("expected 5 windows, got %d", sessions[0].Windows)
	}
	if sessions[1].Name != "build-farm" {
		t.Errorf("expected build-farm, got %s", sessions[1].Name)
	}
	if sessions[1].Attached != 1 {
		t.Errorf("expected 1 attached, got %d", sessions[1].Attached)
	}
}

func TestScanHost_NoServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	withFakeSSH(t, `#!/bin/sh
echo "no server running on /tmp/tmux-1000/default" >&2
exit 1
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessions, err := scanner.ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestScanHost_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	withFakeSSH(t, `#!/bin/sh
sleep 10
`)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	sessions, err := scanner.ScanHost(ctx, "clint@indigo", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions on timeout, got %d", len(sessions))
	}
}

func TestScanHost_InvalidSessionName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	withFakeSSH(t, `#!/bin/sh
printf "good-session\t0\t1\n"
printf "bad;session\t0\t1\n"
printf "also-good\t0\t2\n"
exit 0
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessions, err := scanner.ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 valid sessions (bad;session filtered), got %d", len(sessions))
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/scanner/ -v`
Expected: FAIL

**Step 3: Implement**

```go
// internal/scanner/scanner.go
package scanner

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/clint/muxwarp/internal/ssh"
)

// Session represents a tmux session on a remote host.
type Session struct {
	Host      string // full SSH target (clint@indigo)
	HostShort string // display name (indigo)
	Name      string // tmux session name
	Attached  int    // number of attached clients
	Windows   int    // number of windows
}

// Key returns a stable identity for this session.
func (s Session) Key() string {
	return s.Host + "/" + s.Name
}

// ScanHost scans a single host for tmux sessions.
// Returns empty slice (not error) for unreachable hosts, no tmux, or no server.
func ScanHost(ctx context.Context, target, timeoutSec string) ([]Session, error) {
	args := ssh.BuildScanArgs(target, timeoutSec)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	out, err := cmd.Output()
	if err != nil {
		// Any failure (timeout, auth, no tmux, no server) -> 0 sessions
		return nil, nil
	}

	return parseSessions(target, string(out)), nil
}

func parseSessions(target, output string) []Session {
	hostShort := ssh.HostShort(target)
	var sessions []Session

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		name := parts[0]
		if !ssh.ValidSessionName(name) {
			continue
		}

		attached, _ := strconv.Atoi(parts[1])
		windows, _ := strconv.Atoi(parts[2])

		sessions = append(sessions, Session{
			Host:      target,
			HostShort: hostShort,
			Name:      name,
			Attached:  attached,
			Windows:   windows,
		})
	}

	return sessions
}
```

**Step 4: Run tests**

Run: `go test ./internal/scanner/ -v -race`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/scanner/
git commit -m "feat: single-host scanner with session parsing and validation"
```

### Task 3.2: Parallel scan-all with semaphore

**Files:**
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/scanner/scanner_test.go`

**Step 1: Write the failing test**

Add to `scanner_test.go`:

```go
func TestScanAll(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// Fake ssh that returns different sessions based on the target
	withFakeSSH(t, `#!/bin/sh
# Extract target from args (last arg before "tmux")
for arg in "$@"; do
  case "$arg" in
    *@*) TARGET="$arg" ;;
  esac
done
case "$TARGET" in
  *indigo*)
    printf "cjdos\t0\t5\n"
    printf "build\t1\t2\n"
    ;;
  *devboi*)
    printf "hacking\t0\t3\n"
    ;;
esac
exit 0
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hosts := []string{"clint@indigo", "clint@devboi"}
	var allSessions []scanner.Session

	err := scanner.ScanAll(ctx, hosts, 8, "3", func(host string, sessions []scanner.Session) {
		allSessions = append(allSessions, sessions...)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allSessions) != 3 {
		t.Fatalf("expected 3 sessions total, got %d", len(allSessions))
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/scanner/ -run TestScanAll -v`
Expected: FAIL

**Step 3: Implement ScanAll**

Add to `scanner.go`:

```go
import "sync"

// ScanAll scans all hosts in parallel with a bounded concurrency.
// The onBatch callback is called (possibly from different goroutines) as each host completes.
func ScanAll(ctx context.Context, hosts []string, maxParallel int, timeoutSec string, onBatch func(host string, sessions []Session)) error {
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			sessions, _ := ScanHost(ctx, h, timeoutSec)
			if len(sessions) > 0 {
				onBatch(h, sessions)
			}
		}(host)
	}

	wg.Wait()
	return nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/scanner/ -v -race`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/scanner/
git commit -m "feat: parallel scanner with semaphore-bounded concurrency"
```

---

## QUALITY GATE 3: Scanner Foundation

**Checklist:**
- [ ] Single-host scan parses tmux output correctly
- [ ] Invalid session names filtered out
- [ ] Timeout/unreachable hosts return empty (not error)
- [ ] No tmux server returns empty (not error)
- [ ] Parallel scan respects semaphore bound
- [ ] Context cancellation stops in-flight scans
- [ ] All tests pass with `-race`
- [ ] No goroutine leaks (verify with test timeout)

**PAL gut-check:** Use `mcp__pal__codereview` on `internal/scanner/scanner.go` — verify concurrency is correct, no races, cancellation works properly.

---

## Phase 4: TUI Skeleton with Fake Data (First WOW Moment)

### Task 4.1: Lip Gloss styles and color palette

**Files:**
- Create: `internal/tui/styles.go`

**Step 1: Implement palette and styles**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorCyan     = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#8BE9FD"}
	colorLavender = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#BD93F9"}
	colorGreen    = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#2EE6A6"}
	colorRed      = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#FF5555"}
	colorSlate    = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#6B7280"}
	colorText     = lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E6E6E6"}

	styleBanner = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorLavender).
			Padding(0, 1)

	styleBannerTitle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	styleStatus = lipgloss.NewStyle().
			Foreground(colorSlate)

	styleSelector = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	styleHost = lipgloss.NewStyle().
			Foreground(colorLavender)

	styleSession = lipgloss.NewStyle().
			Foreground(colorText)

	styleBadgeFree = lipgloss.NewStyle().
			Foreground(colorCyan)

	styleBadgeDocked = lipgloss.NewStyle().
				Foreground(colorGreen)

	styleWindows = lipgloss.NewStyle().
			Foreground(colorSlate)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorSlate)

	styleHighlight = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	styleSelectedBg = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2D2040"})

	styleWarpBar = lipgloss.NewStyle().
			Foreground(colorCyan)

	styleEmpty = lipgloss.NewStyle().
			Foreground(colorSlate).
			Italic(true)
)
```

**Step 2: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: Clean build

**Step 3: Commit**

```bash
git add internal/tui/styles.go
git commit -m "feat: Lip Gloss style palette with adaptive colors"
```

### Task 4.2: Header component

**Files:**
- Create: `internal/tui/header.go`

**Step 1: Implement header renderer**

```go
// internal/tui/header.go
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderHeader(width int, scanning bool, scanDone, scanTotal, sessionCount int) string {
	if width < 40 {
		return renderCompactHeader(scanning, scanDone, scanTotal, sessionCount)
	}

	title := styleBannerTitle.Render("muxwarp")
	banner := styleBanner.Render(fmt.Sprintf(" %s %s", title, styleBannerTitle.Render("warp to tmux")))

	var status string
	if scanning {
		status = styleStatus.Render(fmt.Sprintf("Spooling drives... %d/%d", scanDone, scanTotal))
	} else {
		status = styleStatus.Render(fmt.Sprintf("%d hosts · %d sessions", scanTotal, sessionCount))
	}

	gap := width - lipgloss.Width(banner) - lipgloss.Width(status) - 2
	if gap < 1 {
		gap = 1
	}

	padding := lipgloss.NewStyle().Width(gap).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, banner, padding, status)
}

func renderCompactHeader(scanning bool, scanDone, scanTotal, sessionCount int) string {
	if scanning {
		return styleStatus.Render(fmt.Sprintf("muxwarp — Spooling drives... %d/%d", scanDone, scanTotal))
	}
	return styleStatus.Render(fmt.Sprintf("muxwarp — %d hosts · %d sessions", scanTotal, sessionCount))
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: Clean build

**Step 3: Commit**

```bash
git add internal/tui/header.go
git commit -m "feat: header component with scan progress"
```

### Task 4.3: Bubble Tea model with list, navigation, and view

**Files:**
- Create: `internal/tui/model.go`

**Step 1: Implement the core TUI model**

This is the largest single file. It wires together: list rendering, keyboard navigation, filter mode, incremental results, and the quit-then-warp flow.

```go
// internal/tui/model.go
package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/clint/muxwarp/internal/scanner"
)

// Messages
type SessionBatchMsg struct {
	Host     string
	Sessions []scanner.Session
}
type ScanDoneMsg struct{}

// sessionItem wraps Session for the list component.
type sessionItem struct {
	session scanner.Session
}

func (i sessionItem) FilterValue() string {
	return i.session.HostShort + " " + i.session.Name
}

// delegate renders each row in the session list.
type delegate struct {
	matchInfo map[string]matchInfo
}

type matchInfo struct {
	hostIdx []int
	nameIdx []int
}

func (d delegate) Height() int                             { return 1 }
func (d delegate) Spacing() int                            { return 0 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(sessionItem)
	if !ok {
		return
	}
	s := si.session
	selected := index == m.Index()

	// Selector
	sel := "  "
	if selected {
		sel = styleSelector.Render("> ")
	}

	// Host
	host := fmt.Sprintf("[%-10s]", truncate(s.HostShort, 10))
	host = styleHost.Render(host)

	// Session name
	name := fmt.Sprintf("%-20s", truncate(s.Name, 20))
	// Apply fuzzy highlight if available
	if mi, ok := d.matchInfo[s.Key()]; ok {
		name = highlightChars(s.Name, mi.nameIdx, 20)
	}
	name = styleSession.Render(name)

	// Badge
	var badge string
	if s.Attached > 0 {
		badge = styleBadgeDocked.Render("● DOCKED")
	} else {
		badge = styleBadgeFree.Render("○ FREE  ")
	}

	// Windows
	win := styleWindows.Render(fmt.Sprintf("w%d", s.Windows))

	row := fmt.Sprintf("%s %s %s  %s  %s", sel, host, name, badge, win)

	if selected {
		row = styleSelectedBg.Render(row)
	}

	fmt.Fprint(w, row)
}

// Model is the Bubble Tea model for the muxwarp TUI.
type Model struct {
	sessions     []scanner.Session
	list         list.Model
	spinner      spinner.Model
	filterInput  textinput.Model
	filtering    bool
	scanning     bool
	scanDone     int
	scanTotal    int
	width        int
	height       int
	warpTarget   *scanner.Session
	selectedKey  string
	matchInfoMap map[string]matchInfo
}

// NewModel creates a fresh TUI model.
func NewModel(hostCount int) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."

	del := delegate{}
	l := list.New(nil, del, 80, 20)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return Model{
		list:         l,
		spinner:      sp,
		filterInput:  fi,
		scanning:     true,
		scanTotal:    hostCount,
		matchInfoMap: make(map[string]matchInfo),
	}
}

// WarpTarget returns the session the user selected, or nil if they quit.
func (m Model) WarpTarget() *scanner.Session {
	return m.warpTarget
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-6) // header + footer space
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case SessionBatchMsg:
		m.scanDone++
		m.sessions = append(m.sessions, msg.Sessions...)
		sortSessions(m.sessions)
		m.refreshList()
		return m, nil

	case ScanDoneMsg:
		m.scanning = false
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "enter":
		if item, ok := m.list.SelectedItem().(sessionItem); ok {
			s := item.session
			m.warpTarget = &s
			return m, tea.Quit
		}

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "r":
		// Rescan — caller should handle this via a RescanMsg
		m.scanning = true
		m.scanDone = 0
		m.sessions = nil
		m.refreshList()
		return m, m.spinner.Tick
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterInput.Reset()
		m.matchInfoMap = make(map[string]matchInfo)
		m.refreshList()
		return m, nil

	case "enter":
		if item, ok := m.list.SelectedItem().(sessionItem); ok {
			s := item.session
			m.warpTarget = &s
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter(m.filterInput.Value())
	return m, cmd
}

func (m *Model) applyFilter(pattern string) {
	m.matchInfoMap = make(map[string]matchInfo)
	if pattern == "" {
		m.refreshList()
		return
	}

	corpus := make([]string, len(m.sessions))
	for i, s := range m.sessions {
		corpus[i] = s.HostShort + " " + s.Name
	}

	matches := fuzzy.Find(pattern, corpus)
	items := make([]list.Item, 0, len(matches))
	for _, match := range matches {
		s := m.sessions[match.Index]
		split := len(s.HostShort)
		mi := matchInfo{}
		for _, idx := range match.MatchedIndexes {
			if idx < split {
				mi.hostIdx = append(mi.hostIdx, idx)
			} else if idx > split {
				mi.nameIdx = append(mi.nameIdx, idx-split-1)
			}
		}
		m.matchInfoMap[s.Key()] = mi
		items = append(items, sessionItem{session: s})
	}

	m.list.SetItems(items)
}

func (m *Model) refreshList() {
	key := m.selectedKey
	items := make([]list.Item, 0, len(m.sessions))
	for _, s := range m.sessions {
		items = append(items, sessionItem{session: s})
	}

	// Update delegate with current match info
	m.list.SetDelegate(delegate{matchInfo: m.matchInfoMap})
	m.list.SetItems(items)

	// Restore selection by key
	if key != "" {
		for i, item := range items {
			if si, ok := item.(sessionItem); ok && si.session.Key() == key {
				m.list.Select(i)
				break
			}
		}
	}
}

func (m Model) View() string {
	header := renderHeader(m.width, m.scanning, m.scanDone, m.scanTotal, len(m.sessions))

	var body string
	if len(m.sessions) == 0 && !m.scanning {
		body = renderEmpty()
	} else {
		body = m.list.View()
	}

	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
}

func (m Model) renderFooter() string {
	if m.filtering {
		count := len(m.list.Items())
		return styleFooter.Render(
			fmt.Sprintf("  %s  type to filter · enter warp · esc clear · %d matches",
				m.filterInput.View(), count),
		)
	}
	return styleFooter.Render("  ↑/↓ navigate · enter warp · / filter · r rescan · q quit")
}

func renderEmpty() string {
	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		styleEmpty.Render("    All gates are calm — no active lanes detected."),
		"",
		styleFooter.Render("    Start a session:  ssh <host> -t tmux new -s <name>"),
		"",
		styleFooter.Render("    r rescan · q quit"),
	)
}

// Helpers

func sortSessions(sessions []scanner.Session) {
	sort.SliceStable(sessions, func(i, j int) bool {
		a, b := sessions[i], sessions[j]
		if (a.Attached == 0) != (b.Attached == 0) {
			return a.Attached == 0
		}
		if a.HostShort != b.HostShort {
			return a.HostShort < b.HostShort
		}
		return a.Name < b.Name
	})
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func highlightChars(s string, indexes []int, maxWidth int) string {
	s = truncate(s, maxWidth)
	runes := []rune(s)
	idxSet := make(map[int]bool, len(indexes))
	for _, i := range indexes {
		idxSet[i] = true
	}

	var b strings.Builder
	for i, r := range runes {
		ch := string(r)
		if idxSet[i] {
			b.WriteString(styleHighlight.Render(ch))
		} else {
			b.WriteString(ch)
		}
	}
	// Pad to maxWidth
	for i := len(runes); i < maxWidth; i++ {
		b.WriteRune(' ')
	}
	return b.String()
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: Clean build (may need to fix import paths for Bubble Tea v2)

> **NOTE:** If Bubble Tea v2 import paths differ from `charm.land/bubbletea/v2`, adjust imports accordingly. Run `go doc charm.land/bubbletea/v2` to verify the actual module path.

**Step 3: Commit**

```bash
git add internal/tui/
git commit -m "feat: Bubble Tea model with list, filter, and navigation"
```

---

## QUALITY GATE 4: TUI Foundation

**Checklist:**
- [ ] TUI compiles and imports resolve
- [ ] Model handles WindowSizeMsg, KeyMsg, SessionBatchMsg
- [ ] Filter mode: type to filter, esc to clear, fuzzy matching works
- [ ] Selection stability: tracked by key, not index
- [ ] Sort order: FREE first, then DOCKED, then host, then name
- [ ] Empty state renders when no sessions
- [ ] All tests pass

**RALPH LOOP — TUI Visual Polish:**

```
/ralph-loop "Review and improve the Bubble Tea TUI rendering in internal/tui/.
The spec is at spec.md. Focus on:
1. Does the View() output match the spec layout exactly?
2. Are the Lip Gloss styles applied correctly?
3. Is the adaptive column hiding implemented for narrow terminals?
4. Does the fuzzy highlight render correctly?
Run 'go build ./internal/tui/' after each change to verify compilation.
Output <promise>TUI POLISHED</promise> when the rendering matches the spec." --max-iterations 5
```

**PAL gut-check:** Use `mcp__pal__codereview` on `internal/tui/model.go` — verify Bubble Tea patterns are correct: no blocking in Update, proper Cmd returns, clean Model lifecycle.

---

## Phase 5: Warp Animation

### Task 5.1: Warp animation renderer (runs after TUI exits)

**Files:**
- Create: `internal/tui/warp.go`
- Create: `internal/tui/warp_test.go`

**Step 1: Write the failing test**

```go
// internal/tui/warp_test.go
package tui_test

import (
	"bytes"
	"testing"

	"github.com/clint/muxwarp/internal/tui"
)

func TestRenderWarpFrames(t *testing.T) {
	frames := tui.WarpFrames("indigo", "cjdos", 60)
	if len(frames) != 4 {
		t.Fatalf("expected 4 frames, got %d", len(frames))
	}
	for i, f := range frames {
		if f == "" {
			t.Errorf("frame %d is empty", i)
		}
	}
	// Each successive frame should have more block chars
	for i := 1; i < len(frames); i++ {
		if len(frames[i]) <= len(frames[i-1]) {
			t.Errorf("frame %d should be wider than frame %d", i, i-1)
		}
	}
}

func TestPlayWarpAnimation(t *testing.T) {
	var buf bytes.Buffer
	tui.PlayWarpAnimationTo(&buf, "indigo", "cjdos", 60)
	output := buf.String()
	if output == "" {
		t.Error("expected animation output")
	}
	if !bytes.Contains([]byte(output), []byte("engaging jumpgate")) {
		t.Error("expected 'engaging jumpgate' in output")
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./internal/tui/ -run TestRenderWarp -v`
Expected: FAIL

**Step 3: Implement**

```go
// internal/tui/warp.go
package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// WarpFrames generates the 4 animation frames for the warp sequence.
func WarpFrames(hostShort, sessionName string, termWidth int) []string {
	label := fmt.Sprintf("engaging jumpgate: %s/%s ", hostShort, sessionName)
	maxBar := termWidth - len(label) - 1
	if maxBar < 4 {
		maxBar = 4
	}

	steps := []int{
		maxBar / 4,
		maxBar / 2,
		maxBar * 3 / 4,
		maxBar,
	}

	frames := make([]string, 4)
	for i, barLen := range steps {
		bar := strings.Repeat("█", barLen)
		frames[i] = label + styleWarpBar.Render(bar)
	}
	return frames
}

// PlayWarpAnimation plays the warp animation to stdout.
func PlayWarpAnimation(hostShort, sessionName string, termWidth int) {
	PlayWarpAnimationTo(os.Stdout, hostShort, sessionName, termWidth)
}

// PlayWarpAnimationTo plays the warp animation to a writer (for testing).
func PlayWarpAnimationTo(w io.Writer, hostShort, sessionName string, termWidth int) {
	frames := WarpFrames(hostShort, sessionName, termWidth)
	for _, frame := range frames {
		fmt.Fprintf(w, "\r%s", frame)
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Fprintln(w)
}
```

**Step 4: Run tests**

Run: `go test ./internal/tui/ -run TestRenderWarp -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/warp.go internal/tui/warp_test.go
git commit -m "feat: warp animation with block-bar frames"
```

---

## Phase 6: Wire Everything Together in main.go

### Task 6.1: Full TUI mode in main.go

**Files:**
- Modify: `cmd/muxwarp/main.go`

**Step 1: Implement the full main with TUI + scan + warp**

```go
// cmd/muxwarp/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/clint/muxwarp/internal/config"
	"github.com/clint/muxwarp/internal/scanner"
	"github.com/clint/muxwarp/internal/ssh"
	"github.com/clint/muxwarp/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("muxwarp %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\nExample config:\n%s", err, config.ExampleConfig())
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		directWarp(cfg, os.Args[1])
	} else {
		runTUI(cfg)
	}
}

func runTUI(cfg *config.Config) {
	m := tui.NewModel(len(cfg.Hosts))

	p := tea.NewProgram(m, tea.WithAltScreen())

	// Start scanning in background, sending results to the TUI
	timeout := parseDuration(cfg.Defaults.Timeout)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		scanner.ScanAll(ctx, cfg.Hosts, 8, fmt.Sprintf("%d", int(timeout.Seconds())), func(host string, sessions []scanner.Session) {
			p.Send(tui.SessionBatchMsg{Host: host, Sessions: sessions})
		})
		p.Send(tui.ScanDoneMsg{})
	}()

	finalModel, err := p.Run()
	cancel() // cancel any in-flight scans
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Two-phase: TUI has quit, terminal is restored. Now warp if selected.
	if fm, ok := finalModel.(tui.Model); ok {
		if target := fm.WarpTarget(); target != nil {
			tui.PlayWarpAnimation(target.HostShort, target.Name, 80)
			if err := ssh.ExecReplace(target.Host, cfg.Defaults.Term, target.Name); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func directWarp(cfg *config.Config, pattern string) {
	timeout := parseDuration(cfg.Defaults.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var allSessions []scanner.Session
	fmt.Print("Scanning gates...")
	scanner.ScanAll(ctx, cfg.Hosts, 8, fmt.Sprintf("%d", int(timeout.Seconds())), func(host string, sessions []scanner.Session) {
		allSessions = append(allSessions, sessions...)
	})
	fmt.Println()

	// Fuzzy match
	corpus := make([]string, len(allSessions))
	for i, s := range allSessions {
		corpus[i] = s.HostShort + " " + s.Name
	}

	matches := fuzzyMatchSessions(pattern, allSessions)

	switch len(matches) {
	case 0:
		fmt.Fprintf(os.Stderr, "error: no sessions matching %q\n", pattern)
		os.Exit(1)
	case 1:
		s := matches[0]
		tui.PlayWarpAnimation(s.HostShort, s.Name, 80)
		if err := ssh.ExecReplace(s.Host, cfg.Defaults.Term, s.Name); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		// Multiple matches — launch TUI prefiltered
		// For now, show matches and let user pick
		fmt.Printf("Multiple matches for %q:\n", pattern)
		for _, s := range matches {
			fmt.Printf("  [%s] %s\n", s.HostShort, s.Name)
		}
		fmt.Println("\nRun muxwarp without arguments to use the interactive picker.")
		os.Exit(1)
	}
}

func fuzzyMatchSessions(pattern string, sessions []scanner.Session) []scanner.Session {
	corpus := make([]string, len(sessions))
	for i, s := range sessions {
		corpus[i] = s.HostShort + " " + s.Name
	}
	// Use sahilm/fuzzy
	results := fuzzyFind(pattern, corpus)
	matched := make([]scanner.Session, 0, len(results))
	for _, idx := range results {
		matched = append(matched, sessions[idx])
	}
	return matched
}

func fuzzyFind(pattern string, corpus []string) []int {
	// Simple implementation using sahilm/fuzzy
	import_fuzzy := func() []int {
		// We'll use the fuzzy package
		return nil
	}
	_ = import_fuzzy

	// Direct substring match as a fallback that always works
	var matches []int
	for i, s := range corpus {
		if containsFold(s, pattern) {
			matches = append(matches, i)
		}
	}
	return matches
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 3 * time.Second
	}
	return d
}
```

> **NOTE:** The `fuzzyFind` function above is a placeholder. The actual implementation should use `github.com/sahilm/fuzzy`. The executing agent should replace this with proper fuzzy matching using the library.

**Step 2: Build and test manually**

Run: `go build -o bin/muxwarp ./cmd/muxwarp && ./bin/muxwarp`
Expected: TUI launches, scans configured hosts, shows sessions

**Step 3: Commit**

```bash
git add cmd/muxwarp/main.go
git commit -m "feat: wire TUI, scanner, and warp into main"
```

---

## QUALITY GATE 5: End-to-End Working

**Checklist:**
- [ ] `muxwarp` launches TUI, scans real hosts, shows sessions
- [ ] Can navigate with j/k/arrows and select with Enter
- [ ] Warp animation plays after TUI exits
- [ ] `syscall.Exec` hands off to ssh cleanly (echo works, cursor visible)
- [ ] `muxwarp <name>` does direct warp on single match
- [ ] `muxwarp nonexistent` shows error and exits 1
- [ ] Filter mode works: `/` activates, typing filters, Esc clears
- [ ] `r` key rescans
- [ ] `q` quits cleanly
- [ ] All tests pass: `go test ./... -race`

**RALPH LOOP — Integration Polish:**

```
/ralph-loop "Test muxwarp end-to-end. The spec is at spec.md.
Build and run the binary. Check:
1. Does it scan your configured hosts?
2. Does navigation feel smooth?
3. Does the warp animation play correctly?
4. Does the TTY handoff work (echo, cursor, no artifacts)?
5. Fix any compile errors, runtime panics, or UX issues.
Run 'go build ./cmd/muxwarp' and test manually after each fix.
Output <promise>E2E WORKING</promise> when all flows work." --max-iterations 10
```

**PAL gut-check:** Use `mcp__pal__codereview` on `cmd/muxwarp/main.go` — verify the two-phase quit-then-exec flow is correct, context cancellation is wired properly, and no goroutine leaks.

---

## Phase 7: Direct Warp with Filtered TUI

### Task 7.1: Pre-filtered TUI for multiple matches

When `muxwarp <name>` matches multiple sessions, open the TUI with the filter pre-populated.

**Files:**
- Modify: `internal/tui/model.go` — add `NewModelWithFilter(hostCount int, pattern string) Model`
- Modify: `cmd/muxwarp/main.go` — use `NewModelWithFilter` for multi-match case

**Step 1: Add NewModelWithFilter**

Add to `internal/tui/model.go`:

```go
// NewModelWithFilter creates a model with the filter pre-populated.
func NewModelWithFilter(hostCount int, pattern string) Model {
	m := NewModel(hostCount)
	m.filtering = true
	m.filterInput.SetValue(pattern)
	m.filterInput.Focus()
	return m
}
```

**Step 2: Update main.go direct warp multi-match case**

Replace the multi-match case in `directWarp()` to launch TUI with pre-filter instead of printing a list.

**Step 3: Test manually**

Run: `./bin/muxwarp <partial-name>` with a pattern matching multiple sessions
Expected: TUI opens with filter pre-populated, showing only matching sessions

**Step 4: Commit**

```bash
git add internal/tui/model.go cmd/muxwarp/main.go
git commit -m "feat: direct warp opens pre-filtered TUI on multiple matches"
```

---

## Phase 8: Polish Pass

### Task 8.1: Adaptive column layout for narrow terminals

**Files:**
- Modify: `internal/tui/model.go` — update delegate.Render to check width and hide columns

**Implementation:**
- Width >= 80: Full layout (selector, host, session, badge word, windows)
- Width >= 60: Badge collapses to just `●`/`○`
- Width >= 45: Windows column hides
- Width < 45: Host brackets drop, 3-char prefix only

**Commit:** `git commit -m "feat: adaptive column layout for narrow terminals"`

### Task 8.2: Context-aware footer

Already implemented in Phase 4 — verify footer changes between normal and filter modes.

### Task 8.3: Fuzzy highlight in filter mode

Already implemented in Phase 4 — verify matched characters are highlighted in both host and session columns.

### Task 8.4: Selection stability during resort

Already implemented in Phase 4 — verify cursor follows the same item when new results arrive.

---

## QUALITY GATE 6: Polish Complete

**Checklist:**
- [ ] Narrow terminal (40 cols) renders correctly — columns adapt
- [ ] Wide terminal (120 cols) renders correctly — columns fill
- [ ] Footer changes between normal and filter mode
- [ ] Fuzzy highlight shows bold chars on matched positions
- [ ] Selection doesn't jump when new scan results arrive
- [ ] Empty state renders correctly with helpful text
- [ ] Color palette looks good on dark terminal
- [ ] Color palette looks acceptable on light terminal (AdaptiveColor)

**RALPH LOOP — Visual QA:**

```
/ralph-loop "Visual QA pass on muxwarp TUI. The spec is at spec.md.
Build and run at different terminal widths (40, 60, 80, 120 cols).
Check:
1. Header renders correctly at each width
2. Columns adapt correctly at each width breakpoint
3. Empty state looks good
4. Filter mode highlights work
5. Colors look good
Fix any rendering issues.
Output <promise>VISUAL QA PASSED</promise> when it looks great at all widths." --max-iterations 5
```

---

## Phase 9: Test Hardening

### Task 9.1: Golden file tests for TUI rendering

**Files:**
- Create: `internal/tui/model_test.go`
- Create: `internal/tui/testdata/` (golden files)

**Implementation:**
- Test initial empty view
- Test view with sessions (inject SessionBatchMsg)
- Test filter mode view
- Test empty state view
- Use fixed width (80) via tea.WindowSizeMsg
- Strip ANSI for stable comparison OR use ANSI-aware golden files

### Task 9.2: Scanner integration test suite

**Files:**
- Expand: `internal/scanner/scanner_test.go`

**Test cases:**
- Multiple hosts, mixed success/failure
- All hosts timeout
- Cancellation mid-scan
- Large number of sessions (100+)

### Task 9.3: Config edge case tests

**Files:**
- Expand: `internal/config/config_test.go`

**Test cases:**
- Config with only defaults, no hosts (error)
- Config with extra unknown fields (tolerated)
- Unicode in host strings

### Task 9.4: SSH arg builder tests

Already done in Phase 2 — verify coverage.

**Commit after all tests:**

```bash
git add -A
git commit -m "test: comprehensive test suite for scanner, config, and TUI"
```

---

## QUALITY GATE 7: Test Coverage

**Checklist:**
- [ ] `go test ./... -race -cover` shows >= 70% coverage
- [ ] Scanner tests cover: success, no-server, timeout, invalid names, multi-host
- [ ] Config tests cover: minimal, with-defaults, missing, malformed, empty-hosts
- [ ] SSH tests cover: validation, arg building, host-short extraction
- [ ] TUI golden tests cover: normal view, filter view, empty state
- [ ] No flaky tests

**PAL gut-check:** Use `mcp__pal__testgen` on each package — ask if any critical paths are untested.

---

## Phase 10: Final Build and Packaging

### Task 10.1: Create .goreleaser.yml

**Files:**
- Create: `.goreleaser.yml`

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: muxwarp
    main: ./cmd/muxwarp
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    ldflags:
      - "-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}"
    flags:
      - -trimpath

archives:
  - id: tgz
    builds: [muxwarp]
    format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  use: git
```

### Task 10.2: Verify cross-platform build

Run: `goreleaser build --snapshot --clean`
Expected: Binaries for linux/darwin amd64/arm64

### Task 10.3: Final commit

```bash
git add .goreleaser.yml
git commit -m "chore: add goreleaser config for cross-platform builds"
```

---

## QUALITY GATE 8: Ship-Ready

**Checklist:**
- [ ] `make all` passes (lint + test + build)
- [ ] Binary works end-to-end on real hosts
- [ ] `muxwarp --version` shows version info
- [ ] `goreleaser build --snapshot` produces all platform binaries
- [ ] Config error messages are friendly and helpful
- [ ] No TODOs or placeholder code remaining

**RALPH LOOP — Final Polish:**

```
/ralph-loop "Final polish pass on muxwarp. The spec is at spec.md.
1. Read through ALL source files
2. Check for any TODOs, placeholder code, or unfinished implementations
3. Verify all error messages are user-friendly
4. Check that imports are clean (no unused)
5. Run 'make all' and fix any issues
6. Run the binary end-to-end and verify all flows
Output <promise>SHIP READY</promise> when everything is polished." --max-iterations 5
```

**PAL gut-check:** Use `mcp__pal__codereview` on the entire codebase — final review for code quality, patterns, and any issues before shipping.

---

## Summary of Quality Gates and Ralph Loops

| Gate | After Phase | Focus | Ralph Loop |
|------|-------------|-------|------------|
| 1 | Config | Config loads, errors friendly | - |
| 2 | SSH/Validate | No injection vectors | - |
| 3 | Scanner | Concurrency correct, no leaks | - |
| 4 | TUI Skeleton | Rendering matches spec | TUI Visual Polish (5 iters) |
| 5 | End-to-End | All flows work | Integration Polish (10 iters) |
| 6 | Polish | Adaptive layout, highlights | Visual QA (5 iters) |
| 7 | Tests | >= 70% coverage, no flakes | - |
| 8 | Ship | Everything works, clean code | Final Polish (5 iters) |

## PAL Consultation Points

| Point | Tool | Focus |
|-------|------|-------|
| After Gate 1 | `codereview` | Config robustness |
| After Gate 2 | `secaudit` | Injection vectors |
| After Gate 3 | `codereview` | Concurrency correctness |
| After Gate 4 | `codereview` | Bubble Tea patterns |
| After Gate 5 | `codereview` | Main wiring, lifecycle |
| After Gate 7 | `testgen` | Coverage gaps |
| After Gate 8 | `codereview` | Full codebase review |
