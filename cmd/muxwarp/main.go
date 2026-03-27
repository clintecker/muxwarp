package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/scanner"
	"github.com/clintecker/muxwarp/internal/ssh"
	"github.com/clintecker/muxwarp/internal/sshconfig"
	"github.com/clintecker/muxwarp/internal/tui"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("muxwarp %s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = runWizard()
			if cfg == nil {
				return
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n\nExample config (%s):\n\n%s",
				err, config.DefaultPath(), config.ExampleConfig())
			os.Exit(1)
		}
	}

	timeoutSec := parseTimeoutSec(cfg.Defaults.Timeout)

	if len(os.Args) > 1 {
		directWarp(cfg, timeoutSec, os.Args[1])
		return
	}

	tuiMode(cfg, timeoutSec)
}

// tuiMode runs the interactive TUI in a loop. After warping into an ssh
// session and returning, the TUI restarts with a fresh scan.
func tuiMode(cfg *config.Config, timeoutSec string) {
	for {
		target, termWidth, configChanged := runTUIOnce(cfg, timeoutSec)
		if configChanged {
			// Reload config from disk and restart TUI.
			newCfg, err := config.Load(config.DefaultPath())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
				return
			}
			cfg = newCfg
			timeoutSec = parseTimeoutSec(cfg.Defaults.Timeout)
			continue
		}
		if target == nil {
			return // user quit
		}
		warpChild(cfg, target, termWidth)
		// loop: fresh rescan, new TUI
	}
}

// warpChild creates the session if needed, plays animations, runs ssh
// as a child process, and prints the return message.
func warpChild(cfg *config.Config, target *tui.Session, termWidth int) {
	if err := maybeCreateSession(cfg, target, termWidth); err != nil {
		fmt.Fprintf(os.Stderr, "create error: %v\n", err)
		return
	}

	tui.PlayWarpAnimation(target.HostShort, target.Name, termWidth)

	if err := ssh.ExecChild(target.Host, cfg.Defaults.Term, target.Name); err != nil {
		fmt.Fprintf(os.Stderr, "ssh error: %v\n", err)
	}

	fmt.Println(tui.ReturnMessage())
	time.Sleep(400 * time.Millisecond)
}

// runTUIOnce runs a single TUI session with background scanning. Returns
// the warp target (nil if the user quit), the terminal width, and whether
// the config was modified (requiring a restart with fresh scan).
func runTUIOnce(cfg *config.Config, timeoutSec string) (*tui.Session, int, bool) {
	ctx, cancel := context.WithCancel(context.Background())

	m := tui.NewModel(len(cfg.Hosts))
	m.SetConfig(cfg, config.DefaultPath())
	m.SetSSHHosts(sshconfig.ParseHosts())
	p := tea.NewProgram(m)

	// Start scanning in background, then inject ghosts for desired sessions.
	go scanAndSend(ctx, cfg, timeoutSec, p)

	finalModel, err := p.Run()
	cancel() // stop any in-flight scans

	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return nil, 80, false
	}

	fm, ok := finalModel.(tui.Model)
	if !ok {
		return nil, 80, false
	}

	return fm.WarpTarget(), fm.Width(), fm.ConfigChanged()
}

// scanAndSend runs the scanner and sends results to the TUI, injecting
// ghost sessions for any desired sessions not found in the scan.
func scanAndSend(ctx context.Context, cfg *config.Config, timeoutSec string, p *tea.Program) {
	var found []tui.Session
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(host string, sessions []scanner.Session) {
		batch := scannerToTUI(sessions)
		found = append(found, batch...)
		p.Send(tui.SessionBatchMsg{Host: host, Sessions: batch})
	})

	ghosts := buildGhosts(cfg, found)
	if len(ghosts) > 0 {
		p.Send(tui.SessionBatchMsg{Host: "ghosts", Sessions: ghosts})
	}
	p.Send(tui.ScanDoneMsg{})
}

// directWarp handles `muxwarp <pattern>` — scan, fuzzy match, then warp.
func directWarp(cfg *config.Config, timeoutSec, pattern string) {
	fmt.Fprintf(os.Stderr, "Scanning gates...\n")

	ctx, cancel := context.WithCancel(context.Background())

	var allSessions []tui.Session
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(_ string, sessions []scanner.Session) {
		allSessions = append(allSessions, scannerToTUI(sessions)...)
	})
	cancel()

	allSessions = append(allSessions, buildGhosts(cfg, allSessions)...)

	if len(allSessions) == 0 {
		fmt.Fprintf(os.Stderr, "No sessions found on any host.\n")
		os.Exit(1)
	}

	matches := fuzzyMatch(pattern, allSessions)

	switch len(matches) {
	case 0:
		fmt.Fprintf(os.Stderr, "No sessions matching %q\n", pattern)
		os.Exit(1)

	case 1:
		// Single match: exec-replace (no list to return to).
		warp(cfg, &matches[0])

	default:
		directWarpMultiple(cfg, allSessions, pattern, timeoutSec)
	}
}

// directWarpMultiple runs the TUI pre-filtered when multiple matches are found.
// After the first warp, falls through to the tuiMode loop for reconnection.
func directWarpMultiple(cfg *config.Config, allSessions []tui.Session, pattern, timeoutSec string) {
	m := tui.NewModelWithSessions(allSessions, pattern)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	fm, ok := finalModel.(tui.Model)
	if !ok {
		return
	}

	target := fm.WarpTarget()
	if target == nil {
		return
	}

	warpChild(cfg, target, fm.Width())

	// Fall through to tuiMode loop for reconnection.
	tuiMode(cfg, timeoutSec)
}

// warp plays the animation and execs into ssh. Never returns on success.
func warp(cfg *config.Config, target *tui.Session) {
	if err := maybeCreateSession(cfg, target, 80); err != nil {
		fmt.Fprintf(os.Stderr, "create error: %v\n", err)
		os.Exit(1)
	}

	tui.PlayWarpAnimation(target.HostShort, target.Name, 80)

	if err := ssh.ExecReplace(target.Host, cfg.Defaults.Term, target.Name); err != nil {
		fmt.Fprintf(os.Stderr, "exec error: %v\n", err)
		os.Exit(1)
	}
}

// maybeCreateSession creates a remote tmux session if the target is a ghost.
func maybeCreateSession(cfg *config.Config, target *tui.Session, termWidth int) error {
	if !target.IsGhost() {
		return nil
	}

	tui.PlayCreationAnimation(target.HostShort, target.Name, termWidth)
	return ssh.CreateSession(target.Host, cfg.Defaults.Term, target.Name, target.Desired.Dir, target.Desired.Cmd)
}

// buildGhosts creates ghost sessions for desired sessions not found in the scan.
func buildGhosts(cfg *config.Config, found []tui.Session) []tui.Session {
	var ghosts []tui.Session
	for _, h := range cfg.Hosts {
		ghosts = append(ghosts, ghostsForHost(h, found)...)
	}
	return ghosts
}

// ghostsForHost returns ghost sessions for one host entry.
func ghostsForHost(h config.HostEntry, found []tui.Session) []tui.Session {
	if len(h.Sessions) == 0 {
		return nil
	}

	existing := existingNames(h.Target, found)
	var ghosts []tui.Session
	for _, ds := range h.Sessions {
		if existing[ds.Name] {
			continue
		}
		ghosts = append(ghosts, newGhostSession(h.Target, ds))
	}
	return ghosts
}

// existingNames returns the set of session names already found for a host.
func existingNames(target string, found []tui.Session) map[string]bool {
	names := make(map[string]bool)
	for _, s := range found {
		if s.Host == target {
			names[s.Name] = true
		}
	}
	return names
}

func newGhostSession(target string, ds config.DesiredSession) tui.Session {
	return tui.Session{
		Host:      target,
		HostShort: ssh.HostShort(target),
		Name:      ds.Name,
		Desired:   &tui.DesiredInfo{Dir: ds.Dir, Cmd: ds.Cmd},
	}
}

// fuzzyMatch runs fuzzy matching of pattern against all sessions.
// Returns the matched sessions in match-rank order.
func fuzzyMatch(pattern string, sessions []tui.Session) []tui.Session {
	source := make(fuzzySource, len(sessions))
	copy(source, sessions)

	matches := fuzzy.FindFrom(pattern, source)
	result := make([]tui.Session, 0, len(matches))
	for _, m := range matches {
		result = append(result, sessions[m.Index])
	}
	return result
}

// fuzzySource adapts []tui.Session for the fuzzy.Source interface.
type fuzzySource []tui.Session

func (s fuzzySource) String(i int) string { return s[i].HostShort + " " + s[i].Name }
func (s fuzzySource) Len() int            { return len(s) }

// scannerToTUI converts scanner.Session slices to tui.Session slices.
func scannerToTUI(sessions []scanner.Session) []tui.Session {
	result := make([]tui.Session, len(sessions))
	for i, s := range sessions {
		result[i] = tui.Session{
			Host:      s.Host,
			HostShort: s.HostShort,
			Name:      s.Name,
			Attached:  s.Attached,
			Windows:   s.Windows,
		}
	}
	return result
}

// runWizard runs the first-run wizard TUI and returns the created config,
// or nil if the user quit.
func runWizard() *config.Config {
	m := tui.NewModel(0)
	m.SetSSHHosts(sshconfig.ParseHosts())
	m.SetWizardMode()
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return nil
	}

	fm, ok := finalModel.(tui.Model)
	if !ok {
		return nil
	}

	cfg := fm.WizardConfig()
	if cfg == nil {
		return nil
	}

	if err := config.Save(cfg, config.DefaultPath()); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		return nil
	}

	return cfg
}

// parseTimeoutSec parses a duration string (e.g. "3s") and returns the
// seconds value as a string suitable for SSH ConnectTimeout.
func parseTimeoutSec(timeout string) string {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		// Fall back: try to parse as plain integer seconds.
		if _, err := strconv.Atoi(timeout); err == nil {
			return timeout
		}
		return "3" // safe default
	}
	return strconv.Itoa(max(int(d.Seconds()), 1))
}
