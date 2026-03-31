package ssh

import (
	"strings"
	"testing"
)

func TestValidSessionName_Valid(t *testing.T) {
	t.Parallel()

	valid := []string{
		"cjdos",
		"build-farm",
		"my.session",
		"test_session",
		"a",
		"A",
		"0",
		"Session-With.Mixed_chars123",
		"bllooop[",                   // brackets are valid tmux names
		"test(name)",                 // parentheses
		"hello{world}",              // curly braces
		"name#1",                     // hash
		"@user",                      // at sign
		"~home",                      // tilde
		"100%done",                   // percent
		"hello!",                     // exclamation
		"foo=bar",                    // equals sign
		`foo"bar`,                    // double quote
		"foo'bar",                    // single quote
		"foo|bar",                    // pipe
		"foo&bar",                    // ampersand
		"<script>",                   // angle brackets
		strings.Repeat("x", 256),    // max length
	}

	for _, name := range valid {
		if !ValidSessionName(name) {
			t.Errorf("ValidSessionName(%q) = false, want true", name)
		}
	}
}

func TestValidSessionName_Invalid(t *testing.T) {
	t.Parallel()

	invalid := []struct {
		name string
		desc string
	}{
		{"", "empty string"},
		{"foo:bar", "colon (tmux separator)"},
		{"foo\nbar", "newline"},
		{"foo\tbar", "tab"},
		{"foo\x00bar", "null byte"},
		{strings.Repeat("x", 257), "too long (257 chars)"},
	}

	for _, tc := range invalid {
		if ValidSessionName(tc.name) {
			t.Errorf("ValidSessionName(%q) [%s] = true, want false", tc.name, tc.desc)
		}
	}
}

func TestValidRepoSlug_Valid(t *testing.T) {
	t.Parallel()

	valid := []string{
		"owner/repo",
		"my-org/my-repo",
		"user_name/repo_name",
		"org.name/repo.name",
		"A/B",
		"clintecker/muxwarp",
		"anthropics/claude-code",
		"foo-bar.baz/qux_123",
	}

	for _, slug := range valid {
		if !ValidRepoSlug(slug) {
			t.Errorf("ValidRepoSlug(%q) = false, want true", slug)
		}
	}
}

func TestValidRepoSlug_Invalid(t *testing.T) {
	t.Parallel()

	invalid := []struct {
		slug string
		desc string
	}{
		{"", "empty string"},
		{"noslash", "no slash"},
		{"a/b/c", "too many segments"},
		{"/repo", "empty owner"},
		{"owner/", "empty repo"},
		{"owner/ repo", "space in repo"},
		{"own er/repo", "space in owner"},
		{"https://github.com/owner/repo", "full URL"},
		{"../repo", "path traversal in owner"},
		{"owner/..", "path traversal in repo"},
	}

	for _, tc := range invalid {
		if ValidRepoSlug(tc.slug) {
			t.Errorf("ValidRepoSlug(%q) [%s] = true, want false", tc.slug, tc.desc)
		}
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "owner/repo"},
		{"ssh://git@github.com/owner/repo", "owner/repo"},
		{"ssh://git@github.com:22/owner/repo.git", "owner/repo"},
		{"owner/repo", "owner/repo"},
		{"  git@github.com:owner/repo.git  ", "owner/repo"},
		{"", ""},
		{"https://github.com/owner/repo/", "owner/repo"},
	}

	for _, tc := range tests {
		got := NormalizeRemoteURL(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
