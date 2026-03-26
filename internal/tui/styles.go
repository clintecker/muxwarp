// Package tui implements the Bubble Tea v2 terminal user interface.
package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette.
var (
	colorCyan           = lipgloss.Color("#8BE9FD")
	colorLavender       = lipgloss.Color("#BD93F9")
	colorGreen          = lipgloss.Color("#2EE6A6")
	colorRed            = lipgloss.Color("#FF5555")
	colorSlate          = lipgloss.Color("#6B7280")
	colorText           = lipgloss.Color("#E6E6E6")
	colorDimBg          = lipgloss.Color("#1E1E2E")
	colorNeonBlue       = lipgloss.Color("#00D9FF")
	colorElectricPurple = lipgloss.Color("#C891FF")
	colorDimHost        = lipgloss.Color("#4A4A5E")
)

// Re-usable styles.
var (
	// Header triangle (▲) style.
	headerTriangleStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Bold(true)

	// Header title ("muxwarp") style.
	headerTitleStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	// Status text (right side of header).
	statusStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Scan-in-progress status.
	scanActiveStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	// Selected row gets a background tint.
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorDimBg).
				Bold(true)

	// Selector character for the active row.
	selectorStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	// Host tag (column-aligned, lavender).
	hostStyle = lipgloss.NewStyle().
			Foreground(colorLavender)

	// Session name.
	sessionNameStyle = lipgloss.NewStyle().
				Foreground(colorText)

	// IDLE badge (detached session).
	idleBadgeStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	// LIVE badge (attached session).
	liveBadgeStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	// Window dot (▪) style.
	windowDotStyle = lipgloss.NewStyle().
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

	// NEW badge (ghost/desired session).
	newBadgeStyle = lipgloss.NewStyle().
			Foreground(colorLavender).
			Bold(true)

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
	_ color.Color = colorNeonBlue
	_ color.Color = colorElectricPurple
	_ color.Color = colorDimHost
)
