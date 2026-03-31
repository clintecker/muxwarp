package tui

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ts   int64
		want string
	}{
		{"now", now.Unix(), "now"},
		{"30s_ago", now.Add(-30 * time.Second).Unix(), "now"},
		{"5m_ago", now.Add(-5 * time.Minute).Unix(), "5m"},
		{"59m_ago", now.Add(-59 * time.Minute).Unix(), "59m"},
		{"2h_ago", now.Add(-2 * time.Hour).Unix(), "2h"},
		{"23h_ago", now.Add(-23 * time.Hour).Unix(), "23h"},
		{"3d_ago", now.Add(-3 * 24 * time.Hour).Unix(), "3d"},
		{"13d_ago", now.Add(-13 * 24 * time.Hour).Unix(), "13d"},
		{"2w_ago", now.Add(-14 * 24 * time.Hour).Unix(), "2w"},
		{"8w_ago", now.Add(-59 * 24 * time.Hour).Unix(), "8w"},
		{"3mo_ago", now.Add(-90 * 24 * time.Hour).Unix(), "3mo"},
		{"11mo_ago", now.Add(-340 * 24 * time.Hour).Unix(), "11mo"},
		{"1y_ago", now.Add(-366 * 24 * time.Hour).Unix(), "1y"},
		{"zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAgeSince(tt.ts, now)
			if got != tt.want {
				t.Errorf("formatAgeSince(%d) = %q, want %q", tt.ts, got, tt.want)
			}
		})
	}
}
