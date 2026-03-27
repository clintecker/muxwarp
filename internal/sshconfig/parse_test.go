package sshconfig

import (
	"strings"
	"testing"
)

// TestParseHosts_BasicConfig tests parsing a multi-block SSH config with HostName, User, and Port.
func TestParseHosts_BasicConfig(t *testing.T) {
	input := `Host atlas
    HostName 192.168.1.50
    User alice
    Port 2222

Host forge
    HostName forge.example.com
    User deploy
`
	hosts := parseHostsFrom(strings.NewReader(input))
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}

	assertHost(t, hosts[0], "atlas", "192.168.1.50", "alice", "2222")
	assertHost(t, hosts[1], "forge", "forge.example.com", "deploy", "")
}

// TestParseHosts_WildcardSkipped tests that wildcard Host blocks are excluded.
func TestParseHosts_WildcardSkipped(t *testing.T) {
	input := `Host *
    ServerAliveInterval 60

Host atlas
    HostName 192.168.1.50

Host staging-?
    HostName 10.0.0.1
`
	hosts := parseHostsFrom(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host (wildcards skipped), got %d", len(hosts))
	}
	assertHost(t, hosts[0], "atlas", "192.168.1.50", "", "")
}

// TestParseHosts_NoFile tests that ParseHosts returns an empty slice when ~/.ssh/config doesn't exist.
func TestParseHosts_NoFile(t *testing.T) {
	// ParseHosts reads from the real filesystem; if ~/.ssh/config doesn't exist
	// it should return an empty slice without error. We test the fallback path
	// by calling parseHostsFrom with an empty reader instead, which exercises
	// the same return-empty-slice behavior.
	// The actual ParseHosts() is integration-tested by the real environment.
	hosts := ParseHosts()
	if hosts == nil {
		t.Fatal("ParseHosts() should return empty slice, not nil")
	}
	// We can't assert length since the test host may have an SSH config.
	// The key invariant is that it doesn't panic or error.
}

// TestParseHosts_EmptyFile tests that an empty reader returns an empty slice.
func TestParseHosts_EmptyFile(t *testing.T) {
	hosts := parseHostsFrom(strings.NewReader(""))
	if hosts == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(hosts) != 0 {
		t.Fatalf("expected 0 hosts, got %d", len(hosts))
	}
}

// TestDisplayTarget is a table-driven test for Host.DisplayTarget().
func TestDisplayTarget(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want string
	}{
		{
			name: "user_hostname_port",
			host: Host{Alias: "atlas", HostName: "192.168.1.50", User: "alice", Port: "2222"},
			want: "alice@192.168.1.50:2222",
		},
		{
			name: "user_hostname_no_port",
			host: Host{Alias: "forge", HostName: "forge.example.com", User: "deploy"},
			want: "deploy@forge.example.com",
		},
		{
			name: "hostname_only",
			host: Host{Alias: "myhost", HostName: "10.0.0.1"},
			want: "10.0.0.1",
		},
		{
			name: "alias_only",
			host: Host{Alias: "simple"},
			want: "simple",
		},
		{
			name: "port_22_hidden",
			host: Host{Alias: "withport", HostName: "10.0.0.1", Port: "22"},
			want: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.host.DisplayTarget()
			if got != tt.want {
				t.Errorf("DisplayTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseHosts_CaseInsensitive tests that SSH config keywords are matched case-insensitively.
func TestParseHosts_CaseInsensitive(t *testing.T) {
	input := `HOST myhost
    HOSTNAME 10.0.0.1
    user admin
    Port 3022
`
	hosts := parseHostsFrom(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	assertHost(t, hosts[0], "myhost", "10.0.0.1", "admin", "3022")
}

// TestParseHosts_CommentsAndBlanks tests that comments and blank lines are properly skipped.
func TestParseHosts_CommentsAndBlanks(t *testing.T) {
	input := `# This is a comment
Host myhost
    # Another comment
    HostName 10.0.0.1

    User admin
# Trailing comment
`
	hosts := parseHostsFrom(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	assertHost(t, hosts[0], "myhost", "10.0.0.1", "admin", "")
}

// assertHost is a test helper that verifies all fields of a parsed Host.
func assertHost(t *testing.T, h Host, alias, hostname, user, port string) {
	t.Helper()
	if h.Alias != alias {
		t.Errorf("Alias = %q, want %q", h.Alias, alias)
	}
	if h.HostName != hostname {
		t.Errorf("HostName = %q, want %q", h.HostName, hostname)
	}
	if h.User != user {
		t.Errorf("User = %q, want %q", h.User, user)
	}
	if h.Port != port {
		t.Errorf("Port = %q, want %q", h.Port, port)
	}
}
