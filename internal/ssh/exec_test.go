package ssh

import (
	"testing"
)

func TestBuildAttachArgs(t *testing.T) {
	t.Parallel()

	args := BuildAttachArgs("clint@indigo", "xterm-256color", "cjdos")

	// Expected: ssh -t clint@indigo -- env TERM=xterm-256color tmux attach-session -t cjdos
	want := []string{
		"ssh", "-t", "clint@indigo", "--",
		"env", "TERM=xterm-256color", "tmux", "attach-session", "-t", "cjdos",
	}

	if len(args) != len(want) {
		t.Fatalf("BuildAttachArgs length = %d, want %d\nargs: %v", len(args), len(want), args)
	}

	for i := range want {
		if args[i] != want[i] {
			t.Errorf("BuildAttachArgs[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestBuildAttachArgs_NoShellInterpolation(t *testing.T) {
	t.Parallel()

	// Even if a session name looks dangerous, it must appear as a single
	// argument element with no shell wrapping or quoting.
	args := BuildAttachArgs("user@host", "xterm-256color", "evil;rm -rf /")

	// The session name must be a single argv element — no shell quoting applied.
	lastArg := args[len(args)-1]
	if lastArg != "evil;rm -rf /" {
		t.Errorf("session name was modified: got %q, want %q", lastArg, "evil;rm -rf /")
	}

	// Verify "--" separator is present before the remote command.
	foundSep := false
	for _, a := range args {
		if a == "--" {
			foundSep = true
			break
		}
	}
	if !foundSep {
		t.Error("BuildAttachArgs missing '--' separator")
	}
}

func TestBuildScanArgs(t *testing.T) {
	t.Parallel()

	args := BuildScanArgs("clint@indigo", "3")

	// Must contain ConnectTimeout and BatchMode.
	assertArgHasSSHOption(t, args, "ConnectTimeout=3")
	assertArgHasSSHOption(t, args, "BatchMode=yes")

	// Must contain target.
	assertArgContains(t, args, "clint@indigo", "target")

	// Must contain the tmux list-sessions format string (double-quoted for remote shell).
	assertArgContains(t, args, "\"#{session_name}\t#{session_attached}\t#{session_windows}\"", "tmux format string")
}

// assertArgHasSSHOption checks that args contains "-o" followed by the expected option value.
func assertArgHasSSHOption(t *testing.T, args []string, option string) {
	t.Helper()
	for i, a := range args {
		if a == "-o" && i+1 < len(args) && args[i+1] == option {
			return
		}
	}
	t.Errorf("BuildScanArgs missing -o %s: %v", option, args)
}

// assertArgContains checks that args contains the expected value.
func assertArgContains(t *testing.T, args []string, value, label string) {
	t.Helper()
	for _, a := range args {
		if a == value {
			return
		}
	}
	t.Errorf("args missing %s (%q): %v", label, value, args)
}

func assertArgsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d\ngot:  %v\nwant: %v", label, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}

func TestBuildCreateSessionArgs_Basic(t *testing.T) {
	t.Parallel()

	args, err := BuildCreateSessionArgs("clint@indigo", "xterm-256color", "myproj", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"ssh", "-t", "clint@indigo", "--",
		"env", "TERM=xterm-256color", "tmux", "new-session", "-d", "-s", "myproj",
	}
	assertArgsEqual(t, "BuildCreateSessionArgs", args, want)
}

func TestBuildCreateSessionArgs_WithDir(t *testing.T) {
	t.Parallel()

	args, err := BuildCreateSessionArgs("clint@indigo", "xterm-256color", "myproj", "~/code/myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"ssh", "-t", "clint@indigo", "--",
		"env", "TERM=xterm-256color", "tmux", "new-session", "-d", "-s", "myproj",
		"-c", "~/code/myproj",
	}
	assertArgsEqual(t, "BuildCreateSessionArgs", args, want)
}

func TestBuildCreateSessionArgs_InvalidName(t *testing.T) {
	t.Parallel()

	_, err := BuildCreateSessionArgs("user@host", "xterm-256color", "bad:name", "")
	if err == nil {
		t.Fatal("expected error for invalid session name, got nil")
	}
}

func TestBuildSendKeysArgs(t *testing.T) {
	t.Parallel()

	args := BuildSendKeysArgs("clint@indigo", "myproj", "claude --dangerously-skip-permissions")

	want := []string{
		"ssh", "clint@indigo", "--",
		"tmux", "send-keys", "-t", "myproj", "-l", "'claude --dangerously-skip-permissions'",
		"\\;", "send-keys", "-t", "myproj", "Enter",
	}
	assertArgsEqual(t, "BuildSendKeysArgs", args, want)
}

func TestBuildSendKeysArgs_QuotedCmd(t *testing.T) {
	t.Parallel()

	args := BuildSendKeysArgs("clint@indigo", "test-123", `echo "READY"`)

	// The cmd must be single-quoted for the remote shell.
	assertArgContains(t, args, `'echo "READY"'`, "single-quoted cmd")
	// Enter must be the last arg (tmux key name).
	if args[len(args)-1] != "Enter" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "Enter")
	}
}

func TestBuildSendKeysArgs_EmbeddedSingleQuote(t *testing.T) {
	t.Parallel()

	args := BuildSendKeysArgs("host", "sess", "echo 'hello'")

	// Single quotes in cmd must be escaped for the remote shell.
	assertArgContains(t, args, `'echo '\''hello'\'''`, "escaped single quotes")
}

func TestSingleQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{`echo "READY"`, `'echo "READY"'`},
		{`simple`, `'simple'`},
		{`it's here`, `'it'\''s here'`},
		{`a'b'c`, `'a'\''b'\''c'`},
	}

	for _, tc := range tests {
		got := singleQuote(tc.input)
		if got != tc.want {
			t.Errorf("singleQuote(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestHostShort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target string
		want   string
	}{
		{"clint@indigo", "indigo"},
		{"clint@clint-devboi", "clint-devboi"},
		{"indigo", "indigo"},
		{"user@host.example.com", "host.example.com"},
		{"root@192.168.1.1", "192.168.1.1"},
	}

	for _, tc := range tests {
		got := HostShort(tc.target)
		if got != tc.want {
			t.Errorf("HostShort(%q) = %q, want %q", tc.target, got, tc.want)
		}
	}
}
