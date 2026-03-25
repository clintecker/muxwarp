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

	sel := renderSelector(selected)
	host := m.renderHostColumn(s, m.width)
	name := m.renderSessionName(s)
	badge := renderBadge(s, m.width)
	wins := renderWindows(s, m.width)

	row := composeRow(sel, host, name, badge, wins)
	return applyRowSelection(row, selected, m.width)
}

// renderSelector returns the cursor indicator for a row.
func renderSelector(selected bool) string {
	if selected {
		return selectorStyle.Render("> ")
	}
	return "  "
}

// renderBadge returns the status badge adapted to terminal width.
func renderBadge(s Session, termWidth int) string {
	if termWidth >= 80 {
		return renderFullBadge(s)
	}
	return renderCompactBadge(s)
}

// renderFullBadge returns a badge with dot and text label.
func renderFullBadge(s Session) string {
	if s.Attached == 0 {
		return freeBadgeStyle.Render("○ FREE")
	}
	return dockedBadgeStyle.Render("● DOCKED")
}

// renderCompactBadge returns a dot-only badge.
func renderCompactBadge(s Session) string {
	if s.Attached == 0 {
		return freeBadgeStyle.Render("○")
	}
	return dockedBadgeStyle.Render("●")
}

// renderWindows returns the window count string, or empty below width 60.
func renderWindows(s Session, termWidth int) string {
	if termWidth >= 60 {
		return windowStyle.Render(fmt.Sprintf("w%d", s.Windows))
	}
	return ""
}

// composeRow joins the columns into a single row string.
func composeRow(sel, host, name, badge, wins string) string {
	if wins != "" {
		return fmt.Sprintf("%s %s %s  %s  %s", sel, host, name, badge, wins)
	}
	return fmt.Sprintf("%s %s %s  %s", sel, host, name, badge)
}

// applyRowSelection pads and highlights the row if it is selected.
func applyRowSelection(row string, selected bool, width int) string {
	if !selected {
		return row
	}
	rowWidth := lipgloss.Width(row)
	if rowWidth < width {
		row += strings.Repeat(" ", width-rowWidth)
	}
	return selectedRowStyle.Render(row)
}

// renderHostColumn renders the host label with adaptive width and fuzzy highlights.
func (m Model) renderHostColumn(s Session, termWidth int) string {
	hostText := s.HostShort
	hostMatchSet := m.hostMatchSet(s)

	if termWidth < 45 {
		return renderNarrowHost(hostText, hostMatchSet)
	}
	return renderBracketedHost(hostText, hostMatchSet, m.maxHostWidth())
}

// hostMatchSet returns the set of character positions in the host that
// matched the current filter.
func (m Model) hostMatchSet(s Session) map[int]bool {
	mi := m.matchInfo[s.Key()]
	set := make(map[int]bool)
	for _, idx := range mi.indexes {
		if idx >= 0 && idx < len(s.HostShort) {
			set[idx] = true
		}
	}
	return set
}

// renderNarrowHost renders a 3-char prefix host without brackets.
func renderNarrowHost(hostText string, matchSet map[int]bool) string {
	if len(hostText) > 3 {
		hostText = hostText[:3]
	}
	return renderHighlightedString(hostText, matchSet, hostStyle, matchHighlightStyle)
}

// renderBracketedHost renders "[host]" padded to hostW width.
func renderBracketedHost(hostText string, matchSet map[int]bool, hostW int) string {
	var b strings.Builder
	b.WriteString(hostStyle.Render("["))
	b.WriteString(renderHighlightedString(hostText, matchSet, hostStyle, matchHighlightStyle))
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
	nameMatchSet := m.nameMatchSet(s, mi)
	return renderHighlightedString(s.Name, nameMatchSet, sessionNameStyle, matchHighlightStyle)
}

// nameMatchSet builds a set of character positions within the session name
// that matched the filter. The matchInfo indexes are relative to "hostShort/name",
// so they are offset by len(hostShort)+1.
func (m Model) nameMatchSet(s Session, mi matchInfo) map[int]bool {
	prefix := len(s.HostShort) + 1
	set := make(map[int]bool)
	for _, idx := range mi.indexes {
		nameIdx := idx - prefix
		if nameIdx >= 0 && nameIdx < len(s.Name) {
			set[nameIdx] = true
		}
	}
	return set
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
