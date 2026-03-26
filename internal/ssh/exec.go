package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// BuildAttachArgs constructs the full ssh argv for attaching to a tmux session.
//
// The returned slice is suitable for syscall.Exec: the first element is "ssh".
// No shell interpolation — each argument is a separate element.
//
// Produces:
//
//	ssh -t <target> -- env TERM=<term> tmux attach-session -t <sessionName>
func BuildAttachArgs(target, term, sessionName string) []string {
	return []string{
		"ssh", "-t", target, "--",
		"env", "TERM=" + term, "tmux", "attach-session", "-t", sessionName,
	}
}

// BuildScanArgs constructs the ssh argv for listing tmux sessions on a remote host.
//
// Produces:
//
//	ssh -o ConnectTimeout=<timeoutSec> -o BatchMode=yes <target>
//	    tmux list-sessions -F '#{session_name}\t#{session_attached}\t#{session_windows}'
func BuildScanArgs(target, timeoutSec string) []string {
	return []string{
		"ssh",
		"-o", "ConnectTimeout=" + timeoutSec,
		"-o", "BatchMode=yes",
		target,
		"tmux", "list-sessions", "-F",
		"\"#{session_name}\t#{session_attached}\t#{session_windows}\"",
	}
}

// HostShort extracts the hostname portion from an SSH target string.
// For "user@host" it returns "host"; for a bare hostname it returns
// the input unchanged.
func HostShort(target string) string {
	if i := strings.LastIndex(target, "@"); i >= 0 {
		return target[i+1:]
	}
	return target
}

// ExecReplace replaces the current process with ssh, attaching to the
// specified tmux session. On success this function never returns.
func ExecReplace(target, term, sessionName string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	argv := BuildAttachArgs(target, term, sessionName)

	// syscall.Exec replaces the process — clean TTY handoff, no orphan.
	return syscall.Exec(sshPath, argv, os.Environ())
}

// ExecChild runs ssh as a child process and waits for it to exit.
// Unlike ExecReplace, this returns when ssh exits, allowing the caller
// to resume (e.g. re-launch the TUI).
func ExecChild(target, term, sessionName string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	argv := BuildAttachArgs(target, term, sessionName)

	cmd := exec.Command(sshPath, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
