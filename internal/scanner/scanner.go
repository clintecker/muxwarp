// Package scanner discovers remote tmux sessions over SSH.
//
// ScanHost probes a single host; ScanAll fans out across multiple hosts
// with bounded parallelism. Errors (timeouts, auth failures, absent tmux
// server) are silently swallowed — the scanner returns only what it finds.
package scanner

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clintecker/muxwarp/internal/logging"
	"github.com/clintecker/muxwarp/internal/ssh"
)

// Session represents a single tmux session discovered on a remote host.
type Session struct {
	Host      string // full SSH target (e.g. "clint@indigo")
	HostShort string // display name (e.g. "indigo")
	Name      string // tmux session name (validated)
	Attached  int    // number of attached clients
	Windows   int    // number of windows
}

// Key returns a unique identifier for the session: "host/name".
func (s Session) Key() string { return s.Host + "/" + s.Name }

// ScanHost runs ssh to list tmux sessions on a single host.
//
// It returns an empty slice (not an error) for any failure — timeout,
// authentication, no tmux server, or unparseable output. Only sessions
// with valid names (per ssh.ValidSessionName) are included.
func ScanHost(ctx context.Context, target, timeoutSec string) ([]Session, error) {
	out, err := runScanCmd(ctx, target, timeoutSec)
	if err != nil {
		logging.Log().Debug("scan failed", "host", target, "error", err)
		return nil, nil
	}
	sessions := parseSessions(out, target)
	logging.Log().Debug("scan succeeded", "host", target, "sessions", len(sessions))
	return sessions, nil
}

// runScanCmd builds and executes the SSH command for listing tmux sessions.
func runScanCmd(ctx context.Context, target, timeoutSec string) ([]byte, error) {
	args := ssh.BuildScanArgs(target, timeoutSec)
	// args[0] is "ssh"; exec.CommandContext resolves it via PATH.
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	// Ensure Output returns promptly after the process is killed.
	// Without WaitDelay, Output blocks until all I/O pipes close,
	// which may not happen if the shell script spawns child processes.
	cmd.WaitDelay = 2 * time.Second
	return cmd.Output()
}

// parseSessions parses raw tmux list-sessions output into Session values.
// Lines that are empty, malformed, or have invalid session names are skipped.
func parseSessions(out []byte, target string) []Session {
	hostShort := ssh.HostShort(target)
	var sessions []Session
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if s, ok := parseSessionLine(sc.Text(), target, hostShort); ok {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// splitSessionFields splits a tab-separated line into (name, attached, windows).
// Returns false if the line is empty, has the wrong number of fields, or
// contains an invalid session name.
func splitSessionFields(line string) (name string, attached, windows int, ok bool) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 || !ssh.ValidSessionName(parts[0]) {
		return "", 0, 0, false
	}
	a, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, false
	}
	w, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, false
	}
	return parts[0], a, w, true
}

// parseSessionLine parses a single tab-separated line into a Session.
// Returns false if the line is empty, malformed, or has an invalid name.
func parseSessionLine(line, target, hostShort string) (Session, bool) {
	name, attached, windows, ok := splitSessionFields(line)
	if !ok {
		return Session{}, false
	}
	return Session{
		Host:      target,
		HostShort: hostShort,
		Name:      name,
		Attached:  attached,
		Windows:   windows,
	}, true
}

// ScanAll probes multiple hosts in parallel with bounded concurrency.
//
// For each host that has sessions, onBatch is called with the results.
// The callback may be invoked from multiple goroutines; callers must
// synchronise if needed. Hosts that return no sessions are silently
// skipped. Context cancellation stops launching new scans and
// propagates to in-flight ScanHost calls.
func ScanAll(ctx context.Context, hosts []string, maxParallel int, timeoutSec string, onBatch func(host string, sessions []Session)) error {
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Go(func() {
			scanOneHost(ctx, host, timeoutSec, sem, onBatch)
		})
	}

	wg.Wait()
	return nil
}

// scanOneHost acquires a semaphore slot, scans a single host, and calls
// onBatch if sessions are found.
func scanOneHost(ctx context.Context, host, timeoutSec string, sem chan struct{}, onBatch func(string, []Session)) {
	// Acquire semaphore slot, respecting context cancellation.
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return
	}

	logging.Log().Debug("scanning host", "host", host)
	sessions, _ := ScanHost(ctx, host, timeoutSec)
	if len(sessions) > 0 {
		onBatch(host, sessions)
	}
}
