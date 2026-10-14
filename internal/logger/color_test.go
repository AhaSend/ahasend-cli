package logger_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// initializedWith initializes the logger from a command given args, with the
// logging flags of the root command, writing to stderr, and returns its text
// formatter.
func initializedWith(t *testing.T, stderr io.Writer, args ...string) *logrus.TextFormatter {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("debug", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.SetErr(stderr)
	require.NoError(t, cmd.ParseFlags(args))

	logger.Initialize(cmd)
	formatter, ok := logger.Get().Formatter.(*logrus.TextFormatter)
	require.True(t, ok)
	return formatter
}

// TestLogColorsFollowTheColorDecisionOfStderr checks that the logs are in
// color exactly when terminal.UseColor says so for stderr, whatever logrus
// would decide itself.
func TestLogColorsFollowTheColorDecisionOfStderr(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	tty := testutil.NewTerminal(t).Stdin

	onATerminal := initializedWith(t, tty)
	assert.True(t, onATerminal.ForceColors)
	assert.False(t, onATerminal.DisableColors)

	withNoColor := initializedWith(t, tty, "--no-color")
	assert.False(t, withNoColor.ForceColors)
	assert.True(t, withNoColor.DisableColors)

	redirected := initializedWith(t, &bytes.Buffer{})
	assert.False(t, redirected.ForceColors)
	assert.True(t, redirected.DisableColors)
}
