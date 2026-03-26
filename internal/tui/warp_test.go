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
	frames := WarpFrames("atlas", "api-server", 80)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}

	assertAllFramesContain(t, frames, "engaging jumpgate", "atlas/api-server", "█")
	assertGrowingFrames(t, frames)
}

func TestWarpFrames_NarrowTerminal(t *testing.T) {
	frames := WarpFrames("atlas", "api-server", 30)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}
}

func TestPlayWarpAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayWarpAnimationTo(&buf, "atlas", "api-server", 80)

	output := buf.String()

	assertOutputContains(t, output, "engaging jumpgate", "'engaging jumpgate'")
	assertOutputContains(t, output, "atlas/api-server", "'atlas/api-server'")
	assertOutputContains(t, output, "█", "block character")
	assertOutputContains(t, output, "\r", "carriage return (\\r)")

	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}

func TestCreationFrames(t *testing.T) {
	frames := CreationFrames("atlas", "api-server", 80)

	if len(frames) != 4 {
		t.Fatalf("CreationFrames returned %d frames, want 4", len(frames))
	}

	assertAllFramesContain(t, frames, "materializing lane", "atlas/api-server", "░")
	assertGrowingFrames(t, frames)
}

func TestPlayCreationAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayCreationAnimationTo(&buf, "atlas", "api-server", 80)

	output := buf.String()

	assertOutputContains(t, output, "materializing lane", "'materializing lane'")
	assertOutputContains(t, output, "atlas/api-server", "'atlas/api-server'")
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
