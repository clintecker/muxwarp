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
	if result, cmd, handled := m.handleEditorMsg(msg); handled {
		return result, cmd
	}
	return m.handleCoreMsg(msg)
}

// handleEditorMsg processes editor and wizard messages. Returns handled=true if consumed.
func (m Model) handleEditorMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case editor.SavedMsg:
		result, cmd := m.handleEditorSaved(msg)
		return result, cmd, true
	case editor.CanceledMsg:
		m.mode = ModeList
		return m, nil, true
	case editor.WizardSavedMsg:
		m.wizardConfig = &msg.Config
		return m, tea.Quit, true
	case editor.WizardQuitMsg:
		return m, tea.Quit, true
	}
	return m, nil, false
}

// handleCoreMsg processes non-editor messages (keys, scan, latency, resize).
func (m Model) handleCoreMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case SessionBatchMsg:
		return m.handleSessionBatch(msg)
	case PromoteGhostMsg:
		m.promoteGhosts(msg)
		return m, nil
	}
	return m.handleAsyncMsg(msg)
}

// handleAsyncMsg processes scan completion and latency messages.
func (m Model) handleAsyncMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ScanDoneMsg:
		return m.handleScanDone()
	case latencyTickMsg:
		return m.handleLatencyTick()
	case LatencyMsg:
		return m.handleLatencyMsg(msg)
	}
	return m.forwardToSubModel(msg)
}

// handleWindowSize processes terminal resize events.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
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
}

// handleSessionBatch processes a batch of sessions from a single host.
func (m Model) handleSessionBatch(msg SessionBatchMsg) (tea.Model, tea.Cmd) {
	m.sessions = append(m.sessions, msg.Sessions...)
	sortSessions(m.sessions)
	m.scanDone++
	m.applyFilter()
	m.ensureViewport()
	return m, nil
}

// handleScanDone processes the scan completion signal.
func (m Model) handleScanDone() (tea.Model, tea.Cmd) {
	m.scanning = false
	hosts := m.uniqueHosts()
	if len(hosts) > 0 {
		return m, probeAllLatencies(hosts)
	}
	return m, nil
}

// handleLatencyTick processes a periodic latency measurement tick.
func (m Model) handleLatencyTick() (tea.Model, tea.Cmd) {
	hosts := m.uniqueHosts()
	if len(hosts) == 0 {
		return m, latencyTickCmd()
	}
	return m, tea.Batch(probeAllLatencies(hosts), latencyTickCmd())
}

// handleLatencyMsg processes latency measurement results.
func (m Model) handleLatencyMsg(msg LatencyMsg) (tea.Model, tea.Cmd) {
	for host, d := range msg.Results {
		m.latency[host] = d
	}
	return m, nil
}

// forwardToSubModel delegates messages to the active sub-model.
func (m Model) forwardToSubModel(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Delegate to handler by mode.
	switch m.mode {
	case ModeEdit, ModeWizard:
		return m.handleSubModelKey(msg)
	case ModeTagPicker:
		return m.handleTagPickerKey(msg, key)
	case ModeFilter:
		return m.handleFilterKey(msg, key)
	}
	return m.handleNormalKey(key)
}

// handleSubModelKey delegates key events to the active sub-model (editor or wizard).
func (m Model) handleSubModelKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == ModeWizard {
		return m.updateWizard(msg)
	}
	return m.updateEditor(msg)
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
	}
	return m.handleViewOrConfigAction(key)
}

// handleViewOrConfigAction dispatches view (rescan, tags) and config (add, edit, delete) keys.
func (m Model) handleViewOrConfigAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "r":
		return m.handleRescan()
	case "t":
		return m.handleTagPicker()
	}
	return m.handleConfigAction(key)
}

// handleConfigAction dispatches config-editing keys (add, edit, delete).
func (m Model) handleConfigAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "a":
		return m.handleAddHost()
	case "e":
		return m.handleEditHost()
	case "d":
		return m.handleDeleteHost()
	}
	return m, nil
}

// handleTagPicker opens the tag picker, or clears an active tag filter.
func (m Model) handleTagPicker() (tea.Model, tea.Cmd) {
	tags := m.allTags()
	if len(tags) == 0 {
		return m, nil
	}
	if m.tagFilter != "" {
		m.tagFilter = ""
		m.applyFilter()
		m.ensureViewport()
		return m, nil
	}
	m.mode = ModeTagPicker
	m.tagCursor = 0
	return m, nil
}

// handleTagPickerKey processes key events while the tag picker is open.
func (m Model) handleTagPickerKey(_ tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	tags := m.allTags()
	switch key {
	case "esc", "t":
		m.mode = ModeList
		return m, nil
	case "enter":
		return m.handleTagPickerEnter(tags)
	case "up", "k", "left", "h":
		m.tagCursor = max(m.tagCursor-1, 0)
		return m, nil
	case "down", "j", "right", "l":
		m.tagCursor = min(m.tagCursor+1, len(tags)-1)
		return m, nil
	}
	return m, nil
}

// handleTagPickerEnter selects the highlighted tag and returns to list mode.
func (m Model) handleTagPickerEnter(tags []string) (tea.Model, tea.Cmd) {
	if m.tagCursor >= 0 && m.tagCursor < len(tags) {
		m.tagFilter = tags[m.tagCursor]
		m.applyFilter()
		m.ensureViewport()
	}
	m.mode = ModeList
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
func (m Model) handleEditorSaved(msg editor.SavedMsg) (tea.Model, tea.Cmd) {
	if m.config == nil {
		m.mode = ModeList
		return m, nil
	}

	logging.Log().Info("editor saved", "target", msg.Entry.Target, "sessions", len(msg.Entry.Sessions))

	clone := cloneConfig(m.config)
	applyEditorEntry(clone, msg)

	if err := config.Save(clone, m.configPath); err != nil {
		m.mode = ModeList
		return m, nil
	}

	m.config = clone
	m.configChanged = true
	return m, tea.Quit
}

// cloneConfig returns a deep copy of the given Config.
func cloneConfig(src *config.Config) *config.Config {
	dst := &config.Config{Defaults: src.Defaults}
	dst.Hosts = make([]config.HostEntry, len(src.Hosts))
	for i, h := range src.Hosts {
		dst.Hosts[i] = cloneHostEntry(h)
	}
	return dst
}

// cloneHostEntry returns a deep copy of a HostEntry.
func cloneHostEntry(h config.HostEntry) config.HostEntry {
	clone := config.HostEntry{Target: h.Target}
	clone.Tags = append([]string(nil), h.Tags...)
	clone.Sessions = append([]config.DesiredSession(nil), h.Sessions...)
	return clone
}

// applyEditorEntry updates or adds the entry from the editor into the config.
func applyEditorEntry(cfg *config.Config, msg editor.SavedMsg) {
	if msg.EditIndex >= 0 && msg.EditIndex < len(cfg.Hosts) {
		cfg.Hosts[msg.EditIndex] = msg.Entry
		return
	}
	mergeOrAppendToConfig(cfg, msg.Entry)
}

// mergeOrAppendToConfig merges sessions into an existing host or appends a new one.
func mergeOrAppendToConfig(cfg *config.Config, entry config.HostEntry) {
	idx := findHostByTarget(cfg.Hosts, entry.Target)
	if idx < 0 {
		cfg.Hosts = append(cfg.Hosts, entry)
		return
	}
	entry.Sessions = mergeSessions(cfg.Hosts[idx].Sessions, entry.Sessions)
	cfg.Hosts[idx] = entry
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
			names[s.Name] = true
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
