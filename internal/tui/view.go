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
	box := headerBoxStyle.Render(
		"╭──── muxwarp ────╮\n" +
			"│  warp to tmux   │\n" +
			"╰─────────────────╯",
	)

	// We render a simpler box using the lipgloss border style.
	box = headerBoxStyle.Render("muxwarp — warp to tmux")

	var status string
	if m.scanning {
		status = scanActiveStyle.Render(
			fmt.Sprintf("Spooling drives… %d/%d", m.scanDone, m.scanTotal),
		)
	} else {
		count := len(m.sessions)
		label := "sessions"
		if count == 1 {
			label = "session"
		}
		status = statusStyle.Render(fmt.Sprintf("%d %s across %d hosts",
			count, label, m.countHosts()))
	}

	// Place box on left, status on right.
	boxWidth := lipgloss.Width(box)
	statusWidth := lipgloss.Width(status)
	gap := m.width - boxWidth - statusWidth
	if gap < 2 {
		gap = 2
	}

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

	end := m.viewOffset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

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

// renderRow renders a single session row.
func (m Model) renderRow(idx int) string {
	s := m.filtered[idx]
	selected := idx == m.cursor

	// Selector.
	sel := "  "
	if selected {
		sel = selectorStyle.Render("> ")
	}

	// Host label.
	host := hostStyle.Render(fmt.Sprintf("[%s]", s.HostShort))

	// Session name — apply fuzzy highlights if available.
	name := m.renderSessionName(s)

	// Badge.
	var badge string
	if s.Attached == 0 {
		badge = freeBadgeStyle.Render("FREE")
	} else {
		badge = dockedBadgeStyle.Render(fmt.Sprintf("DOCKED(%d)", s.Attached))
	}

	// Window count.
	wins := windowStyle.Render(fmt.Sprintf("w%d", s.Windows))

	// Compose the row.
	row := fmt.Sprintf("%s %s %s  %s  %s", sel, host, name, badge, wins)

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

	var b strings.Builder
	for i, ch := range s.Name {
		if nameMatchSet[i] {
			b.WriteString(matchHighlightStyle.Render(string(ch)))
		} else {
			b.WriteString(sessionNameStyle.Render(string(ch)))
		}
	}
	return b.String()
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
		gap := m.width - filterWidth - matchWidth
		if gap < 2 {
			gap = 2
		}

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
