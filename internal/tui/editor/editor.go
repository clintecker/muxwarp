package editor

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/ssh"
	"github.com/clintecker/muxwarp/internal/sshconfig"
)

// Focus identifies which editor component has keyboard focus.
type Focus int

const (
	FocusHost Focus = iota
	FocusList // session mini-list
	FocusName
	FocusDir
	FocusCmd
)

const focusCount = 5

// EditorSavedMsg is sent when the user saves the editor form.
type EditorSavedMsg struct {
	Entry     config.HostEntry
	EditIndex int // -1 for new host
}

// EditorCanceledMsg is sent when the user cancels the editor.
type EditorCanceledMsg struct{}

// Model is the config editor sub-model.
type Model struct {
	hostInput     textinput.Model
	nameInput     textinput.Model
	dirInput      textinput.Model
	cmdInput      textinput.Model
	focus         Focus
	sessions      []config.DesiredSession
	sessionCursor int
	editing       bool // true = edit existing, false = add new
	editIndex     int  // index in Config.Hosts (-1 for new)
	width, height int
	sshHosts      []sshconfig.Host
	confirmDelete bool
	saveErr       string
}

// New creates a new editor model for adding a host.
func New(sshHosts []sshconfig.Host, width, height int) Model {
	m := Model{
		hostInput: newHostInput(sshHosts),
		nameInput: newSessionNameInput(),
		dirInput:  newDirInput(),
		cmdInput:  newCmdInput(),
		editIndex: -1,
		width:     width,
		height:    height,
		sshHosts:  sshHosts,
	}
	m.focusField(FocusHost)
	return m
}

// NewForEdit creates a new editor model pre-populated for editing an existing host.
// selectedSession pre-selects or creates a session with that name.
func NewForEdit(entry config.HostEntry, index int, selectedSession string, sshHosts []sshconfig.Host, width, height int) Model {
	m := New(sshHosts, width, height)
	m.editing = true
	m.editIndex = index
	m.hostInput.SetValue(entry.Target)
	m.sessions = make([]config.DesiredSession, len(entry.Sessions))
	copy(m.sessions, entry.Sessions)

	// Pre-select or create the requested session.
	if selectedSession != "" {
		found := false
		for i, s := range m.sessions {
			if s.Name == selectedSession {
				m.sessionCursor = i
				found = true
				break
			}
		}
		if !found {
			m.sessions = append(m.sessions, config.DesiredSession{Name: selectedSession})
			m.sessionCursor = len(m.sessions) - 1
		}
	}

	if len(m.sessions) > 0 {
		m.loadSession()
		m.focusField(FocusList)
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.hostInput.Focus()
}

// Update handles messages for the editor.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	// Delegate to focused textinput for non-key messages (blink, etc.).
	return m.updateFocusedInput(msg)
}

// View renders the editor form as a string (parent wraps in tea.View).
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(m.renderHostSection())
	b.WriteString("\n\n")
	b.WriteString(m.renderSessionListSection())
	b.WriteString("\n\n")
	b.WriteString(m.renderSessionFieldsSection())
	b.WriteString("\n\n")
	b.WriteString(m.renderEditorFooter())

	if m.saveErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.saveErr))
	}

	return b.String()
}

// --- Key handling ---

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// Handle delete confirmation first.
	if m.confirmDelete {
		return m.handleDeleteConfirm(msg)
	}

	// Global keys.
	if updated, cmd, handled := m.handleGlobalKey(msg); handled {
		return updated, cmd
	}

	// Delegate by focus.
	switch m.focus {
	case FocusHost:
		return m.handleHostKey(msg)
	case FocusList:
		return m.handleListKey(msg)
	default:
		return m.handleSessionFieldKey(msg)
	}
}

func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	k := msg.String()
	switch k {
	case "ctrl+s":
		updated, cmd := m.trySave()
		return updated, cmd, true
	case "esc":
		return m, sendCanceled(), true
	case "tab":
		if m.focus == FocusHost && len(m.sessions) == 0 {
			m.addSession()
			return m, nil, true
		}
		return m.cycleFocus(1), nil, true
	case "shift+tab":
		return m.cycleFocus(-1), nil, true
	}
	return m, nil, false
}

func (m Model) handleHostKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.hostInput, cmd = m.hostInput.Update(msg)
	m.saveErr = ""
	return m, cmd
}

func (m Model) handleListKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "up", "k":
		if m.sessionCursor > 0 {
			m.sessionCursor--
			m.loadSession()
		}
		return m, nil
	case "down", "j":
		if m.sessionCursor < len(m.sessions)-1 {
			m.sessionCursor++
			m.loadSession()
		}
		return m, nil
	case "ctrl+n":
		m.addSession()
		return m, nil
	case "ctrl+d":
		if len(m.sessions) > 0 {
			m.confirmDelete = true
		}
		return m, nil
	case "enter":
		if len(m.sessions) > 0 {
			m.focusField(FocusName)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleSessionFieldKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case FocusName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case FocusDir:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case FocusCmd:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	}
	m.syncSessionFromInputs()
	m.saveErr = ""
	return m, cmd
}

func (m Model) handleDeleteConfirm(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "y":
		m.confirmDelete = false
		m.deleteSession()
	default:
		m.confirmDelete = false
	}
	return m, nil
}

// --- Focus management ---

func (m Model) cycleFocus(delta int) Model {
	next := int(m.focus)
	for range focusCount {
		next = (next + delta + focusCount) % focusCount
		if m.focusAvailable(Focus(next)) {
			break
		}
	}
	m.focusField(Focus(next))
	return m
}

// focusAvailable returns whether the given focus target is usable.
// Session-related focuses are skipped when there are no sessions.
func (m Model) focusAvailable(f Focus) bool {
	if len(m.sessions) > 0 {
		return true
	}
	return f == FocusHost
}

func (m *Model) focusField(f Focus) {
	m.hostInput.Blur()
	m.nameInput.Blur()
	m.dirInput.Blur()
	m.cmdInput.Blur()
	m.focus = f
	switch f {
	case FocusHost:
		m.hostInput.Focus()
	case FocusName:
		m.nameInput.Focus()
	case FocusDir:
		m.dirInput.Focus()
	case FocusCmd:
		m.cmdInput.Focus()
	}
}

// --- Save / build ---

func (m Model) trySave() (Model, tea.Cmd) {
	host := strings.TrimSpace(m.hostInput.Value())
	if host == "" {
		m.saveErr = "host target required"
		return m, nil
	}

	// Validate all session names.
	for _, s := range m.sessions {
		if s.Name == "" {
			continue // empty sessions are stripped
		}
		if !ssh.ValidSessionName(s.Name) {
			m.saveErr = fmt.Sprintf("invalid session name %q — use [A-Za-z0-9._-]", s.Name)
			return m, nil
		}
	}

	entry := m.buildEntry()
	return m, func() tea.Msg {
		return EditorSavedMsg{Entry: entry, EditIndex: m.editIndex}
	}
}

func (m Model) buildEntry() config.HostEntry {
	entry := config.HostEntry{
		Target: strings.TrimSpace(m.hostInput.Value()),
	}
	for _, s := range m.sessions {
		if s.Name != "" {
			entry.Sessions = append(entry.Sessions, s)
		}
	}
	return entry
}

// --- Session list operations ---

func (m *Model) loadSession() {
	if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
		s := m.sessions[m.sessionCursor]
		m.nameInput.SetValue(s.Name)
		m.dirInput.SetValue(s.Dir)
		m.cmdInput.SetValue(s.Cmd)
	}
}

func (m *Model) addSession() {
	m.syncSessionFromInputs()
	m.sessions = append(m.sessions, config.DesiredSession{})
	m.sessionCursor = len(m.sessions) - 1
	m.loadSession()
	m.focusField(FocusName)
}

func (m *Model) deleteSession() {
	if len(m.sessions) == 0 {
		return
	}
	m.sessions = append(m.sessions[:m.sessionCursor], m.sessions[m.sessionCursor+1:]...)
	if m.sessionCursor >= len(m.sessions) {
		m.sessionCursor = max(len(m.sessions)-1, 0)
	}
	if len(m.sessions) > 0 {
		m.loadSession()
	} else {
		m.nameInput.SetValue("")
		m.dirInput.SetValue("")
		m.cmdInput.SetValue("")
		m.focusField(FocusHost)
	}
}

func (m *Model) syncSessionFromInputs() {
	if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
		m.sessions[m.sessionCursor].Name = strings.TrimSpace(m.nameInput.Value())
		m.sessions[m.sessionCursor].Dir = strings.TrimSpace(m.dirInput.Value())
		m.sessions[m.sessionCursor].Cmd = strings.TrimSpace(m.cmdInput.Value())
	}
}

// --- Rendering ---

func (m Model) renderHostSection() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("  Host target"))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.hostInput, FocusHost))
	b.WriteString("\n")
	b.WriteString(helperStyle.Render("    e.g. user@hostname, 192.168.1.50, or SSH config alias"))
	return b.String()
}

func (m Model) renderSessionListSection() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Sessions"))

	if len(m.sessions) == 0 {
		b.WriteString("\n")
		b.WriteString(helperStyle.Render("    No sessions. Press ctrl+n to add one."))
		return b.String()
	}

	for i, s := range m.sessions {
		b.WriteString("\n")
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		line := fmt.Sprintf("    %s", name)
		if s.Dir != "" {
			line += "  " + helperStyle.Render(s.Dir)
		}

		if i == m.sessionCursor && m.focus == FocusList {
			b.WriteString(sessionSelectedStyle.Render("  ▸ " + name))
			if s.Dir != "" {
				b.WriteString("  " + helperStyle.Render(s.Dir))
			}
		} else if i == m.sessionCursor {
			b.WriteString(labelStyle.Render("  ▸ " + name))
			if s.Dir != "" {
				b.WriteString("  " + helperStyle.Render(s.Dir))
			}
		} else {
			b.WriteString(sessionNormalStyle.Render("    " + name))
			if s.Dir != "" {
				b.WriteString("  " + helperStyle.Render(s.Dir))
			}
		}
	}

	if m.confirmDelete && len(m.sessions) > 0 {
		b.WriteString("\n")
		b.WriteString(deletePromptStyle.Render("    Delete this session? y/n"))
	}

	b.WriteString("\n")
	hint := "    ctrl+n add │ ctrl+d delete"
	b.WriteString(helperStyle.Render(hint))

	return b.String()
}

func (m Model) renderSessionFieldsSection() string {
	if len(m.sessions) == 0 {
		return ""
	}

	var b strings.Builder
	// Show which session is being edited.
	name := m.sessions[m.sessionCursor].Name
	if name == "" {
		name = "(new session)"
	}
	b.WriteString(sectionStyle.Render(fmt.Sprintf("  Editing: %s", name)))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("  Session name"))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.nameInput, FocusName))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Working directory"))
	b.WriteString(helperStyle.Render(" (optional)"))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.dirInput, FocusDir))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Command"))
	b.WriteString(helperStyle.Render(" (optional)"))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.cmdInput, FocusCmd))

	return b.String()
}

func (m Model) renderInput(ti textinput.Model, f Focus) string {
	style := blurredBorderStyle
	if m.focus == f {
		style = focusedBorderStyle
	}
	inputWidth := min(m.width-6, 60)
	if inputWidth < 20 {
		inputWidth = 20
	}
	return style.Width(inputWidth).Render(ti.View())
}

func (m Model) renderEditorFooter() string {
	switch m.focus {
	case FocusList:
		return footerHintStyle.Render("  ↑/↓ navigate │ enter edit │ ctrl+n add │ ctrl+d delete │ ctrl+s save")
	default:
		return footerHintStyle.Render("  ctrl+s save │ esc cancel │ tab next field")
	}
}

// --- Helpers ---

func (m Model) updateFocusedInput(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case FocusHost:
		m.hostInput, cmd = m.hostInput.Update(msg)
	case FocusName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case FocusDir:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case FocusCmd:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	}
	return m, cmd
}

func sendCanceled() tea.Cmd {
	return func() tea.Msg { return EditorCanceledMsg{} }
}

// --- Input constructors ---

func newHostInput(sshHosts []sshconfig.Host) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "user@hostname"
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.ShowSuggestions = true

	// Override AcceptSuggestion to right-arrow (Tab is for focus cycling).
	km := textinput.DefaultKeyMap()
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap = km

	// Set SSH host aliases as suggestions.
	aliases := make([]string, len(sshHosts))
	for i, h := range sshHosts {
		aliases[i] = h.Alias
	}
	ti.SetSuggestions(aliases)

	return ti
}

func newSessionNameInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.Prompt = ""
	ti.CharLimit = 256
	// Override AcceptSuggestion to right-arrow.
	km := textinput.DefaultKeyMap()
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap = km
	return ti
}

func newDirInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "~/code/project"
	ti.Prompt = ""
	ti.CharLimit = 512
	km := textinput.DefaultKeyMap()
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap = km
	return ti
}

func newCmdInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "nvim"
	ti.Prompt = ""
	ti.CharLimit = 512
	km := textinput.DefaultKeyMap()
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap = km
	return ti
}

// Resize updates the editor dimensions.
func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
}

// Focus returns the current focus.
func (m Model) GetFocus() Focus { return m.focus }

// Sessions returns the current session list.
func (m Model) Sessions() []config.DesiredSession { return m.sessions }

// HostValue returns the current host input value.
func (m Model) HostValue() string { return m.hostInput.Value() }

// SaveErr returns the current save error, if any.
func (m Model) SaveErr() string { return m.saveErr }

// ConfirmingDelete returns whether a delete confirmation is active.
func (m Model) ConfirmingDelete() bool { return m.confirmDelete }

// _ ensures lipgloss is used (for renderInput).
var _ = lipgloss.Width
