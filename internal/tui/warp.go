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

// WarpFrames generates the 4 animation frames for the warp sequence.
// Each frame shows a styled label followed by a gradient block bar.
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

// PlayWarpAnimationTo plays the warp animation to a writer (for testing).
func PlayWarpAnimationTo(w io.Writer, hostShort, sessionName string, termWidth int) {
	frames := WarpFrames(hostShort, sessionName, termWidth)
	for i, frame := range frames {
		if i < len(frames)-1 {
			fmt.Fprintf(w, "\r%s", frame)
		} else {
			fmt.Fprintf(w, "\r%s\n", frame)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
