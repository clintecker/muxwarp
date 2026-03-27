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

	var b strings.Builder
	existingSet := make(map[string]bool)
	for _, t := range existingTargets {
		existingSet[t] = true
	}

	for i, host := range d.filtered {
		isSelected := i == d.cursor

		// Format: "  ▸ atlas        alice@192.168.1.50"
		prefix := "    "
		if isSelected {
			prefix = "  ▸ "
		}

		// Alias left-aligned, target right-aligned with spacing.
		alias := host.Alias
		target := host.DisplayTarget()

		// Add "(added)" tag if already configured.
		added := ""
		if existingSet[alias] {
			added = " " + helperStyle.Render("(added)")
		}

		line := prefix + alias
		// Pad alias to 14 characters for alignment.
		if len(alias) < 14 {
			line += strings.Repeat(" ", 14-len(alias))
		} else {
			line += " "
		}
		line += target + added

		if isSelected {
			b.WriteString(dropdownSelectedStyle.Render(line))
		} else {
			b.WriteString(sessionNormalStyle.Render(line))
		}

		if i < len(d.filtered)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
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
		if strings.Contains(strings.ToLower(h.Alias), lower) ||
			strings.Contains(strings.ToLower(h.HostName), lower) ||
			strings.Contains(strings.ToLower(h.User), lower) {
			matches = append(matches, h)
		}
	}

	return matches
}

// renderHostMetadata returns the metadata preview line for the best matching SSH host.
// Format: "  atlas → alice@192.168.1.50:22"
// Returns empty string if no match found.
func renderHostMetadata(input string, sshHosts []sshconfig.Host) string {
	if input == "" {
		return ""
	}

	// Find the best match (prefix match on alias).
	var bestMatch *sshconfig.Host
	for i, h := range sshHosts {
		if strings.HasPrefix(strings.ToLower(h.Alias), strings.ToLower(input)) {
			bestMatch = &sshHosts[i]
			// Prefer exact match.
			if strings.EqualFold(h.Alias, input) {
				break
			}
		}
	}

	if bestMatch == nil {
		return ""
	}

	line := "  " + bestMatch.Alias + " → " + bestMatch.DisplayTarget()
	return helperStyle.Render(line)
}

// dropdownSelectedStyle renders the selected item in the dropdown.
var dropdownSelectedStyle = sessionSelectedStyle.Foreground(colorCyan)
