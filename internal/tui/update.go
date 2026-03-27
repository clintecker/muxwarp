package tui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/logging"
	"github.com/clintecker/muxwarp/internal/tui/editor"
)

// Update implements tea.Model. It handles key presses, window resize events,
// scanner messages, and editor messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		switch m.mode {
		case ModeEdit:
			m.editor.Resize(msg.Width, msg.Height)
		case ModeWizard:
			m.wizard.WizardResize(msg.Width, msg.Height)
		}
		m.ensureViewport()
		return m, nil

	case editor.EditorSavedMsg:
		return m.handleEditorSaved(msg)

	case editor.EditorCanceledMsg:
		m.mode = ModeList
		return m, nil

	case editor.WizardSavedMsg:
		m.wizardConfig = &msg.Config
		return m, tea.Quit

	case editor.WizardQuitMsg:
		return m, tea.Quit

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

	// Forward other messages (cursor blink, etc.) to sub-models.
	switch m.mode {
	case ModeEdit:
		return m.updateEditor(msg)
	case ModeWizard:
		return m.updateWizard(msg)
	}

	return m, nil
}

// handleKey routes key presses to the appropriate handler based on the
// current mode.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys that work in any mode.
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Delegate to sub-models by mode.
	switch m.mode {
	case ModeEdit:
		return m.updateEditor(msg)
	case ModeWizard:
		return m.updateWizard(msg)
	}

	if m.mode == ModeFilter {
		return m.handleFilterKey(msg, key)
	}
	return m.handleNormalKey(key)
}

// handleNormalKey processes keys in normal (non-filter) mode.
func (m Model) handleNormalKey(key string) (tea.Model, tea.Cmd) {
	// Handle pending delete confirmation first.
	if m.confirmDeleteTarget != "" {
		return m.handleDeleteConfirm(key)
	}
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
	case "a":
		return m.handleAddHost()
	case "e":
		return m.handleEditHost()
	case "d":
		return m.handleDeleteHost()
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
		logging.Log().Info("warp target selected", "host", s.Host, "session", s.Name, "is_ghost", s.IsGhost())
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

// --- Editor integration ---

// updateEditor delegates a message to the editor sub-model.
func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

// updateWizard delegates a message to the wizard sub-model.
func (m Model) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.Update(msg)
	return m, cmd
}

// handleEditorSaved persists the saved config entry and restarts the TUI.
func (m Model) handleEditorSaved(msg editor.EditorSavedMsg) (tea.Model, tea.Cmd) {
	if m.config == nil {
		m.mode = ModeList
		return m, nil
	}

	logging.Log().Info("editor saved", "target", msg.Entry.Target, "sessions", len(msg.Entry.Sessions))

	if msg.EditIndex >= 0 && msg.EditIndex < len(m.config.Hosts) {
		m.config.Hosts[msg.EditIndex] = msg.Entry
	} else {
		// Adding: merge into existing host if target already configured.
		if idx := findHostByTarget(m.config.Hosts, msg.Entry.Target); idx >= 0 {
			m.config.Hosts[idx].Sessions = mergeSessions(m.config.Hosts[idx].Sessions, msg.Entry.Sessions)
		} else {
			m.config.Hosts = append(m.config.Hosts, msg.Entry)
		}
	}

	if err := config.Save(m.config, m.configPath); err != nil {
		m.mode = ModeList
		return m, nil
	}

	m.configChanged = true
	return m, tea.Quit
}

// findHostByTarget returns the index of the host with the given target, or -1.
func findHostByTarget(hosts []config.HostEntry, target string) int {
	for i, h := range hosts {
		if h.Target == target {
			return i
		}
	}
	return -1
}

// mergeSessions appends new sessions that don't already exist in existing.
func mergeSessions(existing, incoming []config.DesiredSession) []config.DesiredSession {
	names := make(map[string]bool)
	for _, s := range existing {
		names[s.Name] = true
	}
	result := make([]config.DesiredSession, len(existing))
	copy(result, existing)
	for _, s := range incoming {
		if s.Name != "" && !names[s.Name] {
			result = append(result, s)
		}
	}
	return result
}

// handleAddHost opens the editor for adding a new host.
func (m Model) handleAddHost() (tea.Model, tea.Cmd) {
	if m.config == nil {
		return m, nil
	}
	m.editor = editor.New(m.sshHosts, m.width, m.height)
	m.mode = ModeEdit
	return m, m.editor.Init()
}

// handleEditHost opens the editor pre-populated for the current session's host.
func (m Model) handleEditHost() (tea.Model, tea.Cmd) {
	entry, idx, ok := m.findHostEntry()
	if !ok {
		return m, nil
	}
	selectedSession := ""
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		selectedSession = m.filtered[m.cursor].Name
	}
	m.editor = editor.NewForEdit(entry, idx, selectedSession, m.sshHosts, m.width, m.height)
	m.mode = ModeEdit
	return m, m.editor.Init()
}

// handleDeleteHost initiates delete confirmation for the current session's host.
func (m Model) handleDeleteHost() (tea.Model, tea.Cmd) {
	if m.config == nil || len(m.filtered) == 0 {
		return m, nil
	}
	_, _, ok := m.findHostEntry()
	if !ok {
		return m, nil
	}
	m.confirmDeleteTarget = m.filtered[m.cursor].Host
	return m, nil
}

// handleDeleteConfirm handles y/n response to delete confirmation.
func (m Model) handleDeleteConfirm(key string) (tea.Model, tea.Cmd) {
	target := m.confirmDeleteTarget
	m.confirmDeleteTarget = ""

	if key != "y" {
		return m, nil
	}

	logging.Log().Info("host deleted", "target", target)

	for i, h := range m.config.Hosts {
		if h.Target == target {
			m.config.Hosts = append(m.config.Hosts[:i], m.config.Hosts[i+1:]...)
			break
		}
	}

	if err := config.Save(m.config, m.configPath); err != nil {
		return m, nil
	}

	m.configChanged = true
	return m, tea.Quit
}
