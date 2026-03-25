package tui

import (
	"sort"

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

// NewModelWithFakeData creates a Model pre-populated with fake sessions for
// visual testing. The real scanner will be wired in later.
func NewModelWithFakeData() Model {
	sessions := []Session{
		{Host: "web-prod-01.acme.io", HostShort: "web-prod-01", Name: "deploy", Attached: 0, Windows: 3},
		{Host: "web-prod-01.acme.io", HostShort: "web-prod-01", Name: "monitoring", Attached: 1, Windows: 2},
		{Host: "db-replica-03.acme.io", HostShort: "db-replica-03", Name: "psql", Attached: 0, Windows: 1},
		{Host: "api-staging.acme.io", HostShort: "api-staging", Name: "dev", Attached: 0, Windows: 4},
		{Host: "api-staging.acme.io", HostShort: "api-staging", Name: "tests", Attached: 2, Windows: 2},
		{Host: "worker-01.acme.io", HostShort: "worker-01", Name: "sidekiq", Attached: 0, Windows: 1},
		{Host: "worker-01.acme.io", HostShort: "worker-01", Name: "console", Attached: 1, Windows: 1},
		{Host: "bastion.acme.io", HostShort: "bastion", Name: "tunnel", Attached: 0, Windows: 1},
		{Host: "ml-gpu-02.acme.io", HostShort: "ml-gpu-02", Name: "training", Attached: 1, Windows: 5},
		{Host: "ml-gpu-02.acme.io", HostShort: "ml-gpu-02", Name: "jupyter", Attached: 0, Windows: 2},
	}

	m := Model{
		sessions:  sessions,
		scanDone:  5,
		scanTotal: 5,
		width:     80,
		height:    24,
		matchInfo: make(map[string]matchInfo),
	}
	sortSessions(m.sessions)
	m.filtered = m.sessions
	return m
}

// NewModel creates an empty Model ready to receive scan results.
func NewModel() Model {
	return Model{
		scanning:  true,
		width:     80,
		height:    24,
		matchInfo: make(map[string]matchInfo),
	}
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
	sort.Slice(sessions, func(i, j int) bool {
		ai := sessions[i]
		aj := sessions[j]

		// FREE before DOCKED
		freeI := ai.Attached == 0
		freeJ := aj.Attached == 0
		if freeI != freeJ {
			return freeI
		}

		// Alphabetical by host
		if ai.Host != aj.Host {
			return ai.Host < aj.Host
		}

		// Alphabetical by name
		return ai.Name < aj.Name
	})
}

// visibleRows returns the number of session rows that fit on screen,
// accounting for header (4 lines) and footer (2 lines).
func (m Model) visibleRows() int {
	// header: 3 lines (box) + 1 blank line = 4
	// footer: 1 blank line + 1 help line = 2
	overhead := 6
	rows := m.height - overhead
	if rows < 1 {
		return 1
	}
	return rows
}
