package tui

import (
	"testing"
)

func TestNewModel(t *testing.T) {
	m := NewModel(5)

	if !m.scanning {
		t.Error("NewModel: scanning should be true")
	}
	if m.scanTotal != 5 {
		t.Errorf("NewModel: scanTotal = %d, want 5", m.scanTotal)
	}
	if m.scanDone != 0 {
		t.Errorf("NewModel: scanDone = %d, want 0", m.scanDone)
	}
	if len(m.sessions) != 0 {
		t.Errorf("NewModel: sessions should be empty, got %d", len(m.sessions))
	}
	if len(m.filtered) != 0 {
		t.Errorf("NewModel: filtered should be empty, got %d", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("NewModel: cursor = %d, want 0", m.cursor)
	}
	if m.filtering {
		t.Error("NewModel: filtering should be false")
	}
	if m.warpTarget != nil {
		t.Error("NewModel: warpTarget should be nil")
	}
	if m.width != 80 {
		t.Errorf("NewModel: width = %d, want 80", m.width)
	}
	if m.height != 24 {
		t.Errorf("NewModel: height = %d, want 24", m.height)
	}
	if m.matchInfo == nil {
		t.Error("NewModel: matchInfo map should be initialized")
	}
}

func TestNewModelWithSessions(t *testing.T) {
	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "build", Attached: 1, Windows: 2},
		{Host: "beta", HostShort: "beta", Name: "dev", Attached: 0, Windows: 1},
	}

	m := NewModelWithSessions(sessions, "")

	if m.scanning {
		t.Error("NewModelWithSessions: scanning should be false")
	}
	if len(m.sessions) != 2 {
		t.Errorf("NewModelWithSessions: sessions = %d, want 2", len(m.sessions))
	}
	if len(m.filtered) != 2 {
		t.Errorf("NewModelWithSessions: filtered = %d, want 2", len(m.filtered))
	}
	// FREE sessions (Attached==0) should come first after sorting.
	if m.filtered[0].Name != "dev" {
		t.Errorf("NewModelWithSessions: first filtered should be 'dev' (FREE), got %q", m.filtered[0].Name)
	}
}

func TestNewModelWithSessionsFilter(t *testing.T) {
	sessions := []Session{
		{Host: "alpha", HostShort: "alpha", Name: "build", Attached: 0, Windows: 2},
		{Host: "beta", HostShort: "beta", Name: "dev", Attached: 0, Windows: 1},
	}

	m := NewModelWithSessions(sessions, "dev")

	if !m.filtering {
		t.Error("NewModelWithSessions with filter: filtering should be true")
	}
	if m.filterText != "dev" {
		t.Errorf("filterText = %q, want %q", m.filterText, "dev")
	}
	// Filter "dev" should match at least the "dev" session.
	if len(m.filtered) == 0 {
		t.Error("filtered should not be empty for filter 'dev'")
	}
}

func TestSessionKey(t *testing.T) {
	tests := []struct {
		host string
		name string
		want string
	}{
		{"clint@indigo", "cjdos", "clint@indigo/cjdos"},
		{"alpha", "build", "alpha/build"},
		{"user@host.example.com", "my-session", "user@host.example.com/my-session"},
	}

	for _, tt := range tests {
		s := Session{Host: tt.host, Name: tt.name}
		got := s.Key()
		if got != tt.want {
			t.Errorf("Session{Host:%q, Name:%q}.Key() = %q, want %q",
				tt.host, tt.name, got, tt.want)
		}
	}
}

func TestSortSessions(t *testing.T) {
	sessions := []Session{
		{Host: "beta", HostShort: "beta", Name: "prod", Attached: 1, Windows: 3},
		{Host: "alpha", HostShort: "alpha", Name: "dev", Attached: 0, Windows: 1},
		{Host: "alpha", HostShort: "alpha", Name: "build", Attached: 0, Windows: 2},
		{Host: "beta", HostShort: "beta", Name: "staging", Attached: 0, Windows: 1},
		{Host: "alpha", HostShort: "alpha", Name: "test", Attached: 1, Windows: 1},
	}

	sortSessions(sessions)

	// Expected order:
	// 1. FREE: alpha/build (FREE, alpha < beta, build < dev)
	// 2. FREE: alpha/dev   (FREE, alpha < beta)
	// 3. FREE: beta/staging (FREE, beta)
	// 4. DOCKED: alpha/test (DOCKED, alpha < beta)
	// 5. DOCKED: beta/prod  (DOCKED, beta)

	expected := []struct {
		host string
		name string
		free bool
	}{
		{"alpha", "build", true},
		{"alpha", "dev", true},
		{"beta", "staging", true},
		{"alpha", "test", false},
		{"beta", "prod", false},
	}

	if len(sessions) != len(expected) {
		t.Fatalf("sessions length = %d, want %d", len(sessions), len(expected))
	}

	for i, want := range expected {
		got := sessions[i]
		isFree := got.Attached == 0
		if got.Host != want.host || got.Name != want.name || isFree != want.free {
			t.Errorf("sessions[%d] = {Host:%q, Name:%q, Free:%v}, want {Host:%q, Name:%q, Free:%v}",
				i, got.Host, got.Name, isFree, want.host, want.name, want.free)
		}
	}
}

func TestInitReturnsNil(t *testing.T) {
	m := NewModel(3)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestWarpTargetNilByDefault(t *testing.T) {
	m := NewModel(3)
	if m.WarpTarget() != nil {
		t.Error("WarpTarget() should be nil for a new model")
	}
}

func TestVisibleRows(t *testing.T) {
	m := NewModel(1)
	m.height = 30
	// overhead is 6 (header 4 + footer 2), so visible = 30-6 = 24
	got := m.visibleRows()
	if got != 24 {
		t.Errorf("visibleRows() = %d, want 24 (height=30, overhead=6)", got)
	}

	// Very small terminal: should clamp to 1.
	m.height = 5
	got = m.visibleRows()
	if got != 1 {
		t.Errorf("visibleRows() = %d, want 1 for small terminal", got)
	}
}
