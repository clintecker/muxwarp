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

	// Verify defaults were applied
	if cfg.Defaults.Timeout != "3s" {
		t.Errorf("expected default timeout %q, got %q", "3s", cfg.Defaults.Timeout)
	}
	if cfg.Defaults.Term != "xterm-256color" {
		t.Errorf("expected default term %q, got %q", "xterm-256color", cfg.Defaults.Term)
	}

	// Verify hosts were loaded
	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0] != "server1" {
		t.Errorf("expected host[0] %q, got %q", "server1", cfg.Hosts[0])
	}
	if cfg.Hosts[1] != "server2" {
		t.Errorf("expected host[1] %q, got %q", "server2", cfg.Hosts[1])
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
