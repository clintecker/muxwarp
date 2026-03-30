package editor

import (
	"strings"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

// DropdownState manages the Ctrl+Space dropdown picker state.
type DropdownState struct {
	Active   bool
	items    []sshconfig.Host
	filtered []sshconfig.Host
	cursor   int
}

// NewDropdown creates a new dropdown state with the given SSH hosts.
func NewDropdown(hosts []sshconfig.Host) DropdownState {
	return DropdownState{
		items: hosts,
	}
}

// Open activates the dropdown with an optional filter.
func (d *DropdownState) Open(filter string) {
	d.Active = true
	d.filtered = filterHosts(d.items, filter)
	d.cursor = 0
}

// Close deactivates the dropdown.
func (d *DropdownState) Close() {
	d.Active = false
	d.filtered = nil
	d.cursor = 0
}

// Toggle opens or closes the dropdown based on current state.
func (d *DropdownState) Toggle(filter string) {
	if d.Active {
		d.Close()
	} else {
		d.Open(filter)
	}
}

// MoveUp moves the cursor up in the dropdown list.
func (d *DropdownState) MoveUp() {
	if d.cursor > 0 {
		d.cursor--
	}
}

// MoveDown moves the cursor down in the dropdown list.
func (d *DropdownState) MoveDown() {
	if len(d.filtered) > 0 && d.cursor < len(d.filtered)-1 {
		d.cursor++
	}
}

// Selected returns the currently selected host, if any.
func (d *DropdownState) Selected() (sshconfig.Host, bool) {
	if !d.Active || d.cursor >= len(d.filtered) {
		return sshconfig.Host{}, false
	}
	return d.filtered[d.cursor], true
}

// View renders the dropdown list.
// existingTargets is a list of already-configured host targets that should be marked as "(added)".
func (d DropdownState) View(existingTargets []string) string {
	if !d.Active || len(d.filtered) == 0 {
		return ""
	}

	existingSet := toStringSet(existingTargets)
	var b strings.Builder
	for i, host := range d.filtered {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(d.renderDropdownRow(i, host, existingSet))
	}
	return b.String()
}

func toStringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func (d DropdownState) renderDropdownRow(i int, host sshconfig.Host, existingSet map[string]bool) string {
	isSelected := i == d.cursor
	line := d.formatDropdownLine(host, isSelected, existingSet)
	if isSelected {
		return dropdownSelectedStyle.Render(line)
	}
	return sessionNormalStyle.Render(line)
}

func (d DropdownState) formatDropdownLine(host sshconfig.Host, selected bool, existingSet map[string]bool) string {
	prefix := "    "
	if selected {
		prefix = "  ▸ "
	}
	line := prefix + host.Alias
	line += aliasPadding(host.Alias)
	line += host.DisplayTarget()
	if existingSet[host.Alias] {
		line += " " + helperStyle.Render("(added)")
	}
	return line
}

func aliasPadding(alias string) string {
	if len(alias) < 14 {
		return strings.Repeat(" ", 14-len(alias))
	}
	return " "
}

// filterHosts performs case-insensitive substring matching against alias, hostname, and user.
// Returns all hosts if query is empty.
func filterHosts(hosts []sshconfig.Host, query string) []sshconfig.Host {
	if query == "" {
		return hosts
	}

	lower := strings.ToLower(query)
	var matches []sshconfig.Host
	for _, h := range hosts {
		if hostMatchesQuery(h, lower) {
			matches = append(matches, h)
		}
	}
	return matches
}

func hostMatchesQuery(h sshconfig.Host, lower string) bool {
	return strings.Contains(strings.ToLower(h.Alias), lower) ||
		strings.Contains(strings.ToLower(h.HostName), lower) ||
		strings.Contains(strings.ToLower(h.User), lower)
}

// renderHostMetadata returns the metadata preview line for the best matching SSH host.
// Format: "  atlas → alice@192.168.1.50:22"
// Returns empty string if no match found.
func renderHostMetadata(input string, sshHosts []sshconfig.Host) string {
	if input == "" {
		return ""
	}
	bestMatch := findBestHostMatch(input, sshHosts)
	if bestMatch == nil {
		return ""
	}
	line := "  " + bestMatch.Alias + " → " + bestMatch.DisplayTarget()
	return helperStyle.Render(line)
}

// findBestHostMatch returns the best prefix match for input, preferring exact matches.
func findBestHostMatch(input string, hosts []sshconfig.Host) *sshconfig.Host {
	var best *sshconfig.Host
	lower := strings.ToLower(input)
	for i, h := range hosts {
		if !strings.HasPrefix(strings.ToLower(h.Alias), lower) {
			continue
		}
		best = &hosts[i]
		if strings.EqualFold(h.Alias, input) {
			return best
		}
	}
	return best
}

// dropdownSelectedStyle renders the selected item in the dropdown.
var dropdownSelectedStyle = sessionSelectedStyle.Foreground(colorCyan)
