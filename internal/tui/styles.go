// Package tui implements the Bubble Tea v2 terminal user interface.
package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette.
var (
	colorCyan     = lipgloss.Color("#8BE9FD")
	colorLavender = lipgloss.Color("#BD93F9")
	colorGreen    = lipgloss.Color("#2EE6A6")
	colorRed      = lipgloss.Color("#FF5555")
	colorSlate    = lipgloss.Color("#6B7280")
	colorText     = lipgloss.Color("#E6E6E6")
	colorDimBg    = lipgloss.Color("#1E1E2E")
)

// Re-usable styles.
var (
	// Header box style.
	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorLavender).
			Foreground(colorLavender).
			Padding(0, 1).
			Bold(true)

	// Status text (right side of header).
	statusStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Scan-in-progress status.
	scanActiveStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	// Normal row in the session list.
	rowStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// Selected row gets a background tint.
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorDimBg).
				Bold(true)

	// Selector character for the active row.
	selectorStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	// Host label in brackets.
	hostStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Session name.
	sessionNameStyle = lipgloss.NewStyle().
				Foreground(colorText)

	// FREE badge.
	freeBadgeStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	// DOCKED badge.
	dockedBadgeStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)

	// Window count.
	windowStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Footer help text.
	footerStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Filter prompt text.
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	// Filter input text.
	filterInputStyle = lipgloss.NewStyle().
				Foreground(colorText)

	// Matched characters in fuzzy filter results.
	matchHighlightStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	// Empty state message.
	emptyStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Empty state command hint.
	emptyHintStyle = lipgloss.NewStyle().
			Foreground(colorLavender)
)

// ensure color vars satisfy the color.Color interface at compile time.
var (
	_ color.Color = colorCyan
	_ color.Color = colorLavender
	_ color.Color = colorGreen
	_ color.Color = colorRed
	_ color.Color = colorSlate
	_ color.Color = colorText
	_ color.Color = colorDimBg
)
