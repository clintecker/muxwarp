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
	err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755) //nolint:gosec // fake ssh must be executable
	if err != nil {
		t.Fatalf("writing fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertSessionStrings(t *testing.T, got Session, wantHost, wantHostShort, wantName string) {
	t.Helper()
	if got.Host != wantHost {
		t.Errorf("Host = %q, want %q", got.Host, wantHost)
	}
	if got.HostShort != wantHostShort {
		t.Errorf("HostShort = %q, want %q", got.HostShort, wantHostShort)
	}
	if got.Name != wantName {
		t.Errorf("Name = %q, want %q", got.Name, wantName)
	}
}

func assertSessionInts(t *testing.T, got Session, wantAttached, wantWindows int) {
	t.Helper()
	if got.Attached != wantAttached {
		t.Errorf("Attached = %d, want %d", got.Attached, wantAttached)
	}
	if got.Windows != wantWindows {
		t.Errorf("Windows = %d, want %d", got.Windows, wantWindows)
	}
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

	t.Run("first_session", func(t *testing.T) {
		assertSessionStrings(t, sessions[0], "clint@indigo", "indigo", "cjdos")
		assertSessionInts(t, sessions[0], 1, 3)
	})

	t.Run("second_session", func(t *testing.T) {
		assertSessionStrings(t, sessions[1], "clint@indigo", "indigo", "build")
		assertSessionInts(t, sessions[1], 0, 1)
	})

	t.Run("key", func(t *testing.T) {
		if sessions[1].Key() != "clint@indigo/build" {
			t.Errorf("Key() = %q, want %q", sessions[1].Key(), "clint@indigo/build")
		}
	})
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
	// Colons and control characters are rejected; most other characters are valid.
	withFakeSSH(t, `#!/bin/sh
printf 'good-session\t1\t2\n'
printf 'bad:name\t0\t1\n'
printf 'also_good\t0\t5\n'
printf 'bad\x01ctrl\t1\t1\n'
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

	t.Run("alpha_has_one_session", func(t *testing.T) {
		assertHostCount(t, collected, "clint@alpha", 1)
	})

	t.Run("beta_has_two_sessions", func(t *testing.T) {
		assertHostCount(t, collected, "clint@beta", 2)
	})

	t.Run("gamma_not_called", func(t *testing.T) {
		if _, ok := collected["clint@gamma"]; ok {
			t.Errorf("gamma: expected no callback, but got sessions: %v", collected["clint@gamma"])
		}
	})
}

func assertHostCount(t *testing.T, collected map[string][]Session, host string, want int) {
	t.Helper()
	if len(collected[host]) != want {
		t.Errorf("%s: got %d sessions, want %d", host, len(collected[host]), want)
	}
}
