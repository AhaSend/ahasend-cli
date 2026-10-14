package progress

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestShowMetricsPrintsAPlainHeading(t *testing.T) {
	var output bytes.Buffer
	ShowMetrics(Stats{Total: 2, Sent: 2, SuccessRate: 100, Duration: time.Second, EmailsPerSec: 2}, &output)
	assert.Contains(t, output.String(), "\nPerformance Metrics:\n   Total messages: 2\n")
}

func TestShowMetricsGivesTheSuccessRateWithTwoDecimals(t *testing.T) {
	var output bytes.Buffer
	ShowMetrics(Stats{Total: 3, Sent: 2, Failed: 1, SuccessRate: 200.0 / 3, Duration: time.Second, EmailsPerSec: 2}, &output)
	assert.Contains(t, output.String(), "\n   Success rate: 66.67%\n")
}

// TestTheProgressBarShowsTheCountOfEveryFinishedJob checks that jobs
// finishing in quick succession each redraw the bar with the latest count.
func TestTheProgressBarShowsTheCountOfEveryFinishedJob(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 250, startTime: time.Now(), output: &output}

	reporter.Record(100, 0, 0)
	reporter.Record(99, 1, 0)
	reporter.Record(50, 0, 0)

	assert.Contains(t, output.String(), "(100/250)")
	assert.Contains(t, output.String(), "(200/250) (199 sent, 1 failed)")
	assert.True(t, strings.HasSuffix(output.String(), "(250/250) (249 sent, 1 failed)\n"), output.String())
}

// TestTheProgressBarClearsTheWholeLine checks that every redraw and the
// final result erase the whole line, however long the bar drawn before it.
func TestTheProgressBarClearsTheWholeLine(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 100000, startTime: time.Now().Add(-time.Hour), output: &output}

	reporter.Record(10000, 10000, 0)
	reporter.Record(1, 0, 0)
	drawn := output.String()
	output.Reset()
	reporter.Finish()

	assert.Equal(t, 2, strings.Count(drawn, "\r\033[K["), drawn)
	assert.Greater(t, len([]rune(strings.Split(drawn, "\r\033[K")[1])), 80, "the bar is longer than 80 columns")
	assert.True(t, strings.HasPrefix(output.String(), "\r\033[KSent 10001/100000 messages, 10000 failed"), output.String())
}

// TestUnknownOutcomesAreCountedApart checks that recipients whose outcome is
// unknown count as done but neither as sent nor as failed, in the progress
// bar, its final line, the stats and the metrics.
func TestUnknownOutcomesAreCountedApart(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 4, startTime: time.Now(), output: &output}

	reporter.Record(2, 0, 0)
	reporter.Record(0, 0, 1)
	assert.True(t, strings.HasSuffix(output.String(), "(3/4) (2 sent, 1 unknown) ETA: 0s"), output.String())
	reporter.Record(0, 1, 0)
	output.Reset()
	stats := reporter.Finish()

	assert.True(t, strings.HasPrefix(output.String(), "\r\033[KSent 2/4 messages, 1 failed, 1 unknown ("), output.String())
	assert.Equal(t, 2, stats.Sent)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 1, stats.Unknown)
	assert.Equal(t, 50.0, stats.SuccessRate)

	var metrics bytes.Buffer
	ShowMetrics(stats, &metrics)
	assert.Contains(t, metrics.String(), "\n   Failed: 1\n   Unknown outcome: 1\n   Success rate: 50.00%\n")
}

// TestShowMetricsLeavesOutAZeroUnknownCount checks that metrics name the
// unknown outcomes only when there are some.
func TestShowMetricsLeavesOutAZeroUnknownCount(t *testing.T) {
	var output bytes.Buffer
	ShowMetrics(Stats{Total: 2, Sent: 2, SuccessRate: 100, Duration: time.Second}, &output)
	assert.NotContains(t, output.String(), "Unknown")
}

// TestTheSuccessRateIsOverTheRecipientsWithAResult checks that recipients
// never sent, such as after an interruption, do not lower the success rate.
func TestTheSuccessRateIsOverTheRecipientsWithAResult(t *testing.T) {
	reporter := &Reporter{total: 10, startTime: time.Now(), output: &bytes.Buffer{}}

	reporter.Record(1, 1, 0)

	assert.Equal(t, 50.0, reporter.Finish().SuccessRate)
}

// TestTheProgressBarIsOffInDebugMode checks that --progress draws the bar on
// a terminal, but not with --debug, whose logs would break it, nor on
// anything else.
func TestTheProgressBarIsOffInDebugMode(t *testing.T) {
	tests := []struct {
		terminal, progress, debug, enabled bool
	}{
		{terminal: true, progress: true, debug: false, enabled: true},
		{terminal: true, progress: true, debug: true, enabled: false},
		{terminal: true, progress: false, debug: false, enabled: false},
		{terminal: false, progress: true, debug: false, enabled: false},
	}
	previous := isTerminal
	t.Cleanup(func() { isTerminal = previous })
	for _, test := range tests {
		isTerminal = func(io.Writer) bool { return test.terminal }

		reporter := NewReporter(10, test.progress, test.debug, io.Discard)

		assert.Equal(t, test.enabled, reporter.enabled, "%+v", test)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := map[time.Duration]string{
		45 * time.Second:  "45s",
		90 * time.Second:  "1.5m",
		150 * time.Minute: "2.5h",
	}
	for duration, want := range tests {
		assert.Equal(t, want, formatDuration(duration))
	}
}

// TestTheProgressLineFitsTheTerminal checks that on a narrow terminal the
// line leaves out the ETA first, then shrinks and leaves out the bar, then
// the counts, so it never wraps.
func TestTheProgressLineFitsTheTerminal(t *testing.T) {
	const stats, eta = " (40 sent, 10 failed)", " ETA: 3s"
	tests := []struct {
		width int
		want  string
	}{
		{0, "[" + strings.Repeat("█", 20) + strings.Repeat("░", 20) + "] 50.0% (50/100) (40 sent, 10 failed) ETA: 3s"},
		{90, "[" + strings.Repeat("█", 20) + strings.Repeat("░", 20) + "] 50.0% (50/100) (40 sent, 10 failed) ETA: 3s"},
		{80, "[" + strings.Repeat("█", 20) + strings.Repeat("░", 20) + "] 50.0% (50/100) (40 sent, 10 failed)"},
		{70, "[" + strings.Repeat("█", 16) + strings.Repeat("░", 16) + "] 50.0% (50/100) (40 sent, 10 failed)"},
		{50, "[" + strings.Repeat("█", 6) + strings.Repeat("░", 6) + "] 50.0% (50/100) (40 sent, 10 failed)"},
		{40, "50.0% (50/100) (40 sent, 10 failed)"},
		{20, "50.0% (50/100)"},
	}
	for _, tt := range tests {
		line := progressLine(tt.width, 50, 50, 100, stats, eta)

		assert.Equal(t, tt.want, line, "width %d", tt.width)
		if tt.width > len("50.0% (50/100)") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), tt.width)
		}
	}
}

func TestTheProgressBarFitsTheWidthOfItsTerminal(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 100000, startTime: time.Now().Add(-time.Hour), output: &output, width: 60}

	reporter.Record(10000, 10000, 0)

	line := strings.TrimPrefix(output.String(), "\r\033[K")
	assert.LessOrEqual(t, utf8.RuneCountInString(line), 60, line)
	assert.Contains(t, line, "(10000 sent, 10000 failed)")
}

func TestNewReporterFitsTheLineToTheTerminalWidth(t *testing.T) {
	previous := terminalWidth
	t.Cleanup(func() { terminalWidth = previous })
	previousTerminal := isTerminal
	isTerminal = func(io.Writer) bool { return true }
	t.Cleanup(func() { isTerminal = previousTerminal })
	terminalWidth = func(io.Writer) int { return 59 }

	assert.Equal(t, 59, NewReporter(10, true, false, io.Discard).width)
}

// failingWriter fails every write, and counts them.
type failingWriter struct{ writes int }

func (w *failingWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("write failed")
}

// TestTheProgressBarStopsAtItsFirstFailedWrite checks that the bar turns off
// when its output fails, so the send goes on without writing to it, and the
// stats still count every job.
func TestTheProgressBarStopsAtItsFirstFailedWrite(t *testing.T) {
	output := &failingWriter{}
	reporter := &Reporter{enabled: true, total: 3, startTime: time.Now(), output: output}

	reporter.Record(1, 0, 0)
	reporter.Record(1, 0, 0)
	stats := reporter.Finish()

	assert.Equal(t, 1, output.writes)
	assert.Equal(t, 2, stats.Sent)
}

// TestWritesAboveTheProgressBarClearItsLineFirst checks that what else is
// written to stderr while the bar is drawn, such as a retry notice, starts
// on a cleared line, and that the bar is drawn again below it; once the bar
// ended with its newline, writes go through as they are.
func TestWritesAboveTheProgressBarClearItsLineFirst(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 3, startTime: time.Now(), output: &output}
	stderr := reporter.Writer(&output)

	reporter.Record(1, 0, 0)
	bar := strings.TrimPrefix(output.String(), clearLine)
	output.Reset()
	fmt.Fprint(stderr, "Waiting 6 seconds before sending the request of 1 recipient again\n")

	assert.Equal(t, clearLine+"Waiting 6 seconds before sending the request of 1 recipient again\n"+clearLine+bar, output.String())

	reporter.Record(2, 0, 0)
	output.Reset()
	fmt.Fprint(stderr, "after\n")
	assert.Equal(t, "after\n", output.String())
}

// TestWritesGoStraightThroughWithoutABar checks that without a bar, the
// writer for other stderr output is stderr itself.
func TestWritesGoStraightThroughWithoutABar(t *testing.T) {
	var output bytes.Buffer
	reporter := NewReporter(3, false, false, &output)

	assert.Same(t, &output, reporter.Writer(&output))
}

// TestTheProgressBarNeedsATerminalThatIsNotDumb checks, on a terminal, that
// --progress draws the bar, but not when TERM is dumb, the name of a
// terminal that acts on no escape sequence, as colors are off there too.
func TestTheProgressBarNeedsATerminalThatIsNotDumb(t *testing.T) {
	tty := testutil.NewTerminal(t).Stdin
	for term, enabled := range map[string]bool{"xterm-256color": true, "dumb": false} {
		t.Setenv("TERM", term)

		assert.Equal(t, enabled, NewReporter(10, true, false, tty).enabled, term)
	}
}

// TestTheResultLineAndTheMetricsGiveTheDurationAlike checks that the line
// that ends the progress bar and --show-metrics write the send's duration
// in one format.
func TestTheResultLineAndTheMetricsGiveTheDurationAlike(t *testing.T) {
	var output bytes.Buffer
	reporter := &Reporter{enabled: true, total: 1, startTime: time.Now().Add(-2700 * time.Millisecond), output: &output}

	stats := reporter.Finish()
	ShowMetrics(stats, &output)

	assert.Regexp(t, `messages \(2\.7s\)\n`, output.String())
	assert.Contains(t, output.String(), "   Duration: 2.7s\n")
}
