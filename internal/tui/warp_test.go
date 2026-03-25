package tui

import (
	"bytes"
	"strings"
	"testing"
)

func TestWarpFrames(t *testing.T) {
	frames := WarpFrames("indigo", "cjdos", 80)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}

	// Every frame must contain the jumpgate label.
	for i, f := range frames {
		if !strings.Contains(f, "engaging jumpgate") {
			t.Errorf("frame %d missing 'engaging jumpgate': %q", i, f)
		}
		if !strings.Contains(f, "indigo/cjdos") {
			t.Errorf("frame %d missing 'indigo/cjdos': %q", i, f)
		}
	}

	// Each frame should be longer than the previous one (growing bar).
	for i := 1; i < len(frames); i++ {
		if len(frames[i]) <= len(frames[i-1]) {
			t.Errorf("frame %d (len %d) should be longer than frame %d (len %d)",
				i, len(frames[i]), i-1, len(frames[i-1]))
		}
	}

	// Every frame must contain at least one block character.
	for i, f := range frames {
		if !strings.Contains(f, "█") {
			t.Errorf("frame %d missing block character: %q", i, f)
		}
	}
}

func TestWarpFramesMinBar(t *testing.T) {
	// Very narrow terminal: bar width should be clamped to minimum 4.
	frames := WarpFrames("indigo", "cjdos", 20)

	if len(frames) != 4 {
		t.Fatalf("WarpFrames returned %d frames, want 4", len(frames))
	}

	// Should still contain the label and block chars.
	for i, f := range frames {
		if !strings.Contains(f, "engaging jumpgate") {
			t.Errorf("frame %d missing 'engaging jumpgate': %q", i, f)
		}
		if !strings.Contains(f, "█") {
			t.Errorf("frame %d missing block character: %q", i, f)
		}
	}
}

func TestPlayWarpAnimationTo(t *testing.T) {
	var buf bytes.Buffer
	PlayWarpAnimationTo(&buf, "indigo", "cjdos", 80)

	output := buf.String()

	if !strings.Contains(output, "engaging jumpgate") {
		t.Error("output missing 'engaging jumpgate'")
	}
	if !strings.Contains(output, "indigo/cjdos") {
		t.Error("output missing 'indigo/cjdos'")
	}
	if !strings.Contains(output, "█") {
		t.Error("output missing block character")
	}

	// The output must use carriage returns to overwrite the line.
	if !strings.Contains(output, "\r") {
		t.Error("output missing carriage return (\\r)")
	}

	// The output must end with a newline.
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}
