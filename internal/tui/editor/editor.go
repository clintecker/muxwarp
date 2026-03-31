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
	FocusRepo
	FocusCmd
)

const focusCount = 6

// SavedMsg is sent when the user saves the editor form.
type SavedMsg struct {
	Entry     config.HostEntry
	EditIndex int // -1 for new host
}

// CanceledMsg is sent when the user cancels the editor.
type CanceledMsg struct{}

// Model is the config editor sub-model.
type Model struct {
	hostInput     textinput.Model
	nameInput     textinput.Model
	dirInput      textinput.Model
	repoInput     textinput.Model
	cmdInput      textinput.Model
	focus         Focus
	sessions      []config.DesiredSession
	sessionCursor int
	editing       bool // true = edit existing, false = add new
	editIndex     int  // index in Config.Hosts (-1 for new)
	originalTags  []string
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
		repoInput: newRepoInput(),
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

	m.originalTags = entry.Tags
	m.preselectSession(selectedSession)

	if len(m.sessions) > 0 {
		m.loadSession()
		m.focusField(FocusList)
	}
	return m
}

// preselectSession selects an existing session by name, or creates one if not found.
func (m *Model) preselectSession(name string) {
	if name == "" {
		return
	}
	for i, s := range m.sessions {
		if s.Name == name {
			m.sessionCursor = i
			return
		}
	}
	m.sessions = append(m.sessions, config.DesiredSession{Name: name})
	m.sessionCursor = len(m.sessions) - 1
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.hostInput.Focus()
}

// Update handles messages for the editor.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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

	return m.handleFocusedKey(msg)
}

// handleFocusedKey delegates key events to the handler for the currently focused field.
func (m Model) handleFocusedKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
		updated := m.handleTab()
		return updated, nil, true
	case "shift+tab":
		return m.cycleFocus(-1), nil, true
	}
	return m, nil, false
}

// handleTab handles the tab key: auto-creates a session if needed, otherwise cycles focus.
func (m Model) handleTab() Model {
	if m.focus == FocusHost && len(m.sessions) == 0 {
		m.addSession()
		return m
	}
	return m.cycleFocus(1)
}

func (m Model) handleHostKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.hostInput, cmd = m.hostInput.Update(msg)
	m.saveErr = ""
	return m, cmd
}

func (m Model) handleListKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	k := msg.String()
	if delta, ok := listNavDelta(k); ok {
		m.moveSessionCursor(delta)
	} else {
		m.handleListAction(k)
	}
	return m, nil
}

// listNavDelta maps list navigation keys to cursor deltas.
func listNavDelta(k string) (int, bool) {
	switch k {
	case "up", "k":
		return -1, true
	case "down", "j":
		return 1, true
	}
	return 0, false
}

// handleListAction handles non-navigation keys in list mode.
func (m *Model) handleListAction(k string) {
	switch k {
	case "ctrl+n":
		m.addSession()
	case "ctrl+d":
		m.maybeConfirmDelete()
	case "enter":
		m.maybeEditSession()
	}
}

// moveSessionCursor moves the session cursor by delta and reloads the session.
func (m *Model) moveSessionCursor(delta int) {
	next := m.sessionCursor + delta
	if next >= 0 && next < len(m.sessions) {
		m.sessionCursor = next
		m.loadSession()
	}
}

// maybeConfirmDelete starts delete confirmation if sessions exist.
func (m *Model) maybeConfirmDelete() {
	if len(m.sessions) > 0 {
		m.confirmDelete = true
	}
}

// maybeEditSession focuses the name field if sessions exist.
func (m *Model) maybeEditSession() {
	if len(m.sessions) > 0 {
		m.focusField(FocusName)
	}
}

func (m Model) handleSessionFieldKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case FocusName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case FocusDir:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case FocusRepo:
		m.repoInput, cmd = m.repoInput.Update(msg)
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
	m.blurAll()
	m.focus = f
	m.focusInput(f)
}

// blurAll removes focus from all text inputs.
func (m *Model) blurAll() {
	m.hostInput.Blur()
	m.nameInput.Blur()
	m.dirInput.Blur()
	m.repoInput.Blur()
	m.cmdInput.Blur()
}

// focusInput sets focus on the input matching the given focus target.
func (m *Model) focusInput(f Focus) {
	if f == FocusHost {
		m.hostInput.Focus()
		return
	}
	m.focusSessionInput(f)
}

// focusSessionInput sets focus on a session-related input field.
func (m *Model) focusSessionInput(f Focus) {
	switch f {
	case FocusName:
		m.nameInput.Focus()
	case FocusDir:
		m.dirInput.Focus()
	case FocusRepo:
		m.repoInput.Focus()
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

	if err := validateSessions(m.sessions); err != "" {
		m.saveErr = err
		return m, nil
	}

	entry := m.buildEntry()
	return m, func() tea.Msg {
		return SavedMsg{Entry: entry, EditIndex: m.editIndex}
	}
}

// validateSessions checks all sessions and returns an error message, or "" if valid.
func validateSessions(sessions []config.DesiredSession) string {
	for _, s := range sessions {
		if err := validateSessionEntry(s); err != "" {
			return err
		}
	}
	return ""
}

// validateSessionEntry validates a single session entry, handling both named and unnamed cases.
func validateSessionEntry(s config.DesiredSession) string {
	if s.Name == "" {
		return checkOrphanedFields(s)
	}
	return validateOneSession(s)
}

// checkOrphanedFields returns an error if a nameless session has dir or cmd populated.
func checkOrphanedFields(s config.DesiredSession) string {
	if s.Dir != "" || s.Cmd != "" {
		return "session has working directory or command but no name — add a name or clear the fields"
	}
	return ""
}

// validateOneSession validates a single session's name and repo fields.
func validateOneSession(s config.DesiredSession) string {
	if !ssh.ValidSessionName(s.Name) {
		return fmt.Sprintf("invalid session name %q — avoid control characters and colons", s.Name)
	}
	return validateSessionRepo(s)
}

// validateSessionRepo validates the repo-related fields of a session.
func validateSessionRepo(s config.DesiredSession) string {
	if s.Repo == "" {
		return ""
	}
	if s.Dir == "" {
		return fmt.Sprintf("session %q: repo requires a working directory", s.Name)
	}
	if !ssh.ValidRepoSlug(s.Repo) {
		return fmt.Sprintf("invalid repo slug %q — use owner/repo format", s.Repo)
	}
	return ""
}

func (m Model) buildEntry() config.HostEntry {
	entry := config.HostEntry{
		Target: strings.TrimSpace(m.hostInput.Value()),
		Tags:   m.originalTags,
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
		m.repoInput.SetValue(s.Repo)
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
		m.repoInput.SetValue("")
		m.cmdInput.SetValue("")
		m.focusField(FocusHost)
	}
}

func (m *Model) syncSessionFromInputs() {
	if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
		m.sessions[m.sessionCursor].Name = strings.TrimSpace(m.nameInput.Value())
		m.sessions[m.sessionCursor].Dir = strings.TrimSpace(m.dirInput.Value())
		m.sessions[m.sessionCursor].Repo = strings.TrimSpace(m.repoInput.Value())
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
		b.WriteString(m.renderSessionItem(i, s))
	}

	m.renderDeletePrompt(&b)

	b.WriteString("\n")
	b.WriteString(helperStyle.Render("    ctrl+n add │ ctrl+d delete"))

	return b.String()
}

// renderSessionItem renders a single session list item with appropriate styling.
func (m Model) renderSessionItem(i int, s config.DesiredSession) string {
	name := s.Name
	if name == "" {
		name = "(unnamed)"
	}
	styled := m.styleSessionName(i, name)
	if s.Dir != "" {
		styled += "  " + helperStyle.Render(s.Dir)
	}
	return styled
}

// styleSessionName returns the styled session name based on cursor and focus.
func (m Model) styleSessionName(i int, name string) string {
	if i != m.sessionCursor {
		return sessionNormalStyle.Render("    " + name)
	}
	if m.focus == FocusList {
		return sessionSelectedStyle.Render("  ▸ " + name)
	}
	return labelStyle.Render("  ▸ " + name)
}

// renderDeletePrompt appends the delete confirmation prompt if active.
func (m Model) renderDeletePrompt(b *strings.Builder) {
	if m.confirmDelete && len(m.sessions) > 0 {
		b.WriteString("\n")
		b.WriteString(deletePromptStyle.Render("    Delete this session? y/n"))
	}
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

	b.WriteString(labelStyle.Render("  Repo"))
	b.WriteString(helperStyle.Render(" (optional, GitHub owner/repo)"))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.repoInput, FocusRepo))
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
	cmd := m.delegateToInput(msg)
	return m, cmd
}

// delegateToInput sends a message to the currently focused text input.
func (m *Model) delegateToInput(msg tea.Msg) tea.Cmd {
	if m.focus == FocusHost {
		var cmd tea.Cmd
		m.hostInput, cmd = m.hostInput.Update(msg)
		return cmd
	}
	return m.delegateToSessionInput(msg)
}

// delegateToSessionInput sends a message to the currently focused session input.
func (m *Model) delegateToSessionInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focus {
	case FocusName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case FocusDir:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case FocusRepo:
		m.repoInput, cmd = m.repoInput.Update(msg)
	default:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	}
	return cmd
}

func sendCanceled() tea.Cmd {
	return func() tea.Msg { return CanceledMsg{} }
}

// --- Input constructors ---

// newInput creates a textinput with common defaults: no prompt, right-arrow
// for suggestion acceptance (Tab is reserved for focus cycling).
func newInput(placeholder string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.CharLimit = charLimit
	km := textinput.DefaultKeyMap()
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	ti.KeyMap = km
	return ti
}

func newHostInput(sshHosts []sshconfig.Host) textinput.Model {
	ti := newInput("user@hostname", 256)
	ti.ShowSuggestions = true

	aliases := make([]string, len(sshHosts))
	for i, h := range sshHosts {
		aliases[i] = h.Alias
	}
	ti.SetSuggestions(aliases)

	return ti
}

func newSessionNameInput() textinput.Model { return newInput("session-name", 256) }
func newDirInput() textinput.Model         { return newInput("~/code/project", 512) }
func newRepoInput() textinput.Model        { return newInput("owner/repo", 256) }
func newCmdInput() textinput.Model         { return newInput("nvim", 512) }

// Resize updates the editor dimensions.
func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
}

// GetFocus returns the current focus.
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
