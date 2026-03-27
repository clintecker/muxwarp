// Package ssh manages SSH command construction and session name validation.
//
// Security note: session names come from remote tmux output and must never
// be shell-interpolated. All command construction uses separate argv elements.
package ssh

import "regexp"

// validSession matches tmux session names: any printable non-control characters
// except colon (tmux's session:window:pane separator). Since muxwarp passes
// all args as direct argv elements with no shell interpolation, shell-special
// characters like brackets, quotes, and dollar signs are safe.
var validSession = regexp.MustCompile(`^[^\x00-\x1f\x7f:]{1,256}$`)

// ValidSessionName reports whether name is a valid tmux session name.
// Rejects empty strings, control characters, colons, and names over 256 chars.
func ValidSessionName(name string) bool {
	return validSession.MatchString(name)
}
