package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	"charm.land/lipgloss/v2"
)

// Warp label styles.
var (
	warpPrefixStyle = lipgloss.NewStyle().Foreground(colorSlate)
	warpTargetStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	returnStyle     = lipgloss.NewStyle().Foreground(colorLavender)
)

// WarpFrames generates 4 single-line animation frames: a growing gradient
// block bar trailing the label.
func WarpFrames(hostShort, sessionName string, termWidth int) []string {
	label := warpPrefixStyle.Render("engaging jumpgate: ") +
		warpTargetStyle.Render(hostShort+"/"+sessionName) + " "
	labelWidth := lipgloss.Width(label)
	maxBar := max(termWidth-labelWidth-1, 4)

	frames := make([]string, 4)
	for i := range 4 {
		pct := float64(i+1) / 4.0
		barLen := max(int(pct*float64(maxBar)), 1)
		bar := renderGradient(barLen, "█")
		frames[i] = label + bar
	}
	return frames
}

// ReturnMessage returns a styled "gate closed" message shown after ssh exits.
func ReturnMessage() string {
	return returnStyle.Render("⟪ gate closed ⟫")
}

// PlayWarpAnimation plays the warp animation to stdout.
func PlayWarpAnimation(hostShort, sessionName string, termWidth int) {
	PlayWarpAnimationTo(os.Stdout, hostShort, sessionName, termWidth)
}

// PlayWarpAnimationTo plays the warp animation to a writer.
func PlayWarpAnimationTo(w io.Writer, hostShort, sessionName string, termWidth int) {
	playFrames(w, WarpFrames(hostShort, sessionName, termWidth))
}

// CreationFrames generates 4 animation frames for session creation.
// Uses ░ (light block) and "materializing lane" prefix.
func CreationFrames(hostShort, sessionName string, termWidth int) []string {
	label := warpPrefixStyle.Render("materializing lane: ") +
		warpTargetStyle.Render(hostShort+"/"+sessionName) + " "
	labelWidth := lipgloss.Width(label)
	maxBar := max(termWidth-labelWidth-1, 4)

	frames := make([]string, 4)
	for i := range 4 {
		pct := float64(i+1) / 4.0
		barLen := max(int(pct*float64(maxBar)), 1)
		bar := renderGradient(barLen, "░")
		frames[i] = label + bar
	}
	return frames
}

// PlayCreationAnimation plays the creation animation to stdout.
func PlayCreationAnimation(hostShort, sessionName string, termWidth int) {
	PlayCreationAnimationTo(os.Stdout, hostShort, sessionName, termWidth)
}

// PlayCreationAnimationTo plays the creation animation to a writer (for testing).
func PlayCreationAnimationTo(w io.Writer, hostShort, sessionName string, termWidth int) {
	playFrames(w, CreationFrames(hostShort, sessionName, termWidth))
}

// playFrames renders single-line animation frames with carriage-return overwriting.
func playFrames(w io.Writer, frames []string) {
	for i, frame := range frames {
		if i < len(frames)-1 {
			fmt.Fprintf(w, "\r%s", frame)
		} else {
			fmt.Fprintf(w, "\r%s\n", frame)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
