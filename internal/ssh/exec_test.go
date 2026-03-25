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
	var hasTimeout, hasBatch bool
	for i, a := range args {
		if a == "-o" && i+1 < len(args) {
			switch args[i+1] {
			case "ConnectTimeout=3":
				hasTimeout = true
			case "BatchMode=yes":
				hasBatch = true
			}
		}
	}

	if !hasTimeout {
		t.Errorf("BuildScanArgs missing ConnectTimeout: %v", args)
	}
	if !hasBatch {
		t.Errorf("BuildScanArgs missing BatchMode=yes: %v", args)
	}

	// Must contain target.
	found := false
	for _, a := range args {
		if a == "clint@indigo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BuildScanArgs missing target: %v", args)
	}

	// Must contain the tmux list-sessions format string.
	foundFmt := false
	for _, a := range args {
		if a == "#{session_name}\t#{session_attached}\t#{session_windows}" {
			foundFmt = true
			break
		}
	}
	if !foundFmt {
		t.Errorf("BuildScanArgs missing tmux format string: %v", args)
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
