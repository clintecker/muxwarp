package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/clintecker/muxwarp/internal/logging"
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
	logging.Log().Info("exec replace", "argv", argv)

	// syscall.Exec replaces the process — clean TTY handoff, no orphan.
	return syscall.Exec(sshPath, argv, os.Environ())
}

// BuildCreateSessionArgs constructs the ssh argv for creating a detached tmux session.
//
// Produces:
//
//	ssh -t <target> -- env TERM=<term> tmux new-session -d -s <name> [-c <dir>]
func BuildCreateSessionArgs(target, term, name, dir string) ([]string, error) {
	if !ValidSessionName(name) {
		return nil, fmt.Errorf("invalid session name: %q", name)
	}

	args := []string{
		"ssh", "-t", target, "--",
		"env", "TERM=" + term, "tmux", "new-session", "-d", "-s", name,
	}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	return args, nil
}

// BuildSendKeysArgs constructs the ssh argv for typing a command into a tmux
// session via send-keys. The command text is sent literally (-l flag), then
// Enter is sent as a key press via tmux command chaining.
//
// Produces (after SSH remote-shell evaluation):
//
//	tmux send-keys -t <name> -l '<cmd>' ; send-keys -t <name> Enter
func BuildSendKeysArgs(target, name, cmd string) []string {
	return []string{
		"ssh", target, "--",
		"tmux", "send-keys", "-t", name, "-l", singleQuote(cmd),
		"\\;", "send-keys", "-t", name, "Enter",
	}
}

// singleQuote wraps s in POSIX single quotes, escaping any embedded single
// quotes. This ensures the string passes through SSH's remote shell as a
// single argument to the target command.
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// CreateSession creates a detached tmux session on a remote host via SSH.
// If cmd is specified, it is sent to the session via send-keys after creation,
// keeping the session's shell alive even if the command exits.
func CreateSession(target, term, name, dir, cmd string) error {
	argv, err := BuildCreateSessionArgs(target, term, name, dir)
	if err != nil {
		return err
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	logging.Log().Info("create session", "argv", argv)

	c := exec.Command(sshPath, argv[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		logging.Log().Error("create session failed", "error", err)
		return err
	}
	logging.Log().Info("session created", "name", name)

	if cmd != "" {
		sendArgv := BuildSendKeysArgs(target, name, cmd)
		logging.Log().Info("sending command", "argv", sendArgv)
		s := exec.Command(sshPath, sendArgv[1:]...)
		s.Stderr = os.Stderr
		if err := s.Run(); err != nil {
			logging.Log().Error("send-keys failed", "error", err)
			return err
		}
		logging.Log().Info("command sent", "cmd", cmd)
	}

	return nil
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
	logging.Log().Info("exec child", "argv", argv)

	cmd := exec.Command(sshPath, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
