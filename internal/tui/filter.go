package tui

import (
	"slices"

	"github.com/sahilm/fuzzy"
)

// sessionSource adapts a []Session slice for the fuzzy.Source interface.
type sessionSource []Session

func (s sessionSource) String(i int) string {
	// Match against "host/name" so both host and session name are searchable.
	return s[i].HostShort + "/" + s[i].Name
}

func (s sessionSource) Len() int { return len(s) }

// applyFilter runs the tag filter then the fuzzy filter, updating m.filtered
// and m.matchInfo. The cursor is restored to the previously selected session.
func (m *Model) applyFilter() {
	m.matchInfo = make(map[string]matchInfo)
	base := m.sessions
	if m.tagFilter != "" {
		base = m.filterByTag(base)
	}
	if m.filterText == "" {
		m.filtered = base
		m.restoreSelection()
		m.clampCursor()
		return
	}
	matches := fuzzy.FindFrom(m.filterText, sessionSource(base))
	m.filtered = make([]Session, 0, len(matches))
	for _, match := range matches {
		s := base[match.Index]
		m.filtered = append(m.filtered, s)
		m.matchInfo[s.Key()] = matchInfo{indexes: match.MatchedIndexes}
	}
	m.restoreSelection()
	m.clampCursor()
}

// filterByTag returns sessions that contain the active tag filter.
func (m Model) filterByTag(sessions []Session) []Session {
	var result []Session
	for _, s := range sessions {
		if slices.Contains(s.Tags, m.tagFilter) {
			result = append(result, s)
		}
	}
	return result
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
