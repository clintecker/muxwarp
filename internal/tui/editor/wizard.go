package editor

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/sshconfig"
)

// WizardStep identifies the current wizard step.
type WizardStep int

const (
	WizardStepHost    WizardStep = iota // step 1: enter host target
	WizardStepSession                   // step 2: optional session
)

// WizardSavedMsg is sent when the wizard completes with a config.
type WizardSavedMsg struct {
	Config config.Config
}

// WizardQuitMsg is sent when the user quits the wizard.
type WizardQuitMsg struct{}

// WizardModel is the first-run wizard sub-model.
type WizardModel struct {
	step       WizardStep
	hostInput  textinput.Model
	nameInput  textinput.Model
	dirInput   textinput.Model
	cmdInput   textinput.Model
	focusField int // 0=name, 1=dir, 2=cmd in step 2
	sshHosts   []sshconfig.Host
	hostTarget string // saved from step 1
	width      int
	height     int
	saveErr    string
}

// NewWizard creates a new wizard model.
func NewWizard(sshHosts []sshconfig.Host, width, height int) WizardModel {
	return WizardModel{
		hostInput: newHostInput(sshHosts),
		nameInput: newSessionNameInput(),
		dirInput:  newDirInput(),
		cmdInput:  newCmdInput(),
		sshHosts:  sshHosts,
		width:     width,
		height:    height,
	}
}

// Init returns the initial command (focus host input).
func (m WizardModel) Init() tea.Cmd {
	return m.hostInput.Focus()
}

// Update handles messages for the wizard.
func (m WizardModel) Update(msg tea.Msg) (WizardModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		return m.handleWizardKey(msg)
	}
	return m.updateWizardFocusedInput(msg)
}

func (m WizardModel) handleWizardKey(msg tea.KeyPressMsg) (WizardModel, tea.Cmd) {
	switch m.step {
	case WizardStepHost:
		return m.handleHostStepKey(msg)
	case WizardStepSession:
		return m.handleSessionStepKey(msg)
	}
	return m, nil
}

func (m WizardModel) handleHostStepKey(msg tea.KeyPressMsg) (WizardModel, tea.Cmd) {
	k := msg.String()
	switch k {
	case "enter":
		host := strings.TrimSpace(m.hostInput.Value())
		if host == "" {
			m.saveErr = "host target required"
			return m, nil
		}
		m.hostTarget = host
		m.step = WizardStepSession
		m.saveErr = ""
		m.hostInput.Blur()
		return m, m.nameInput.Focus()
	case "esc", "q":
		return m, func() tea.Msg { return WizardQuitMsg{} }
	}
	var cmd tea.Cmd
	m.hostInput, cmd = m.hostInput.Update(msg)
	m.saveErr = ""
	return m, cmd
}

func (m WizardModel) handleSessionStepKey(msg tea.KeyPressMsg) (WizardModel, tea.Cmd) {
	k := msg.String()
	switch k {
	case "enter", "esc":
		return m.saveWizard(k == "enter")
	case "tab", "shift+tab":
		return m.handleWizardTab(k), nil
	}
	return m.updateSessionStepInput(msg)
}

func (m WizardModel) saveWizard(includeSession bool) (WizardModel, tea.Cmd) {
	cfg := m.buildConfig(includeSession)
	return m, func() tea.Msg { return WizardSavedMsg{Config: cfg} }
}

func (m WizardModel) handleWizardTab(k string) WizardModel {
	delta := 1
	if k == "shift+tab" {
		delta = -1
	}
	m.cycleSessionFocus(delta)
	return m
}

func (m WizardModel) updateSessionStepInput(msg tea.KeyPressMsg) (WizardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focusField {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case 2:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	}
	return m, cmd
}

func (m *WizardModel) cycleSessionFocus(delta int) {
	m.nameInput.Blur()
	m.dirInput.Blur()
	m.cmdInput.Blur()
	m.focusField = (m.focusField + delta + 3) % 3
	switch m.focusField {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.dirInput.Focus()
	case 2:
		m.cmdInput.Focus()
	}
}

func (m WizardModel) buildConfig(includeSession bool) config.Config {
	cfg := config.Config{
		Defaults: config.Defaults{
			Timeout: "3s",
			Term:    "xterm-256color",
		},
	}
	entry := config.HostEntry{Target: m.hostTarget}

	if includeSession {
		name := strings.TrimSpace(m.nameInput.Value())
		if name != "" {
			entry.Sessions = append(entry.Sessions, config.DesiredSession{
				Name: name,
				Dir:  strings.TrimSpace(m.dirInput.Value()),
				Cmd:  strings.TrimSpace(m.cmdInput.Value()),
			})
		}
	}

	cfg.Hosts = append(cfg.Hosts, entry)
	return cfg
}

// View renders the wizard.
func (m WizardModel) View() string {
	var b strings.Builder

	switch m.step {
	case WizardStepHost:
		m.renderHostStep(&b)
	case WizardStepSession:
		m.renderSessionStep(&b)
	}

	return b.String()
}

func (m WizardModel) renderHostStep(b *strings.Builder) {
	b.WriteString(sectionStyle.Render("  Welcome to muxwarp!"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("  Enter your first SSH host target"))
	b.WriteString("\n")
	b.WriteString(m.renderWizardInput(m.hostInput, -1)) // always focused
	b.WriteString("\n")
	b.WriteString(helperStyle.Render("    e.g. user@hostname, 192.168.1.50, or SSH config alias"))

	if m.saveErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.saveErr))
	}

	b.WriteString("\n\n")
	b.WriteString(footerHintStyle.Render("  enter continue │ q quit"))
}

func (m WizardModel) renderSessionStep(b *strings.Builder) {
	b.WriteString(sectionStyle.Render("  Add a session (optional)"))
	b.WriteString("\n")
	b.WriteString(helperStyle.Render("    for " + m.hostTarget))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Session name"))
	b.WriteString("\n")
	b.WriteString(m.renderWizardInput(m.nameInput, 0))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Working directory"))
	b.WriteString(helperStyle.Render(" (optional)"))
	b.WriteString("\n")
	b.WriteString(m.renderWizardInput(m.dirInput, 1))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Command"))
	b.WriteString(helperStyle.Render(" (optional)"))
	b.WriteString("\n")
	b.WriteString(m.renderWizardInput(m.cmdInput, 2))
	b.WriteString("\n\n")

	b.WriteString(footerHintStyle.Render("  enter save │ esc skip │ tab next field"))
}

func (m WizardModel) renderWizardInput(ti textinput.Model, fieldIndex int) string {
	style := focusedBorderStyle
	if fieldIndex >= 0 && m.focusField != fieldIndex {
		style = blurredBorderStyle
	}
	inputWidth := min(m.width-6, 60)
	if inputWidth < 20 {
		inputWidth = 20
	}
	return style.Width(inputWidth).Render(ti.View())
}

func (m WizardModel) updateWizardFocusedInput(msg tea.Msg) (WizardModel, tea.Cmd) {
	if m.step == WizardStepHost {
		var cmd tea.Cmd
		m.hostInput, cmd = m.hostInput.Update(msg)
		return m, cmd
	}
	return m.updateWizardSessionInput(msg)
}

func (m WizardModel) updateWizardSessionInput(msg tea.Msg) (WizardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focusField {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.dirInput, cmd = m.dirInput.Update(msg)
	case 2:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	}
	return m, cmd
}

// WizardResize updates the wizard dimensions.
func (m *WizardModel) WizardResize(width, height int) {
	m.width = width
	m.height = height
}

// Step returns the current wizard step.
func (m WizardModel) Step() WizardStep { return m.step }

// HostTarget returns the host target entered in step 1.
func (m WizardModel) HostTarget() string { return m.hostTarget }

// WizardSaveErr returns the current error, if any.
func (m WizardModel) WizardSaveErr() string { return m.saveErr }
