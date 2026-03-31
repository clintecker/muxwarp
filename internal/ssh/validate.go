// Package ssh manages SSH command construction and session name validation.
//
// Security note: session names come from remote tmux output and must never
// be shell-interpolated. All command construction uses separate argv elements.
package ssh

import (
	"regexp"
	"strings"
)

// validSession matches tmux session names: any printable non-control characters
// except colon (tmux's session:window:pane separator). Since muxwarp passes
// all args as direct argv elements with no shell interpolation, shell-special
// characters like brackets, quotes, and dollar signs are safe.
var validSession = regexp.MustCompile(`^[^\x00-\x1f\x7f:]{1,256}$`)

// validRepoSlug matches GitHub "owner/repo" format: alphanumeric, hyphens,
// underscores, and dots in each segment.
var validRepoSlug = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// ValidSessionName reports whether name is a valid tmux session name.
// Rejects empty strings, control characters, colons, and names over 256 chars.
func ValidSessionName(name string) bool {
	return validSession.MatchString(name)
}

// ValidRepoSlug reports whether s is a valid GitHub "owner/repo" slug.
// Rejects path traversal segments like ".." and ".".
func ValidRepoSlug(s string) bool {
	if !validRepoSlug.MatchString(s) {
		return false
	}
	parts := strings.SplitN(s, "/", 2)
	for _, p := range parts {
		if p == "." || p == ".." {
			return false
		}
	}
	return true
}

// NormalizeRemoteURL extracts the "owner/repo" slug from a git remote URL.
// Handles SSH, HTTPS, and scp-style URLs. Returns the input unchanged if
// it cannot be parsed.
func NormalizeRemoteURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// URL-style first: https://github.com/owner/repo or ssh://git@github.com/owner/repo
	// Must check before scp-style to avoid "ssh:" matching as scp-style.
	if i := strings.Index(url, "://"); i >= 0 {
		path := url[i+3:]
		if slash := strings.Index(path, "/"); slash >= 0 {
			path = path[slash+1:]
		}
		return lastTwoSegments(path)
	}

	// scp-style: git@github.com:owner/repo
	if i := strings.Index(url, ":"); i >= 0 && !strings.Contains(url[:i], "/") {
		path := url[i+1:]
		return lastTwoSegments(path)
	}

	return url
}

// lastTwoSegments returns the last two slash-separated segments of path.
func lastTwoSegments(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return path
}
