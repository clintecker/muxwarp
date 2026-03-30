package config

import (
	"testing"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

func TestGenerateFromSSHConfig(t *testing.T) {
	hosts := []sshconfig.Host{
		{Alias: "indigo", HostName: "192.168.1.10", User: "clint"},
		{Alias: "atlas", HostName: "10.0.0.5"},
		{Alias: "github.com", HostName: "github.com", User: "git"},
		{Alias: "gitlab.com", HostName: "gitlab.com"},
		{Alias: "forge", HostName: "forge.local", User: "admin"},
	}
	cfg := GenerateFromSSHConfig(hosts)
	if cfg.Defaults.Timeout != "3s" {
		t.Errorf("timeout = %q, want %q", cfg.Defaults.Timeout, "3s")
	}
	if len(cfg.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(cfg.Hosts))
	}
	assertString(t, "hosts[0].Target", cfg.Hosts[0].Target, "indigo")
	assertString(t, "hosts[1].Target", cfg.Hosts[1].Target, "atlas")
	assertString(t, "hosts[2].Target", cfg.Hosts[2].Target, "forge")
}

func TestGenerateFromSSHConfig_Empty(t *testing.T) {
	cfg := GenerateFromSSHConfig(nil)
	if len(cfg.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(cfg.Hosts))
	}
}

func TestGenerateFromSSHConfig_AllFiltered(t *testing.T) {
	hosts := []sshconfig.Host{
		{Alias: "github.com"},
		{Alias: "gitlab.com"},
		{Alias: "bitbucket.org"},
	}
	cfg := GenerateFromSSHConfig(hosts)
	if len(cfg.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(cfg.Hosts))
	}
}

func TestIsGitHost(t *testing.T) {
	tests := []struct {
		alias string
		want  bool
	}{
		{"github.com", true},
		{"gitlab.com", true},
		{"bitbucket.org", true},
		{"bitbucket.com", true},
		{"ssh.dev.azure.com", true},
		{"my-git-server", true},
		{"indigo", false},
		{"atlas", false},
		{"forge", false},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got := isGitHost(tt.alias)
			if got != tt.want {
				t.Errorf("isGitHost(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}
