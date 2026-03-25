package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestView_ReturnsAltScreen(t *testing.T) {
	m := newTestModel(1)
	v := m.View()

	if !v.AltScreen {
		t.Error("View() should set AltScreen = true")
	}
}

func TestView_TypeIsTeaView(t *testing.T) {
	m := newTestModel(1)
	v := m.View()

	// Compile-time check that v is tea.View.
	var _ tea.View = v
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

	// Normal mode footer should contain navigation hints.
	if !strings.Contains(footer, "navigate") {
		t.Errorf("normal footer should contain 'navigate', got: %q", footer)
	}
	if !strings.Contains(footer, "enter") {
		t.Errorf("normal footer should contain 'enter', got: %q", footer)
	}
	if !strings.Contains(footer, "filter") {
		t.Errorf("normal footer should contain 'filter', got: %q", footer)
	}
	if !strings.Contains(footer, "quit") {
		t.Errorf("normal footer should contain 'quit', got: %q", footer)
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

	// Empty state footer should show rescan and quit hints.
	if !strings.Contains(footer, "rescan") {
		t.Errorf("empty footer should contain 'rescan', got: %q", footer)
	}
	if !strings.Contains(footer, "quit") {
		t.Errorf("empty footer should contain 'quit', got: %q", footer)
	}
	// Should NOT contain navigate/filter since there's nothing to navigate.
	if strings.Contains(footer, "navigate") {
		t.Errorf("empty footer should not contain 'navigate', got: %q", footer)
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
