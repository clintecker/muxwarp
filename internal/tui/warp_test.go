package tui

import (
	"bytes"
	"strings"
	"testing"
)

func assertFrameContains(t *testing.T, frameIdx int, frame, substr string) {
	t.Helper()
	if !strings.Contains(frame, substr) {
		t.Errorf("frame %d missing %q: %q", frameIdx, substr, frame)
	}
}

func assertAllFramesContain(t *testing.T, frames []string, substrs ...string) {
	t.Helper()
	for i, f := range frames {
		for _, s := range substrs {
			assertFrameContains(t, i, f, s)
		}
	}
}

func assertGrowingFrames(t *testing.T, frames []string) {
	t.Helper()
	for i := 1; i < len(frames); i++ {
		if len(frames[i]) <= len(frames[i-1]) {
			t.Errorf("frame %d (len %d) should be longer than frame %d (len %d)",
				i, len(frames[i]), i-1, len(frames[i-1]))
		}
	}
}

func assertOutputContains(t *testing.T, output, substr, label string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output missing %s", label)
	}
}

func TestWarpFrames(t *testing.T) {
	frames := WarpFrames("indigo", "cjdos", 80)

	// 8 frames: 5 convergence + 1 punch + 1 exhaust + 1 collapse.
	if len(frames) != 8 {
		t.Fatalf("WarpFrames returned %d frames, want 8", len(frames))
	}

	// Convergence frames should contain tunnel chars.
	for i := range 5 {
		assertFrameContains(t, i, frames[i], "▸")
		assertFrameContains(t, i, frames[i], "◂")
		assertFrameContains(t, i, frames[i], "═")
	}

	// Convergence frames should be multi-line.
	for i := range 5 {
		lines := strings.Count(frames[i], "\n")
		if lines < 6 {
			t.Errorf("convergence frame %d has %d newlines, want >= 6", i, lines)
		}
	}
}

func TestWarpFrames_PunchAndCollapse(t *testing.T) {
	frames := WarpFrames("indigo", "cjdos", 80)

	// Punch-through frame (index 5) should contain the label.
	assertFrameContains(t, 5, frames[5], "engaging jumpgate")
	assertFrameContains(t, 5, frames[5], "indigo/cjdos")
	assertFrameContains(t, 5, frames[5], "━")

	// Exhaust frame (index 6) should contain the label.
	assertFrameContains(t, 6, frames[6], "engaging jumpgate")

	// Collapse frame (index 7) should have block chars.
	assertFrameContains(t, 7, frames[7], "█")
	assertFrameContains(t, 7, frames[7], "engaging jumpgate")
	assertFrameContains(t, 7, frames[7], "indigo/cjdos")
}

func TestWarpFrames_NarrowTerminal(t *testing.T) {
	// Should not panic even with a very narrow terminal.
	frames := WarpFrames("indigo", "cjdos", 30)

	if len(frames) != 8 {
		t.Fatalf("WarpFrames returned %d frames, want 8", len(frames))
	}
}

func TestPlayWarpAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayWarpAnimationTo(&buf, "indigo", "cjdos", 80)

	output := buf.String()

	assertOutputContains(t, output, "engaging jumpgate", "'engaging jumpgate'")
	assertOutputContains(t, output, "indigo/cjdos", "'indigo/cjdos'")
	assertOutputContains(t, output, "█", "block character")
	assertOutputContains(t, output, "▸", "right chevron")
	assertOutputContains(t, output, "◂", "left chevron")

	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}

func TestCreationFrames(t *testing.T) {
	frames := CreationFrames("indigo", "cjdos", 80)

	if len(frames) != 4 {
		t.Fatalf("CreationFrames returned %d frames, want 4", len(frames))
	}

	assertAllFramesContain(t, frames, "materializing lane", "indigo/cjdos", "░")
	assertGrowingFrames(t, frames)
}

func TestPlayCreationAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayCreationAnimationTo(&buf, "indigo", "cjdos", 80)

	output := buf.String()

	assertOutputContains(t, output, "materializing lane", "'materializing lane'")
	assertOutputContains(t, output, "indigo/cjdos", "'indigo/cjdos'")
	assertOutputContains(t, output, "░", "light block character")
	assertOutputContains(t, output, "\r", "carriage return (\\r)")

	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}

func TestReturnMessage(t *testing.T) {
	msg := ReturnMessage()

	if !strings.Contains(msg, "gate closed") {
		t.Errorf("ReturnMessage should contain 'gate closed', got: %q", msg)
	}
	if !strings.Contains(msg, "⟪") || !strings.Contains(msg, "⟫") {
		t.Errorf("ReturnMessage should contain bracket decorations, got: %q", msg)
	}
}

func TestMultiLineTimings(t *testing.T) {
	timings := multiLineTimings(8)
	if len(timings) != 8 {
		t.Fatalf("multiLineTimings(8) returned %d, want 8", len(timings))
	}
	// Convergence should accelerate (later frames faster).
	if timings[0] <= timings[4] {
		t.Error("convergence frames should decelerate (frame 0 should be slower than frame 4)")
	}
}
