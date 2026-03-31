# Growth Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add session metadata display, host tags, `muxwarp init`, shell completions, Homebrew tap config, and a VHS demo environment — all in one PR.

**Architecture:** Six independent features layered onto existing code. Session metadata extends scanner → TUI data flow. Host tags extend config → TUI. Init is a new subcommand. Completions are embedded static scripts. Homebrew is goreleaser config. Demo is Docker + VHS.

**Tech Stack:** Go 1.25, Bubble Tea v2, lipgloss v2, gopkg.in/yaml.v3, embed, Docker, VHS

---

## Task 1: Session Metadata — Scanner Changes

**Files:**
- Modify: `internal/ssh/exec.go:34-42` (BuildScanArgs format string)
- Modify: `internal/scanner/scanner.go:22-28` (Session struct)
- Modify: `internal/scanner/scanner.go:78-92` (splitSessionFields)
- Modify: `internal/scanner/scanner.go:96-108` (parseSessionLine)
- Modify: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Write failing test for expanded scanner fields**

Add to `internal/scanner/scanner_test.go`:

```go
func TestScanHost_WithTimestamps(t *testing.T) {
	// Fake ssh prints sessions with 5 tab-separated fields:
	// name, attached, windows, created, last_activity
	withFakeSSH(t, `#!/bin/sh
printf 'dev\t1\t3\t1711756800\t1711843100\n'
printf 'build\t0\t1\t1711670400\t1711756700\n'
`)

	ctx := context.Background()
	sessions, err := ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}

	t.Run("timestamps_parsed", func(t *testing.T) {
		if sessions[0].Created != 1711756800 {
			t.Errorf("Created = %d, want 1711756800", sessions[0].Created)
		}
		if sessions[0].LastActivity != 1711843100 {
			t.Errorf("LastActivity = %d, want 1711843100", sessions[0].LastActivity)
		}
	})

	t.Run("second_session_timestamps", func(t *testing.T) {
		if sessions[1].Created != 1711670400 {
			t.Errorf("Created = %d, want 1711670400", sessions[1].Created)
		}
		if sessions[1].LastActivity != 1711756700 {
			t.Errorf("LastActivity = %d, want 1711756700", sessions[1].LastActivity)
		}
	})
}

func TestScanHost_MissingTimestamps(t *testing.T) {
	// Old tmux or unexpected output: only 3 fields. Should still parse.
	withFakeSSH(t, `#!/bin/sh
printf 'dev\t1\t3\n'
`)

	ctx := context.Background()
	sessions, err := ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}

	if sessions[0].Created != 0 {
		t.Errorf("Created = %d, want 0 for missing timestamp", sessions[0].Created)
	}
	if sessions[0].LastActivity != 0 {
		t.Errorf("LastActivity = %d, want 0 for missing timestamp", sessions[0].LastActivity)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestScanHost_WithTimestamps ./internal/scanner/`
Expected: FAIL — `splitSessionFields` expects exactly 3 fields

- [ ] **Step 3: Update BuildScanArgs format string**

In `internal/ssh/exec.go`, change `BuildScanArgs`:

```go
func BuildScanArgs(target, timeoutSec string) []string {
	return []string{
		"ssh",
		"-o", "ConnectTimeout=" + timeoutSec,
		"-o", "BatchMode=yes",
		target,
		"tmux", "list-sessions", "-F",
		"\"#{session_name}\t#{session_attached}\t#{session_windows}\t#{session_created}\t#{session_activity}\"",
	}
}
```

- [ ] **Step 4: Add fields to Session struct**

In `internal/scanner/scanner.go`, update the `Session` struct:

```go
type Session struct {
	Host         string // full SSH target (e.g. "clint@indigo")
	HostShort    string // display name (e.g. "indigo")
	Name         string // tmux session name (validated)
	Attached     int    // number of attached clients
	Windows      int    // number of windows
	Created      int64  // unix timestamp of session creation (0 if unavailable)
	LastActivity int64  // unix timestamp of last activity (0 if unavailable)
}
```

- [ ] **Step 5: Update splitSessionFields to accept 3 or 5 fields**

In `internal/scanner/scanner.go`, replace `splitSessionFields`:

```go
func splitSessionFields(line string) (name string, attached, windows int, created, lastActivity int64, ok bool) {
	parts := strings.Split(line, "\t")
	if len(parts) < 3 || !ssh.ValidSessionName(parts[0]) {
		return "", 0, 0, 0, 0, false
	}
	a, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	w, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}

	var c, la int64
	if len(parts) >= 5 {
		c, _ = strconv.ParseInt(parts[3], 10, 64)
		la, _ = strconv.ParseInt(parts[4], 10, 64)
	}

	return parts[0], a, w, c, la, true
}
```

- [ ] **Step 6: Update parseSessionLine to pass through new fields**

```go
func parseSessionLine(line, target, hostShort string) (Session, bool) {
	name, attached, windows, created, lastActivity, ok := splitSessionFields(line)
	if !ok {
		return Session{}, false
	}
	return Session{
		Host:         target,
		HostShort:    hostShort,
		Name:         name,
		Attached:     attached,
		Windows:      windows,
		Created:      created,
		LastActivity: lastActivity,
	}, true
}
```

- [ ] **Step 7: Run all scanner tests**

Run: `go test -race -count=1 ./internal/scanner/`
Expected: ALL PASS (including existing tests which produce 3-field output — they should still work with `len(parts) < 3` guard)

- [ ] **Step 8: Commit**

```bash
git add internal/ssh/exec.go internal/scanner/scanner.go internal/scanner/scanner_test.go
git commit -m "feat: add session timestamps to scanner output

Expand tmux list-sessions format to include session_created and
session_activity. Parser accepts both 3-field (legacy) and 5-field
output for backward compatibility."
```

---

## Task 2: Session Metadata — TUI Time Formatting

**Files:**
- Create: `internal/tui/timeformat.go`
- Create: `internal/tui/timeformat_test.go`

- [ ] **Step 1: Write failing tests for formatAge**

Create `internal/tui/timeformat_test.go`:

```go
package tui

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ts   int64
		want string
	}{
		{"now", now.Unix(), "now"},
		{"30s_ago", now.Add(-30 * time.Second).Unix(), "now"},
		{"5m_ago", now.Add(-5 * time.Minute).Unix(), "5m"},
		{"59m_ago", now.Add(-59 * time.Minute).Unix(), "59m"},
		{"2h_ago", now.Add(-2 * time.Hour).Unix(), "2h"},
		{"23h_ago", now.Add(-23 * time.Hour).Unix(), "23h"},
		{"3d_ago", now.Add(-3 * 24 * time.Hour).Unix(), "3d"},
		{"13d_ago", now.Add(-13 * 24 * time.Hour).Unix(), "13d"},
		{"2w_ago", now.Add(-14 * 24 * time.Hour).Unix(), "2w"},
		{"8w_ago", now.Add(-59 * 24 * time.Hour).Unix(), "8w"},
		{"3mo_ago", now.Add(-90 * 24 * time.Hour).Unix(), "3mo"},
		{"11mo_ago", now.Add(-340 * 24 * time.Hour).Unix(), "11mo"},
		{"1y_ago", now.Add(-366 * 24 * time.Hour).Unix(), "1y"},
		{"zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAgeSince(tt.ts, now)
			if got != tt.want {
				t.Errorf("formatAgeSince(%d) = %q, want %q", tt.ts, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestFormatAge ./internal/tui/`
Expected: FAIL — `formatAgeSince` undefined

- [ ] **Step 3: Implement formatAgeSince**

Create `internal/tui/timeformat.go`:

```go
package tui

import (
	"fmt"
	"time"
)

// formatAgeSince returns a compact human-readable age string for a unix
// timestamp relative to the given now time. Returns "" if ts is 0.
func formatAgeSince(ts int64, now time.Time) string {
	if ts == 0 {
		return ""
	}

	d := now.Sub(time.Unix(ts, 0))
	if d < 0 {
		d = 0
	}
	return formatDuration(d)
}

// formatDuration returns a compact string for a duration.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race -count=1 -run TestFormatAge ./internal/tui/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/timeformat.go internal/tui/timeformat_test.go
git commit -m "feat: add compact time formatting for session metadata"
```

---

## Task 3: Session Metadata — TUI Display

**Files:**
- Modify: `internal/tui/model.go:32-39` (Session struct — add Created, LastActivity, Tags)
- Modify: `internal/tui/view.go:140-158` (columnWidths, computeColumnWidths)
- Modify: `internal/tui/view.go:186-219` (renderRow)
- Modify: `internal/tui/styles.go` (new styles)
- Modify: `cmd/muxwarp/main.go:372-384` (scannerToTUI)
- Modify: `internal/tui/view_test.go`

- [ ] **Step 1: Write failing test for metadata in row rendering**

Add to `internal/tui/view_test.go`:

```go
func TestView_SessionMetadata(t *testing.T) {
	now := time.Now()
	m := newTestModel(1)

	sessions := []Session{
		{
			Host: "alpha", HostShort: "alpha", Name: "dev",
			Attached: 2, Windows: 3,
			Created:      now.Add(-3 * 24 * time.Hour).Unix(),
			LastActivity: now.Add(-5 * time.Minute).Unix(),
		},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "alpha", Sessions: sessions})
	m = newM.(Model)
	newM, _ = m.Update(ScanDoneMsg{})
	m = newM.(Model)

	v := m.View()
	stripped := ansiRE.ReplaceAllString(v.Content, "")

	// At width 80, should show attached count for multi-attach.
	if !strings.Contains(stripped, "2↗") {
		t.Error("expected attached count '2↗' in output")
	}

	// Should show age.
	if !strings.Contains(stripped, "3d") {
		t.Error("expected age '3d' in output")
	}

	// Should show last activity.
	if !strings.Contains(stripped, "5m ago") {
		t.Error("expected last activity '5m ago' in output")
	}
}

func TestView_SessionMetadata_SingleAttach(t *testing.T) {
	now := time.Now()
	m := newTestModel(1)

	sessions := []Session{
		{
			Host: "alpha", HostShort: "alpha", Name: "dev",
			Attached: 1, Windows: 2,
			Created:      now.Add(-1 * time.Hour).Unix(),
			LastActivity: now.Add(-10 * time.Second).Unix(),
		},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "alpha", Sessions: sessions})
	m = newM.(Model)
	newM, _ = m.Update(ScanDoneMsg{})
	m = newM.(Model)

	v := m.View()
	stripped := ansiRE.ReplaceAllString(v.Content, "")

	// Single attach should NOT show count.
	if strings.Contains(stripped, "1↗") {
		t.Error("single attach should not show '1↗'")
	}
}

func TestView_SessionMetadata_GhostNoMetadata(t *testing.T) {
	m := newTestModel(1)

	sessions := []Session{
		{
			Host: "alpha", HostShort: "alpha", Name: "ghost",
			Desired: &DesiredInfo{Dir: "~/code"},
		},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "alpha", Sessions: sessions})
	m = newM.(Model)

	v := m.View()
	stripped := ansiRE.ReplaceAllString(v.Content, "")

	// Ghost sessions should not show age or activity.
	if strings.Contains(stripped, "ago") {
		t.Error("ghost session should not show 'ago'")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestView_SessionMetadata ./internal/tui/`
Expected: FAIL — `Created` and `LastActivity` not fields on `tui.Session`

- [ ] **Step 3: Add fields to tui.Session**

In `internal/tui/model.go`, update the `Session` struct:

```go
type Session struct {
	Host         string       // full hostname
	HostShort    string       // abbreviated hostname
	Name         string       // tmux session name
	Attached     int          // number of attached clients (0 = free)
	Windows      int          // number of windows
	Created      int64        // unix timestamp of creation (0 if unknown)
	LastActivity int64        // unix timestamp of last activity (0 if unknown)
	Tags         []string     // host tags from config (empty if untagged)
	Desired      *DesiredInfo // non-nil for ghost sessions (desired but not yet created)
}
```

- [ ] **Step 4: Update scannerToTUI in main.go**

In `cmd/muxwarp/main.go`, update the `scannerToTUI` function:

```go
func scannerToTUI(sessions []scanner.Session) []tui.Session {
	result := make([]tui.Session, len(sessions))
	for i, s := range sessions {
		result[i] = tui.Session{
			Host:         s.Host,
			HostShort:    s.HostShort,
			Name:         s.Name,
			Attached:     s.Attached,
			Windows:      s.Windows,
			Created:      s.Created,
			LastActivity: s.LastActivity,
		}
	}
	return result
}
```

- [ ] **Step 5: Add metadata styles**

In `internal/tui/styles.go`, add after `windowDotStyle`:

```go
	// Age/activity metadata text.
	metadataStyle = lipgloss.NewStyle().
		Foreground(colorSlate)

	// Attached count indicator.
	attachedStyle = lipgloss.NewStyle().
		Foreground(colorGreen)
```

- [ ] **Step 6: Add metadata columns to columnWidths and computeColumnWidths**

In `internal/tui/view.go`, replace `columnWidths` and `computeColumnWidths`:

```go
type columnWidths struct {
	maxName     int // max session name length across visible sessions
	maxDots     int // max window count across visible sessions (0 if width < 60)
	maxAge      int // max age string length (0 if width < 70)
	maxActivity int // max last-active string length (0 if width < 80)
}

func (m Model) computeColumnWidths() columnWidths {
	now := time.Now()
	var cols columnWidths
	showDots := m.width >= 60
	showAge := m.width >= 70
	showActivity := m.width >= 80
	for _, s := range m.filtered {
		cols.maxName = max(cols.maxName, len(s.Name))
		if showDots {
			cols.maxDots = max(cols.maxDots, s.Windows)
		}
		if showAge && !s.IsGhost() {
			cols.maxAge = max(cols.maxAge, len(formatAgeSince(s.Created, now)))
		}
		if showActivity && !s.IsGhost() {
			a := formatAgeSince(s.LastActivity, now)
			if a != "" && a != "now" {
				cols.maxActivity = max(cols.maxActivity, len(a+" ago"))
			} else if a == "now" {
				cols.maxActivity = max(cols.maxActivity, 3) // "now"
			}
		}
	}
	return cols
}
```

Add `"time"` to the imports in `view.go`.

- [ ] **Step 7: Add renderAttached, renderAge, renderActivity helper functions**

Add to `internal/tui/view.go`:

```go
// renderAttached returns the attached count indicator, e.g. "2↗".
// Returns empty string if count <= 1 or terminal too narrow.
func renderAttached(s Session, termWidth int) string {
	if termWidth < 65 || s.Attached <= 1 || s.IsGhost() {
		return ""
	}
	return attachedStyle.Render(fmt.Sprintf("%d↗", s.Attached))
}

// renderAge returns the session age, e.g. "3d".
func renderAge(s Session, termWidth int, now time.Time) string {
	if termWidth < 70 || s.IsGhost() {
		return ""
	}
	return metadataStyle.Render(formatAgeSince(s.Created, now))
}

// renderLastActive returns the last activity string, e.g. "5m ago".
func renderLastActive(s Session, termWidth int, now time.Time) string {
	if termWidth < 80 || s.IsGhost() {
		return ""
	}
	age := formatAgeSince(s.LastActivity, now)
	if age == "" {
		return ""
	}
	if age == "now" {
		return metadataStyle.Render("now")
	}
	return metadataStyle.Render(age + " ago")
}
```

- [ ] **Step 8: Update renderRow to include metadata columns**

Replace `renderRow` in `internal/tui/view.go`:

```go
func (m Model) renderRow(idx int, cols columnWidths) string {
	s := m.filtered[idx]
	selected := idx == m.cursor
	now := time.Now()

	sel := renderSelector(selected)
	name := m.renderSessionName(s)
	badge := renderBadge(s, m.width)
	attached := renderAttached(s, m.width)
	dots := renderWindows(s, m.width)
	age := renderAge(s, m.width, now)
	activity := renderLastActive(s, m.width, now)
	host := m.renderHostTag(s, m.width)

	// Pad session name to align badge column.
	namePad := cols.maxName - len(s.Name)
	if namePad > 0 {
		name += strings.Repeat(" ", namePad)
	}

	// Pad dots to align next column.
	dotCount := 0
	if m.width >= 60 {
		dotCount = s.Windows
	}
	dotPad := cols.maxDots - dotCount
	if dotPad > 0 {
		dots += strings.Repeat(" ", dotPad)
	}

	// Build left content.
	left := sel + " " + name + "  " + badge
	if attached != "" {
		left += " " + attached
	}
	if dots != "" {
		left += "  " + dots
	}
	if age != "" {
		left += "  " + age
	}
	if activity != "" {
		left += "  " + activity
	}

	return applyRowSelection(left, host, selected, m.width)
}
```

Remove the now-unused `composLeftContent` function.

- [ ] **Step 9: Run all TUI tests**

Run: `go test -race -count=1 ./internal/tui/`
Expected: ALL PASS

- [ ] **Step 10: Run full test suite**

Run: `make check`
Expected: ALL PASS (lint + tests)

- [ ] **Step 11: Commit**

```bash
git add internal/tui/model.go internal/tui/view.go internal/tui/styles.go internal/tui/view_test.go cmd/muxwarp/main.go
git commit -m "feat: display session metadata in TUI

Show attached count (2↗), age (3d), and last activity (5m ago)
in the session list. Columns are responsive and drop at narrow
terminal widths."
```

---

## Task 4: Host Tags — Config

**Files:**
- Modify: `internal/config/config.go:23-26` (HostEntry struct)
- Modify: `internal/config/config.go:210-277` (marshalHostEntry, marshalMappingHost)
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for tags in config**

Add to `internal/config/config_test.go`:

```go
func TestLoad_WithTags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - target: clint@indigo
    tags: [prod, api]
    sessions:
      - name: dev
  - target: deploy@atlas
    tags: [staging]
  - server3
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	t.Run("host_with_tags", func(t *testing.T) {
		if len(cfg.Hosts[0].Tags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(cfg.Hosts[0].Tags))
		}
		assertString(t, "tag[0]", cfg.Hosts[0].Tags[0], "prod")
		assertString(t, "tag[1]", cfg.Hosts[0].Tags[1], "api")
	})

	t.Run("host_with_one_tag", func(t *testing.T) {
		if len(cfg.Hosts[1].Tags) != 1 {
			t.Fatalf("expected 1 tag, got %d", len(cfg.Hosts[1].Tags))
		}
		assertString(t, "tag[0]", cfg.Hosts[1].Tags[0], "staging")
	})

	t.Run("host_without_tags", func(t *testing.T) {
		if len(cfg.Hosts[2].Tags) != 0 {
			t.Errorf("expected 0 tags, got %d", len(cfg.Hosts[2].Tags))
		}
	})
}

func TestSave_RoundTrip_WithTags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "roundtrip-tags.yaml")

	original := &Config{
		Defaults: Defaults{Timeout: "3s", Term: "xterm-256color"},
		Hosts: []HostEntry{
			{Target: "alice@atlas", Tags: []string{"prod", "api"}},
			{Target: "bob@neptune"},
		},
	}

	if err := Save(original, cfgPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Hosts[0].Tags) != 2 {
		t.Fatalf("expected 2 tags after round-trip, got %d", len(loaded.Hosts[0].Tags))
	}
	assertString(t, "tags[0]", loaded.Hosts[0].Tags[0], "prod")
	assertString(t, "tags[1]", loaded.Hosts[0].Tags[1], "api")

	if len(loaded.Hosts[1].Tags) != 0 {
		t.Errorf("expected 0 tags for untagged host, got %d", len(loaded.Hosts[1].Tags))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestLoad_WithTags ./internal/config/`
Expected: FAIL — `Tags` field doesn't exist on `HostEntry`

- [ ] **Step 3: Add Tags field to HostEntry**

In `internal/config/config.go`:

```go
type HostEntry struct {
	Target   string           `yaml:"target"`
	Tags     []string         `yaml:"tags,omitempty"`
	Sessions []DesiredSession `yaml:"sessions,omitempty"`
}
```

- [ ] **Step 4: Update marshalHostEntry to emit mapping for tagged hosts**

Replace `marshalHostEntry` in `internal/config/config.go`:

```go
func marshalHostEntry(h HostEntry) (yaml.Node, error) {
	if len(h.Sessions) == 0 && len(h.Tags) == 0 {
		return marshalScalarHost(h.Target), nil
	}
	return marshalMappingHost(h)
}
```

- [ ] **Step 5: Update marshalMappingHost to include tags**

In `marshalMappingHost`, add tags output between target and sessions. Replace the function:

```go
func marshalMappingHost(h HostEntry) (yaml.Node, error) {
	mapping := yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	// Add "target" key-value pair.
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "target"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: h.Target},
	)

	// Add "tags" if present.
	if len(h.Tags) > 0 {
		tagsSeq := yaml.Node{
			Kind:  yaml.SequenceNode,
			Tag:   "!!seq",
			Style: yaml.FlowStyle,
		}
		for _, tag := range h.Tags {
			tagsSeq.Content = append(tagsSeq.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tag},
			)
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tags"},
			&tagsSeq,
		)
	}

	// Add "sessions" key and sequence value (if present).
	if len(h.Sessions) > 0 {
		sessionsSeq := yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  "!!seq",
		}
		for _, ds := range h.Sessions {
			sessionMap := yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			}
			sessionMap.Content = append(sessionMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Name},
			)
			if ds.Dir != "" {
				sessionMap.Content = append(sessionMap.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "dir"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Dir},
				)
			}
			if ds.Cmd != "" {
				sessionMap.Content = append(sessionMap.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "cmd"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Cmd},
				)
			}
			sessionsSeq.Content = append(sessionsSeq.Content, &sessionMap)
		}

		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "sessions"},
			&sessionsSeq,
		)
	}

	return mapping, nil
}
```

- [ ] **Step 6: Run all config tests**

Run: `go test -race -count=1 ./internal/config/`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add tags field to host config entries

Tags are optional string arrays on host entries. Hosts with tags
(but no sessions) are serialized as mapping nodes. Tags use YAML
flow style for compact output."
```

---

## Task 5: Host Tags — TUI Tag Filter

**Files:**
- Modify: `internal/tui/model.go` (add tagFilter, allTags fields + ModeTagPicker)
- Modify: `internal/tui/filter.go` (compose tag filter with fuzzy filter)
- Modify: `internal/tui/update.go` (handle `t` key, tag picker interactions)
- Modify: `internal/tui/view.go` (render tag indicator in footer, tag picker overlay)
- Modify: `internal/tui/update_test.go`
- Modify: `cmd/muxwarp/main.go` (propagate tags from config to TUI sessions)

- [ ] **Step 1: Write failing test for tag filtering**

Add to `internal/tui/update_test.go`:

```go
func TestTagFilter(t *testing.T) {
	m := newTestModel(2)

	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1, Tags: []string{"prod"}},
		{Host: "beta", HostShort: "beta", Name: "staging", Attached: 0, Windows: 2, Tags: []string{"staging"}},
		{Host: "gamma", HostShort: "gamma", Name: "build", Attached: 0, Windows: 1, Tags: []string{"prod", "infra"}},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "mixed", Sessions: sessions})
	m = newM.(Model)
	newM, _ = m.Update(ScanDoneMsg{})
	m = newM.(Model)

	if len(m.filtered) != 3 {
		t.Fatalf("before tag filter: got %d filtered, want 3", len(m.filtered))
	}

	// Apply tag filter "prod"
	m.tagFilter = "prod"
	m.applyFilter()

	if len(m.filtered) != 2 {
		t.Fatalf("after tag filter 'prod': got %d filtered, want 2", len(m.filtered))
	}

	// Clear tag filter
	m.tagFilter = ""
	m.applyFilter()

	if len(m.filtered) != 3 {
		t.Fatalf("after clearing tag filter: got %d filtered, want 3", len(m.filtered))
	}
}

func TestAllTags(t *testing.T) {
	m := newTestModel(1)

	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "dev", Tags: []string{"prod", "api"}},
		{Host: "beta", HostShort: "beta", Name: "staging", Tags: []string{"staging"}},
		{Host: "gamma", HostShort: "gamma", Name: "build", Tags: []string{"prod"}},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "mixed", Sessions: sessions})
	m = newM.(Model)

	tags := m.allTags()
	// Should have 3 unique tags: api, prod, staging (sorted)
	if len(tags) != 3 {
		t.Fatalf("allTags() = %v, want 3 tags", tags)
	}
	if tags[0] != "api" || tags[1] != "prod" || tags[2] != "staging" {
		t.Errorf("allTags() = %v, want [api prod staging]", tags)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestTagFilter ./internal/tui/`
Expected: FAIL — `tagFilter` field doesn't exist

- [ ] **Step 3: Add tag state to Model**

In `internal/tui/model.go`, add to the `Model` struct fields (after `matchInfo`):

```go
	tagFilter   string   // active tag filter (empty = no filter)
	tagCursor   int      // cursor position in tag picker
```

Add `ModeTagPicker` to the `Mode` constants:

```go
const (
	ModeList      Mode = iota
	ModeFilter
	ModeTagPicker
	ModeEdit
	ModeWizard
)
```

Add the `allTags` method to `model.go`:

```go
// allTags returns sorted unique tags across all sessions.
func (m Model) allTags() []string {
	seen := make(map[string]bool)
	for _, s := range m.sessions {
		for _, tag := range s.Tags {
			seen[tag] = true
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return tags
}
```

- [ ] **Step 4: Update applyFilter to compose tag filter**

In `internal/tui/filter.go`, update `applyFilter` to apply the tag filter:

```go
func (m *Model) applyFilter() {
	m.matchInfo = make(map[string]matchInfo)

	// Start with all sessions, optionally tag-filtered.
	base := m.sessions
	if m.tagFilter != "" {
		base = m.filterByTag(base)
	}

	if m.filterText == "" {
		m.filtered = base
		m.restoreSelection()
		m.clampCursor()
		return
	}

	matches := fuzzy.FindFrom(m.filterText, sessionSource(base))

	m.filtered = make([]Session, 0, len(matches))
	for _, match := range matches {
		s := base[match.Index]
		m.filtered = append(m.filtered, s)
		m.matchInfo[s.Key()] = matchInfo{indexes: match.MatchedIndexes}
	}

	m.restoreSelection()
	m.clampCursor()
}

// filterByTag returns sessions whose Tags contain the active tag filter.
func (m Model) filterByTag(sessions []Session) []Session {
	var result []Session
	for _, s := range sessions {
		if slices.Contains(s.Tags, m.tagFilter) {
			result = append(result, s)
		}
	}
	return result
}
```

Add `"slices"` to the imports in `filter.go`.

- [ ] **Step 5: Handle `t` key in update.go**

In `internal/tui/update.go`, add `"t"` case to `handleNormalAction`:

```go
case "t":
	return m.handleTagPicker()
```

Add the tag picker handlers:

```go
// handleTagPicker opens or toggles the tag picker.
func (m Model) handleTagPicker() (tea.Model, tea.Cmd) {
	tags := m.allTags()
	if len(tags) == 0 {
		return m, nil
	}
	if m.tagFilter != "" {
		// Already filtering: clear it.
		m.tagFilter = ""
		m.applyFilter()
		m.ensureViewport()
		return m, nil
	}
	m.mode = ModeTagPicker
	m.tagCursor = 0
	return m, nil
}

// handleTagPickerKey processes keys in the tag picker.
func (m Model) handleTagPickerKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	tags := m.allTags()
	switch key {
	case "esc", "t":
		m.mode = ModeList
		return m, nil
	case "enter":
		if m.tagCursor >= 0 && m.tagCursor < len(tags) {
			m.tagFilter = tags[m.tagCursor]
			m.applyFilter()
			m.ensureViewport()
		}
		m.mode = ModeList
		return m, nil
	case "up", "k":
		m.tagCursor = max(m.tagCursor-1, 0)
		return m, nil
	case "down", "j":
		m.tagCursor = min(m.tagCursor+1, len(tags)-1)
		return m, nil
	}
	return m, nil
}
```

In `handleKey`, add the `ModeTagPicker` case before the `ModeFilter` check:

```go
if m.mode == ModeTagPicker {
	return m.handleTagPickerKey(msg, key)
}
```

- [ ] **Step 6: Render tag picker and tag filter indicator in view.go**

Add to `internal/tui/view.go`:

In `renderFooter`, add tag filter display. Before the final return (the normal mode footer), insert:

```go
	footer := "enter warp │ / filter │ t tags │ a add │ e edit │ d delete │ q quit"
	if m.tagFilter != "" {
		tagCount := len(m.filtered)
		tagLabel := filterPromptStyle.Render("tag: " + m.tagFilter)
		countLabel := statusStyle.Render(fmt.Sprintf("%d sessions", tagCount))
		footer = tagLabel + "  " + countLabel + "\n" + footerStyle.Render(
			"t clear │ / filter │ enter warp │ q quit")
		return footer
	}
```

Replace the existing default footer return with:

```go
	return footerStyle.Render(footer)
```

Add `renderTagPicker` method:

```go
// renderTagPicker renders an inline tag selection list in the footer area.
func (m Model) renderTagPicker() string {
	tags := m.allTags()
	var b strings.Builder
	b.WriteString(filterPromptStyle.Render("Select tag:"))
	b.WriteRune('\n')
	for i, tag := range tags {
		if i == m.tagCursor {
			b.WriteString(selectorStyle.Render("▸ "))
			b.WriteString(sessionNameStyle.Render(tag))
		} else {
			b.WriteString("  ")
			b.WriteString(metadataStyle.Render(tag))
		}
		if i < len(tags)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteRune('\n')
	b.WriteString(footerStyle.Render("enter select │ esc cancel"))
	return b.String()
}
```

In `renderListScreen`, swap the footer for tag picker when in ModeTagPicker:

```go
	b.WriteRune('\n')
	if m.mode == ModeTagPicker {
		b.WriteString(m.renderTagPicker())
	} else {
		b.WriteString(m.renderFooter())
	}
```

- [ ] **Step 7: Propagate tags from config to TUI sessions in main.go**

In `cmd/muxwarp/main.go`, update `scanAndSend` to attach tags. Add a helper:

```go
// tagsForHost returns the tags for a given host target from config.
func tagsForHost(cfg *config.Config, target string) []string {
	for _, h := range cfg.Hosts {
		if h.Target == target {
			return h.Tags
		}
	}
	return nil
}
```

Update `scanAndSend` — after building the batch, attach tags:

```go
func scanAndSend(ctx context.Context, cfg *config.Config, timeoutSec string, p *tea.Program) {
	logging.Log().Info("scan starting", "hosts", len(cfg.Hosts))
	var found []tui.Session
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(host string, sessions []scanner.Session) {
		batch := scannerToTUI(sessions)
		tags := tagsForHost(cfg, host)
		for i := range batch {
			batch[i].Tags = tags
		}
		found = append(found, batch...)
		logging.Log().Debug("scan batch", "host", host, "sessions", len(batch))
		p.Send(tui.SessionBatchMsg{Host: host, Sessions: batch})
	})

	ghosts := buildGhosts(cfg, found)
	if len(ghosts) > 0 {
		logging.Log().Info("injecting ghosts", "count", len(ghosts))
		p.Send(tui.SessionBatchMsg{Host: "ghosts", Sessions: ghosts})
	}
	logging.Log().Info("scan complete", "found", len(found), "ghosts", len(ghosts))
	p.Send(tui.ScanDoneMsg{})
}
```

Also update `newGhostSession` to include tags:

```go
func newGhostSession(target string, tags []string, ds config.DesiredSession) tui.Session {
	return tui.Session{
		Host:      target,
		HostShort: ssh.HostShort(target),
		Name:      ds.Name,
		Tags:      tags,
		Desired:   &tui.DesiredInfo{Dir: ds.Dir, Cmd: ds.Cmd},
	}
}
```

Update `ghostsForHost` signature and call site:

```go
func ghostsForHost(h config.HostEntry, found []tui.Session) []tui.Session {
	if len(h.Sessions) == 0 {
		return nil
	}

	existing := existingNames(h.Target, found)
	var ghosts []tui.Session
	for _, ds := range h.Sessions {
		if existing[ds.Name] {
			continue
		}
		ghosts = append(ghosts, newGhostSession(h.Target, h.Tags, ds))
	}
	return ghosts
}
```

- [ ] **Step 8: Run full test suite**

Run: `make check`
Expected: ALL PASS

- [ ] **Step 9: Commit**

```bash
git add internal/tui/model.go internal/tui/filter.go internal/tui/update.go internal/tui/view.go internal/tui/update_test.go cmd/muxwarp/main.go
git commit -m "feat: add host tag filtering to TUI

Press 't' to open tag picker, select a tag to filter sessions.
Tags compose with fuzzy filter. Press 't' again to clear.
Tags propagated from config through scanner to TUI."
```

---

## Task 6: `muxwarp init` Command

**Files:**
- Create: `internal/config/init.go`
- Create: `internal/config/init_test.go`
- Modify: `cmd/muxwarp/main.go` (add `init` subcommand handling)

- [ ] **Step 1: Write failing test for GenerateFromSSHConfig**

Create `internal/config/init_test.go`:

```go
package config

import (
	"strings"
	"testing"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

func TestGenerateFromSSHConfig(t *testing.T) {
	hosts := []sshconfig.Host{
		{Alias: "indigo", HostName: "192.168.1.10", User: "clint"},
		{Alias: "atlas", HostName: "10.0.0.5"},
		{Alias: "github.com", HostName: "github.com", User: "git"},
		{Alias: "gitlab.com", HostName: "gitlab.com"},
		{Alias: "forge", HostName: "forge.local", User: "admin"},
	}

	cfg := GenerateFromSSHConfig(hosts)

	if cfg.Defaults.Timeout != "3s" {
		t.Errorf("timeout = %q, want %q", cfg.Defaults.Timeout, "3s")
	}

	// github.com and gitlab.com should be filtered out.
	if len(cfg.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(cfg.Hosts))
	}

	assertString(t, "hosts[0].Target", cfg.Hosts[0].Target, "indigo")
	assertString(t, "hosts[1].Target", cfg.Hosts[1].Target, "atlas")
	assertString(t, "hosts[2].Target", cfg.Hosts[2].Target, "forge")
}

func TestGenerateFromSSHConfig_Empty(t *testing.T) {
	cfg := GenerateFromSSHConfig(nil)
	if len(cfg.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(cfg.Hosts))
	}
}

func TestGenerateFromSSHConfig_AllFiltered(t *testing.T) {
	hosts := []sshconfig.Host{
		{Alias: "github.com"},
		{Alias: "gitlab.com"},
		{Alias: "bitbucket.org"},
	}

	cfg := GenerateFromSSHConfig(hosts)
	if len(cfg.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(cfg.Hosts))
	}
}

func TestIsGitHost(t *testing.T) {
	tests := []struct {
		alias string
		want  bool
	}{
		{"github.com", true},
		{"gitlab.com", true},
		{"bitbucket.org", true},
		{"bitbucket.com", true},
		{"ssh.dev.azure.com", true},
		{"my-git-server", true},
		{"indigo", false},
		{"atlas", false},
		{"forge", false},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got := isGitHost(tt.alias)
			if got != tt.want {
				t.Errorf("isGitHost(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -count=1 -run TestGenerate ./internal/config/`
Expected: FAIL — `GenerateFromSSHConfig` undefined

- [ ] **Step 3: Implement GenerateFromSSHConfig**

Create `internal/config/init.go`:

```go
package config

import (
	"strings"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

// knownGitHosts are SSH aliases that are code hosting services, not servers.
var knownGitHosts = []string{
	"github.com",
	"gitlab.com",
	"bitbucket.org",
	"bitbucket.com",
	"ssh.dev.azure.com",
}

// GenerateFromSSHConfig creates a Config from parsed SSH config hosts,
// filtering out wildcard and known git hosting entries.
func GenerateFromSSHConfig(hosts []sshconfig.Host) *Config {
	cfg := &Config{
		Defaults: Defaults{
			Timeout: "3s",
			Term:    "xterm-256color",
		},
	}

	for _, h := range hosts {
		if isGitHost(h.Alias) {
			continue
		}
		cfg.Hosts = append(cfg.Hosts, HostEntry{Target: h.Alias})
	}

	return cfg
}

// isGitHost returns true if the alias looks like a code hosting service.
func isGitHost(alias string) bool {
	lower := strings.ToLower(alias)
	for _, known := range knownGitHosts {
		if lower == known {
			return true
		}
	}
	return strings.Contains(lower, "git")
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race -count=1 -run TestGenerate ./internal/config/`
Run: `go test -race -count=1 -run TestIsGitHost ./internal/config/`
Expected: ALL PASS

- [ ] **Step 5: Wire `init` subcommand into main.go**

In `cmd/muxwarp/main.go`, after the `--help` check (line 55) and before the config load (line 57), add:

```go
	if len(args) > 0 && args[0] == "init" {
		runInit(args[1:])
		return
	}
```

Add the `runInit` function:

```go
// runInit handles the `muxwarp init` subcommand.
func runInit(args []string) {
	force := len(args) > 0 && args[0] == "--force"

	cfgPath := config.DefaultPath()

	if !force {
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Fprintf(os.Stderr, "Config already exists: %s\nUse --force to overwrite.\n", cfgPath)
			os.Exit(1)
		}
	}

	hosts := sshconfig.ParseHosts()
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "No ~/.ssh/config found or no hosts defined.")
		fmt.Fprintln(os.Stderr, "Create ~/.muxwarp.config.yaml manually or use the TUI wizard: muxwarp")
		os.Exit(1)
	}

	cfg := config.GenerateFromSSHConfig(hosts)

	if err := config.Save(cfg, cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s with %d hosts from ~/.ssh/config\n", cfgPath, len(cfg.Hosts))
	fmt.Println("Run 'muxwarp' to start scanning. Press 'e' in the TUI to edit config.")
}
```

Update `printUsage` to include `init`:

```go
func printUsage() {
	fmt.Printf(`muxwarp %s — warp into tmux sessions on remote machines

Usage:
  muxwarp                     Launch interactive TUI
  muxwarp <pattern>           Fuzzy-match and warp directly
  muxwarp init [--force]      Generate config from ~/.ssh/config
  muxwarp --log <path>        Write debug logs to file
  muxwarp --version           Print version and exit
  muxwarp --help              Show this help
`, version)
}
```

- [ ] **Step 6: Run full test suite**

Run: `make check`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/config/init.go internal/config/init_test.go cmd/muxwarp/main.go
git commit -m "feat: add 'muxwarp init' to generate config from SSH config

Reads ~/.ssh/config, filters out git hosting services, and writes
~/.muxwarp.config.yaml. Use --force to overwrite existing config."
```

---

## Task 7: Shell Completions

**Files:**
- Create: `completions/muxwarp.bash`
- Create: `completions/muxwarp.zsh`
- Create: `completions/muxwarp.fish`
- Create: `completions/embed.go`
- Modify: `cmd/muxwarp/main.go` (add `--completions` flag)

- [ ] **Step 1: Create bash completion script**

Create `completions/muxwarp.bash`:

```bash
# bash completion for muxwarp

_muxwarp() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "${prev}" in
        --log)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        --completions)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            return 0
            ;;
        init)
            COMPREPLY=( $(compgen -W "--force" -- "${cur}") )
            return 0
            ;;
    esac

    if [[ "${cur}" == -* ]]; then
        opts="--help --version --log --completions"
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi

    # Subcommands
    COMPREPLY=( $(compgen -W "init" -- "${cur}") )
    return 0
}

complete -F _muxwarp muxwarp
```

- [ ] **Step 2: Create zsh completion script**

Create `completions/muxwarp.zsh`:

```zsh
#compdef muxwarp

_muxwarp() {
    local -a commands flags

    flags=(
        '--help[Show help]'
        '--version[Print version]'
        '--log[Write debug logs]:log file:_files'
        '--completions[Output shell completions]:shell:(bash zsh fish)'
    )

    commands=(
        'init:Generate config from ~/.ssh/config'
    )

    _arguments -s \
        "${flags[@]}" \
        '1:command:->command' \
        '*::arg:->args'

    case "$state" in
        command)
            _describe -t commands 'muxwarp command' commands
            ;;
        args)
            case "${words[1]}" in
                init)
                    _arguments '--force[Overwrite existing config]'
                    ;;
            esac
            ;;
    esac
}

_muxwarp "$@"
```

- [ ] **Step 3: Create fish completion script**

Create `completions/muxwarp.fish`:

```fish
# fish completion for muxwarp

complete -c muxwarp -l help -d 'Show help'
complete -c muxwarp -l version -d 'Print version'
complete -c muxwarp -l log -r -F -d 'Write debug logs to file'
complete -c muxwarp -l completions -r -f -a 'bash zsh fish' -d 'Output shell completions'

complete -c muxwarp -n '__fish_use_subcommand' -a init -d 'Generate config from ~/.ssh/config'
complete -c muxwarp -n '__fish_seen_subcommand_from init' -l force -d 'Overwrite existing config'
```

- [ ] **Step 4: Create embed.go for embedding completion scripts**

Create `completions/embed.go`:

```go
// Package completions embeds shell completion scripts.
package completions

import "embed"

//go:embed muxwarp.bash muxwarp.zsh muxwarp.fish
var Scripts embed.FS
```

- [ ] **Step 5: Wire --completions flag into main.go**

In `cmd/muxwarp/main.go`, after the `--help` check:

```go
	if len(args) > 0 && args[0] == "--completions" {
		printCompletions(args[1:])
		return
	}
```

Add the handler:

```go
func printCompletions(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "--completions requires an argument: bash, zsh, or fish")
		os.Exit(1)
	}

	fileMap := map[string]string{
		"bash": "muxwarp.bash",
		"zsh":  "muxwarp.zsh",
		"fish": "muxwarp.fish",
	}

	filename, ok := fileMap[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown shell %q. Supported: bash, zsh, fish\n", args[0])
		os.Exit(1)
	}

	data, err := completions.Scripts.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading completion script: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(data))
}
```

Add the import: `"github.com/clintecker/muxwarp/completions"`

Update `printUsage` to include `--completions`:

```go
func printUsage() {
	fmt.Printf(`muxwarp %s — warp into tmux sessions on remote machines

Usage:
  muxwarp                          Launch interactive TUI
  muxwarp <pattern>                Fuzzy-match and warp directly
  muxwarp init [--force]           Generate config from ~/.ssh/config
  muxwarp --log <path>             Write debug logs to file
  muxwarp --completions <shell>    Output shell completions (bash, zsh, fish)
  muxwarp --version                Print version and exit
  muxwarp --help                   Show this help
`, version)
}
```

- [ ] **Step 6: Run build and verify**

Run: `make build && ./bin/muxwarp --completions bash | head -5`
Expected: First 5 lines of bash completion script

Run: `make check`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add completions/ cmd/muxwarp/main.go
git commit -m "feat: add shell completions for bash, zsh, and fish

Completions are embedded in the binary and printed via
'muxwarp --completions <shell>'. Covers flags, subcommands,
and argument values."
```

---

## Task 8: Homebrew Tap — GoReleaser Config

**Files:**
- Modify: `.goreleaser.yml`

Note: The actual `clintecker/homebrew-tap` repo must be created on GitHub separately. This task only configures goreleaser.

- [ ] **Step 1: Update .goreleaser.yml**

Replace the full `.goreleaser.yml`:

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
    files:
      - completions/*
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

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

checksum:
  name_template: "checksums.txt"

changelog:
  use: git
```

- [ ] **Step 2: Validate goreleaser config**

Run: `goreleaser check` (if goreleaser is installed)
If not installed, just verify YAML is valid: `python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yml'))"`

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat: add Homebrew tap and completion files to goreleaser

Configures goreleaser to push formula to clintecker/homebrew-tap
and include shell completions in release archives."
```

---

## Task 9: VHS Demo Environment

**Files:**
- Create: `demo/Dockerfile`
- Create: `demo/docker-compose.yml`
- Create: `demo/entrypoint.sh`
- Create: `demo/muxwarp.config.yaml`
- Create: `demo/demo.tape`
- Generate: `demo/id_ed25519`, `demo/id_ed25519.pub` (SSH key pair for demo)
- Modify: `Makefile` (add demo targets)
- Modify: `.gitignore` (ignore demo.gif)

- [ ] **Step 1: Generate demo SSH key pair**

Run:
```bash
mkdir -p demo
ssh-keygen -t ed25519 -f demo/id_ed25519 -N "" -C "muxwarp-demo"
```

- [ ] **Step 2: Create Dockerfile**

Create `demo/Dockerfile`:

```dockerfile
FROM alpine:3.21

RUN apk add --no-cache openssh-server tmux bash

RUN ssh-keygen -A

RUN adduser -D -s /bin/bash demo && \
    mkdir -p /home/demo/.ssh && \
    chmod 700 /home/demo/.ssh

COPY id_ed25519.pub /home/demo/.ssh/authorized_keys
RUN chmod 600 /home/demo/.ssh/authorized_keys && \
    chown -R demo:demo /home/demo/.ssh

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22

ENTRYPOINT ["/entrypoint.sh"]
```

- [ ] **Step 3: Create entrypoint.sh**

Create `demo/entrypoint.sh`:

```bash
#!/bin/bash
set -e

# Create tmux sessions as the demo user if SESSIONS is set.
# Format: "name:windows name:windows ..."
if [ -n "$SESSIONS" ]; then
    for entry in $SESSIONS; do
        name="${entry%%:*}"
        windows="${entry##*:}"
        su - demo -c "tmux new-session -d -s '$name'" 2>/dev/null || true
        # Add extra windows (session starts with 1).
        for ((i=1; i<windows; i++)); do
            su - demo -c "tmux new-window -t '$name'" 2>/dev/null || true
        done
    done
fi

# Start sshd in foreground.
exec /usr/sbin/sshd -D -e
```

- [ ] **Step 4: Create docker-compose.yml**

Create `demo/docker-compose.yml`:

```yaml
services:
  atlas:
    build: .
    hostname: atlas
    ports:
      - "2201:22"
    environment:
      SESSIONS: "api-server:3 web-dev:2"

  forge:
    build: .
    hostname: forge
    ports:
      - "2202:22"
    environment:
      SESSIONS: "monitoring:1 build-main:3"

  nebula:
    build: .
    hostname: nebula
    ports:
      - "2203:22"
    environment:
      SESSIONS: "data-pipeline:2"

  comet:
    build: .
    hostname: comet
    ports:
      - "2204:22"
    environment:
      SESSIONS: ""
```

- [ ] **Step 5: Create demo muxwarp config**

Create `demo/muxwarp.config.yaml`:

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

- [ ] **Step 6: Create VHS tape file**

Create `demo/demo.tape`:

```vhs
Output demo/demo.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set Theme "Dracula"

# Set up SSH to not check host keys for demo.
Type `MUXWARP_CONFIG=demo/muxwarp.config.yaml SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i demo/id_ed25519" ./bin/muxwarp`
Enter
Sleep 4s

# Navigate the list.
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Filter by name.
Type "/"
Sleep 500ms
Type "api"
Sleep 1500ms
Escape
Sleep 500ms

# Filter by tag.
Type "t"
Sleep 1s
Enter
Sleep 1500ms
Type "t"
Sleep 500ms

Type "q"
Sleep 500ms
```

- [ ] **Step 7: Update Makefile with demo targets**

Add to `Makefile`:

```makefile
demo-up:
	docker compose -f demo/docker-compose.yml up -d --build

demo-down:
	docker compose -f demo/docker-compose.yml down

demo-record: build
	vhs demo/demo.tape
```

- [ ] **Step 8: Update .gitignore**

Add to `.gitignore` (create if needed):

```gitignore
demo/demo.gif
```

- [ ] **Step 9: Test the demo environment**

Run:
```bash
make demo-up
sleep 3
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i demo/id_ed25519 -p 2201 demo@localhost tmux list-sessions
make demo-down
```

Expected: Output shows `api-server` and `web-dev` sessions.

- [ ] **Step 10: Commit**

```bash
git add demo/ Makefile .gitignore
git commit -m "feat: add Docker demo environment and VHS recording setup

Four sshd containers with pre-created tmux sessions for trying
muxwarp without real servers. VHS tape for recording demo GIF."
```

---

## Task 10: Final Verification and PR

- [ ] **Step 1: Run full test suite**

Run: `make check`
Expected: ALL PASS

- [ ] **Step 2: Verify build**

Run: `make build && ./bin/muxwarp --help`
Expected: Shows updated usage with `init` and `--completions`

Run: `./bin/muxwarp --completions bash | wc -l`
Expected: Non-zero line count

Run: `./bin/muxwarp --version`
Expected: Version output

- [ ] **Step 3: Create PR**

Create branch, push, and open PR with summary of all 5 features.
