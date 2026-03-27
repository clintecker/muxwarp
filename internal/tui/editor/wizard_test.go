package editor

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestWizard_Step1_EnterAdvances(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.Step() != WizardStepSession {
		t.Errorf("step = %d, want WizardStepSession", updated.Step())
	}
	if updated.HostTarget() != "alice@atlas" {
		t.Errorf("hostTarget = %q, want %q", updated.HostTarget(), "alice@atlas")
	}
	if cmd == nil {
		t.Error("enter should return focus command for name input")
	}
}

func TestWizard_Step1_EmptyHost_Blocked(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	// Don't set host value.

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.Step() != WizardStepHost {
		t.Errorf("step = %d, want WizardStepHost (should not advance)", updated.Step())
	}
	if updated.WizardSaveErr() == "" {
		t.Error("expected error for empty host")
	}
}

func TestWizard_Step2_SavesWithSession(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")

	// Advance to step 2.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step() != WizardStepSession {
		t.Fatal("should be in session step")
	}

	m.nameInput.SetValue("api-server")
	m.dirInput.SetValue("~/code/api")

	// Press enter to save.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return save command")
	}

	msg := cmd()
	saved, ok := msg.(WizardSavedMsg)
	if !ok {
		t.Fatalf("expected WizardSavedMsg, got %T", msg)
	}
	if len(saved.Config.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(saved.Config.Hosts))
	}
	if saved.Config.Hosts[0].Target != "alice@atlas" {
		t.Errorf("target = %q, want %q", saved.Config.Hosts[0].Target, "alice@atlas")
	}
	if len(saved.Config.Hosts[0].Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(saved.Config.Hosts[0].Sessions))
	}
	if saved.Config.Hosts[0].Sessions[0].Name != "api-server" {
		t.Errorf("session name = %q, want %q", saved.Config.Hosts[0].Sessions[0].Name, "api-server")
	}
}

func TestWizard_Step2_EscSkipsSession(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")

	// Advance to step 2.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Press esc to skip session.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should return save command")
	}

	msg := cmd()
	saved, ok := msg.(WizardSavedMsg)
	if !ok {
		t.Fatalf("expected WizardSavedMsg, got %T", msg)
	}
	if len(saved.Config.Hosts[0].Sessions) != 0 {
		t.Errorf("expected 0 sessions when skipped, got %d", len(saved.Config.Hosts[0].Sessions))
	}
}

func TestWizard_Quit(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should return quit command")
	}

	msg := cmd()
	if _, ok := msg.(WizardQuitMsg); !ok {
		t.Errorf("expected WizardQuitMsg, got %T", msg)
	}
}

func TestWizard_Step2_TabCycles(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")

	// Advance to step 2.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Should start at focusField 0 (name).
	if m.focusField != 0 {
		t.Errorf("initial focusField = %d, want 0", m.focusField)
	}

	// Tab to dir.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusField != 1 {
		t.Errorf("after tab, focusField = %d, want 1", m.focusField)
	}

	// Tab to cmd.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusField != 2 {
		t.Errorf("after second tab, focusField = %d, want 2", m.focusField)
	}

	// Tab wraps to name.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusField != 0 {
		t.Errorf("after third tab, focusField = %d, want 0 (wrap)", m.focusField)
	}
}

func TestWizard_ViewNotEmpty(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	v := m.View()
	if v == "" {
		t.Error("View() should not be empty")
	}
}

func TestWizard_Defaults(t *testing.T) {
	m := NewWizard(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("test-host")

	// Advance to step 2 and save immediately (empty session).
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	msg := cmd()
	saved := msg.(WizardSavedMsg)

	if saved.Config.Defaults.Timeout != "3s" {
		t.Errorf("timeout = %q, want %q", saved.Config.Defaults.Timeout, "3s")
	}
	if saved.Config.Defaults.Term != "xterm-256color" {
		t.Errorf("term = %q, want %q", saved.Config.Defaults.Term, "xterm-256color")
	}
}
