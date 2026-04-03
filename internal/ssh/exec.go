package ssh

import (
	"bytes"
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
//	    tmux list-sessions -F '#{session_name}\t#{session_attached}\t#{session_windows}\t#{session_created}\t#{session_activity}'
func BuildScanArgs(target, timeoutSec string) []string {
	return []string{
		"ssh",
		"-o", "ConnectTimeout=" + timeoutSec,
		"-o", "BatchMode=yes",
		target,
		"tmux", "list-sessions", "-F",
		"\"#{session_name}\t#{session_attached}\t#{session_windows}\t#{session_created}\t#{session_activity}\"",
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
	logging.Log().Info("exec replace", "target", target, "session", sessionName)

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
// If cmd is specified, it is sent via tmux command chaining in the same SSH
// connection, avoiding an extra round-trip.
func CreateSession(target, term, name, dir, cmd string) error {
	argv, err := BuildCreateSessionArgs(target, term, name, dir)
	if err != nil {
		return err
	}

	// Chain send-keys onto the same tmux command to avoid a second SSH call.
	if cmd != "" {
		argv = append(argv,
			"\\;", "send-keys", "-t", name, "-l", singleQuote(cmd),
			"\\;", "send-keys", "-t", name, "Enter",
		)
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	logging.Log().Info("create session", "target", target, "session", name)

	c := exec.Command(sshPath, argv[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		logging.Log().Error("create session failed", "error", err)
		return err
	}
	logging.Log().Info("session created", "name", name)

	return nil
}

// BuildEnsureRepoScript returns a shell script that ensures dir exists and
// contains the expected repo. It runs as a single SSH command, avoiding
// multiple round-trips.
//
// Exit codes:
//
//	0 = repo present and matches (or clone succeeded)
//	2 = repo mismatch (stdout has the actual remote URL)
//	3 = clone failed
//	other = unexpected error
func BuildEnsureRepoScript(dir, repo string) string {
	// Replace leading ~ with $HOME so it expands inside double quotes.
	// Single-quoting would prevent tilde expansion on the remote shell.
	shellDir := shellQuoteDir(dir)
	shellRepo := singleQuote(repo)
	return fmt.Sprintf(`set -e
mkdir -p %[1]s
url=$(git -C %[1]s remote get-url origin 2>/dev/null) || {
  gh repo clone %[2]s %[1]s || exit 3
  exit 0
}
echo "$url"
exit 2
`, shellDir, shellRepo)
}

// shellQuoteDir quotes a directory path for use in a shell script.
// Replaces a leading ~ with $HOME and uses double quotes, so the path
// expands correctly on the remote host.
func shellQuoteDir(dir string) string {
	if strings.HasPrefix(dir, "~/") {
		return `"$HOME/` + strings.TrimPrefix(dir, "~/") + `"`
	}
	return singleQuote(dir)
}

// BuildEnsureRepoArgs constructs the ssh argv for running the ensure-repo
// script on a remote host in a single connection.
func BuildEnsureRepoArgs(target, dir, repo string) []string {
	return []string{"ssh", target, "--", "sh", "-c", singleQuote(BuildEnsureRepoScript(dir, repo))}
}

// EnsureRepo ensures the target directory exists and contains the expected repo.
// Runs as a single SSH connection: mkdir, check git remote, clone if missing.
// Returns an error if a different repo is already there.
func EnsureRepo(target, dir, repo string) error {
	if err := validateEnsureRepoArgs(dir, repo); err != nil {
		return err
	}

	stdout, stderr, err := runEnsureRepo(target, dir, repo)
	if err == nil {
		logging.Log().Info("ensure repo: ok", "repo", repo)
		return nil
	}

	return handleEnsureRepoError(err, dir, repo, stdout, stderr)
}

// validateEnsureRepoArgs validates the inputs to EnsureRepo.
func validateEnsureRepoArgs(dir, repo string) error {
	if dir == "" {
		return fmt.Errorf("ensure repo: dir must not be empty (repo %s requires a directory)", repo)
	}
	if !ValidRepoSlug(repo) {
		return fmt.Errorf("ensure repo: invalid repo slug %q", repo)
	}
	return nil
}

// runEnsureRepo executes the ensure-repo SSH command and returns stdout, stderr, and error.
func runEnsureRepo(target, dir, repo string) (string, string, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return "", "", fmt.Errorf("ssh not found in PATH: %w", err)
	}

	argv := BuildEnsureRepoArgs(target, dir, repo)
	logging.Log().Info("ensure repo", "target", target, "dir", dir, "repo", repo)

	cmd := exec.Command(sshPath, argv[1:]...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	return strings.TrimSpace(stdoutBuf.String()), strings.TrimSpace(stderrBuf.String()), err
}

// handleEnsureRepoError interprets a non-nil error from the ensure-repo script.
func handleEnsureRepoError(err error, dir, repo, stdout, stderr string) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return fmt.Errorf("ensure repo: %w", err)
	}
	return handleEnsureRepoExit(exitErr.ExitCode(), dir, repo, stdout, stderr)
}

// handleEnsureRepoExit maps exit codes to specific error messages.
func handleEnsureRepoExit(code int, dir, repo, stdout, stderr string) error {
	switch code {
	case 2:
		return checkRepoMatch(dir, repo, stdout)
	case 3:
		return fmt.Errorf("gh repo clone %s failed: %s", repo, stderr)
	default:
		return fmt.Errorf("ensure repo: %s: exit code %d", stderr, code)
	}
}

// checkRepoMatch compares the existing remote URL with the expected repo.
func checkRepoMatch(dir, repo, stdout string) error {
	existing := NormalizeRemoteURL(stdout)
	if existing == repo {
		logging.Log().Info("ensure repo: match", "repo", repo)
		return nil
	}
	if !ValidRepoSlug(existing) {
		return fmt.Errorf("directory %s has unrecognized remote URL %q; expected %q", dir, stdout, repo)
	}
	return fmt.Errorf("directory %s contains repo %q, expected %q", dir, existing, repo)
}

// BuildGhostWarpScript builds a shell script that ensures repo, creates
// a tmux session, sends a startup command, and attaches — all in one shot.
// The final `exec` replaces the shell with tmux attach, so when the user
// detaches, SSH exits cleanly.
func BuildGhostWarpScript(term, name, dir, repo, cmd string) string {
	var b strings.Builder

	b.WriteString("set -e\n")

	shellDir := shellQuoteDir(dir)

	// Ensure repo (optional).
	if repo != "" && dir != "" {
		shellRepo := singleQuote(repo)
		fmt.Fprintf(&b, "mkdir -p %s\n", shellDir)
		fmt.Fprintf(&b, "url=$(git -C %s remote get-url origin 2>/dev/null) || {\n", shellDir)
		fmt.Fprintf(&b, "  gh repo clone %s %s || { echo 'clone failed' >&2; exit 3; }\n", shellRepo, shellDir)
		fmt.Fprintf(&b, "}\n")
	}

	// Create detached session if it doesn't already exist.
	fmt.Fprintf(&b, "if ! tmux has-session -t %s 2>/dev/null; then\n", singleQuote(name))
	fmt.Fprintf(&b, "  tmux new-session -d -s %s", singleQuote(name))
	if dir != "" {
		fmt.Fprintf(&b, " -c %s", shellDir)
	}
	b.WriteString("\n")

	// Send startup command only for newly created sessions.
	if cmd != "" {
		fmt.Fprintf(&b, "  tmux send-keys -t %s -l %s\n", singleQuote(name), singleQuote(cmd))
		fmt.Fprintf(&b, "  tmux send-keys -t %s Enter\n", singleQuote(name))
	}
	b.WriteString("fi\n")

	// Attach — exec replaces the shell so detach exits SSH cleanly.
	fmt.Fprintf(&b, "exec env TERM=%s tmux attach-session -t %s\n", term, singleQuote(name))

	return b.String()
}

// GhostWarpChild runs a ghost warp as a child process: ensure repo, create
// session, send cmd, attach. Single SSH connection. Returns when user detaches.
func GhostWarpChild(target, term, name, dir, repo, cmd string) error {
	if !ValidSessionName(name) {
		return fmt.Errorf("invalid session name: %q", name)
	}

	script := BuildGhostWarpScript(term, name, dir, repo, cmd)
	argv := []string{"ssh", "-t", target, "--", "sh", "-c", singleQuote(script)}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	logging.Log().Info("ghost warp child", "target", target, "session", name)

	c := exec.Command(sshPath, argv[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// GhostWarpReplace runs a ghost warp via exec-replace: ensure repo, create
// session, send cmd, attach. Single SSH connection. Never returns on success.
func GhostWarpReplace(target, term, name, dir, repo, cmd string) error {
	if !ValidSessionName(name) {
		return fmt.Errorf("invalid session name: %q", name)
	}

	script := BuildGhostWarpScript(term, name, dir, repo, cmd)
	argv := []string{"ssh", "-t", target, "--", "sh", "-c", singleQuote(script)}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	logging.Log().Info("ghost warp replace", "target", target, "session", name)

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
	logging.Log().Info("exec child", "target", target, "session", sessionName)

	cmd := exec.Command(sshPath, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
