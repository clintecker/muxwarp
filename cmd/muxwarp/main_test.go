package main

import (
	"testing"
)

func TestExtractLogFlag_NoFlag(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag([]string{"--version"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "" {
		t.Errorf("logPath = %q, want empty", logPath)
	}
	assertStringsEqual(t, rest, []string{"--version"})
}

func TestExtractLogFlag_SpaceForm(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag([]string{"--log", "/tmp/mux.log"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "/tmp/mux.log" {
		t.Errorf("logPath = %q, want %q", logPath, "/tmp/mux.log")
	}
	assertStringsEqual(t, rest, nil)
}

func TestExtractLogFlag_EqualsForm(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag([]string{"--log=/tmp/mux.log"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "/tmp/mux.log" {
		t.Errorf("logPath = %q, want %q", logPath, "/tmp/mux.log")
	}
	assertStringsEqual(t, rest, nil)
}

func TestExtractLogFlag_MiddleOfArgs(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag([]string{"--log", "/tmp/mux.log", "devbox"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "/tmp/mux.log" {
		t.Errorf("logPath = %q, want %q", logPath, "/tmp/mux.log")
	}
	assertStringsEqual(t, rest, []string{"devbox"})
}

func TestExtractLogFlag_EqualsMiddleOfArgs(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag([]string{"--log=/tmp/mux.log", "devbox"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "/tmp/mux.log" {
		t.Errorf("logPath = %q, want %q", logPath, "/tmp/mux.log")
	}
	assertStringsEqual(t, rest, []string{"devbox"})
}

func TestExtractLogFlag_MissingValue(t *testing.T) {
	t.Parallel()
	_, _, errMsg := extractLogFlag([]string{"--log"})
	if errMsg == "" {
		t.Fatal("expected error for --log without value")
	}
}

func TestExtractLogFlag_EmptyEqualsValue(t *testing.T) {
	t.Parallel()
	_, _, errMsg := extractLogFlag([]string{"--log="})
	if errMsg == "" {
		t.Fatal("expected error for --log= without value")
	}
}

func TestExtractLogFlag_EmptyArgs(t *testing.T) {
	t.Parallel()
	logPath, rest, errMsg := extractLogFlag(nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if logPath != "" {
		t.Errorf("logPath = %q, want empty", logPath)
	}
	if len(rest) != 0 {
		t.Errorf("rest = %v, want empty", rest)
	}
}

func assertStringsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
