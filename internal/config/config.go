// Package config handles loading and parsing of muxwarp configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level muxwarp configuration.
type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Hosts    []string `yaml:"hosts"`
}

// Defaults holds default settings applied to all SSH sessions.
type Defaults struct {
	Timeout string `yaml:"timeout"` // e.g. "3s", default "3s"
	Term    string `yaml:"term"`    // default "xterm-256color"
}

// Load reads a YAML config file from the given path, applies defaults, and validates it.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults fills in missing configuration fields with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.Defaults.Timeout == "" {
		cfg.Defaults.Timeout = "3s"
	}
	if cfg.Defaults.Term == "" {
		cfg.Defaults.Term = "xterm-256color"
	}
}

// validate checks that the configuration is usable.
func validate(cfg *Config) error {
	if len(cfg.Hosts) == 0 {
		return fmt.Errorf("no hosts configured")
	}
	return nil
}

// DefaultPath returns the default location for the muxwarp config file.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to HOME env var if UserHomeDir fails.
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".muxwarp.config.yaml")
}

// ExampleConfig returns an example YAML configuration string for friendly error messages.
func ExampleConfig() string {
	return `# ~/.muxwarp.config.yaml
defaults:
  timeout: "3s"
  term: "xterm-256color"
hosts:
  - server1
  - server2
  - server3
`
}
