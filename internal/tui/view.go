package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model. It renders the full TUI screen.
func (m Model) View() tea.View {
	var content string

	switch m.mode {
	case ModeEdit:
		content = m.renderEditorScreen()
	case ModeWizard:
		content = m.renderWizardScreen()
	default:
		content = m.renderListScreen()
	}

	// Pad to full height so the layout doesn't jump.
	contentHeight := lipgloss.Height(content)
	if contentHeight < m.height {
		content += strings.Repeat("\n", m.height-contentHeight)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderListScreen renders the session list screen (ModeList and ModeFilter).
func (m Model) renderListScreen() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteRune('\n')

	if len(m.filtered) == 0 && !m.scanning {
		b.WriteString(m.renderEmpty())
	} else {
		b.WriteString(m.renderList())
	}

	b.WriteRune('\n')
	if m.mode == ModeTagPicker {
		b.WriteString(m.renderTagPicker())
	} else {
		b.WriteString(m.renderFooter())
	}

	return b.String()
}

// renderTagPicker renders an inline tag selection picker in the footer area.
func (m Model) renderTagPicker() string {
	tags := m.allTags()
	var b strings.Builder
	b.WriteString(filterPromptStyle.Render("Select tag:"))
	b.WriteRune('\n')
	b.WriteString(renderTagList(tags, m.tagCursor))
	b.WriteRune('\n')
	b.WriteString(footerStyle.Render("h/l navigate │ enter select │ esc cancel"))
	return b.String()
}

// renderTagList renders the horizontal list of tags with cursor highlighting.
func renderTagList(tags []string, cursor int) string {
	var b strings.Builder
	for i, tag := range tags {
		b.WriteString(renderTagItem(tag, i == cursor))
		if i < len(tags)-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}

// renderTagItem renders a single tag as either selected or unselected.
func renderTagItem(tag string, selected bool) string {
	if selected {
		return selectorStyle.Render("▸ ") + sessionNameStyle.Render(tag)
	}
	return "  " + metadataStyle.Render(tag)
}

// renderEditorScreen renders the config editor screen.
func (m Model) renderEditorScreen() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteRune('\n')
	b.WriteRune('\n')
	b.WriteString(m.editor.View())
	return b.String()
}

// renderWizardScreen renders the first-run wizard screen.
func (m Model) renderWizardScreen() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteRune('\n')
	b.WriteRune('\n')
	b.WriteString(m.wizard.View())
	return b.String()
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
	maxName     int
	maxDots     int
	maxAttached int // display width of widest attached indicator (0 if none)
	maxAge      int
	maxActivity int
}

// computeColumnWidths computes the max name length and max dot count
// across all filtered sessions for column alignment.
func (m Model) computeColumnWidths(now time.Time) columnWidths {
	var cols columnWidths
	for _, s := range m.filtered {
		cols.maxName = max(cols.maxName, len(s.Name))
		updateAttachedWidth(&cols, s, m.width)
		updateDotWidth(&cols, s, m.width)
		updateAgeWidths(&cols, s, m.width, now)
	}
	return cols
}

// updateAttachedWidth updates the maxAttached column width for a session.
func updateAttachedWidth(cols *columnWidths, s Session, width int) {
	if width >= 65 && s.Attached > 1 && !s.IsGhost() {
		w := lipgloss.Width(fmt.Sprintf("%d↗", s.Attached))
		cols.maxAttached = max(cols.maxAttached, w)
	}
}

// updateDotWidth updates the maxDots column width for a session.
func updateDotWidth(cols *columnWidths, s Session, width int) {
	if width >= 60 {
		cols.maxDots = max(cols.maxDots, s.Windows)
	}
}

// updateAgeWidths updates the maxAge and maxActivity column widths for a session.
func updateAgeWidths(cols *columnWidths, s Session, width int, now time.Time) {
	if s.IsGhost() {
		return
	}
	if width >= 70 {
		cols.maxAge = max(cols.maxAge, len(formatAgeSince(s.Created, now)))
	}
	if width >= 80 {
		cols.maxActivity = max(cols.maxActivity, activityWidth(s, now))
	}
}

// activityWidth returns the display width of the last-activity field for a session.
func activityWidth(s Session, now time.Time) int {
	a := formatAgeSince(s.LastActivity, now)
	switch a {
	case "":
		return 0
	case "now":
		return 3
	default:
		return len(a + " ago")
	}
}

// renderList builds the scrolling session list.
func (m Model) renderList() string {
	visible := m.visibleRows()
	now := time.Now()
	cols := m.computeColumnWidths(now)
	var b strings.Builder

	end := min(m.viewOffset+visible, len(m.filtered))

	for i := m.viewOffset; i < end; i++ {
		if i > m.viewOffset {
			b.WriteRune('\n')
		}
		b.WriteString(m.renderRow(i, cols, now))
	}

	// Pad remaining rows if the list is shorter than the viewport.
	rendered := end - m.viewOffset
	for rendered < visible {
		b.WriteRune('\n')
		rendered++
	}

	return b.String()
}

// renderAttached returns the attached-count indicator (e.g. "2↗") when multiple
// clients are attached and the terminal is wide enough.
func renderAttached(s Session, termWidth int) string {
	if termWidth < 65 || s.Attached <= 1 || s.IsGhost() {
		return ""
	}
	return attachedStyle.Render(fmt.Sprintf("%d↗", s.Attached))
}

// renderAge returns the session creation age string when the terminal is wide enough.
func renderAge(s Session, termWidth int, now time.Time) string {
	if termWidth < 70 || s.IsGhost() {
		return ""
	}
	return metadataStyle.Render(formatAgeSince(s.Created, now))
}

// renderLastActive returns the last-activity age string when the terminal is wide enough.
func renderLastActive(s Session, termWidth int, now time.Time) string {
	if termWidth < 80 || s.IsGhost() {
		return ""
	}
	age := formatAgeSince(s.LastActivity, now)
	if age == "" {
		return ""
	}
	if age == "now" {
		return metadataStyle.Render("now")
	}
	return metadataStyle.Render(age + " ago")
}

// renderRow renders a single session row with column-aligned layout.
//
// Columns are padded so names, badges, dots, and hosts align vertically:
//
//	▸  name(padded)  ◇ IDLE  ▪▪(padded)   host
func (m Model) renderRow(idx int, cols columnWidths, now time.Time) string {
	s := m.filtered[idx]
	selected := idx == m.cursor
	name, dots := m.paddedNameAndDots(s, cols)
	left := buildRowLeft(s, name, dots, cols, m.width, now, renderSelector(selected))
	host := m.renderHostTag(s, m.width) + m.renderLatencyTag(s)
	return applyRowSelection(left, host, selected, m.width)
}

// paddedNameAndDots returns the name and dots strings with column padding applied.
func (m Model) paddedNameAndDots(s Session, cols columnWidths) (name, dots string) {
	name = m.renderSessionName(s)
	if pad := cols.maxName - len(s.Name); pad > 0 {
		name += strings.Repeat(" ", pad)
	}
	dots = renderWindows(s, m.width)
	dotCount := 0
	if m.width >= 60 {
		dotCount = s.Windows
	}
	if pad := cols.maxDots - dotCount; pad > 0 {
		dots += strings.Repeat(" ", pad)
	}
	return name, dots
}

// buildRowLeft assembles the left portion of a row: selector + name + badge + optional fields.
func buildRowLeft(s Session, name, dots string, cols columnWidths, width int, now time.Time, sel string) string {
	badge := renderBadge(s, width)
	left := sel + " " + name + "  " + badge
	left = appendPadded(left, " ", renderAttached(s, width), cols.maxAttached)
	left = appendIfNonEmpty(left, "  ", dots)
	left = appendIfNonEmpty(left, "  ", renderAge(s, width, now))
	left = appendIfNonEmpty(left, "  ", renderLastActive(s, width, now))
	return left
}

// appendPadded appends sep+val to base, padding val to targetWidth.
// If targetWidth is 0, nothing is appended. If val is empty, padding is still applied.
func appendPadded(base, sep, val string, targetWidth int) string {
	if targetWidth == 0 {
		return base
	}
	valWidth := lipgloss.Width(val)
	padded := val + strings.Repeat(" ", max(targetWidth-valWidth, 0))
	return base + sep + padded
}

// appendIfNonEmpty appends sep+val to base only when val is non-empty.
func appendIfNonEmpty(base, sep, val string) string {
	if val == "" {
		return base
	}
	return base + sep + val
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
	if s.IsGhost() {
		return renderNewBadge(termWidth)
	}
	if termWidth >= 80 {
		return renderFullBadge(s)
	}
	return renderCompactBadge(s)
}

// renderNewBadge returns the NEW badge for ghost sessions.
func renderNewBadge(termWidth int) string {
	if termWidth >= 80 {
		return newBadgeStyle.Render("◌ NEW")
	}
	return newBadgeStyle.Render("◌")
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

// renderWindows returns window dots (▪), or empty below width 60 or for ghosts.
func renderWindows(s Session, termWidth int) string {
	if termWidth < 60 || s.Windows == 0 || s.IsGhost() {
		return ""
	}
	return windowDotStyle.Render(strings.Repeat("▪", s.Windows))
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
	if m.mode == ModeFilter {
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
		return footerStyle.Render("a add host │ r rescan │ q quit")
	}

	if m.confirmDeleteTarget != "" {
		prompt := lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(
			fmt.Sprintf("Delete host %q? ", m.confirmDeleteTarget))
		hint := footerStyle.Render("y confirm │ any key cancel")
		return prompt + hint
	}

	if m.tagFilter != "" {
		tagLabel := filterPromptStyle.Render("tag: " + m.tagFilter)
		countLabel := statusStyle.Render(fmt.Sprintf("%d sessions", len(m.filtered)))
		return tagLabel + "  " + countLabel + "\n" +
			footerStyle.Render("t clear │ / filter │ enter warp │ q quit")
	}
	return footerStyle.Render("enter warp │ / filter │ t tags │ a add │ e edit │ d delete │ q quit")
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
