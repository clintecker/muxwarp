// Package editor implements the config editor sub-model for muxwarp.
package editor

import (
	"charm.land/lipgloss/v2"
)

// Color palette — duplicated from tui/styles.go to avoid circular import.
var (
	colorCyan     = lipgloss.Color("#8BE9FD")
	colorLavender = lipgloss.Color("#BD93F9")
	colorGreen    = lipgloss.Color("#2EE6A6")
	colorRed      = lipgloss.Color("#FF5555")
	colorSlate    = lipgloss.Color("#6B7280")
	colorText     = lipgloss.Color("#E6E6E6")
	colorDimBg    = lipgloss.Color("#1E1E2E")
)

// Editor form styles.
var (
	// Focused field border.
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorCyan)

	// Blurred field border.
	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSlate)

	// Field label.
	labelStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	// Helper text below fields.
	helperStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Error text.
	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	// Selected session in mini-list.
	sessionSelectedStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorDimBg).
				Bold(true)

	// Normal session in mini-list.
	sessionNormalStyle = lipgloss.NewStyle().
				Foreground(colorSlate)

	// Section title style.
	sectionStyle = lipgloss.NewStyle().
			Foreground(colorLavender).
			Bold(true)

	// Footer keybinding hints.
	footerHintStyle = lipgloss.NewStyle().
			Foreground(colorSlate)

	// Delete confirmation prompt.
	deletePromptStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)
)
