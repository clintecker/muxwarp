// Package config handles loading and parsing of muxwarp configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clintecker/muxwarp/internal/ssh"
	"gopkg.in/yaml.v3"
)

// DesiredSession describes a tmux session that should exist on a remote host.
type DesiredSession struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir,omitempty"`
	Cmd  string `yaml:"cmd,omitempty"`
}

// HostEntry represents a single host in the config. It may be specified as
// a plain string (backward compat) or as an object with optional desired sessions.
type HostEntry struct {
	Target   string           `yaml:"target"`
	Sessions []DesiredSession `yaml:"sessions,omitempty"`
}

// Config is the top-level muxwarp configuration.
type Config struct {
	Defaults Defaults    `yaml:"defaults"`
	Hosts    []HostEntry `yaml:"-"`
}

// rawConfig is used for initial YAML unmarshaling before custom host parsing.
type rawConfig struct {
	Defaults Defaults   `yaml:"defaults"`
	Hosts    []yaml.Node `yaml:"hosts"`
}

// Defaults holds default settings applied to all SSH sessions.
type Defaults struct {
	Timeout string `yaml:"timeout"` // e.g. "3s", default "3s"
	Term    string `yaml:"term"`    // default "xterm-256color"
}

// UnmarshalYAML implements custom unmarshaling for Config to handle mixed
// string/object host lists.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	var raw rawConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Defaults = raw.Defaults

	for i := range raw.Hosts {
		entry, err := decodeHostNode(&raw.Hosts[i])
		if err != nil {
			return fmt.Errorf("hosts[%d]: %w", i, err)
		}
		c.Hosts = append(c.Hosts, entry)
	}
	return nil
}

// decodeHostNode decodes a single YAML node as either a string or HostEntry object.
func decodeHostNode(node *yaml.Node) (HostEntry, error) {
	if node.Kind == yaml.ScalarNode {
		return decodeScalarHost(node)
	}
	if node.Kind == yaml.MappingNode {
		return decodeMappingHost(node)
	}
	return HostEntry{}, fmt.Errorf("expected string or object, got %v", node.Kind)
}

func decodeScalarHost(node *yaml.Node) (HostEntry, error) {
	var s string
	if err := node.Decode(&s); err != nil {
		return HostEntry{}, err
	}
	return HostEntry{Target: s}, nil
}

func decodeMappingHost(node *yaml.Node) (HostEntry, error) {
	var entry HostEntry
	if err := node.Decode(&entry); err != nil {
		return HostEntry{}, err
	}
	return entry, nil
}

// HostTargets returns a flat list of SSH targets (for the scanner).
func (c *Config) HostTargets() []string {
	targets := make([]string, len(c.Hosts))
	for i, h := range c.Hosts {
		targets[i] = h.Target
	}
	return targets
}

// DesiredSessionsFor returns the desired sessions configured for a given target.
func (c *Config) DesiredSessionsFor(target string) []DesiredSession {
	for _, h := range c.Hosts {
		if h.Target == target {
			return h.Sessions
		}
	}
	return nil
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
	if len(cfg.HostTargets()) == 0 {
		return fmt.Errorf("no hosts configured")
	}
	return validateDesiredSessions(cfg)
}

// validateDesiredSessions checks all desired session names are valid tmux names.
func validateDesiredSessions(cfg *Config) error {
	for _, h := range cfg.Hosts {
		if err := validateHostSessions(h); err != nil {
			return err
		}
	}
	return nil
}

func validateHostSessions(h HostEntry) error {
	for _, ds := range h.Sessions {
		if !ssh.ValidSessionName(ds.Name) {
			return fmt.Errorf("invalid session name %q for host %q", ds.Name, h.Target)
		}
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

// marshalableConfig is the structure used for YAML marshaling with a yaml.Node
// hosts list to support mixed scalar/mapping output.
type marshalableConfig struct {
	Defaults Defaults  `yaml:"defaults"`
	Hosts    yaml.Node `yaml:"hosts"`
}

// MarshalYAML implements custom marshaling for Config to produce mixed format:
// plain scalars for hosts without sessions, mapping nodes for hosts with sessions.
func (c Config) MarshalYAML() (interface{}, error) {
	hostsSeq := yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}

	for _, h := range c.Hosts {
		node, err := marshalHostEntry(h)
		if err != nil {
			return nil, err
		}
		hostsSeq.Content = append(hostsSeq.Content, &node)
	}

	return &marshalableConfig{
		Defaults: c.Defaults,
		Hosts:    hostsSeq,
	}, nil
}

// marshalHostEntry marshals a single HostEntry as either a scalar or mapping node.
func marshalHostEntry(h HostEntry) (yaml.Node, error) {
	if len(h.Sessions) == 0 {
		return marshalScalarHost(h.Target), nil
	}
	return marshalMappingHost(h)
}

// marshalScalarHost returns a yaml.ScalarNode with the target string.
func marshalScalarHost(target string) yaml.Node {
	return yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: target,
	}
}

// marshalMappingHost encodes a HostEntry with sessions as a yaml.MappingNode.
func marshalMappingHost(h HostEntry) (yaml.Node, error) {
	mapping := yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	// Add "target" key-value pair.
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "target"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: h.Target},
	)

	// Add "sessions" key and sequence value.
	sessionsSeq := yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}
	for _, ds := range h.Sessions {
		sessionMap := yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
		}
		// name (always present)
		sessionMap.Content = append(sessionMap.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Name},
		)
		// dir (optional)
		if ds.Dir != "" {
			sessionMap.Content = append(sessionMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "dir"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Dir},
			)
		}
		// cmd (optional)
		if ds.Cmd != "" {
			sessionMap.Content = append(sessionMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "cmd"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ds.Cmd},
			)
		}
		sessionsSeq.Content = append(sessionsSeq.Content, &sessionMap)
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "sessions"},
		&sessionsSeq,
	)

	return mapping, nil
}

// Save marshals the config to YAML and writes it atomically to the given path.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return writeAtomic(path, data)
}

// writeAtomic writes data to a temporary file in the same directory as path,
// then renames it over the target to ensure atomic replacement.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".muxwarp-config-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Clean up the temp file on any error path.
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if err := os.Chmod(tmpName, 0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("setting file permissions: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Rename succeeded; clear tmpName so defer doesn't remove the final file.
	tmpName = ""
	return nil
}

// ExampleConfig returns an example YAML configuration string for friendly error messages.
func ExampleConfig() string {
	return `# ~/.muxwarp.config.yaml
defaults:
  timeout: "3s"
  term: "xterm-256color"
hosts:
  - server1
  - target: user@server2
    sessions:
      - name: myproject
        dir: ~/code/myproject
  - server3
`
}
