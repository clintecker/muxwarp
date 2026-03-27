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
	var base string
	switch {
	case h.User != "" && h.HostName != "":
		base = h.User + "@" + h.HostName
	case h.HostName != "":
		base = h.HostName
	default:
		base = h.Alias
	}
	if h.Port != "" && h.Port != "22" {
		base += ":" + h.Port
	}
	return base
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
			// Flush the previous block before starting a new one.
			if current != nil && !isWildcard(current.Alias) {
				hosts = append(hosts, *current)
			}
			current = &Host{Alias: alias}
		}
	}

	// Flush the last block.
	if current != nil && !isWildcard(current.Alias) {
		hosts = append(hosts, *current)
	}

	if len(hosts) == 0 {
		return []Host{}
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

	fields := strings.SplitN(trimmed, " ", 2)
	if len(fields) < 2 {
		// Handle tab-separated values as well.
		fields = strings.SplitN(trimmed, "\t", 2)
		if len(fields) < 2 {
			return "", false
		}
	}

	keyword := strings.ToLower(fields[0])
	value := strings.TrimSpace(fields[1])

	switch keyword {
	case "host":
		return value, true
	case "hostname":
		if current != nil {
			current.HostName = value
		}
	case "user":
		if current != nil {
			current.User = value
		}
	case "port":
		if current != nil {
			current.Port = value
		}
	}
	return "", false
}

// isWildcard returns true if the alias contains wildcard characters.
func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?")
}
