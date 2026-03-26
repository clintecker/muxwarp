package tui

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Warp label styles.
var (
	warpPrefixStyle = lipgloss.NewStyle().Foreground(colorSlate)
	warpTargetStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	returnStyle     = lipgloss.NewStyle().Foreground(colorLavender)
)

// tunnelHeight is the number of lines in the hyperspace tunnel (must be odd).
const tunnelHeight = 7

// WarpFrames generates multi-line animation frames for the hyperspace tunnel.
// The tunnel is diamond-shaped: chevrons converge inward over several frames,
// then a gradient bar punches through the center, then collapses to a single line.
func WarpFrames(hostShort, sessionName string, termWidth int) []string {
	label := warpPrefixStyle.Render("engaging jumpgate: ") +
		warpTargetStyle.Render(hostShort+"/"+sessionName)
	labelWidth := lipgloss.Width(label)

	// Build 8 frames: 5 convergence + 1 punch-through + 1 exhaust + 1 collapse.
	frames := make([]string, 0, 8)
	frames = append(frames, convergenceFrames(termWidth)...)
	frames = append(frames, punchFrame(label, labelWidth, termWidth))
	frames = append(frames, exhaustFrame(label, labelWidth, termWidth))
	frames = append(frames, collapseFrame(label, labelWidth, termWidth))
	return frames
}

// convergenceFrames generates 5 tunnel frames where chevrons rush inward.
func convergenceFrames(termWidth int) []string {
	frames := make([]string, 5)
	for f := range 5 {
		frames[f] = renderTunnelFrame(f, termWidth)
	}
	return frames
}

// renderTunnelFrame renders one diamond-shaped tunnel frame.
func renderTunnelFrame(frame, termWidth int) string {
	mid := tunnelHeight / 2
	var lines []string
	for row := range tunnelHeight {
		dist := abs(row - mid)
		line := renderTunnelRow(frame, dist, termWidth)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderTunnelRow renders a single row of the diamond tunnel.
// dist=0 is the center (narrowest tunnel), dist increases toward edges.
func renderTunnelRow(frame, dist, termWidth int) string {
	// Chevrons move inward as frame increases. More chevrons at wider rows.
	chevCount := dist + 1
	// Inset from edge increases each frame (convergence).
	inset := (frame * 3) + (dist * 2)
	// Tunnel wall width shrinks toward center and as frames advance.
	wallWidth := max(termWidth-(inset*2)-(chevCount*4)-4, 3)

	leftChev := renderChevrons(chevCount, "▸", frame)
	rightChev := renderChevrons(chevCount, "◂", frame)
	wall := renderGradient(wallWidth, "═")

	pad := strings.Repeat(" ", inset)
	gap := " "
	return pad + leftChev + gap + wall + gap + rightChev
}

// renderChevrons renders n chevron characters with color shifting by frame.
func renderChevrons(n int, char string, frame int) string {
	var b strings.Builder
	for i := range n {
		c := chevronColor(frame, i)
		style := lipgloss.NewStyle().Foreground(c).Bold(true)
		b.WriteString(style.Render(char))
		if i < n-1 {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// chevronColor shifts from cyan to white as the animation progresses.
func chevronColor(frame, idx int) color.Color {
	colors := []color.Color{
		lipgloss.Color("#8BE9FD"), // cyan
		lipgloss.Color("#A5EFFF"), // light cyan
		lipgloss.Color("#C0F5FF"), // lighter
		lipgloss.Color("#D5FAFF"), // near white
		lipgloss.Color("#EEFFFF"), // almost white
	}
	ci := min(frame+idx, len(colors)-1)
	return colors[ci]
}

// punchFrame renders the punch-through: collapsed tunnel with gradient bar
// blasting across the center line.
func punchFrame(label string, labelWidth, termWidth int) string {
	mid := tunnelHeight / 2
	var lines []string
	for row := range tunnelHeight {
		dist := abs(row - mid)
		switch {
		case dist == 0:
			lines = append(lines, renderPunchCenter(label, labelWidth, termWidth))
		case dist <= 2:
			lines = append(lines, renderPunchFlank(dist, termWidth))
		default:
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// renderPunchCenter renders the center line with gradient bars flanking the label.
func renderPunchCenter(label string, labelWidth, termWidth int) string {
	barWidth := max((termWidth-labelWidth-4)/2, 2)
	leftBar := renderGradient(barWidth, "━")
	rightBar := renderGradient(barWidth, "━")
	return leftBar + " " + label + " " + rightBar
}

// renderPunchFlank renders the residual tunnel lines during punch-through.
func renderPunchFlank(dist, termWidth int) string {
	chevCount := dist
	inset := 12 + (dist * 2)
	wallWidth := max(termWidth-(inset*2)-(chevCount*4)-4, 3)

	leftChev := renderChevrons(chevCount, "▸", 4)
	rightChev := renderChevrons(chevCount, "◂", 4)
	wall := renderGradient(wallWidth, "═")

	pad := strings.Repeat(" ", inset)
	return pad + leftChev + " " + wall + " " + rightChev
}

// exhaustFrame renders the exhaust blast: center line with label,
// flanking chevrons now point outward (reversed).
func exhaustFrame(label string, labelWidth, termWidth int) string {
	mid := tunnelHeight / 2
	var lines []string
	for row := range tunnelHeight {
		dist := abs(row - mid)
		switch {
		case dist == 0:
			lines = append(lines, renderPunchCenter(label, labelWidth, termWidth))
		case dist <= 1:
			lines = append(lines, renderExhaustFlank(dist, termWidth))
		default:
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// renderExhaustFlank renders flanking lines with reversed chevrons (exhaust blast).
func renderExhaustFlank(dist, termWidth int) string {
	chevCount := dist + 1
	inset := 14 + (dist * 2)
	wallWidth := max(termWidth-(inset*2)-(chevCount*4)-4, 3)

	// Reversed: left side points left, right side points right (exhaust).
	leftChev := renderChevrons(chevCount, "◂", 4)
	rightChev := renderChevrons(chevCount, "▸", 4)
	wall := renderGradient(wallWidth, "═")

	pad := strings.Repeat(" ", inset)
	return pad + leftChev + " " + wall + " " + rightChev
}

// collapseFrame renders the final single-line frame with gradient blocks
// flanking the label — clean handoff to ssh.
func collapseFrame(label string, labelWidth, termWidth int) string {
	barWidth := max((termWidth-labelWidth-4)/2, 2)
	leftBar := renderGradient(barWidth, "█")
	rightBar := renderGradient(barWidth, "█")
	return leftBar + " " + label + " " + rightBar
}

// ReturnMessage returns a styled "gate closed" message shown after ssh exits.
func ReturnMessage() string {
	return returnStyle.Render("⟪ gate closed ⟫")
}

// PlayWarpAnimation plays the hyperspace tunnel animation to stdout.
func PlayWarpAnimation(hostShort, sessionName string, termWidth int) {
	PlayWarpAnimationTo(os.Stdout, hostShort, sessionName, termWidth)
}

// PlayWarpAnimationTo plays the hyperspace tunnel animation to a writer.
func PlayWarpAnimationTo(w io.Writer, hostShort, sessionName string, termWidth int) {
	frames := WarpFrames(hostShort, sessionName, termWidth)
	playMultiLineFrames(w, frames)
}

// CreationFrames generates the 4 animation frames for session creation.
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

// playMultiLineFrames renders multi-line animation frames, using ANSI cursor-up
// to overwrite previous frames in place.
func playMultiLineFrames(w io.Writer, frames []string) {
	timings := multiLineTimings(len(frames))
	for i, frame := range frames {
		if i > 0 {
			prevLines := strings.Count(frames[i-1], "\n") + 1
			fmt.Fprintf(w, "\033[%dA\r", prevLines)
		}
		writeFrameLines(w, frame)
		time.Sleep(timings[i])
	}
}

// writeFrameLines writes a multi-line frame, clearing each line to avoid artifacts.
func writeFrameLines(w io.Writer, frame string) {
	for j, line := range strings.Split(frame, "\n") {
		if j > 0 {
			fmt.Fprint(w, "\n")
		}
		fmt.Fprintf(w, "\033[2K%s", line)
	}
	fmt.Fprint(w, "\n")
}

// multiLineTimings returns per-frame durations: convergence frames are fast
// and accelerating, punch/exhaust are slightly longer, collapse is brief.
func multiLineTimings(n int) []time.Duration {
	timings := make([]time.Duration, n)
	for i := range n {
		timings[i] = 50 * time.Millisecond // default
	}
	if n >= 8 {
		// Convergence: decelerating approach (frames 0-4).
		timings[0] = 60 * time.Millisecond
		timings[1] = 55 * time.Millisecond
		timings[2] = 45 * time.Millisecond
		timings[3] = 35 * time.Millisecond
		timings[4] = 30 * time.Millisecond
		// Punch-through: brief pause for impact.
		timings[5] = 70 * time.Millisecond
		// Exhaust: quick flash.
		timings[6] = 50 * time.Millisecond
		// Collapse: snap.
		timings[7] = 40 * time.Millisecond
	}
	return timings
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
