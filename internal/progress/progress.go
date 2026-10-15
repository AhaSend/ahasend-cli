// Package progress reports the progress of messages send and its
// performance metrics.
//
// A Reporter draws a progress bar, with the counts and an ETA, on stderr
// when progress was asked for, stderr is a terminal that acts on escape
// sequences and debug mode is off. In debug mode it logs each update as a
// debug line instead; otherwise it shows nothing. While the bar is drawn,
// everything else written to stderr goes through Writer, which moves the
// bar below it. Finish returns the send's Stats, which ShowMetrics prints.
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/terminal"
)

// Reporter handles progress reporting for batch operations. Its methods
// may be called from several goroutines.
type Reporter struct {
	// mu guards the counts and the output.
	mu        sync.Mutex
	enabled   bool
	debugMode bool
	total     int
	sent      int
	failed    int
	unknown   int
	startTime time.Time
	output    io.Writer
	// width is how many columns the terminal has, which the progress line
	// fits in, or 0 for no limit.
	width int
	// bar is the progress line on the screen, which no newline has ended
	// yet, or "".
	bar string
}

// Stats holds performance metrics. Unknown counts the recipients whose
// outcome is unknown: they may have been sent. SuccessRate is the share of
// the recipients with a result, sent, failed or unknown, that were sent.
type Stats struct {
	Total        int
	Sent         int
	Failed       int
	Unknown      int
	SuccessRate  float64
	Duration     time.Duration
	EmailsPerSec float64
}

// NewReporter creates a progress reporter that draws on output, stderr.
func NewReporter(total int, showProgress, debugMode bool, output io.Writer) *Reporter {
	enabled := showProgress && isTerminal(output) && !debugMode

	if showProgress && debugMode {
		logger.Debug("Progress bar disabled in debug mode, using periodic updates")
	}

	reporter := &Reporter{
		enabled:   enabled,
		debugMode: debugMode,
		total:     total,
		startTime: time.Now(),
		output:    output,
	}
	if enabled {
		reporter.width = terminalWidth(output)
	}
	return reporter
}

// Writer returns the writer of w, the output the reporter draws on, for
// everything else written to it while the bar may be drawn, such as notices
// and logs. Each write clears the bar's line first and draws the bar again
// below it, so neither runs into the other. Without a bar, it is w.
func (r *Reporter) Writer(w io.Writer) io.Writer {
	if !r.enabled {
		return w
	}
	return barWriter{reporter: r, w: w}
}

// barWriter writes to w around the bar of reporter, as Writer describes.
type barWriter struct {
	reporter *Reporter
	w        io.Writer
}

func (b barWriter) Write(p []byte) (int, error) {
	r := b.reporter
	r.mu.Lock()
	defer r.mu.Unlock()
	bar := r.bar
	if bar != "" {
		r.print(clearLine)
	}
	n, err := b.w.Write(p)
	if bar != "" {
		r.print(clearLine + bar)
	}
	return n, err
}

// Start initializes the progress reporter
func (r *Reporter) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.enabled {
		r.print(fmt.Sprintf("Sending %d messages...\n", r.total))
	} else if r.debugMode {
		logger.Get().WithField("total_messages", r.total).Debug("Starting batch send operation")
	}
}

// Record reports the result of a finished send job: how many of its
// recipients were sent, how many failed and how many are of unknown outcome.
// The progress bar always shows the latest count.
func (r *Reporter) Record(sent, failed, unknown int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent += sent
	r.failed += failed
	r.unknown += unknown

	if r.enabled {
		r.updateProgressBar(time.Now())
	} else if r.debugMode {
		completed := r.completed()
		logger.Get().WithFields(map[string]interface{}{
			"completed":  completed,
			"total":      r.total,
			"sent":       r.sent,
			"failed":     r.failed,
			"unknown":    r.unknown,
			"percentage": int(float64(completed) / float64(r.total) * 100),
		}).Debug("Batch progress update")
	}
}

// completed is how many recipients have a result: sent, failed or unknown.
func (r *Reporter) completed() int {
	return r.sent + r.failed + r.unknown
}

// counts lists the failed and unknown counts that are not zero, such as
// ", 2 failed, 1 unknown".
func (r *Reporter) counts() string {
	counts := ""
	if r.failed > 0 {
		counts += fmt.Sprintf(", %d failed", r.failed)
	}
	if r.unknown > 0 {
		counts += fmt.Sprintf(", %d unknown", r.unknown)
	}
	return counts
}

// Finish completes the progress reporting and returns stats
func (r *Reporter) Finish() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	duration := time.Since(r.startTime)
	completed := r.completed()
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
		Unknown:      r.unknown,
		SuccessRate:  successRate,
		Duration:     duration,
		EmailsPerSec: emailsPerSec,
	}

	if r.enabled {
		// Clear progress bar and show final result
		r.clearProgressBar()
		r.print(fmt.Sprintf("Sent %d/%d messages%s (%s)\n",
			r.sent, r.total, r.counts(), formatElapsed(duration)))
	}

	return stats
}

// updateProgressBar renders the progress bar
func (r *Reporter) updateProgressBar(now time.Time) {
	completed := r.completed()
	percentage := float64(completed) / float64(r.total) * 100

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
	if counts := r.counts(); counts != "" {
		stats = fmt.Sprintf(" (%d sent%s)", r.sent, counts)
	}

	// Clear the line, which a longer bar may have filled, and write progress
	r.bar = progressLine(r.width, percentage, completed, r.total, stats, eta)
	r.print(clearLine + r.bar)

	// Add newline if complete
	if completed >= r.total {
		r.print("\n")
		r.bar = ""
	}
}

// The width of the progress bar, which shrinks down to minBarWidth on a
// narrow terminal before it is left out.
const (
	barWidth    = 40
	minBarWidth = 10
)

// progressLine returns the progress line: the bar, the percentage and count
// done, then stats and eta. A line wider than width columns, which would
// wrap and leave a stale line at each redraw, leaves out its least important
// parts first: the ETA, then the bar, which shrinks to minBarWidth before it
// goes, then the stats. A width of 0 sets no limit.
func progressLine(width int, percentage float64, completed, total int, stats, eta string) string {
	done := fmt.Sprintf("%.1f%% (%d/%d)", percentage, completed, total)
	line := func(bar int, stats, eta string) string {
		if bar == 0 {
			return done + stats + eta
		}
		filled := int(percentage * float64(bar) / 100)
		return "[" + strings.Repeat("█", filled) + strings.Repeat("░", bar-filled) + "] " + done + stats + eta
	}
	fits := func(line string) bool { return width == 0 || utf8.RuneCountInString(line) <= width }

	if full := line(barWidth, stats, eta); fits(full) {
		return full
	}
	for bar := barWidth; bar >= minBarWidth; bar-- {
		if shorter := line(bar, stats, ""); fits(shorter) {
			return shorter
		}
	}
	if withStats := line(0, stats, ""); fits(withStats) {
		return withStats
	}
	return line(0, "", "")
}

// clearLine moves the cursor to the start of the line and erases the whole
// line. The bar is drawn on a terminal only, which understands it.
const clearLine = "\r\033[K"

// clearProgressBar clears the current progress bar line
func (r *Reporter) clearProgressBar() {
	r.print(clearLine)
	r.bar = ""
}

// print writes s to the output while the bar is on. A failed write, as to a
// terminal that went away, turns the bar off, so the send goes on without
// it rather than writing to an output that fails. The caller holds mu.
func (r *Reporter) print(s string) {
	if !r.enabled {
		return
	}
	if _, err := io.WriteString(r.output, s); err != nil {
		r.enabled = false
	}
}

// isTerminal reports whether output is an interactive terminal that acts on
// the escape sequence that clears the line, as terminal.IsEscapeTerminal
// decides for colors too. Tests replace it.
var isTerminal = terminal.IsEscapeTerminal

// terminalWidth returns how many columns the progress line may take on
// output: one less than the terminal has, so the line never reaches the last
// column, where some terminals wrap, or 79 when the width is unknown.
var terminalWidth = func(output io.Writer) int {
	file, ok := output.(*os.File)
	if !ok {
		return 79
	}
	columns, _, err := term.GetSize(int(file.Fd()))
	if err != nil || columns < 2 {
		return 79
	}
	return columns - 1
}

// formatElapsed formats how long a send took, in the result line and the
// metrics alike: seconds with one decimal, such as "2.7s".
func formatElapsed(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// formatDuration formats the ETA of the progress bar
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

	fmt.Fprintf(output, "\nPerformance Metrics:\n")
	fmt.Fprintf(output, "   Total messages: %d\n", stats.Total)
	fmt.Fprintf(output, "   Successfully sent: %d\n", stats.Sent)
	if stats.Failed > 0 {
		fmt.Fprintf(output, "   Failed: %d\n", stats.Failed)
	}
	if stats.Unknown > 0 {
		fmt.Fprintf(output, "   Unknown outcome: %d\n", stats.Unknown)
	}
	fmt.Fprintf(output, "   Success rate: %.2f%%\n", stats.SuccessRate)
	fmt.Fprintf(output, "   Duration: %s\n", formatElapsed(stats.Duration))
	fmt.Fprintf(output, "   Performance: %.1f emails/sec\n", stats.EmailsPerSec)
}
