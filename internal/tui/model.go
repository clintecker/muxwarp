package tui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"
)

// DesiredInfo holds creation metadata for a ghost session (desired but not yet existing).
type DesiredInfo struct {
	Dir string
	Cmd string
}

// Session represents a remote tmux session discovered by the scanner.
type Session struct {
	Host      string       // full hostname
	HostShort string       // abbreviated hostname
	Name      string       // tmux session name
	Attached  int          // number of attached clients (0 = free)
	Windows   int          // number of windows
	Desired   *DesiredInfo // non-nil for ghost sessions (desired but not yet created)
}

// IsGhost returns true if this session is desired but doesn't exist yet.
func (s Session) IsGhost() bool { return s.Desired != nil }

// Key returns a unique identifier for this session.
func (s Session) Key() string { return s.Host + "/" + s.Name }

// matchInfo holds fuzzy match highlight positions for a session.
type matchInfo struct {
	indexes []int // character positions that matched the filter
}

// Model is the main Bubble Tea model for the muxwarp TUI.
type Model struct {
	sessions    []Session          // all sessions from scanner
	filtered    []Session          // filtered subset (or all if no filter)
	cursor      int                // selected index in filtered list
	filterText  string             // current filter input
	filtering   bool               // in filter mode?
	scanning    bool               // scan in progress?
	scanDone    int                // hosts completed
	scanTotal   int                // total hosts
	width       int                // terminal width
	height      int                // terminal height
	warpTarget  *Session           // set on Enter, triggers tea.Quit
	selectedKey string             // stable selection tracking across filter changes
	matchInfo   map[string]matchInfo // fuzzy highlight indexes keyed by Session.Key()
	viewOffset  int                // first visible row in the scrolling list
}

// SessionBatchMsg delivers a batch of sessions from one host.
type SessionBatchMsg struct {
	Host     string
	Sessions []Session
}

// ScanDoneMsg signals that scanning is complete.
type ScanDoneMsg struct{}

// NewModel creates an empty Model ready to receive scan results.
// scanTotal is the number of hosts that will be scanned.
func NewModel(scanTotal int) Model {
	return Model{
		scanning:  true,
		scanTotal: scanTotal,
		width:     80,
		height:    24,
		matchInfo: make(map[string]matchInfo),
	}
}

// NewModelWithSessions creates a Model pre-populated with sessions and an
// optional filter. Used by direct warp mode when multiple matches are found.
func NewModelWithSessions(sessions []Session, filter string) Model {
	m := Model{
		sessions:  sessions,
		width:     80,
		height:    24,
		matchInfo: make(map[string]matchInfo),
	}
	sortSessions(m.sessions)
	if filter != "" {
		m.filtering = true
		m.filterText = filter
		m.applyFilter()
	} else {
		m.filtered = m.sessions
	}
	return m
}

// Init implements tea.Model. No startup command needed for now.
func (m Model) Init() tea.Cmd {
	return nil
}

// WarpTarget returns the session the user chose, or nil if they quit.
func (m Model) WarpTarget() *Session {
	return m.warpTarget
}

// Width returns the current terminal width. Used by main.go for warp
// animation after the TUI exits.
func (m Model) Width() int { return m.width }

// sortSessions sorts sessions: IDLE (Attached==0) first, then LIVE,
// then alphabetically by host then name within each group.
func sortSessions(sessions []Session) {
	slices.SortFunc(sessions, sessionLess)
}

// sessionLess compares two sessions for sorting: IDLE before LIVE,
// then alphabetical by host, then by name.
func sessionLess(a, b Session) int {
	if c := cmp.Compare(attachedRank(a), attachedRank(b)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Host, b.Host); c != 0 {
		return c
	}
	return cmp.Compare(a.Name, b.Name)
}

// attachedRank returns 0 for IDLE, 1 for LIVE, 2 for NEW (ghost) sessions.
func attachedRank(s Session) int {
	if s.IsGhost() {
		return 2
	}
	if s.Attached == 0 {
		return 0
	}
	return 1
}

// visibleRows returns the number of session rows that fit on screen,
// accounting for header (1 line + 1 blank) and footer (1 blank + 1 help).
func (m Model) visibleRows() int {
	// header: 1 line (rule) + 1 blank line = 2
	// footer: 1 blank line + 1 help line = 2
	overhead := 4
	return max(m.height-overhead, 1)
}
