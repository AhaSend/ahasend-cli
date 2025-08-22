// Package progress provides progress reporting and performance metrics for batch operations.
//
// This package implements intelligent progress reporting with:
//
//   - TTY detection for appropriate progress bar display
//   - Debug mode compatibility (text updates instead of progress bars)
//   - Real-time performance metrics (emails/second, success rate, ETA)
//   - Automatic fallback for non-interactive environments
//   - Performance statistics collection and reporting
//   - Integration with logging system for debug information
//
// The Reporter automatically adapts its output based on the environment,
// showing progress bars in interactive terminals and periodic text updates
// in non-interactive or debug modes.
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/AhaSend/ahasend-cli/internal/logger"
)

// Reporter handles progress reporting for batch operations
type Reporter struct {
	enabled    bool
	debugMode  bool
	total      int
	sent       int
	failed     int
	startTime  time.Time
	lastUpdate time.Time
	output     io.Writer
}

// Stats holds performance metrics
type Stats struct {
	Total        int           `json:"total"`
	Sent         int           `json:"sent"`
	Failed       int           `json:"failed"`
	SuccessRate  float64       `json:"success_rate"`
	Duration     time.Duration `json:"duration"`
	EmailsPerSec float64       `json:"emails_per_sec"`
}

// NewReporter creates a new progress reporter
func NewReporter(total int, showProgress, debugMode bool) *Reporter {
	enabled := showProgress && isTerminal() && !debugMode

	if showProgress && debugMode {
		logger.Debug("Progress bar disabled in debug mode, using periodic updates")
	}

	return &Reporter{
		enabled:   enabled,
		debugMode: debugMode,
		total:     total,
		startTime: time.Now(),
		output:    os.Stderr,
	}
}

// Start initializes the progress reporter
func (r *Reporter) Start() {
	if r.enabled {
		fmt.Fprintf(r.output, "Sending %d messages...\n", r.total)
	} else if r.debugMode {
		logger.Get().WithField("total_messages", r.total).Debug("Starting batch send operation")
	}
}

// Update reports progress for a single message result
func (r *Reporter) Update(success bool) {
	if success {
		r.sent++
	} else {
		r.failed++
	}

	now := time.Now()

	if r.enabled {
		// Update progress bar
		r.updateProgressBar(now)
	} else if r.debugMode {
		// Periodic debug updates (every 10 messages or every 5 seconds)
		completed := r.sent + r.failed
		if completed%10 == 0 || now.Sub(r.lastUpdate) > 5*time.Second {
			logger.Get().WithFields(map[string]interface{}{
				"completed":  completed,
				"total":      r.total,
				"sent":       r.sent,
				"failed":     r.failed,
				"percentage": int(float64(completed) / float64(r.total) * 100),
			}).Debug("Batch progress update")
			r.lastUpdate = now
		}
	}
}

// Finish completes the progress reporting and returns stats
func (r *Reporter) Finish() Stats {
	duration := time.Since(r.startTime)
	completed := r.sent + r.failed
	successRate := 0.0
	emailsPerSec := 0.0

	if completed > 0 {
		successRate = float64(r.sent) / float64(completed) * 100
	}

	if duration.Seconds() > 0 {
		emailsPerSec = float64(completed) / duration.Seconds()
	}

	stats := Stats{
		Total:        r.total,
		Sent:         r.sent,
		Failed:       r.failed,
		SuccessRate:  successRate,
		Duration:     duration,
		EmailsPerSec: emailsPerSec,
	}

	if r.enabled {
		// Clear progress bar and show final result
		r.clearProgressBar()
		if r.failed == 0 {
			fmt.Fprintf(r.output, "✓ Successfully sent %d/%d messages (%.1fs)\n",
				r.sent, r.total, duration.Seconds())
		} else {
			fmt.Fprintf(r.output, "⚠ Sent %d/%d messages (%d failed) (%.1fs)\n",
				r.sent, r.total, r.failed, duration.Seconds())
		}
	}

	return stats
}

// updateProgressBar renders the progress bar
func (r *Reporter) updateProgressBar(now time.Time) {
	// Only update every 100ms to avoid flicker
	if now.Sub(r.lastUpdate) < 100*time.Millisecond {
		return
	}
	r.lastUpdate = now

	completed := r.sent + r.failed
	percentage := float64(completed) / float64(r.total) * 100

	// Create progress bar (40 characters wide)
	barWidth := 40
	filled := int(percentage * float64(barWidth) / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Calculate ETA
	elapsed := now.Sub(r.startTime)
	var eta string
	if completed > 0 && completed < r.total {
		remaining := r.total - completed
		avgTimePerMessage := elapsed / time.Duration(completed)
		etaTime := avgTimePerMessage * time.Duration(remaining)
		eta = fmt.Sprintf(" ETA: %s", formatDuration(etaTime))
	}

	// Show current stats
	stats := ""
	if r.failed > 0 {
		stats = fmt.Sprintf(" (%d sent, %d failed)", r.sent, r.failed)
	}

	// Clear line and write progress
	fmt.Fprintf(r.output, "\r[%s] %.1f%% (%d/%d)%s%s",
		bar, percentage, completed, r.total, stats, eta)

	// Add newline if complete
	if completed >= r.total {
		fmt.Fprintf(r.output, "\n")
	}
}

// clearProgressBar clears the current progress bar line
func (r *Reporter) clearProgressBar() {
	fmt.Fprintf(r.output, "\r%s\r", strings.Repeat(" ", 80))
}

// isTerminal checks if we're running in an interactive terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// ShowMetrics displays performance metrics
func ShowMetrics(stats Stats, output io.Writer) {
	if output == nil {
		output = os.Stdout
	}

	fmt.Fprintf(output, "\n📊 Performance Metrics:\n")
	fmt.Fprintf(output, "   Total messages: %d\n", stats.Total)
	fmt.Fprintf(output, "   Successfully sent: %d\n", stats.Sent)
	if stats.Failed > 0 {
		fmt.Fprintf(output, "   Failed: %d\n", stats.Failed)
	}
	fmt.Fprintf(output, "   Success rate: %.1f%%\n", stats.SuccessRate)
	fmt.Fprintf(output, "   Duration: %s\n", formatDuration(stats.Duration))
	fmt.Fprintf(output, "   Performance: %.1f emails/sec\n", stats.EmailsPerSec)
}
