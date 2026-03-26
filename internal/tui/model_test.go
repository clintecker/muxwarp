package tui

import (
	"testing"
)

func assertModelInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}

func assertModelBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func assertModelNil(t *testing.T, field string, isNil bool) {
	t.Helper()
	if !isNil {
		t.Errorf("%s should be nil", field)
	}
}

func assertModelNotNil(t *testing.T, field string, isNil bool) {
	t.Helper()
	if isNil {
		t.Errorf("%s should not be nil", field)
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel(5)

	t.Run("scan_state", func(t *testing.T) {
		assertModelBool(t, "scanning", m.scanning, true)
		assertModelInt(t, "scanTotal", m.scanTotal, 5)
		assertModelInt(t, "scanDone", m.scanDone, 0)
	})

	t.Run("collections_empty", func(t *testing.T) {
		assertModelInt(t, "sessions", len(m.sessions), 0)
		assertModelInt(t, "filtered", len(m.filtered), 0)
	})

	t.Run("ui_defaults", func(t *testing.T) {
		assertModelInt(t, "cursor", m.cursor, 0)
		assertModelBool(t, "filtering", m.filtering, false)
		assertModelNil(t, "warpTarget", m.warpTarget == nil)
		assertModelInt(t, "width", m.width, 80)
		assertModelInt(t, "height", m.height, 24)
		assertModelNotNil(t, "matchInfo", m.matchInfo == nil)
	})
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
	// IDLE sessions (Attached==0) should come first after sorting.
	if m.filtered[0].Name != "dev" {
		t.Errorf("NewModelWithSessions: first filtered should be 'dev' (IDLE), got %q", m.filtered[0].Name)
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

func assertSortedSession(t *testing.T, i int, got Session, wantHost, wantName string, wantFree bool) {
	t.Helper()
	isFree := got.Attached == 0
	if got.Host != wantHost || got.Name != wantName || isFree != wantFree {
		t.Errorf("sessions[%d] = {Host:%q, Name:%q, Free:%v}, want {Host:%q, Name:%q, Free:%v}",
			i, got.Host, got.Name, isFree, wantHost, wantName, wantFree)
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

	// Expected order: IDLE first (sorted by host, name), then LIVE (sorted by host, name).
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
		assertSortedSession(t, i, sessions[i], want.host, want.name, want.free)
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
	// overhead is 4 (header 2 + footer 2), so visible = 30-4 = 26
	got := m.visibleRows()
	if got != 26 {
		t.Errorf("visibleRows() = %d, want 26 (height=30, overhead=4)", got)
	}

	// Very small terminal: should clamp to 1.
	m.height = 5
	got = m.visibleRows()
	if got != 1 {
		t.Errorf("visibleRows() = %d, want 1 for small terminal", got)
	}
}
