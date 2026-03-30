package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ansiRE strips ANSI escape sequences for visual position testing.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestView_ReturnsAltScreen(t *testing.T) {
	m := newTestModel(1)
	v := m.View()

	if !v.AltScreen {
		t.Error("View() should set AltScreen = true")
	}
}

func TestView_TypeIsTeaView(_ *testing.T) {
	m := newTestModel(1)
	v := m.View()

	// Compile-time check that v is tea.View (type inferred from View()).
	_ = v
}

func TestView_ContentNotEmpty(t *testing.T) {
	m := newTestModel(1)
	v := m.View()

	if v.Content == "" {
		t.Error("View().Content should not be empty")
	}
}

func TestView_WithSessions(t *testing.T) {
	m := newTestModelWithSessions()
	v := m.View()

	// Should contain the header.
	if !strings.Contains(v.Content, "muxwarp") {
		t.Error("View should contain 'muxwarp' in the header")
	}

	// Should contain session names somewhere in the output.
	// Note: session names are rendered through lipgloss styles, so the raw
	// text may include ANSI codes. We check for the text being present.
	if !strings.Contains(v.Content, "dev") {
		t.Error("View should contain session name 'dev'")
	}
}

func TestView_Scanning(t *testing.T) {
	m := newTestModel(5)
	v := m.View()

	// During scanning, the header should show progress.
	if !strings.Contains(v.Content, "Spooling drives") {
		t.Error("View during scanning should show 'Spooling drives'")
	}
}

func TestRenderEmpty(t *testing.T) {
	m := newTestModel(0)
	// Mark scan done with no sessions.
	newM, _ := m.Update(ScanDoneMsg{})
	m = newM.(Model)

	empty := m.renderEmpty()

	if !strings.Contains(empty, "All gates are calm") {
		t.Errorf("renderEmpty should contain 'All gates are calm', got: %q", empty)
	}
	if !strings.Contains(empty, "no active lanes detected") {
		t.Errorf("renderEmpty should contain 'no active lanes detected', got: %q", empty)
	}
	if !strings.Contains(empty, "ssh") {
		t.Errorf("renderEmpty should contain hint about ssh, got: %q", empty)
	}
	if !strings.Contains(empty, "tmux new") {
		t.Errorf("renderEmpty should contain 'tmux new' hint, got: %q", empty)
	}
}

func TestRenderEmpty_ShownInView(t *testing.T) {
	m := newTestModel(0)
	newM, _ := m.Update(ScanDoneMsg{})
	m = newM.(Model)

	v := m.View()

	if !strings.Contains(v.Content, "All gates are calm") {
		t.Error("View with no sessions should contain the empty state text")
	}
}

func TestRenderFooter_NormalMode(t *testing.T) {
	m := newTestModelWithSessions()
	footer := m.renderFooter()

	// Normal mode footer should contain key hints.
	for _, want := range []string{"warp", "filter", "add", "edit", "delete", "quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("normal footer should contain %q, got: %q", want, footer)
		}
	}
}

func TestRenderFooter_FilterMode(t *testing.T) {
	m := newTestModelWithSessions()

	// Enter filter mode.
	newM, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = newM.(Model)

	footer := m.renderFooter()

	// Filter mode footer should show the filter prompt and help.
	if !strings.Contains(footer, "/") {
		t.Errorf("filter footer should contain '/', got: %q", footer)
	}
	if !strings.Contains(footer, "matches") {
		t.Errorf("filter footer should contain 'matches', got: %q", footer)
	}
	if !strings.Contains(footer, "esc") {
		t.Errorf("filter footer should contain 'esc', got: %q", footer)
	}
}

func TestRenderFooter_EmptySessions(t *testing.T) {
	m := newTestModel(0)
	newM, _ := m.Update(ScanDoneMsg{})
	m = newM.(Model)

	footer := m.renderFooter()

	// Empty state footer should show add, rescan, and quit hints.
	for _, want := range []string{"add", "rescan", "quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("empty footer should contain %q, got: %q", want, footer)
		}
	}
	// Should NOT contain filter since there's nothing to filter.
	if strings.Contains(footer, "filter") {
		t.Errorf("empty footer should not contain 'filter', got: %q", footer)
	}
}

func TestRenderHeader_Scanning(t *testing.T) {
	m := newTestModel(5)

	// Add one batch so scanDone = 1.
	newM, _ := m.Update(SessionBatchMsg{
		Host: "alpha",
		Sessions: []Session{
			{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1},
		},
	})
	m = newM.(Model)

	header := m.renderHeader()

	if !strings.Contains(header, "muxwarp") {
		t.Error("header should contain 'muxwarp'")
	}
	if !strings.Contains(header, "1/5") {
		t.Errorf("scanning header should show progress '1/5', got: %q", header)
	}
}

func TestRenderHeader_DoneScanning(t *testing.T) {
	m := newTestModelWithSessions()
	header := m.renderHeader()

	if !strings.Contains(header, "muxwarp") {
		t.Error("header should contain 'muxwarp'")
	}
	// Should show host/session counts.
	if !strings.Contains(header, "sessions") {
		t.Errorf("done header should contain 'sessions', got: %q", header)
	}
	if !strings.Contains(header, "hosts") {
		t.Errorf("done header should contain 'hosts', got: %q", header)
	}
}

func TestComputeColumnWidths(t *testing.T) {
	m := newTestModelWithSessions()

	cols := m.computeColumnWidths()

	// Sessions: "dev" (3), "build" (5), "staging" (7) — max name = 7
	if cols.maxName != 7 {
		t.Errorf("maxName = %d, want 7", cols.maxName)
	}

	// Windows: dev=1, build=3, staging=2 — max dots = 3 (width=80 >= 60)
	if cols.maxDots != 3 {
		t.Errorf("maxDots = %d, want 3", cols.maxDots)
	}
}

func TestComputeColumnWidths_NarrowTerminal(t *testing.T) {
	m := newTestModelWithSessions()
	m.width = 50 // below 60 threshold

	cols := m.computeColumnWidths()

	if cols.maxDots != 0 {
		t.Errorf("maxDots should be 0 for narrow terminal, got %d", cols.maxDots)
	}
}

func TestColumnAlignment_HostsAligned(t *testing.T) {
	m := newTestModelWithSessions()
	m.width = 100

	cols := m.computeColumnWidths()

	// Render all rows, strip ANSI, find host at rune level (not byte level,
	// since ▪/◇/◆ are multi-byte UTF-8 but 1 visual column each).
	hostPositions := make(map[int]bool)
	for i := range m.filtered {
		row := m.renderRow(i, cols)
		plain := ansiRE.ReplaceAllString(row, "")
		host := m.filtered[i].HostShort
		runes := []rune(plain)
		hostRunes := []rune(host)
		pos := runeIndex(runes, hostRunes)
		if pos == -1 {
			t.Errorf("row %d missing host %q in plain text: %q", i, host, plain)
			continue
		}
		hostPositions[pos] = true
	}

	// All hosts should start at the same visual column.
	if len(hostPositions) > 1 {
		t.Errorf("hosts not column-aligned, found %d different positions", len(hostPositions))
	}
}

// runeIndex finds the last rune-position of needle in haystack.
func runeIndex(haystack, needle []rune) int {
	pos := -1
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if runesMatch(haystack[i:i+len(needle)], needle) {
			pos = i
		}
	}
	return pos
}

func runesMatch(a, b []rune) bool {
	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestView_GhostBadge(t *testing.T) {
	m := newTestModel(1)

	// Add a ghost session and a normal session.
	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1},
		{Host: "alpha", HostShort: "alpha", Name: "newproj", Desired: &DesiredInfo{Dir: "~/code"}},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "alpha", Sessions: sessions})
	m = newM.(Model)
	newM, _ = m.Update(ScanDoneMsg{})
	m = newM.(Model)

	v := m.View()

	if !strings.Contains(v.Content, "NEW") {
		t.Error("View should contain 'NEW' badge for ghost session")
	}
	if !strings.Contains(v.Content, "◌") {
		t.Error("View should contain '◌' symbol for ghost session")
	}
}

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

	if !strings.Contains(stripped, "2↗") {
		t.Error("expected attached count '2↗' in output")
	}
	if !strings.Contains(stripped, "3d") {
		t.Error("expected age '3d' in output")
	}
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

	if strings.Contains(stripped, "ago") {
		t.Error("ghost session should not show 'ago'")
	}
}
