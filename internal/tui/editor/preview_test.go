package editor

import (
	"regexp"
	"strings"
	"testing"

	"github.com/clintecker/muxwarp/internal/config"
)

func assertPreviewContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, output)
	}
}

func assertPreviewExcludes(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("expected output to NOT contain %q, got:\n%s", substr, output)
	}
}

func TestRenderPreview_HostOnly(t *testing.T) {
	entry := config.HostEntry{Target: "user@server1"}
	output := RenderPreview(entry, 10)

	assertPreviewContains(t, output, "user@server1")
	assertPreviewContains(t, output, "target")
	assertPreviewExcludes(t, output, "sessions")
}

func TestRenderPreview_WithSessions(t *testing.T) {
	entry := config.HostEntry{
		Target: "user@server2",
		Sessions: []config.DesiredSession{
			{Name: "myproject", Dir: "~/code/myproject", Cmd: "nvim"},
			{Name: "logs", Dir: "/var/log"},
		},
	}

	output := RenderPreview(entry, 20)

	for _, want := range []string{
		"user@server2", "sessions", "myproject", "logs",
		"~/code/myproject", "/var/log", "nvim",
		"name", "dir", "cmd",
	} {
		assertPreviewContains(t, output, want)
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
