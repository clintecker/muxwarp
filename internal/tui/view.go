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

// renderHeader builds the header: ▲ muxwarp ──gradient── status
func (m Model) renderHeader() string {
	triangle := headerTriangleStyle.Render("▲")
	title := headerTitleStyle.Render(" muxwarp ")

	var status string
	if m.scanning {
		status = scanActiveStyle.Render(
			fmt.Sprintf("Spooling drives… %d/%d", m.scanDone, m.scanTotal),
		)
	} else {
		hosts := m.countHosts()
		sessions := len(m.sessions)
		status = statusStyle.Render(fmt.Sprintf("%s · %s",
			pluralize(hosts, "host"), pluralize(sessions, "session")))
	}

	// Build gradient rule to fill between title and status.
	leftWidth := lipgloss.Width(triangle) + lipgloss.Width(title)
	statusWidth := lipgloss.Width(status)
	ruleWidth := m.width - leftWidth - statusWidth - 2 // 2 for spacing
	if ruleWidth < 3 {
		ruleWidth = 3
	}

	rule := renderGradientRule(ruleWidth)

	return triangle + title + rule + " " + status
}

// pluralize returns "1 host" or "N hosts".
func pluralize(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// renderGradient generates a string of repeated char with a neon-blue→electric-purple gradient.
func renderGradient(width int, char string) string {
	if width <= 0 {
		return ""
	}
	colors := lipgloss.Blend1D(width, colorNeonBlue, colorElectricPurple)
	var b strings.Builder
	for _, c := range colors {
		style := lipgloss.NewStyle().Foreground(c)
		b.WriteString(style.Render(char))
	}
	return b.String()
}

// renderGradientRule generates a string of ─ characters with a cyan→purple gradient.
func renderGradientRule(width int) string {
	return renderGradient(width, "─")
}

// countHosts returns the number of unique hosts in the session list.
func (m Model) countHosts() int {
	seen := make(map[string]bool)
	for _, s := range m.sessions {
		seen[s.Host] = true
	}
	return len(seen)
}

// columnWidths holds computed column widths for aligned row rendering.
type columnWidths struct {
	maxName int // max session name length across visible sessions
	maxDots int // max window count across visible sessions (0 if width < 60)
}

// computeColumnWidths computes the max name length and max dot count
// across all filtered sessions for column alignment.
func (m Model) computeColumnWidths() columnWidths {
	var cols columnWidths
	showDots := m.width >= 60
	for _, s := range m.filtered {
		cols.maxName = max(cols.maxName, len(s.Name))
		if showDots {
			cols.maxDots = max(cols.maxDots, s.Windows)
		}
	}
	return cols
}

// renderList builds the scrolling session list.
func (m Model) renderList() string {
	visible := m.visibleRows()
	cols := m.computeColumnWidths()
	var b strings.Builder

	end := min(m.viewOffset+visible, len(m.filtered))

	for i := m.viewOffset; i < end; i++ {
		if i > m.viewOffset {
			b.WriteRune('\n')
		}
		b.WriteString(m.renderRow(i, cols))
	}

	// Pad remaining rows if the list is shorter than the viewport.
	rendered := end - m.viewOffset
	for rendered < visible {
		b.WriteRune('\n')
		rendered++
	}

	return b.String()
}

// renderRow renders a single session row with column-aligned layout.
//
// Columns are padded so names, badges, dots, and hosts align vertically:
//   ▸  name(padded)  ◇ IDLE  ▪▪(padded)   host
func (m Model) renderRow(idx int, cols columnWidths) string {
	s := m.filtered[idx]
	selected := idx == m.cursor

	sel := renderSelector(selected)
	name := m.renderSessionName(s)
	badge := renderBadge(s, m.width)
	dots := renderWindows(s, m.width)
	host := m.renderHostTag(s, m.width)

	// Pad session name to align badge column.
	namePad := cols.maxName - len(s.Name)
	if namePad > 0 {
		name += strings.Repeat(" ", namePad)
	}

	// Pad dots to align host column.
	dotCount := 0
	if m.width >= 60 {
		dotCount = s.Windows
	}
	dotPad := cols.maxDots - dotCount
	if dotPad > 0 {
		dots += strings.Repeat(" ", dotPad)
	}

	// Build left content: selector + name + badge + dots
	left := composLeftContent(sel, name, badge, dots)

	return applyRowSelection(left, host, selected, m.width)
}

// renderSelector returns the cursor indicator for a row.
func renderSelector(selected bool) string {
	if selected {
		return selectorStyle.Render("▸ ")
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

// renderFullBadge returns a badge with diamond symbol and text label.
func renderFullBadge(s Session) string {
	if s.Attached == 0 {
		return idleBadgeStyle.Render("◇ IDLE")
	}
	return liveBadgeStyle.Render("◆ LIVE")
}

// renderCompactBadge returns a diamond-only badge.
func renderCompactBadge(s Session) string {
	if s.Attached == 0 {
		return idleBadgeStyle.Render("◇")
	}
	return liveBadgeStyle.Render("◆")
}

// renderWindows returns window dots (▪), or empty below width 60.
func renderWindows(s Session, termWidth int) string {
	if termWidth < 60 || s.Windows == 0 {
		return ""
	}
	return windowDotStyle.Render(strings.Repeat("▪", s.Windows))
}

// composLeftContent joins selector, name, badge, and dots.
func composLeftContent(sel, name, badge, dots string) string {
	if dots != "" {
		return fmt.Sprintf("%s %s  %s  %s", sel, name, badge, dots)
	}
	return fmt.Sprintf("%s %s  %s", sel, name, badge)
}

// renderHostTag renders the host as a dim right-aligned tag.
func (m Model) renderHostTag(s Session, termWidth int) string {
	hostText := s.HostShort
	hostMatchSet := m.hostMatchSet(s)

	if termWidth < 45 && len(hostText) > 3 {
		hostText = hostText[:3]
	}

	return renderHighlightedString(hostText, hostMatchSet, hostStyle, matchHighlightStyle)
}

// applyRowSelection joins left content and host with a fixed 3-space gap,
// and highlights the full row if selected.
func applyRowSelection(left, host string, selected bool, width int) string {
	row := left + "   " + host

	if !selected {
		return row
	}
	rowWidth := lipgloss.Width(row)
	if rowWidth < width {
		row += strings.Repeat(" ", width-rowWidth)
	}
	return selectedRowStyle.Render(row)
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
		help := footerStyle.Render("type to filter │ enter warp │ esc clear")

		// Right-align the match count.
		filterWidth := lipgloss.Width(filterLine)
		matchWidth := lipgloss.Width(matchCount)
		gap := max(m.width-filterWidth-matchWidth, 2)

		return filterLine + strings.Repeat(" ", gap) + matchCount + "\n" + help
	}

	if len(m.sessions) == 0 {
		return footerStyle.Render("r rescan │ q quit")
	}

	return footerStyle.Render("↑/↓ navigate │ enter warp │ / filter │ r rescan │ q quit")
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
