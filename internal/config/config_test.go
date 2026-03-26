package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_Minimal tests loading a config with just hosts, verifying defaults are applied.
func TestLoad_Minimal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("hosts:\n  - server1\n  - server2\n")
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	t.Run("defaults", func(t *testing.T) {
		assertString(t, "timeout", cfg.Defaults.Timeout, "3s")
		assertString(t, "term", cfg.Defaults.Term, "xterm-256color")
	})

	t.Run("hosts", func(t *testing.T) {
		if len(cfg.Hosts) != 2 {
			t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
		}
		assertString(t, "host[0].Target", cfg.Hosts[0].Target, "server1")
		assertString(t, "host[1].Target", cfg.Hosts[1].Target, "server2")
	})
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

// TestLoad_WithDefaults tests that explicit defaults in the config override the built-in defaults.
func TestLoad_WithDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`defaults:
  timeout: "10s"
  term: "screen-256color"
hosts:
  - server1
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Defaults.Timeout != "10s" {
		t.Errorf("expected timeout %q, got %q", "10s", cfg.Defaults.Timeout)
	}
	if cfg.Defaults.Term != "screen-256color" {
		t.Errorf("expected term %q, got %q", "screen-256color", cfg.Defaults.Term)
	}
}

// TestLoad_MixedHosts tests that a config with both string and object hosts parses correctly.
func TestLoad_MixedHosts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - server1
  - target: clint@indigo
    sessions:
      - name: cjdos
        dir: ~/code/cjdos
        cmd: claude --dangerously-skip-permissions
      - name: tesseract
        dir: ~/code/tesseract
  - server3
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if len(cfg.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(cfg.Hosts))
	}

	assertString(t, "host[0].Target", cfg.Hosts[0].Target, "server1")
	assertString(t, "host[1].Target", cfg.Hosts[1].Target, "clint@indigo")
	assertString(t, "host[2].Target", cfg.Hosts[2].Target, "server3")

	if len(cfg.Hosts[1].Sessions) != 2 {
		t.Fatalf("expected 2 sessions for host[1], got %d", len(cfg.Hosts[1].Sessions))
	}

	assertString(t, "session[0].Name", cfg.Hosts[1].Sessions[0].Name, "cjdos")
	assertString(t, "session[0].Dir", cfg.Hosts[1].Sessions[0].Dir, "~/code/cjdos")
	assertString(t, "session[0].Cmd", cfg.Hosts[1].Sessions[0].Cmd, "claude --dangerously-skip-permissions")
	assertString(t, "session[1].Name", cfg.Hosts[1].Sessions[1].Name, "tesseract")
	assertString(t, "session[1].Dir", cfg.Hosts[1].Sessions[1].Dir, "~/code/tesseract")
	assertString(t, "session[1].Cmd", cfg.Hosts[1].Sessions[1].Cmd, "")
}

// TestLoad_HostTargets tests that HostTargets returns a flat string list.
func TestLoad_HostTargets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - server1
  - target: clint@indigo
    sessions:
      - name: dev
  - server3
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	targets := cfg.HostTargets()
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	assertString(t, "targets[0]", targets[0], "server1")
	assertString(t, "targets[1]", targets[1], "clint@indigo")
	assertString(t, "targets[2]", targets[2], "server3")
}

func loadDesiredSessionsConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - server1
  - target: clint@indigo
    sessions:
      - name: cjdos
        dir: ~/code/cjdos
      - name: tesseract
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	return cfg
}

func TestLoad_DesiredSessions_WithSessions(t *testing.T) {
	cfg := loadDesiredSessionsConfig(t)
	sessions := cfg.DesiredSessionsFor("clint@indigo")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 desired sessions, got %d", len(sessions))
	}
	assertString(t, "sessions[0].Name", sessions[0].Name, "cjdos")
	assertString(t, "sessions[1].Name", sessions[1].Name, "tesseract")
}

func TestLoad_DesiredSessions_NoSessions(t *testing.T) {
	cfg := loadDesiredSessionsConfig(t)
	sessions := cfg.DesiredSessionsFor("server1")
	if len(sessions) != 0 {
		t.Errorf("expected 0 desired sessions for server1, got %d", len(sessions))
	}
}

func TestLoad_DesiredSessions_UnknownHost(t *testing.T) {
	cfg := loadDesiredSessionsConfig(t)
	sessions := cfg.DesiredSessionsFor("unknown")
	if sessions != nil {
		t.Errorf("expected nil for unknown host, got %v", sessions)
	}
}

// TestLoad_InvalidSessionName tests that a config with an invalid session name returns an error.
func TestLoad_InvalidSessionName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - target: server1
    sessions:
      - name: "bad;name"
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for invalid session name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid session name") {
		t.Errorf("expected error containing %q, got %q", "invalid session name", err.Error())
	}
}

// TestLoad_MissingFile verifies that loading a non-existent file returns an error.
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

// TestLoad_EmptyHosts verifies that a config with no hosts returns an error.
func TestLoad_EmptyHosts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("hosts: []\n")
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for empty hosts, got nil")
	}
	if !strings.Contains(err.Error(), "no hosts configured") {
		t.Errorf("expected error containing %q, got %q", "no hosts configured", err.Error())
	}
}

// TestLoad_MalformedYAML verifies that malformed YAML returns a parse error.
func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("{{{{not valid yaml at all\n")
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for malformed YAML, got nil")
	}
}

// TestDefaultPath verifies the default config path.
func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, ".muxwarp.config.yaml")
	if p != expected {
		t.Errorf("expected %q, got %q", expected, p)
	}
}

// TestExampleConfig verifies that ExampleConfig returns a non-empty string containing expected content.
func TestExampleConfig(t *testing.T) {
	ex := ExampleConfig()
	if ex == "" {
		t.Fatal("ExampleConfig() returned empty string")
	}
	if !strings.Contains(ex, "hosts") {
		t.Error("ExampleConfig() should contain 'hosts'")
	}
	if !strings.Contains(ex, "defaults") {
		t.Error("ExampleConfig() should contain 'defaults'")
	}
}
