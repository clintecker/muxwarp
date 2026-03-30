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

	"github.com/clintecker/muxwarp/completions"
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
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run is the real entry point. Returning an error instead of os.Exit ensures
// deferred cleanup functions always execute.
func run() error {
	args, cleanup, err := parseLogFlag()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	logging.Log().Info("muxwarp starting", "version", version, "commit", commit, "date", date)
	return runWithArgs(args)
}

// parseLogFlag extracts the --log flag, initialises the logger, and returns
// the remaining args plus the cleanup function.
func parseLogFlag() (args []string, cleanup func(), err error) {
	logPath, rest, logErr := extractLogFlag(os.Args[1:])
	if logErr != "" {
		return nil, nil, fmt.Errorf("%s", logErr)
	}
	fn, initErr := logging.Init(logPath)
	if initErr != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", initErr)
	}
	return rest, fn, nil
}

// runWithArgs dispatches flags or runs the TUI/direct-warp flow.
func runWithArgs(args []string) error {
	if dispatchFlag(args) {
		return nil
	}
	cfg, err := loadOrWizard(args)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // wizard cancelled
	}
	timeoutSec := parseTimeoutSec(cfg.Defaults.Timeout)
	if len(args) > 0 {
		directWarp(cfg, timeoutSec, args[0])
		return nil
	}
	tuiMode(cfg, timeoutSec)
	return nil
}

// flagHandlers maps top-level flag/command names to their handler functions.
// Each handler receives the remaining args after the flag itself.
var flagHandlers = map[string]func([]string){
	"--version":    func(_ []string) { printVersion() },
	"--help":       func(_ []string) { printUsage() },
	"-h":           func(_ []string) { printUsage() },
	"--completions": printCompletions,
	"init":          runInit,
}

// printVersion prints the build version line.
func printVersion() {
	fmt.Printf("muxwarp %s (commit %s, built %s)\n", version, commit, date)
}

// dispatchFlag handles flag-style arguments that exit early. Returns true if
// the argument was handled (--version, --help, --completions, init).
func dispatchFlag(args []string) bool {
	if len(args) == 0 {
		return false
	}
	handler, ok := flagHandlers[args[0]]
	if !ok {
		return false
	}
	handler(args[1:])
	return true
}

// loadOrWizard loads the config, running the first-run wizard when none exists.
// Returns (nil, nil) when the wizard is cancelled.
func loadOrWizard(args []string) (*config.Config, error) {
	cfg, err := config.Load(config.DefaultPath())
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%v\n\nExample config (%s):\n\n%s",
			err, config.DefaultPath(), config.ExampleConfig())
	}
	return runWizardFlow(args)
}

// runWizardFlow logs and runs the first-run wizard.
func runWizardFlow(_ []string) (*config.Config, error) {
	logging.Log().Info("no config file, running wizard", "path", config.DefaultPath())
	cfg := runWizard()
	if cfg == nil {
		return nil, nil
	}
	logging.Log().Info("wizard completed", "hosts", len(cfg.Hosts))
	return cfg, nil
}

// extractLogFlag pulls --log <path> or --log=<path> from args, returning
// the log path and the remaining args. If --log is present without a path,
// it returns an error message.
func extractLogFlag(args []string) (logPath string, rest []string, errMsg string) {
	for i := 0; i < len(args); i++ {
		lp, tail, e := matchLogArg(args, i)
		if e != "" {
			return "", nil, e
		}
		if lp != "" {
			return lp, tail, ""
		}
	}
	return "", args, ""
}

// matchLogArg checks whether args[i] is a --log flag in either form and
// returns the path value, the remaining args, and any error message.
func matchLogArg(args []string, i int) (logPath string, rest []string, errMsg string) {
	if args[i] == "--log" {
		return matchLogExact(args, i)
	}
	if strings.HasPrefix(args[i], "--log=") {
		return matchLogEquals(args, i)
	}
	return "", nil, ""
}

// matchLogExact handles the "--log <path>" form.
func matchLogExact(args []string, i int) (logPath string, rest []string, errMsg string) {
	if i+1 >= len(args) {
		return "", nil, "--log requires a file path argument"
	}
	rest = append(rest, args[:i]...)
	rest = append(rest, args[i+2:]...)
	return args[i+1], rest, ""
}

// matchLogEquals handles the "--log=<path>" form.
func matchLogEquals(args []string, i int) (logPath string, rest []string, errMsg string) {
	lp := strings.TrimPrefix(args[i], "--log=")
	if lp == "" {
		return "", nil, "--log requires a file path argument"
	}
	rest = append(rest, args[:i]...)
	rest = append(rest, args[i+1:]...)
	return lp, rest, ""
}

// reloadConfig loads the config from disk and updates timeoutSec.
func reloadConfig(timeoutSec string) (*config.Config, string, error) {
	newCfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return nil, timeoutSec, err
	}
	return newCfg, parseTimeoutSec(newCfg.Defaults.Timeout), nil
}

// tuiMode runs the interactive TUI in a loop. After warping into an ssh
// session and returning, the TUI restarts with a fresh scan.
func tuiMode(cfg *config.Config, timeoutSec string) {
	for {
		var done bool
		cfg, timeoutSec, done = tuiIteration(cfg, timeoutSec)
		if done {
			return
		}
	}
}

// tuiIteration runs one TUI cycle and returns updated cfg/timeout and whether
// the caller should stop looping.
func tuiIteration(cfg *config.Config, timeoutSec string) (*config.Config, string, bool) {
	target, termWidth, configChanged := runTUIOnce(cfg, timeoutSec)
	if configChanged {
		return applyConfigReload(cfg, timeoutSec)
	}
	if target == nil {
		return cfg, timeoutSec, true // user quit
	}
	warpChild(cfg, target, termWidth)
	return cfg, timeoutSec, false // loop: fresh rescan, new TUI
}

// applyConfigReload logs, reloads, and returns the new config and timeout.
// Returns the original values and done=true on error.
func applyConfigReload(cfg *config.Config, timeoutSec string) (*config.Config, string, bool) {
	logging.Log().Info("config changed, reloading")
	newCfg, newTimeout, err := reloadConfig(timeoutSec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
		return cfg, timeoutSec, true
	}
	return newCfg, newTimeout, false
}

// warpChild plays animations and runs ssh as a child process.
// For ghost sessions, ensure-repo + create + send-keys + attach happen
// in a single SSH connection.
func warpChild(cfg *config.Config, target *tui.Session, termWidth int) {
	logging.Log().Info("warp child", "host", target.Host, "session", target.Name, "is_ghost", target.IsGhost())

	tui.PlayWarpAnimation(target.HostShort, target.Name, termWidth)

	var err error
	if target.IsGhost() {
		d := target.Desired
		err = ssh.GhostWarpChild(target.Host, cfg.Defaults.Term, target.Name, d.Dir, d.Repo, d.Cmd)
	} else {
		err = ssh.ExecChild(target.Host, cfg.Defaults.Term, target.Name)
	}
	if err != nil {
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

// tagsForHost returns the tags configured for the given target host.
func tagsForHost(cfg *config.Config, target string) []string {
	for _, h := range cfg.Hosts {
		if h.Target == target {
			return h.Tags
		}
	}
	return nil
}

// scanAndSend injects ghost sessions immediately from config, then scans
// hosts in the background. As real sessions are found, matching ghosts are
// removed via PromoteGhostMsg.
func scanAndSend(ctx context.Context, cfg *config.Config, timeoutSec string, p *tea.Program) {
	// Inject all ghosts up front so the list is populated instantly.
	allGhosts := buildAllGhosts(cfg)
	if len(allGhosts) > 0 {
		logging.Log().Info("injecting ghosts", "count", len(allGhosts))
		p.Send(tui.SessionBatchMsg{Host: "ghosts", Sessions: allGhosts})
	}

	logging.Log().Info("scan starting", "hosts", len(cfg.Hosts))
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(host string, sessions []scanner.Session) {
		batch := scannerToTUI(sessions)
		tags := tagsForHost(cfg, host)
		for i := range batch {
			batch[i].Tags = tags
		}
		logging.Log().Debug("scan batch", "host", host, "sessions", len(batch))
		// Send real sessions; the TUI will promote matching ghosts.
		p.Send(tui.PromoteGhostMsg{Host: host, Sessions: batch})
	})

	logging.Log().Info("scan complete")
	p.Send(tui.ScanDoneMsg{})
}

// directWarp handles `muxwarp <pattern>` — scan, fuzzy match, then warp.
func directWarp(cfg *config.Config, timeoutSec, pattern string) {
	logging.Log().Info("direct warp", "pattern", pattern)
	fmt.Fprintf(os.Stderr, "Scanning gates...\n")

	allSessions := scanAllWithTags(cfg, timeoutSec)
	allSessions = append(allSessions, buildGhosts(cfg, allSessions)...)

	if len(allSessions) == 0 {
		fmt.Fprintf(os.Stderr, "No sessions found on any host.\n")
		os.Exit(1)
	}

	dispatchWarp(cfg, allSessions, pattern, timeoutSec)
}

// scanAllWithTags scans all hosts and attaches config tags to results.
func scanAllWithTags(cfg *config.Config, timeoutSec string) []tui.Session {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var allSessions []tui.Session
	_ = scanner.ScanAll(ctx, cfg.HostTargets(), 8, timeoutSec, func(host string, sessions []scanner.Session) {
		batch := scannerToTUI(sessions)
		tags := tagsForHost(cfg, host)
		for i := range batch {
			batch[i].Tags = tags
		}
		allSessions = append(allSessions, batch...)
	})
	return allSessions
}

// dispatchWarp fuzzy-matches and warps to the best match.
func dispatchWarp(cfg *config.Config, allSessions []tui.Session, pattern, timeoutSec string) {
	matches := fuzzyMatch(pattern, allSessions)

	switch len(matches) {
	case 0:
		fmt.Fprintf(os.Stderr, "No sessions matching %q\n", pattern)
		os.Exit(1)

	case 1:
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
// For ghost sessions, everything happens in a single SSH connection.
func warp(cfg *config.Config, target *tui.Session) {
	logging.Log().Info("warp exec-replace", "host", target.Host, "session", target.Name, "is_ghost", target.IsGhost())

	tui.PlayWarpAnimation(target.HostShort, target.Name, 80)

	var err error
	if target.IsGhost() {
		d := target.Desired
		err = ssh.GhostWarpReplace(target.Host, cfg.Defaults.Term, target.Name, d.Dir, d.Repo, d.Cmd)
	} else {
		err = ssh.ExecReplace(target.Host, cfg.Defaults.Term, target.Name)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssh error: %v\n", err)
		os.Exit(1)
	}
}

// buildAllGhosts creates ghost sessions for ALL desired sessions in the config.
// Called before scanning so the list is populated immediately.
func buildAllGhosts(cfg *config.Config) []tui.Session {
	var ghosts []tui.Session
	for _, h := range cfg.Hosts {
		for _, ds := range h.Sessions {
			ghosts = append(ghosts, newGhostSession(h.Target, ds))
		}
	}
	return ghosts
}

// buildGhosts creates ghost sessions for desired sessions not found in the scan.
// Used by directWarp where we scan first, then add missing ghosts.
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
		ghosts = append(ghosts, newGhostSession(h.Target, h.Tags, ds))
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

func newGhostSession(target string, tags []string, ds config.DesiredSession) tui.Session {
	return tui.Session{
		Host:      target,
		HostShort: ssh.HostShort(target),
		Name:      ds.Name,
		Tags:      tags,
		Desired:   &tui.DesiredInfo{Dir: ds.Dir, Repo: ds.Repo, Cmd: ds.Cmd},
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
			Host:         s.Host,
			HostShort:    s.HostShort,
			Name:         s.Name,
			Attached:     s.Attached,
			Windows:      s.Windows,
			Created:      s.Created,
			LastActivity: s.LastActivity,
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

func runInit(args []string) {
	force := len(args) > 0 && args[0] == "--force"
	cfgPath := config.DefaultPath()
	checkInitConflict(force, cfgPath)
	hosts := sshconfig.ParseHosts()
	checkInitHosts(hosts)
	cfg := config.GenerateFromSSHConfig(hosts)
	validateInitConfig(cfg)
	if err := config.Save(cfg, cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s with %d hosts from ~/.ssh/config\n", cfgPath, len(cfg.Hosts))
	fmt.Println("Run 'muxwarp' to start scanning. Press 'e' in the TUI to edit config.")
}

func checkInitConflict(force bool, cfgPath string) {
	if force {
		return
	}
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Fprintf(os.Stderr, "Config already exists: %s\nUse --force to overwrite.\n", cfgPath)
		os.Exit(1)
	}
}

func checkInitHosts(hosts []sshconfig.Host) {
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "No ~/.ssh/config found or no hosts defined.")
		fmt.Fprintln(os.Stderr, "Create ~/.muxwarp.config.yaml manually or use the TUI wizard: muxwarp")
		os.Exit(1)
	}
}

// validateInitConfig checks that the generated config has at least one host.
func validateInitConfig(cfg *config.Config) {
	if len(cfg.Hosts) == 0 {
		fmt.Fprintln(os.Stderr, "All SSH hosts were filtered (git hosting services).")
		fmt.Fprintln(os.Stderr, "Create ~/.muxwarp.config.yaml manually or use the TUI wizard: muxwarp")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`muxwarp %s — warp into tmux sessions on remote machines

Usage:
  muxwarp                          Launch interactive TUI
  muxwarp <pattern>                Fuzzy-match and warp directly
  muxwarp init [--force]           Generate config from ~/.ssh/config
  muxwarp --log <path>             Write debug logs to file
  muxwarp --completions <shell>    Output shell completions (bash, zsh, fish)
  muxwarp --version                Print version and exit
  muxwarp --help                   Show this help
`, version)
}

func printCompletions(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "--completions requires an argument: bash, zsh, or fish")
		os.Exit(1)
	}

	fileMap := map[string]string{
		"bash": "muxwarp.bash",
		"zsh":  "muxwarp.zsh",
		"fish": "muxwarp.fish",
	}

	filename, ok := fileMap[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown shell %q. Supported: bash, zsh, fish\n", args[0])
		os.Exit(1)
	}

	data, err := completions.Scripts.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading completion script: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(data))
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
