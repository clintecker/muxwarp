// Package sshconfig parses ~/.ssh/config to extract host entries for autocomplete.
package sshconfig

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Host represents a single Host block from an SSH config file.
type Host struct {
	Alias    string
	HostName string
	User     string
	Port     string
}

// DisplayTarget builds a human-readable connection string like "alice@192.168.1.50:2222".
func (h Host) DisplayTarget() string {
	return h.baseTarget() + h.portSuffix()
}

// baseTarget returns the user@host or host or alias portion of the display string.
func (h Host) baseTarget() string {
	switch {
	case h.User != "" && h.HostName != "":
		return h.User + "@" + h.HostName
	case h.HostName != "":
		return h.HostName
	default:
		return h.Alias
	}
}

// portSuffix returns ":port" when the port is non-default, otherwise empty string.
func (h Host) portSuffix() string {
	if h.Port != "" && h.Port != "22" {
		return ":" + h.Port
	}
	return ""
}

// ParseHosts reads ~/.ssh/config and returns parsed host entries.
// Returns an empty slice on any error (missing file, permission denied, etc.).
func ParseHosts() []Host {
	home, err := os.UserHomeDir()
	if err != nil {
		return []Host{}
	}
	f, err := os.Open(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return []Host{}
	}
	defer f.Close()
	return parseHostsFrom(f)
}

// parseHostsFrom is the testable core that parses host blocks from a reader.
func parseHostsFrom(r io.Reader) []Host {
	var hosts []Host
	var current *Host

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		alias, finished := parseLine(scanner.Text(), current)
		if finished {
			hosts = flushHost(hosts, current)
			current = &Host{Alias: alias}
		}
	}

	hosts = flushHost(hosts, current)

	if len(hosts) == 0 {
		return []Host{}
	}
	return hosts
}

// flushHost appends current to hosts if it is non-nil and non-wildcard.
func flushHost(hosts []Host, current *Host) []Host {
	if current != nil && !isWildcard(current.Alias) {
		return append(hosts, *current)
	}
	return hosts
}

// parseLine processes a single line from an SSH config file.
// If the line is a Host directive, it returns the alias and finished=true to signal
// that the caller should flush the current block and start a new one.
func parseLine(line string, current *Host) (newAlias string, finished bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}

	keyword, value, ok := splitKeyValue(trimmed)
	if !ok {
		return "", false
	}

	if keyword == "host" {
		return value, true
	}
	applyField(current, keyword, value)
	return "", false
}

// applyField sets a field on current based on the keyword, if current is non-nil.
func applyField(current *Host, keyword, value string) {
	if current == nil {
		return
	}
	switch keyword {
	case "hostname":
		current.HostName = value
	case "user":
		current.User = value
	case "port":
		current.Port = value
	}
}

// splitKeyValue splits an SSH config line into keyword and value.
// It tries space separation first, then tab. Returns ok=false if neither works.
func splitKeyValue(s string) (keyword, value string, ok bool) {
	fields := strings.SplitN(s, " ", 2)
	if len(fields) < 2 {
		fields = strings.SplitN(s, "\t", 2)
		if len(fields) < 2 {
			return "", "", false
		}
	}
	return strings.ToLower(fields[0]), strings.TrimSpace(fields[1]), true
}

// isWildcard returns true if the alias contains wildcard characters.
func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?")
}
