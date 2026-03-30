package tui

import (
	"fmt"
	"time"
)

func formatAgeSince(ts int64, now time.Time) string {
	if ts == 0 {
		return ""
	}
	d := now.Sub(time.Unix(ts, 0))
	if d < 0 {
		d = 0
	}
	return formatDuration(d)
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return formatSubHour(d)
	}
	return formatHourPlus(d)
}

func formatSubHour(d time.Duration) string {
	if d < time.Minute {
		return "now"
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

func formatHourPlus(d time.Duration) string {
	if d < 14*24*time.Hour {
		return formatShortDays(d)
	}
	return formatLongDays(d)
}

func formatShortDays(d time.Duration) string {
	hours := d.Hours()
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(hours))
	}
	return fmt.Sprintf("%dd", int(hours/24))
}

func formatLongDays(d time.Duration) string {
	hours := d.Hours()
	switch {
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dw", int(hours/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(hours/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(hours/(24*365)))
	}
}
