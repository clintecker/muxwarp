package editor

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/sshconfig"
)

func testHostsEditor() []sshconfig.Host {
	return []sshconfig.Host{
		{Alias: "atlas", HostName: "192.168.1.50", User: "alice"},
		{Alias: "forge", HostName: "forge.example.com", User: "deploy"},
	}
}

func TestNew_InitialState(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)

	if m.GetFocus() != FocusHost {
		t.Errorf("initial focus = %d, want FocusHost", m.GetFocus())
	}
	if m.editIndex != -1 {
		t.Errorf("editIndex = %d, want -1", m.editIndex)
	}
	if m.editing {
		t.Error("editing should be false for new editor")
	}
	if len(m.Sessions()) != 0 {
		t.Errorf("sessions = %d, want 0", len(m.Sessions()))
	}
	if m.HostValue() != "" {
		t.Errorf("host value = %q, want empty", m.HostValue())
	}
}

func TestNewForEdit_PrePopulated(t *testing.T) {
	entry := config.HostEntry{
		Target: "alice@atlas",
		Sessions: []config.DesiredSession{
			{Name: "api-server", Dir: "~/code/api"},
			{Name: "web-dev", Dir: "~/code/web", Cmd: "nvim"},
		},
	}
	m := NewForEdit(entry, 2, "api-server", testHostsEditor(), 80, 24)

	if m.HostValue() != "alice@atlas" {
		t.Errorf("host = %q, want %q", m.HostValue(), "alice@atlas")
	}
	if !m.editing {
		t.Error("editing should be true for edit mode")
	}
	if m.editIndex != 2 {
		t.Errorf("editIndex = %d, want 2", m.editIndex)
	}
	if len(m.Sessions()) != 2 {
		t.Fatalf("sessions = %d, want 2", len(m.Sessions()))
	}
	if m.Sessions()[0].Name != "api-server" {
		t.Errorf("session[0].Name = %q, want %q", m.Sessions()[0].Name, "api-server")
	}
}

func TestCycleFocus_Forward(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	// Add a session so all focuses are available.
	m.sessions = []config.DesiredSession{{Name: "test"}}

	// Start at FocusHost (0), cycle forward through all.
	expected := []Focus{FocusList, FocusName, FocusDir, FocusCmd, FocusHost}
	for _, want := range expected {
		m = m.cycleFocus(1)
		if m.GetFocus() != want {
			t.Errorf("after cycleFocus(1): got %d, want %d", m.GetFocus(), want)
		}
	}
}

func TestCycleFocus_Backward(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	// Add a session so all focuses are available.
	m.sessions = []config.DesiredSession{{Name: "test"}}

	// Start at FocusHost (0), cycle backward.
	expected := []Focus{FocusCmd, FocusDir, FocusName, FocusList, FocusHost}
	for _, want := range expected {
		m = m.cycleFocus(-1)
		if m.GetFocus() != want {
			t.Errorf("after cycleFocus(-1): got %d, want %d", m.GetFocus(), want)
		}
	}
}

func TestCycleFocus_NoSessions_StaysOnHost(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	// No sessions — cycleFocus should wrap back to Host.

	m = m.cycleFocus(1)
	if m.GetFocus() != FocusHost {
		t.Errorf("after cycleFocus(1) with no sessions: got %d, want FocusHost", m.GetFocus())
	}
	m = m.cycleFocus(-1)
	if m.GetFocus() != FocusHost {
		t.Errorf("after cycleFocus(-1) with no sessions: got %d, want FocusHost", m.GetFocus())
	}
}

func TestTab_NoSessions_CreatesSessionAndFocusesName(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	// Tab from Host with no sessions should auto-create a session.

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if len(m.Sessions()) != 1 {
		t.Fatalf("expected 1 session after Tab, got %d", len(m.Sessions()))
	}
	if m.GetFocus() != FocusName {
		t.Errorf("focus = %d, want FocusName", m.GetFocus())
	}
}

func TestSave_ValidHost(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")

	updated, cmd := m.trySave()
	if updated.SaveErr() != "" {
		t.Errorf("unexpected save error: %q", updated.SaveErr())
	}
	if cmd == nil {
		t.Fatal("trySave should return a command on success")
	}

	msg := cmd()
	saved, ok := msg.(EditorSavedMsg)
	if !ok {
		t.Fatalf("expected EditorSavedMsg, got %T", msg)
	}
	if saved.Entry.Target != "alice@atlas" {
		t.Errorf("saved target = %q, want %q", saved.Entry.Target, "alice@atlas")
	}
	if saved.EditIndex != -1 {
		t.Errorf("editIndex = %d, want -1", saved.EditIndex)
	}
}

func TestSave_EmptyHost_Error(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	// Don't set host value — it's empty.

	updated, cmd := m.trySave()
	if updated.SaveErr() == "" {
		t.Fatal("expected save error for empty host")
	}
	if cmd != nil {
		t.Error("trySave should return nil command on error")
	}
}

func TestSave_InvalidSessionName_Error(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")
	m.sessions = []config.DesiredSession{
		{Name: "bad;name"},
	}

	updated, cmd := m.trySave()
	if updated.SaveErr() == "" {
		t.Fatal("expected save error for invalid session name")
	}
	if cmd != nil {
		t.Error("trySave should return nil command on error")
	}
}

func TestCancel_ProducesMsg(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	_ = updated
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(EditorCanceledMsg); !ok {
		t.Errorf("expected EditorCanceledMsg, got %T", msg)
	}
}

func TestAddSession(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)

	if len(m.Sessions()) != 0 {
		t.Fatal("should start with no sessions")
	}

	m.addSession()
	if len(m.Sessions()) != 1 {
		t.Fatalf("expected 1 session, got %d", len(m.Sessions()))
	}
	if m.sessionCursor != 0 {
		t.Errorf("cursor = %d, want 0", m.sessionCursor)
	}
	if m.GetFocus() != FocusName {
		t.Errorf("focus = %d, want FocusName", m.GetFocus())
	}
}

func TestDeleteSession(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	m.sessions = []config.DesiredSession{
		{Name: "first"},
		{Name: "second"},
	}
	m.sessionCursor = 0

	m.deleteSession()
	if len(m.Sessions()) != 1 {
		t.Fatalf("expected 1 session after delete, got %d", len(m.Sessions()))
	}
	if m.Sessions()[0].Name != "second" {
		t.Errorf("remaining session = %q, want %q", m.Sessions()[0].Name, "second")
	}
}

func TestDeleteSession_Cancel(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	m.sessions = []config.DesiredSession{{Name: "keep"}}
	m.sessionCursor = 0

	// Initiate delete confirmation.
	m.confirmDelete = true

	// Press 'n' to cancel.
	updated, _ := m.handleDeleteConfirm(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if updated.ConfirmingDelete() {
		t.Error("confirmDelete should be false after 'n'")
	}
	if len(updated.Sessions()) != 1 {
		t.Errorf("session should not be deleted after 'n', got %d", len(updated.Sessions()))
	}
}

func TestNewForEdit_SelectedSession_Existing(t *testing.T) {
	entry := config.HostEntry{
		Target: "alice@atlas",
		Sessions: []config.DesiredSession{
			{Name: "api-server", Dir: "~/code/api"},
			{Name: "web-dev", Dir: "~/code/web"},
		},
	}
	m := NewForEdit(entry, 0, "web-dev", testHostsEditor(), 80, 24)

	// Should select existing session, not create a new one.
	if len(m.Sessions()) != 2 {
		t.Fatalf("sessions = %d, want 2", len(m.Sessions()))
	}
	if m.sessionCursor != 1 {
		t.Errorf("sessionCursor = %d, want 1 (web-dev)", m.sessionCursor)
	}
	if m.GetFocus() != FocusList {
		t.Errorf("focus = %d, want FocusList", m.GetFocus())
	}
}

func TestNewForEdit_SelectedSession_New(t *testing.T) {
	entry := config.HostEntry{Target: "alice@atlas"}
	m := NewForEdit(entry, 0, "tracker", testHostsEditor(), 80, 24)

	// Should create a new session with that name.
	if len(m.Sessions()) != 1 {
		t.Fatalf("sessions = %d, want 1", len(m.Sessions()))
	}
	if m.Sessions()[0].Name != "tracker" {
		t.Errorf("session name = %q, want %q", m.Sessions()[0].Name, "tracker")
	}
	if m.GetFocus() != FocusList {
		t.Errorf("focus = %d, want FocusList", m.GetFocus())
	}
}

func TestView_NotEmpty(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	v := m.View()
	if v == "" {
		t.Error("View() should not be empty")
	}
}

func TestSave_WithSessions(t *testing.T) {
	m := New(testHostsEditor(), 80, 24)
	m.hostInput.SetValue("alice@atlas")
	m.sessions = []config.DesiredSession{
		{Name: "api-server", Dir: "~/code/api"},
		{Name: "", Dir: ""}, // empty session should be stripped
		{Name: "web-dev", Cmd: "nvim"},
	}

	_, cmd := m.trySave()
	if cmd == nil {
		t.Fatal("trySave should return a command")
	}
	msg := cmd()
	saved := msg.(EditorSavedMsg)
	if len(saved.Entry.Sessions) != 2 {
		t.Fatalf("expected 2 non-empty sessions, got %d", len(saved.Entry.Sessions))
	}
}
