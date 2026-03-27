package editor

import (
	"regexp"
	"strings"
	"testing"

	"github.com/clintecker/muxwarp/internal/config"
)

func TestRenderPreview_HostOnly(t *testing.T) {
	entry := config.HostEntry{
		Target: "user@server1",
	}

	output := RenderPreview(entry, 10)

	// Should contain the target value.
	if !strings.Contains(output, "user@server1") {
		t.Errorf("expected output to contain target, got:\n%s", output)
	}

	// Should contain "target" key.
	if !strings.Contains(output, "target") {
		t.Errorf("expected output to contain 'target' key, got:\n%s", output)
	}

	// Should NOT contain "sessions".
	if strings.Contains(output, "sessions") {
		t.Errorf("expected output to not contain 'sessions', got:\n%s", output)
	}
}

func TestRenderPreview_WithSessions(t *testing.T) {
	entry := config.HostEntry{
		Target: "user@server2",
		Sessions: []config.DesiredSession{
			{
				Name: "myproject",
				Dir:  "~/code/myproject",
				Cmd:  "nvim",
			},
			{
				Name: "logs",
				Dir:  "/var/log",
			},
		},
	}

	output := RenderPreview(entry, 20)

	// Should contain target.
	if !strings.Contains(output, "user@server2") {
		t.Errorf("expected output to contain target, got:\n%s", output)
	}

	// Should contain "sessions" keyword.
	if !strings.Contains(output, "sessions") {
		t.Errorf("expected output to contain 'sessions', got:\n%s", output)
	}

	// Should contain session names.
	if !strings.Contains(output, "myproject") {
		t.Errorf("expected output to contain 'myproject', got:\n%s", output)
	}
	if !strings.Contains(output, "logs") {
		t.Errorf("expected output to contain 'logs', got:\n%s", output)
	}

	// Should contain dirs.
	if !strings.Contains(output, "~/code/myproject") {
		t.Errorf("expected output to contain dir '~/code/myproject', got:\n%s", output)
	}
	if !strings.Contains(output, "/var/log") {
		t.Errorf("expected output to contain dir '/var/log', got:\n%s", output)
	}

	// Should contain cmd for first session.
	if !strings.Contains(output, "nvim") {
		t.Errorf("expected output to contain cmd 'nvim', got:\n%s", output)
	}

	// Should contain "name", "dir", "cmd" keys.
	if !strings.Contains(output, "name") {
		t.Errorf("expected output to contain 'name' key, got:\n%s", output)
	}
	if !strings.Contains(output, "dir") {
		t.Errorf("expected output to contain 'dir' key, got:\n%s", output)
	}
	if !strings.Contains(output, "cmd") {
		t.Errorf("expected output to contain 'cmd' key, got:\n%s", output)
	}
}

func TestRenderPreview_SyntaxColors(t *testing.T) {
	entry := config.HostEntry{
		Target: "user@server3",
		Sessions: []config.DesiredSession{
			{
				Name: "test",
				Dir:  "/tmp",
			},
		},
	}

	output := RenderPreview(entry, 15)

	// Strip ANSI escape codes.
	plainOutput := stripANSI(output)

	// The styled output should be longer than the plain output due to ANSI codes.
	if len(output) <= len(plainOutput) {
		t.Errorf("expected styled output (%d bytes) to be longer than plain output (%d bytes)",
			len(output), len(plainOutput))
	}

	// The styled output should contain ANSI escape sequences.
	// ANSI codes start with ESC (0x1b or \x1b).
	if !strings.Contains(output, "\x1b[") {
		t.Errorf("expected output to contain ANSI escape codes, got:\n%s", output)
	}
}

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	// Match ANSI escape sequences: ESC [ ... m
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(s, "")
}
