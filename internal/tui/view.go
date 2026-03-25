package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model. It renders the full TUI screen.
func (m Model) View() tea.View {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteRune('\n')

	if len(m.filtered) == 0 && !m.scanning {
		b.WriteString(m.renderEmpty())
	} else {
		b.WriteString(m.renderList())
	}

	b.WriteRune('\n')
	b.WriteString(m.renderFooter())

	// Pad to full height so the layout doesn't jump.
	content := b.String()
	contentHeight := lipgloss.Height(content)
	if contentHeight < m.height {
		content += strings.Repeat("\n", m.height-contentHeight)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderHeader builds the header with the muxwarp box and status.
func (m Model) renderHeader() string {
	box := headerBoxStyle.Render("muxwarp — warp to tmux")

	var status string
	if m.scanning {
		status = scanActiveStyle.Render(
			fmt.Sprintf("Spooling drives… %d/%d", m.scanDone, m.scanTotal),
		)
	} else {
		hosts := m.countHosts()
		sessions := len(m.sessions)
		status = statusStyle.Render(fmt.Sprintf("%d hosts · %d sessions",
			hosts, sessions))
	}

	// Place box on left, status on right.
	boxWidth := lipgloss.Width(box)
	statusWidth := lipgloss.Width(status)
	gap := max(m.width-boxWidth-statusWidth, 2)

	return box + strings.Repeat(" ", gap) + status
}

// countHosts returns the number of unique hosts in the session list.
func (m Model) countHosts() int {
	seen := make(map[string]bool)
	for _, s := range m.sessions {
		seen[s.Host] = true
	}
	return len(seen)
}

// renderList builds the scrolling session list.
func (m Model) renderList() string {
	visible := m.visibleRows()
	var b strings.Builder

	end := min(m.viewOffset+visible, len(m.filtered))

	for i := m.viewOffset; i < end; i++ {
		if i > m.viewOffset {
			b.WriteRune('\n')
		}
		b.WriteString(m.renderRow(i))
	}

	// Pad remaining rows if the list is shorter than the viewport.
	rendered := end - m.viewOffset
	for rendered < visible {
		b.WriteRune('\n')
		rendered++
	}

	return b.String()
}

// maxHostWidth returns the width of the longest HostShort in the filtered list.
func (m Model) maxHostWidth() int {
	w := 0
	for _, s := range m.filtered {
		w = max(w, len(s.HostShort))
	}
	return w
}

// renderRow renders a single session row with adaptive column layout.
//
// Width >= 80: selector [host] session  ● DOCKED  wN
// Width >= 60: selector [host] session  ●  wN      (badge text drops)
// Width >= 45: selector [host] session  ●          (window count drops)
// Width <  45: selector hos session  ●              (brackets drop, 3-char host)
func (m Model) renderRow(idx int) string {
	s := m.filtered[idx]
	selected := idx == m.cursor
	w := m.width

	// Selector column.
	sel := "  "
	if selected {
		sel = selectorStyle.Render("> ")
	}

	// Host column — adaptive, with fuzzy highlight support.
	host := m.renderHostColumn(s, w)

	// Session name — apply fuzzy highlights if available.
	name := m.renderSessionName(s)

	// Badge column — adaptive.
	var badge string
	if w >= 80 {
		// Full badge with text.
		if s.Attached == 0 {
			badge = freeBadgeStyle.Render("○ FREE")
		} else {
			badge = dockedBadgeStyle.Render("● DOCKED")
		}
	} else {
		// Compact: dot only.
		if s.Attached == 0 {
			badge = freeBadgeStyle.Render("○")
		} else {
			badge = dockedBadgeStyle.Render("●")
		}
	}

	// Window count — hide below width 60.
	wins := ""
	if w >= 60 {
		wins = windowStyle.Render(fmt.Sprintf("w%d", s.Windows))
	}

	// Compose the row.
	var row string
	if wins != "" {
		row = fmt.Sprintf("%s %s %s  %s  %s", sel, host, name, badge, wins)
	} else {
		row = fmt.Sprintf("%s %s %s  %s", sel, host, name, badge)
	}

	// Apply row background for selected row.
	if selected {
		// Pad to full width so the background extends.
		rowWidth := lipgloss.Width(row)
		if rowWidth < m.width {
			row += strings.Repeat(" ", m.width-rowWidth)
		}
		row = selectedRowStyle.Render(row)
	}

	return row
}

// renderHostColumn renders the host label with adaptive width and fuzzy highlights.
func (m Model) renderHostColumn(s Session, termWidth int) string {
	hostW := m.maxHostWidth()
	hostText := s.HostShort

	// Build a set of matched positions in the host portion.
	mi := m.matchInfo[s.Key()]
	hostMatchSet := make(map[int]bool)
	for _, idx := range mi.indexes {
		if idx >= 0 && idx < len(s.HostShort) {
			hostMatchSet[idx] = true
		}
	}

	if termWidth < 45 {
		// 3-char prefix, no brackets.
		if len(hostText) > 3 {
			hostText = hostText[:3]
		}
		return renderHighlightedString(hostText, hostMatchSet, hostStyle, matchHighlightStyle)
	}

	// Bracketed, padded to widest host.
	// Build: "[" + highlighted host + padding + "]"
	var b strings.Builder
	b.WriteString(hostStyle.Render("["))
	b.WriteString(renderHighlightedString(hostText, hostMatchSet, hostStyle, matchHighlightStyle))
	// Pad to hostW width.
	if pad := hostW - len(hostText); pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	b.WriteString(hostStyle.Render("]"))
	return b.String()
}

// renderHighlightedString renders a string with certain character positions highlighted.
func renderHighlightedString(s string, matchSet map[int]bool, normalStyle, highlightStyle lipgloss.Style) string {
	if len(matchSet) == 0 {
		return normalStyle.Render(s)
	}
	var b strings.Builder
	for i, ch := range s {
		if matchSet[i] {
			b.WriteString(highlightStyle.Render(string(ch)))
		} else {
			b.WriteString(normalStyle.Render(string(ch)))
		}
	}
	return b.String()
}

// renderSessionName renders the session name with fuzzy match highlighting.
func (m Model) renderSessionName(s Session) string {
	mi, ok := m.matchInfo[s.Key()]
	if !ok || len(mi.indexes) == 0 {
		return sessionNameStyle.Render(s.Name)
	}

	// Build a set of matched positions in the session name.
	// The matchInfo indexes are relative to "hostShort/name", so we need
	// to offset them by len(hostShort)+1 to find matches within the name.
	prefix := len(s.HostShort) + 1 // "hostShort/"
	nameMatchSet := make(map[int]bool)
	for _, idx := range mi.indexes {
		nameIdx := idx - prefix
		if nameIdx >= 0 && nameIdx < len(s.Name) {
			nameMatchSet[nameIdx] = true
		}
	}

	return renderHighlightedString(s.Name, nameMatchSet, sessionNameStyle, matchHighlightStyle)
}

// renderFooter builds the context-sensitive footer.
func (m Model) renderFooter() string {
	if m.filtering {
		// Filter input line.
		prompt := filterPromptStyle.Render("/ ")
		input := filterInputStyle.Render(m.filterText)
		cursor := filterPromptStyle.Render("_")

		matchCount := statusStyle.Render(
			fmt.Sprintf("%d matches", len(m.filtered)),
		)

		filterLine := prompt + input + cursor

		// Help line.
		help := footerStyle.Render("type to filter · enter warp · esc clear")

		// Right-align the match count.
		filterWidth := lipgloss.Width(filterLine)
		matchWidth := lipgloss.Width(matchCount)
		gap := max(m.width-filterWidth-matchWidth, 2)

		return filterLine + strings.Repeat(" ", gap) + matchCount + "\n" + help
	}

	if len(m.sessions) == 0 {
		return footerStyle.Render("r rescan · q quit")
	}

	return footerStyle.Render("↑/↓ navigate · enter warp · / filter · r rescan · q quit")
}

// renderEmpty renders the empty state when no sessions are found.
func (m Model) renderEmpty() string {
	var b strings.Builder

	b.WriteRune('\n')
	b.WriteString(emptyStyle.Render("All gates are calm — no active lanes detected."))
	b.WriteRune('\n')
	b.WriteRune('\n')
	b.WriteString(emptyHintStyle.Render("Start a session:  ssh <host> -t tmux new -s <name>"))
	b.WriteRune('\n')

	return b.String()
}
