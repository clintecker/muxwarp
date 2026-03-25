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

	"github.com/clint/muxwarp/internal/ssh"
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
	args := ssh.BuildScanArgs(target, timeoutSec)

	// args[0] is "ssh"; exec.CommandContext resolves it via PATH.
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Ensure Output returns promptly after the process is killed.
	// Without WaitDelay, Output blocks until all I/O pipes close,
	// which may not happen if the shell script spawns child processes.
	cmd.WaitDelay = 2 * time.Second

	out, err := cmd.Output()
	if err != nil {
		// Any failure (timeout, auth, no tmux, exit code != 0) → empty.
		return nil, nil
	}

	hostShort := ssh.HostShort(target)

	var sessions []Session
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		name := parts[0]
		if !ssh.ValidSessionName(name) {
			continue
		}

		attached, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		windows, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		sessions = append(sessions, Session{
			Host:      target,
			HostShort: hostShort,
			Name:      name,
			Attached:  attached,
			Windows:   windows,
		})
	}

	// Return nil slice when nothing found — callers check len().
	return sessions, nil
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
		// Stop launching new work if context is cancelled.
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			sessions, _ := ScanHost(ctx, host, timeoutSec)
			if len(sessions) > 0 {
				onBatch(host, sessions)
			}
		}()
	}

	wg.Wait()
	return nil
}
