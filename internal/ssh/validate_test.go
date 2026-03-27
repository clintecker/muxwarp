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
