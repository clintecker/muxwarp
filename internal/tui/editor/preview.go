// Package editor implements the config editor sub-model for muxwarp.
package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/clintecker/muxwarp/internal/config"
)

// previewBorderStyle is the style for the preview panel border.
var previewBorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorGreen).
	Padding(0, 1)

// RenderPreview renders a syntax-highlighted YAML representation of a config.HostEntry.
// The output is wrapped in a bordered box and padded/truncated to the given height.
func RenderPreview(entry config.HostEntry, height int) string {
	var lines []string

	// Render the target line.
	lines = append(lines, renderTargetLine(entry.Target))

	// Render sessions if present.
	if len(entry.Sessions) > 0 {
		lines = append(lines, renderSessionsKeyword())
		for _, s := range entry.Sessions {
			lines = append(lines, renderSessionLines(s)...)
		}
	}

	// Pad to fill height (accounting for border).
	// Border adds 2 lines (top + bottom).
	contentHeight := height - 2
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	// Truncate if too tall.
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	content := strings.Join(lines, "\n")
	return previewBorderStyle.Render(content)
}

// renderTargetLine renders the "- target: <value>" line.
func renderTargetLine(target string) string {
	dash := lipgloss.NewStyle().Foreground(colorSlate).Render("- ")
	key := lipgloss.NewStyle().Foreground(colorCyan).Render("target")
	colon := lipgloss.NewStyle().Foreground(colorSlate).Render(": ")
	value := lipgloss.NewStyle().Foreground(colorText).Render(target)
	return dash + key + colon + value
}

// renderSessionsKeyword renders the "  sessions:" line.
func renderSessionsKeyword() string {
	indent := "  "
	key := lipgloss.NewStyle().Foreground(colorLavender).Render("sessions")
	colon := lipgloss.NewStyle().Foreground(colorSlate).Render(":")
	return indent + key + colon
}

// renderSessionLines renders the lines for a single DesiredSession.
func renderSessionLines(s config.DesiredSession) []string {
	lines := []string{renderSessionField("    - ", "name", s.Name)}
	if s.Dir != "" {
		lines = append(lines, renderSessionField("      ", "dir", s.Dir))
	}
	if s.Repo != "" {
		lines = append(lines, renderSessionField("      ", "repo", s.Repo))
	}
	if s.Cmd != "" {
		lines = append(lines, renderSessionField("      ", "cmd", s.Cmd))
	}
	return lines
}

// renderSessionField renders a single field line with the given indent, key, and value.
// For the first field (name), indent includes "- ". For subsequent fields, it's just spaces.
func renderSessionField(indent, key, value string) string {
	// Split indent into plain spaces and optional dash.
	var styledIndent string
	if strings.Contains(indent, "-") {
		// "    - " -> "    " (plain) + "- " (styled)
		plainSpaces := strings.Repeat(" ", 4)
		dash := lipgloss.NewStyle().Foreground(colorSlate).Render("- ")
		styledIndent = plainSpaces + dash
	} else {
		// Pure indent (no dash).
		styledIndent = indent
	}

	styledKey := lipgloss.NewStyle().Foreground(colorCyan).Render(key)
	colon := lipgloss.NewStyle().Foreground(colorSlate).Render(": ")
	styledValue := lipgloss.NewStyle().Foreground(colorText).Render(value)

	return styledIndent + styledKey + colon + styledValue
}
