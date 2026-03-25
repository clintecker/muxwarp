package tui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"
)

// Session represents a remote tmux session discovered by the scanner.
type Session struct {
	Host      string // full hostname
	HostShort string // abbreviated hostname
	Name      string // tmux session name
	Attached  int    // number of attached clients (0 = free)
	Windows   int    // number of windows
}

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

// sortSessions sorts sessions: FREE (Attached==0) first, then DOCKED,
// then alphabetically by host then name within each group.
func sortSessions(sessions []Session) {
	slices.SortFunc(sessions, sessionLess)
}

// sessionLess compares two sessions for sorting: FREE before DOCKED,
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

// attachedRank returns 0 for FREE sessions and 1 for DOCKED, so FREE sorts first.
func attachedRank(s Session) int {
	if s.Attached == 0 {
		return 0
	}
	return 1
}

// visibleRows returns the number of session rows that fit on screen,
// accounting for header (4 lines) and footer (2 lines).
func (m Model) visibleRows() int {
	// header: 3 lines (box) + 1 blank line = 4
	// footer: 1 blank line + 1 help line = 2
	overhead := 6
	return max(m.height-overhead, 1)
}
