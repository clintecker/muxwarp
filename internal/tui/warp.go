package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// WarpFrames generates the 4 animation frames for the warp sequence.
// Each frame shows a growing block bar that fills available terminal width.
func WarpFrames(hostShort, sessionName string, termWidth int) []string {
	label := fmt.Sprintf("engaging jumpgate: %s/%s ", hostShort, sessionName)
	maxBar := max(termWidth-len(label)-1, 4)

	barStyle := lipgloss.NewStyle().Foreground(colorCyan)

	frames := make([]string, 4)
	for i := range 4 {
		pct := float64(i+1) / 4.0
		barLen := max(int(pct*float64(maxBar)), 1)
		bar := barStyle.Render(strings.Repeat("█", barLen))
		frames[i] = label + bar
	}
	return frames
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
