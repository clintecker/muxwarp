package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/clintecker/muxwarp/internal/config"
	"github.com/clintecker/muxwarp/internal/logging"
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
	logPath, args, logErr := extractLogFlag(os.Args[1:])
	if logErr != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", logErr)
		os.Exit(1)
	}
	cleanup, err := logging.Init(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	if cleanup != nil {
		defer cleanup()
	}

	logging.Log().Info("muxwarp starting", "version", version, "commit", commit, "date", date)

	if len(args) > 0 && args[0] == "--version" {
		fmt.Printf("muxwarp %s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printUsage()
		os.Exit(0)
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logging.Log().Info("no config file, running wizard", "path", config.DefaultPath())
			cfg = runWizard()
			if cfg == nil {
				return
			}
			logging.Log().Info("wizard completed", "hosts", len(cfg.Hosts))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n\nExample config (%s):\n\n%s",
				err, config.DefaultPath(), config.ExampleConfig())
			os.Exit(1)
		}
	}

	timeoutSec := parseTimeoutSec(cfg.Defaults.Timeout)

	if len(args) > 0 {
		directWarp(cfg, timeoutSec, args[0])
		return
	}

	tuiMode(cfg, timeoutSec)
}

// extractLogFlag pulls --log <path> or --log=<path> from args, returning
// the log path and the remaining args. If --log is present without a path,
// it returns an error message.
func extractLogFlag(args []string) (logPath string, rest []string, err string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--log" {
			if i+1 >= len(args) {
				return "", nil, "--log requires a file path argument"
			}
			logPath = args[i+1]
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+2:]...)
			return logPath, rest, ""
		}
		if strings.HasPrefix(args[i], "--log=") {
			logPath = strings.TrimPrefix(args[i], "--log=")
			if logPath == "" {
				return "", nil, "--log requires a file path argument"
			}
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return logPath, rest, ""
		}
	}
	return "", args, ""
}

// tuiMode runs the interactive TUI in a loop. After warping into an ssh
// session and returning, the TUI restarts with a fresh scan.
func tuiMode(cfg *config.Config, timeoutSec string) {
	for {
		target, termWidth, configChanged := runTUIOnce(cfg, timeoutSec)
		if configChanged {
			logging.Log().Info("config changed, reloading")
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
	logging.Log().Info("warp child", "host", target.Host, "session", target.Name, "is_ghost", target.IsGhost())

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
	logging.Log().Info("scan starting", "hosts", len(cfg.Hosts))
	var found []tui.Session
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(host string, sessions []scanner.Session) {
		batch := scannerToTUI(sessions)
		found = append(found, batch...)
		logging.Log().Debug("scan batch", "host", host, "sessions", len(batch))
		p.Send(tui.SessionBatchMsg{Host: host, Sessions: batch})
	})

	ghosts := buildGhosts(cfg, found)
	if len(ghosts) > 0 {
		logging.Log().Info("injecting ghosts", "count", len(ghosts))
		p.Send(tui.SessionBatchMsg{Host: "ghosts", Sessions: ghosts})
	}
	logging.Log().Info("scan complete", "found", len(found), "ghosts", len(ghosts))
	p.Send(tui.ScanDoneMsg{})
}

// directWarp handles `muxwarp <pattern>` — scan, fuzzy match, then warp.
func directWarp(cfg *config.Config, timeoutSec, pattern string) {
	logging.Log().Info("direct warp", "pattern", pattern)
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
	logging.Log().Info("warp exec-replace", "host", target.Host, "session", target.Name, "is_ghost", target.IsGhost())

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

	logging.Log().Info("creating ghost session",
		"host", target.Host, "session", target.Name,
		"dir", target.Desired.Dir, "cmd", target.Desired.Cmd)

	tui.PlayCreationAnimation(target.HostShort, target.Name, termWidth)
	err := ssh.CreateSession(target.Host, cfg.Defaults.Term, target.Name, target.Desired.Dir, target.Desired.Cmd)
	if err != nil {
		logging.Log().Error("ghost session creation failed", "error", err)
	} else {
		logging.Log().Info("ghost session created")
	}
	return err
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

func printUsage() {
	fmt.Printf(`muxwarp %s — warp into tmux sessions on remote machines

Usage:
  muxwarp                     Launch interactive TUI
  muxwarp <pattern>           Fuzzy-match and warp directly
  muxwarp --log <path>        Write debug logs to file
  muxwarp --version           Print version and exit
  muxwarp --help              Show this help
`, version)
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
