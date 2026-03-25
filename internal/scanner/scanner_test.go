package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// withFakeSSH creates a fake ssh script in a temp directory and prepends
// it to PATH so that exec.LookPath("ssh") finds it first.
func withFakeSSH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755)
	if err != nil {
		t.Fatalf("writing fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestScanHost_Success(t *testing.T) {
	// Fake ssh prints two valid sessions in tmux list-sessions format.
	withFakeSSH(t, `#!/bin/sh
printf 'cjdos\t1\t3\n'
printf 'build\t0\t1\n'
`)

	ctx := context.Background()
	sessions, err := ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}

	// First session.
	s := sessions[0]
	if s.Host != "clint@indigo" {
		t.Errorf("Host = %q, want %q", s.Host, "clint@indigo")
	}
	if s.HostShort != "indigo" {
		t.Errorf("HostShort = %q, want %q", s.HostShort, "indigo")
	}
	if s.Name != "cjdos" {
		t.Errorf("Name = %q, want %q", s.Name, "cjdos")
	}
	if s.Attached != 1 {
		t.Errorf("Attached = %d, want 1", s.Attached)
	}
	if s.Windows != 3 {
		t.Errorf("Windows = %d, want 3", s.Windows)
	}

	// Second session.
	s = sessions[1]
	if s.Name != "build" {
		t.Errorf("Name = %q, want %q", s.Name, "build")
	}
	if s.Attached != 0 {
		t.Errorf("Attached = %d, want 0", s.Attached)
	}
	if s.Windows != 1 {
		t.Errorf("Windows = %d, want 1", s.Windows)
	}

	// Verify Key().
	if s.Key() != "clint@indigo/build" {
		t.Errorf("Key() = %q, want %q", s.Key(), "clint@indigo/build")
	}
}

func TestScanHost_NoServer(t *testing.T) {
	// Fake ssh exits with code 1 (simulating connection failure or no tmux server).
	withFakeSSH(t, `#!/bin/sh
exit 1
`)

	ctx := context.Background()
	sessions, err := ScanHost(ctx, "clint@indigo", "3")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %d sessions, want 0", len(sessions))
	}
}

func TestScanHost_Timeout(t *testing.T) {
	// Fake ssh sleeps for a long time.
	withFakeSSH(t, `#!/bin/sh
sleep 30
`)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	sessions, err := ScanHost(ctx, "clint@indigo", "1")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %d sessions, want 0 after timeout", len(sessions))
	}
}

func TestScanHost_InvalidSessionName(t *testing.T) {
	// Mix of valid and invalid session names.
	withFakeSSH(t, `#!/bin/sh
printf 'good-session\t1\t2\n'
printf 'evil;rm -rf /\t0\t1\n'
printf 'also_good\t0\t5\n'
printf 'has spaces\t1\t1\n'
`)

	ctx := context.Background()
	sessions, err := ScanHost(ctx, "user@host", "3")
	if err != nil {
		t.Fatalf("ScanHost returned error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2 (only valid names)", len(sessions))
	}
	if sessions[0].Name != "good-session" {
		t.Errorf("sessions[0].Name = %q, want %q", sessions[0].Name, "good-session")
	}
	if sessions[1].Name != "also_good" {
		t.Errorf("sessions[1].Name = %q, want %q", sessions[1].Name, "also_good")
	}
}

func TestScanAll(t *testing.T) {
	// Fake ssh returns different sessions based on the target argument.
	// The target is the last positional arg before "tmux".
	withFakeSSH(t, `#!/bin/sh
# Find the target host from the arguments.
target=""
for arg in "$@"; do
    case "$arg" in
        tmux) break ;;
        -o)  ;;
        ConnectTimeout=*|BatchMode=*) ;;
        *)   target="$arg" ;;
    esac
done

case "$target" in
    clint@alpha)
        printf 'dev\t1\t2\n'
        ;;
    clint@beta)
        printf 'staging\t0\t3\n'
        printf 'prod\t2\t5\n'
        ;;
    clint@gamma)
        # No sessions / tmux not running.
        exit 1
        ;;
esac
`)

	hosts := []string{"clint@alpha", "clint@beta", "clint@gamma"}

	var mu sync.Mutex
	collected := make(map[string][]Session)

	ctx := context.Background()
	err := ScanAll(ctx, hosts, 2, "3", func(host string, sessions []Session) {
		mu.Lock()
		defer mu.Unlock()
		collected[host] = sessions
	})
	if err != nil {
		t.Fatalf("ScanAll returned error: %v", err)
	}

	// alpha should have 1 session.
	if len(collected["clint@alpha"]) != 1 {
		t.Errorf("alpha: got %d sessions, want 1", len(collected["clint@alpha"]))
	} else if collected["clint@alpha"][0].Name != "dev" {
		t.Errorf("alpha session name = %q, want %q", collected["clint@alpha"][0].Name, "dev")
	}

	// beta should have 2 sessions.
	if len(collected["clint@beta"]) != 2 {
		t.Errorf("beta: got %d sessions, want 2", len(collected["clint@beta"]))
	}

	// gamma should NOT appear (exit 1 = no sessions = no callback).
	if _, ok := collected["clint@gamma"]; ok {
		t.Errorf("gamma: expected no callback, but got sessions: %v", collected["clint@gamma"])
	}
}
