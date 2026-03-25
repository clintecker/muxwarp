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

func TestWarpFrames(t *testing.T) {
	frames := WarpFrames("indigo", "cjdos", 80)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}

	assertAllFramesContain(t, frames, "engaging jumpgate", "indigo/cjdos", "\u2588")
	assertGrowingFrames(t, frames)
}

func TestWarpFramesMinBar(t *testing.T) {
	// Very narrow terminal: bar width should be clamped to minimum 4.
	frames := WarpFrames("indigo", "cjdos", 20)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}

	assertAllFramesContain(t, frames, "engaging jumpgate", "\u2588")
}

func assertOutputContains(t *testing.T, output, substr, label string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output missing %s", label)
	}
}

func TestPlayWarpAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayWarpAnimationTo(&buf, "indigo", "cjdos", 80)

	output := buf.String()

	assertOutputContains(t, output, "engaging jumpgate", "'engaging jumpgate'")
	assertOutputContains(t, output, "indigo/cjdos", "'indigo/cjdos'")
	assertOutputContains(t, output, "\u2588", "block character")
	assertOutputContains(t, output, "\r", "carriage return (\\r)")

	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}
