// Package ssh manages SSH command construction and session name validation.
//
// Security note: session names come from remote tmux output and must never
// be shell-interpolated. All command construction uses separate argv elements.
package ssh

import "regexp"

// validSession matches tmux's default allowed session name characters.
// NO COLON — colon is tmux's session:window separator.
var validSession = regexp.MustCompile(`^[A-Za-z0-9._-]{1,256}$`)

// ValidSessionName reports whether name contains only safe characters
// for a tmux session name. Names are validated when parsing tmux
// list-sessions output to reject anything that could be dangerous
// if it somehow escaped into a shell context.
func ValidSessionName(name string) bool {
	return validSession.MatchString(name)
}
