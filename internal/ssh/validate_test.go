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
		strings.Repeat("x", 256), // max length
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
		{"hello world", "space"},
		{"foo;bar", "semicolon"},
		{"foo`bar`", "backtick"},
		{"$(whoami)", "dollar paren"},
		{"foo|bar", "pipe"},
		{"foo&bar", "ampersand"},
		{"foo'bar", "single quote"},
		{`foo"bar`, "double quote"},
		{"foo\nbar", "newline"},
		{"foo\tbar", "tab"},
		{"foo:bar", "colon (tmux separator)"},
		{"foo/bar", "slash"},
		{"foo\\bar", "backslash"},
		{"foo bar", "embedded space"},
		{strings.Repeat("x", 257), "too long (257 chars)"},
		{"hello{world}", "curly braces"},
		{"test(name)", "parentheses"},
		{"<script>", "angle brackets"},
		{"foo=bar", "equals sign"},
		{"name#1", "hash"},
		{"100%done", "percent"},
		{"hello!", "exclamation"},
		{"@user", "at sign"},
		{"~home", "tilde"},
		{"a b", "internal space"},
	}

	for _, tc := range invalid {
		if ValidSessionName(tc.name) {
			t.Errorf("ValidSessionName(%q) [%s] = true, want false", tc.name, tc.desc)
		}
	}
}
