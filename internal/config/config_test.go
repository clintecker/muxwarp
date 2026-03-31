package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
      - name: "bad:name"
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

// TestLoad_WithRepo tests that a config with repo field parses correctly.
func TestLoad_WithRepo(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - target: clint@indigo
    sessions:
      - name: muxwarp
        dir: ~/code/muxwarp
        repo: clintecker/muxwarp
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	assertString(t, "session[0].Repo", cfg.Hosts[0].Sessions[0].Repo, "clintecker/muxwarp")
}

// TestLoad_RepoWithoutDir tests that repo without dir returns a validation error.
func TestLoad_RepoWithoutDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - target: clint@indigo
    sessions:
      - name: muxwarp
        repo: clintecker/muxwarp
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for repo without dir, got nil")
	}
	if !strings.Contains(err.Error(), "repo requires dir") {
		t.Errorf("expected error containing %q, got %q", "repo requires dir", err.Error())
	}
}

// TestLoad_InvalidRepoSlug tests that an invalid repo slug returns a validation error.
func TestLoad_InvalidRepoSlug(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`hosts:
  - target: clint@indigo
    sessions:
      - name: muxwarp
        dir: ~/code/muxwarp
        repo: not-a-valid-slug
`)
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for invalid repo slug, got nil")
	}
	if !strings.Contains(err.Error(), "invalid repo slug") {
		t.Errorf("expected error containing %q, got %q", "invalid repo slug", err.Error())
	}
}

// TestSave_RoundTrip_WithRepo verifies that the repo field survives a save/load round trip.
func TestSave_RoundTrip_WithRepo(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "roundtrip-repo.yaml")

	original := &Config{
		Defaults: Defaults{Timeout: "3s", Term: "xterm-256color"},
		Hosts: []HostEntry{
			{
				Target: "alice@forge",
				Sessions: []DesiredSession{
					{Name: "proj", Dir: "~/code/proj", Repo: "alice/proj"},
				},
			},
		},
	}

	if err := Save(original, cfgPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error after Save: %v", err)
	}

	assertString(t, "hosts[0].sessions[0].Repo", loaded.Hosts[0].Sessions[0].Repo, "alice/proj")
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

// --- Serialization tests ---

// TestSave_RoundTrip creates a Config with mixed hosts, saves it, loads it back,
// and verifies all fields survive the round trip.
func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "roundtrip.yaml")

	original := &Config{
		Defaults: Defaults{
			Timeout: "5s",
			Term:    "screen-256color",
		},
		Hosts: []HostEntry{
			{Target: "alice@atlas"},
			{
				Target: "alice@forge",
				Sessions: []DesiredSession{
					{Name: "api-server", Dir: "~/code/api"},
					{Name: "web-dev", Dir: "~/code/web", Cmd: "nvim"},
				},
			},
			{Target: "bob@neptune"},
		},
	}

	if err := Save(original, cfgPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error after Save: %v", err)
	}

	// Verify defaults.
	assertString(t, "defaults.timeout", loaded.Defaults.Timeout, "5s")
	assertString(t, "defaults.term", loaded.Defaults.Term, "screen-256color")

	// Verify hosts count.
	if len(loaded.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(loaded.Hosts))
	}

	// Verify plain hosts.
	assertString(t, "hosts[0].Target", loaded.Hosts[0].Target, "alice@atlas")
	if len(loaded.Hosts[0].Sessions) != 0 {
		t.Errorf("expected 0 sessions for hosts[0], got %d", len(loaded.Hosts[0].Sessions))
	}

	// Verify mapping host with sessions.
	assertString(t, "hosts[1].Target", loaded.Hosts[1].Target, "alice@forge")
	if len(loaded.Hosts[1].Sessions) != 2 {
		t.Fatalf("expected 2 sessions for hosts[1], got %d", len(loaded.Hosts[1].Sessions))
	}
	assertString(t, "hosts[1].sessions[0].Name", loaded.Hosts[1].Sessions[0].Name, "api-server")
	assertString(t, "hosts[1].sessions[0].Dir", loaded.Hosts[1].Sessions[0].Dir, "~/code/api")
	assertString(t, "hosts[1].sessions[0].Cmd", loaded.Hosts[1].Sessions[0].Cmd, "")
	assertString(t, "hosts[1].sessions[1].Name", loaded.Hosts[1].Sessions[1].Name, "web-dev")
	assertString(t, "hosts[1].sessions[1].Dir", loaded.Hosts[1].Sessions[1].Dir, "~/code/web")
	assertString(t, "hosts[1].sessions[1].Cmd", loaded.Hosts[1].Sessions[1].Cmd, "nvim")

	// Verify trailing plain host.
	assertString(t, "hosts[2].Target", loaded.Hosts[2].Target, "bob@neptune")
	if len(loaded.Hosts[2].Sessions) != 0 {
		t.Errorf("expected 0 sessions for hosts[2], got %d", len(loaded.Hosts[2].Sessions))
	}
}

// TestMarshalYAML_MixedHosts verifies the raw YAML output contains both plain
// scalar strings and mapping objects for hosts with sessions.
func TestMarshalYAML_MixedHosts(t *testing.T) {
	cfg := Config{
		Defaults: Defaults{Timeout: "3s", Term: "xterm-256color"},
		Hosts: []HostEntry{
			{Target: "alice@atlas"},
			{
				Target: "alice@forge",
				Sessions: []DesiredSession{
					{Name: "api-server", Dir: "~/code/api"},
				},
			},
		},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	raw := string(data)

	// The plain host should appear as a bare scalar in the YAML list.
	if !strings.Contains(raw, "- alice@atlas") {
		t.Errorf("expected plain scalar '- alice@atlas' in output:\n%s", raw)
	}

	// The mapping host should have a target key.
	if !strings.Contains(raw, "target: alice@forge") {
		t.Errorf("expected 'target: alice@forge' in output:\n%s", raw)
	}

	// Sessions should appear under the mapping host.
	if !strings.Contains(raw, "name: api-server") {
		t.Errorf("expected 'name: api-server' in output:\n%s", raw)
	}
	if !strings.Contains(raw, "dir: ~/code/api") {
		t.Errorf("expected 'dir: ~/code/api' in output:\n%s", raw)
	}
}

// TestMarshalYAML_PlainOnly verifies that a config with only plain hosts
// (no sessions) produces a simple YAML list of strings.
func TestMarshalYAML_PlainOnly(t *testing.T) {
	cfg := Config{
		Defaults: Defaults{Timeout: "3s", Term: "xterm-256color"},
		Hosts: []HostEntry{
			{Target: "server1"},
			{Target: "server2"},
			{Target: "server3"},
		},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	raw := string(data)

	// All hosts should be plain scalars.
	if !strings.Contains(raw, "- server1") {
		t.Errorf("expected '- server1' in output:\n%s", raw)
	}
	if !strings.Contains(raw, "- server2") {
		t.Errorf("expected '- server2' in output:\n%s", raw)
	}
	if !strings.Contains(raw, "- server3") {
		t.Errorf("expected '- server3' in output:\n%s", raw)
	}

	// Should not contain "target:" since no hosts have sessions.
	if strings.Contains(raw, "target:") {
		t.Errorf("expected no 'target:' key for plain-only hosts, but found one:\n%s", raw)
	}
}

// TestSave_CreatesNewFile verifies that Save creates a new file at a path
// that doesn't exist yet, and the file can be loaded back successfully.
func TestSave_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "new-config.yaml")

	// Create the parent directory so writeAtomic can place the temp file.
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Defaults: Defaults{Timeout: "3s", Term: "xterm-256color"},
		Hosts: []HostEntry{
			{Target: "server1"},
		},
	}

	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the file was created.
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Verify it can be loaded back.
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error after Save: %v", err)
	}
	assertString(t, "hosts[0].Target", loaded.Hosts[0].Target, "server1")
}
