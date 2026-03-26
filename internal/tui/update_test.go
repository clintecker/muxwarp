package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// newTestModel creates a Model with a set window size for testing.
func newTestModel(scanTotal int) Model {
	m := NewModel(scanTotal)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return newM.(Model)
}

// newTestModelWithSessions creates a Model pre-loaded with sessions for testing.
func newTestModelWithSessions() Model {
	m := newTestModel(2)

	// Add sessions via SessionBatchMsg.
	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1},
		{Host: "alpha", HostShort: "alpha", Name: "build", Attached: 1, Windows: 3},
		{Host: "beta", HostShort: "beta", Name: "staging", Attached: 0, Windows: 2},
	}
	newM, _ := m.Update(SessionBatchMsg{Host: "alpha", Sessions: sessions[:2]})
	m = newM.(Model)
	newM, _ = m.Update(SessionBatchMsg{Host: "beta", Sessions: sessions[2:]})
	m = newM.(Model)

	// Mark scanning done.
	newM, _ = m.Update(ScanDoneMsg{})
	m = newM.(Model)
	return m
}

func TestWindowSizeMsg(t *testing.T) {
	m := NewModel(2)
	newM, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(Model)

	if cmd != nil {
		t.Error("WindowSizeMsg should not produce a command")
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}

func assertUpdateInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}

func TestSessionBatchMsg(t *testing.T) {
	m := newTestModel(2)

	batch := SessionBatchMsg{
		Host: "alpha",
		Sessions: []Session{
			{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1},
			{Host: "alpha", HostShort: "alpha", Name: "build", Attached: 1, Windows: 3},
		},
	}

	t.Run("first_batch", func(t *testing.T) {
		newM, cmd := m.Update(batch)
		m = newM.(Model)

		if cmd != nil {
			t.Error("SessionBatchMsg should not produce a command")
		}
		assertUpdateInt(t, "sessions", len(m.sessions), 2)
		assertUpdateInt(t, "scanDone", m.scanDone, 1)
		assertUpdateInt(t, "filtered", len(m.filtered), 2)
	})

	t.Run("second_batch", func(t *testing.T) {
		batch2 := SessionBatchMsg{
			Host: "beta",
			Sessions: []Session{
				{Host: "beta", HostShort: "beta", Name: "staging", Attached: 0, Windows: 2},
			},
		}
		newM, _ := m.Update(batch2)
		m = newM.(Model)

		assertUpdateInt(t, "sessions", len(m.sessions), 3)
		assertUpdateInt(t, "scanDone", m.scanDone, 2)
	})
}

func TestScanDoneMsg(t *testing.T) {
	m := newTestModel(2)

	if !m.scanning {
		t.Error("scanning should be true initially")
	}

	newM, cmd := m.Update(ScanDoneMsg{})
	m = newM.(Model)

	if cmd != nil {
		t.Error("ScanDoneMsg should not produce a command")
	}
	if m.scanning {
		t.Error("scanning should be false after ScanDoneMsg")
	}
}

func TestKeyQ_Quits(t *testing.T) {
	m := newTestModelWithSessions()

	newM, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	_ = newM.(Model)

	if cmd == nil {
		t.Fatal("pressing 'q' should return a command")
	}
	// Execute the command and check it produces a quit message.
	msg := cmd()
	if msg == nil {
		t.Error("quit command should produce a non-nil message")
	}
}

func TestKeyCtrlC_Quits(t *testing.T) {
	m := newTestModelWithSessions()

	newM, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	_ = newM.(Model)

	if cmd == nil {
		t.Fatal("pressing ctrl+c should return a command")
	}
	msg := cmd()
	if msg == nil {
		t.Error("ctrl+c command should produce a non-nil message")
	}
}

func TestKeyEnter_SetsWarpTarget(t *testing.T) {
	m := newTestModelWithSessions()

	newM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := newM.(Model)

	if rm.WarpTarget() == nil {
		t.Fatal("pressing enter with sessions should set warpTarget")
	}
	// The first filtered session should be the warp target (after sort: IDLE first).
	if rm.WarpTarget().Attached != 0 {
		t.Error("warp target should be an IDLE session (first after sort)")
	}
	if cmd == nil {
		t.Error("pressing enter should return tea.Quit command")
	}
}

func TestKeyEnter_NoSessions_NoQuit(t *testing.T) {
	m := newTestModel(1)
	// Mark scan done with no sessions.
	newM, _ := m.Update(ScanDoneMsg{})
	m = newM.(Model)

	newM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := newM.(Model)

	if rm.WarpTarget() != nil {
		t.Error("pressing enter with no sessions should not set warpTarget")
	}
	if cmd != nil {
		t.Error("pressing enter with no sessions should not return a command")
	}
}

func TestKeySlash_EnablesFilterMode(t *testing.T) {
	m := newTestModelWithSessions()

	if m.filtering {
		t.Error("filtering should be false initially")
	}

	newM, cmd := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	rm := newM.(Model)

	if !rm.filtering {
		t.Error("pressing '/' should enable filter mode")
	}
	if cmd != nil {
		t.Error("pressing '/' should not return a command")
	}
}

func TestFilterMode_TypingAddsToFilterText(t *testing.T) {
	m := newTestModelWithSessions()

	// Enter filter mode.
	newM, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = newM.(Model)

	if !m.filtering {
		t.Fatal("should be in filter mode")
	}

	// Type "d".
	newM, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = newM.(Model)

	if m.filterText != "d" {
		t.Errorf("filterText = %q, want %q", m.filterText, "d")
	}

	// Type "e".
	newM, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = newM.(Model)

	if m.filterText != "de" {
		t.Errorf("filterText = %q, want %q", m.filterText, "de")
	}

	// Type "v".
	newM, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = newM.(Model)

	if m.filterText != "dev" {
		t.Errorf("filterText = %q, want %q", m.filterText, "dev")
	}
}

func assertUpdateBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func assertUpdateString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertNoCmd(t *testing.T, label string, cmd tea.Cmd) {
	t.Helper()
	if cmd != nil {
		t.Errorf("%s should not return a command", label)
	}
}

func TestFilterMode_EscapeClearsFilter(t *testing.T) {
	m := newTestModelWithSessions()

	// Enter filter mode and type something.
	newM, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = newM.(Model)

	newM, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = newM.(Model)

	assertUpdateString(t, "filterText", m.filterText, "d")

	// Press Escape.
	newM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = newM.(Model)

	assertUpdateBool(t, "filtering", m.filtering, false)
	assertUpdateString(t, "filterText", m.filterText, "")
	assertNoCmd(t, "Escape", cmd)
	assertUpdateInt(t, "filtered", len(m.filtered), len(m.sessions))
}

func TestFilterMode_BackspaceRemovesLastRune(t *testing.T) {
	m := newTestModelWithSessions()

	// Enter filter mode and type "dev".
	newM, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = newM.(Model)

	for _, ch := range "dev" {
		newM, _ = m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = newM.(Model)
	}

	if m.filterText != "dev" {
		t.Fatalf("filterText = %q, want %q", m.filterText, "dev")
	}

	// Backspace.
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = newM.(Model)

	if m.filterText != "de" {
		t.Errorf("after backspace, filterText = %q, want %q", m.filterText, "de")
	}
}

func TestFilterMode_EnterWarpsToFiltered(t *testing.T) {
	m := newTestModelWithSessions()

	// Enter filter mode and type "dev".
	newM, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = newM.(Model)

	for _, ch := range "dev" {
		newM, _ = m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = newM.(Model)
	}

	// Press Enter to warp.
	newM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := newM.(Model)

	if rm.WarpTarget() == nil {
		t.Fatal("pressing enter in filter mode with matches should set warpTarget")
	}
	if cmd == nil {
		t.Error("pressing enter in filter mode should return tea.Quit")
	}
}

func pressKey(t *testing.T, m Model, code rune) Model {
	t.Helper()
	newM, _ := m.Update(tea.KeyPressMsg{Code: code})
	return newM.(Model)
}

func assertCursor(t *testing.T, label string, m Model, want int) {
	t.Helper()
	if m.cursor != want {
		t.Errorf("%s: cursor = %d, want %d", label, m.cursor, want)
	}
}

func TestKeyNavigation_UpDown(t *testing.T) {
	m := newTestModelWithSessions()
	assertCursor(t, "initial", m, 0)

	m = pressKey(t, m, tea.KeyDown)
	assertCursor(t, "after down", m, 1)

	m = pressKey(t, m, tea.KeyDown)
	assertCursor(t, "after second down", m, 2)

	m = pressKey(t, m, tea.KeyDown)
	assertCursor(t, "clamp at end", m, 2)

	m = pressKey(t, m, tea.KeyUp)
	assertCursor(t, "after up", m, 1)
}

func TestKeyNavigation_JK(t *testing.T) {
	m := newTestModelWithSessions()

	// j = down
	newM, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = newM.(Model)

	if m.cursor != 1 {
		t.Errorf("after 'j', cursor = %d, want 1", m.cursor)
	}

	// k = up
	newM, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = newM.(Model)

	if m.cursor != 0 {
		t.Errorf("after 'k', cursor = %d, want 0", m.cursor)
	}
}

func TestKeyR_TogglesScan(t *testing.T) {
	m := newTestModelWithSessions()

	if m.scanning {
		t.Error("scanning should be false after ScanDoneMsg")
	}

	newM, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	rm := newM.(Model)

	if !rm.scanning {
		t.Error("pressing 'r' should set scanning to true")
	}
}

func TestUnknownMsg_NoOp(t *testing.T) {
	m := newTestModel(1)

	type unknownMsg struct{}
	newM, cmd := m.Update(unknownMsg{})
	rm := newM.(Model)

	if cmd != nil {
		t.Error("unknown message should not produce a command")
	}
	// Model should be unchanged.
	if rm.scanning != m.scanning {
		t.Error("unknown message should not change model state")
	}
}
