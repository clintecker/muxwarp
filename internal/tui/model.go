package tui

import (
	"cmp"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/sshconfig"
	"github.com/clintecker/muxwarp/internal/tui/editor"
)

// Mode represents the current TUI screen.
type Mode int

const (
	ModeList   Mode = iota // default session list
	ModeFilter             // filter input active
	ModeEdit               // config editor
	ModeWizard             // first-run wizard
)

// DesiredInfo holds creation metadata for a ghost session (desired but not yet existing).
type DesiredInfo struct {
	Dir  string
	Repo string
	Cmd  string
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
	mode        Mode               // current TUI screen
	scanning    bool               // scan in progress?
	scanDone    int                // hosts completed
	scanTotal   int                // total hosts
	width       int                // terminal width
	height      int                // terminal height
	warpTarget  *Session           // set on Enter, triggers tea.Quit
	selectedKey string             // stable selection tracking across filter changes
	matchInfo   map[string]matchInfo // fuzzy highlight indexes keyed by Session.Key()
	viewOffset  int                // first visible row in the scrolling list
	configPath          string             // path to the config file
	config              *config.Config     // in-memory config (for editor saves)
	toastText           string             // toast notification text
	toastExpiry         time.Time          // when the toast should disappear
	editor              editor.Model        // config editor sub-model
	wizard              editor.WizardModel  // first-run wizard sub-model
	sshHosts            []sshconfig.Host    // parsed SSH config hosts
	configChanged       bool                   // set after editor save/delete
	confirmDeleteTarget string                 // host target pending delete confirmation
	wizardConfig        *config.Config         // set when wizard completes
	latency             map[string]time.Duration // host target -> last measured latency
}

// SessionBatchMsg delivers a batch of sessions from one host.
type SessionBatchMsg struct {
	Host     string
	Sessions []Session
}

// PromoteGhostMsg delivers real sessions from a host scan. Ghosts matching
// these sessions are replaced with the real versions.
type PromoteGhostMsg struct {
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
		latency:   make(map[string]time.Duration),
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
		latency:   make(map[string]time.Duration),
	}
	sortSessions(m.sessions)
	if filter != "" {
		m.mode = ModeFilter
		m.filterText = filter
		m.applyFilter()
	} else {
		m.filtered = m.sessions
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.mode == ModeWizard {
		return m.wizard.Init()
	}
	return latencyTickCmd()
}

// uniqueHosts returns deduplicated host targets from all sessions.
func (m Model) uniqueHosts() []string {
	seen := make(map[string]bool)
	var hosts []string
	for _, s := range m.sessions {
		if !seen[s.Host] {
			seen[s.Host] = true
			hosts = append(hosts, s.Host)
		}
	}
	return hosts
}

// WarpTarget returns the session the user chose, or nil if they quit.
func (m Model) WarpTarget() *Session {
	return m.warpTarget
}

// Width returns the current terminal width. Used by main.go for warp
// animation after the TUI exits.
func (m Model) Width() int { return m.width }

// SetConfig stores the config and its path for editor saves.
func (m *Model) SetConfig(cfg *config.Config, path string) {
	m.config = cfg
	m.configPath = path
}

// GetMode returns the current TUI mode.
func (m Model) GetMode() Mode { return m.mode }

// SetSSHHosts stores parsed SSH config hosts for editor autocomplete.
func (m *Model) SetSSHHosts(hosts []sshconfig.Host) {
	m.sshHosts = hosts
}

// ConfigChanged returns true if the config was modified (save/delete).
func (m Model) ConfigChanged() bool { return m.configChanged }

// SetWizardMode switches to wizard mode for first-run onboarding.
func (m *Model) SetWizardMode() {
	m.mode = ModeWizard
	m.wizard = editor.NewWizard(m.sshHosts, m.width, m.height)
}

// WizardConfig returns the config produced by the wizard, or nil.
func (m Model) WizardConfig() *config.Config { return m.wizardConfig }

// findHostEntry returns the config entry and index for the currently selected session's host.
func (m Model) findHostEntry() (config.HostEntry, int, bool) {
	if m.config == nil || len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return config.HostEntry{}, -1, false
	}
	target := m.filtered[m.cursor].Host
	for i, h := range m.config.Hosts {
		if h.Target == target {
			return h, i, true
		}
	}
	return config.HostEntry{}, -1, false
}

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

// promoteGhosts replaces ghost sessions with real sessions from scan results.
// Ghosts that match a real session (same host+name) are replaced; ghosts with
// no match are kept. Real sessions with no matching ghost are added.
func (m *Model) promoteGhosts(msg PromoteGhostMsg) {
	// Build set of real session names for this host.
	realNames := make(map[string]Session, len(msg.Sessions))
	for _, s := range msg.Sessions {
		realNames[s.Name] = s
	}

	// Replace matching ghosts with real sessions, track which were matched.
	matched := make(map[string]bool)
	for i, s := range m.sessions {
		if s.Host == msg.Host && s.IsGhost() {
			if real, ok := realNames[s.Name]; ok {
				m.sessions[i] = real
				matched[s.Name] = true
			}
		}
	}

	// Add any real sessions that had no matching ghost.
	for _, s := range msg.Sessions {
		if !matched[s.Name] {
			m.sessions = append(m.sessions, s)
		}
	}

	sortSessions(m.sessions)
	m.scanDone++
	m.applyFilter()
	m.ensureViewport()
}

// visibleRows returns the number of session rows that fit on screen,
// accounting for header (1 line + 1 blank) and footer (1 blank + 1 help).
func (m Model) visibleRows() int {
	// header: 1 line (rule) + 1 blank line = 2
	// footer: 1 blank line + 1 help line = 2
	overhead := 4
	return max(m.height-overhead, 1)
}
