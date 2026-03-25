package tui

import (
	"github.com/sahilm/fuzzy"
)

// sessionSource adapts a []Session slice for the fuzzy.Source interface.
type sessionSource []Session

func (s sessionSource) String(i int) string {
	// Match against "host/name" so both host and session name are searchable.
	return s[i].HostShort + "/" + s[i].Name
}

func (s sessionSource) Len() int { return len(s) }

// applyFilter runs the fuzzy filter on all sessions using the current
// filterText. It updates m.filtered, m.matchInfo, and adjusts the cursor
// to maintain the previously selected session when possible.
func (m *Model) applyFilter() {
	m.matchInfo = make(map[string]matchInfo)

	if m.filterText == "" {
		// No filter: show all sessions in sorted order.
		m.filtered = m.sessions
		m.restoreSelection()
		m.clampCursor()
		return
	}

	matches := fuzzy.FindFrom(m.filterText, sessionSource(m.sessions))

	m.filtered = make([]Session, 0, len(matches))
	for _, match := range matches {
		s := m.sessions[match.Index]
		m.filtered = append(m.filtered, s)
		m.matchInfo[s.Key()] = matchInfo{indexes: match.MatchedIndexes}
	}

	m.restoreSelection()
	m.clampCursor()
}

// restoreSelection tries to keep the cursor on the same session after
// a filter change. If the previously selected session is still in the
// filtered list, the cursor moves to it; otherwise cursor resets to 0.
func (m *Model) restoreSelection() {
	if m.selectedKey == "" {
		return
	}
	for i, s := range m.filtered {
		if s.Key() == m.selectedKey {
			m.cursor = i
			return
		}
	}
	// Previously selected session is no longer visible.
	m.cursor = 0
}

// clampCursor ensures cursor stays within bounds.
func (m *Model) clampCursor() {
	m.cursor = max(m.cursor, 0)
	if len(m.filtered) > 0 {
		m.cursor = min(m.cursor, len(m.filtered)-1)
	}
}

// updateSelectedKey records the key of the currently selected session.
func (m *Model) updateSelectedKey() {
	if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
		m.selectedKey = m.filtered[m.cursor].Key()
	}
}
