package tui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// Update implements tea.Model. It handles key presses, window resize events,
// and scanner messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureViewport()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case SessionBatchMsg:
		m.sessions = append(m.sessions, msg.Sessions...)
		sortSessions(m.sessions)
		m.scanDone++
		m.applyFilter()
		m.ensureViewport()
		return m, nil

	case ScanDoneMsg:
		m.scanning = false
		return m, nil
	}

	return m, nil
}

// handleKey routes key presses to the appropriate handler based on the
// current mode (filtering vs normal).
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys that work in any mode.
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.mode == ModeFilter {
		return m.handleFilterKey(msg, key)
	}
	return m.handleNormalKey(key)
}

// handleNormalKey processes keys in normal (non-filter) mode.
func (m Model) handleNormalKey(key string) (tea.Model, tea.Cmd) {
	if delta, ok := cursorDelta(key); ok {
		return m.handleCursorMove(delta)
	}
	return m.handleNormalAction(key)
}

// handleNormalAction dispatches non-cursor keys in normal mode.
func (m Model) handleNormalAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit
	case "enter":
		return m.handleWarp()
	case "/":
		return m.handleEnterFilter()
	case "r":
		return m.handleRescan()
	}
	return m, nil
}

// cursorDelta maps navigation keys to cursor movement deltas.
func cursorDelta(key string) (int, bool) {
	switch key {
	case "up", "k":
		return -1, true
	case "down", "j":
		return 1, true
	}
	return 0, false
}

// handleCursorMove adjusts the cursor by delta and returns the updated model.
func (m Model) handleCursorMove(delta int) (tea.Model, tea.Cmd) {
	m.moveCursor(delta)
	return m, nil
}

// handleWarp selects the current session and triggers quit.
func (m Model) handleWarp() (tea.Model, tea.Cmd) {
	if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
		s := m.filtered[m.cursor]
		m.warpTarget = &s
		return m, tea.Quit
	}
	return m, nil
}

// handleEnterFilter switches to filter mode.
func (m Model) handleEnterFilter() (tea.Model, tea.Cmd) {
	m.mode = ModeFilter
	return m, nil
}

// handleRescan resets the scanning state for a rescan.
func (m Model) handleRescan() (tea.Model, tea.Cmd) {
	m.scanning = true
	m.scanDone = 0
	m.scanTotal = 5 // fake
	return m, nil
}

// handleFilterKey processes keys while the filter input is active.
func (m Model) handleFilterKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	if delta, ok := cursorDelta(key); ok {
		return m.handleCursorMove(delta)
	}
	return m.handleFilterAction(msg, key)
}

// handleFilterAction dispatches non-cursor keys in filter mode.
func (m Model) handleFilterAction(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		return m.handleFilterEsc()
	case "enter":
		return m.handleWarp()
	case "backspace":
		return m.handleFilterBackspace()
	default:
		return m.handleFilterInput(msg, key)
	}
}

// handleFilterEsc exits filter mode and clears the filter text.
func (m Model) handleFilterEsc() (tea.Model, tea.Cmd) {
	m.mode = ModeList
	m.filterText = ""
	m.applyFilter()
	m.updateSelectedKey()
	return m, nil
}

// handleFilterBackspace removes the last rune from the filter text.
func (m Model) handleFilterBackspace() (tea.Model, tea.Cmd) {
	if len(m.filterText) > 0 {
		_, size := utf8.DecodeLastRuneInString(m.filterText)
		m.filterText = m.filterText[:len(m.filterText)-size]
		m.applyFilter()
		m.updateSelectedKey()
	}
	return m, nil
}

// handleFilterInput appends a typed character to the filter text.
func (m Model) handleFilterInput(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	text := msg.Text
	if key == "space" {
		text = " "
	}
	if text != "" {
		m.filterText += text
		m.applyFilter()
		m.updateSelectedKey()
	}
	return m, nil
}

// moveCursor adjusts the cursor by delta and scrolls the viewport.
func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
	m.updateSelectedKey()
	m.ensureViewport()
}

// ensureViewport adjusts viewOffset so the cursor is always visible.
func (m *Model) ensureViewport() {
	visible := m.visibleRows()
	if m.cursor < m.viewOffset {
		m.viewOffset = m.cursor
	}
	if m.cursor >= m.viewOffset+visible {
		m.viewOffset = m.cursor - visible + 1
	}
	m.viewOffset = max(m.viewOffset, 0)
}
