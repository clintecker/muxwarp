package tui

import (
	"fmt"
	"net"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/clintecker/muxwarp/internal/ssh"
)

const latencyTickInterval = 5 * time.Second

// latencyTickMsg triggers a new round of latency probes.
type latencyTickMsg struct{}

// LatencyMsg delivers latency measurements for hosts.
type LatencyMsg struct {
	Results map[string]time.Duration // host target -> round-trip (0 = unreachable)
}

// latencyTickCmd schedules the next latency tick.
func latencyTickCmd() tea.Cmd {
	return tea.Tick(latencyTickInterval, func(time.Time) tea.Msg {
		return latencyTickMsg{}
	})
}

// probeLatency measures TCP connect time to host:22.
func probeLatency(host string) time.Duration {
	dialHost := ssh.HostShort(host)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", dialHost+":22", 3*time.Second)
	if err != nil {
		return 0
	}
	d := time.Since(start)
	conn.Close()
	return d
}

// probeAllLatencies returns a tea.Cmd that probes all hosts concurrently.
func probeAllLatencies(hosts []string) tea.Cmd {
	return func() tea.Msg {
		results := make(map[string]time.Duration, len(hosts))
		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(len(hosts))
		for _, h := range hosts {
			go func(host string) {
				defer wg.Done()
				d := probeLatency(host)
				mu.Lock()
				results[host] = d
				mu.Unlock()
			}(h)
		}
		wg.Wait()
		return LatencyMsg{Results: results}
	}
}

// Latency display styles.
var (
	colorYellow = lipgloss.Color("#F1FA8C")

	latencyGoodStyle        = lipgloss.NewStyle().Foreground(colorGreen)
	latencyOkStyle          = lipgloss.NewStyle().Foreground(colorYellow)
	latencyBadStyle         = lipgloss.NewStyle().Foreground(colorRed)
	latencyUnreachableStyle = lipgloss.NewStyle().Foreground(colorSlate)
)

// renderLatencyTag renders the latency indicator for a session's host.
func (m Model) renderLatencyTag(s Session) string {
	if m.width < 60 {
		return ""
	}
	d, ok := m.latency[s.Host]
	if !ok {
		return ""
	}
	return formatLatency(d)
}

// formatLatency formats a duration as a styled latency tag.
func formatLatency(d time.Duration) string {
	if d == 0 {
		return latencyUnreachableStyle.Render(" --ms")
	}
	ms := d.Milliseconds()
	text := fmt.Sprintf(" %dms", ms)
	return latencyStyle(ms).Render(text)
}

// latencyStyle returns the appropriate style for a given latency in milliseconds.
func latencyStyle(ms int64) lipgloss.Style {
	if ms < 50 {
		return latencyGoodStyle
	}
	if ms < 150 {
		return latencyOkStyle
	}
	return latencyBadStyle
}
