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
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	}

	if m.filtering {
		return m.handleFilterKey(msg, key)
	}
	return m.handleNormalKey(key)
}

// handleNormalKey processes keys in normal (non-filter) mode.
func (m Model) handleNormalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case "enter":
		if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
			s := m.filtered[m.cursor]
			m.warpTarget = &s
			return m, tea.Quit
		}
		return m, nil

	case "/":
		m.filtering = true
		return m, nil

	case "r":
		// Rescan: reset to scanning state. The real scanner will wire in
		// a command here. For now, we just toggle the visual state.
		m.scanning = true
		m.scanDone = 0
		m.scanTotal = 5 // fake
		return m, nil
	}

	return m, nil
}

// handleFilterKey processes keys while the filter input is active.
func (m Model) handleFilterKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.applyFilter()
		m.updateSelectedKey()
		return m, nil

	case "enter":
		// Warp to the selected session even while filtering.
		if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
			s := m.filtered[m.cursor]
			m.warpTarget = &s
			return m, tea.Quit
		}
		return m, nil

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case "backspace":
		if len(m.filterText) > 0 {
			// Remove last rune.
			_, size := utf8.DecodeLastRuneInString(m.filterText)
			m.filterText = m.filterText[:len(m.filterText)-size]
			m.applyFilter()
			m.updateSelectedKey()
		}
		return m, nil

	default:
		// Only accept printable characters (the Text field has the typed rune).
		if msg.Text != "" && key != "space" {
			m.filterText += msg.Text
			m.applyFilter()
			m.updateSelectedKey()
			return m, nil
		}
		if key == "space" {
			m.filterText += " "
			m.applyFilter()
			m.updateSelectedKey()
			return m, nil
		}
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
