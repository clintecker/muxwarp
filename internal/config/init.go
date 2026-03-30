package config

import (
	"strings"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

var knownGitHosts = []string{
	"github.com",
	"gitlab.com",
	"bitbucket.org",
	"bitbucket.com",
	"ssh.dev.azure.com",
}

func GenerateFromSSHConfig(hosts []sshconfig.Host) *Config {
	cfg := &Config{
		Defaults: Defaults{
			Timeout: "3s",
			Term:    "xterm-256color",
		},
	}
	for _, h := range hosts {
		if isGitHost(h.Alias) {
			continue
		}
		cfg.Hosts = append(cfg.Hosts, HostEntry{Target: h.Alias})
	}
	return cfg
}

func isGitHost(alias string) bool {
	lower := strings.ToLower(alias)
	for _, known := range knownGitHosts {
		if lower == known {
			return true
		}
	}
	return false
}
