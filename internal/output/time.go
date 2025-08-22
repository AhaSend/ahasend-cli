package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// FormatTimeLocal formats a time in the local timezone for display
func FormatTimeLocal(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	// Convert to local timezone for display
	localTime := t.In(time.Local)
	return localTime.Format("2006-01-02 15:04:05")
}

// FormatTimeLocalValue formats a time value (not pointer) in the local timezone for display
func FormatTimeLocalValue(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	// Convert to local timezone for display
	localTime := t.In(time.Local)
	return localTime.Format("2006-01-02 15:04:05")
}

// ParseTimePast parses a time string that can be RFC3339 or relative time in the past
// (e.g., "24h" means 24 hours ago)
func ParseTimePast(input string) (time.Time, error) {
	// Try parsing as RFC3339 first
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t, nil
	}
	// Try parsing as relative time
	input = strings.ToLower(strings.TrimSpace(input))
	now := time.Now()
	// Parse relative time formats like "1h", "24h", "7d", "30d"
	if strings.HasSuffix(input, "h") {
		hours := strings.TrimSuffix(input, "h")
		var h int
		if _, err := fmt.Sscanf(hours, "%d", &h); err == nil {
			return now.Add(-time.Duration(h) * time.Hour), nil
		}
	}
	if strings.HasSuffix(input, "d") {
		days := strings.TrimSuffix(input, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err == nil {
			return now.AddDate(0, 0, -d), nil
		}
	}
	if strings.HasSuffix(input, "m") {
		minutes := strings.TrimSuffix(input, "m")
		var m int
		if _, err := fmt.Sscanf(minutes, "%d", &m); err == nil {
			return now.Add(-time.Duration(m) * time.Minute), nil
		}
	}
	return time.Time{}, errors.NewValidationError(fmt.Sprintf("invalid time format: %s (use RFC3339 or relative like '24h', '7d')", input), nil)
}

// ParseTimeFuture parses a time string that can be RFC3339 or relative time in the future
// (e.g., "30d" means 30 days from now)
func ParseTimeFuture(input string) (time.Time, error) {
	// Try parsing as RFC3339 first
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t, nil
	}

	// Try parsing as relative time (future)
	input = strings.ToLower(strings.TrimSpace(input))
	now := time.Now()

	// Parse relative time formats like "30m", "24h", "7d", "1w", "1mo", "1y"
	var value int
	var unit string
	if _, err := fmt.Sscanf(input, "%d%s", &value, &unit); err != nil {
		return time.Time{}, errors.NewValidationError(fmt.Sprintf("invalid format: %s (use RFC3339 or relative like '30d', '24h')", input), nil)
	}

	if value <= 0 {
		return time.Time{}, errors.NewValidationError("expiration value must be positive", nil)
	}

	switch unit {
	case "m", "min", "mins", "minute", "minutes":
		return now.Add(time.Duration(value) * time.Minute), nil
	case "h", "hr", "hrs", "hour", "hours":
		return now.Add(time.Duration(value) * time.Hour), nil
	case "d", "day", "days":
		return now.AddDate(0, 0, value), nil
	case "w", "week", "weeks":
		return now.AddDate(0, 0, value*7), nil
	case "mo", "month", "months":
		return now.AddDate(0, value, 0), nil
	case "y", "year", "years":
		return now.AddDate(value, 0, 0), nil
	default:
		return time.Time{}, errors.NewValidationError(fmt.Sprintf("unknown time unit '%s', supported: m(inutes), h(ours), d(ays), w(eeks), mo(nths), y(ears)", unit), nil)
	}
}
