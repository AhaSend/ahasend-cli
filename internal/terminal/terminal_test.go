package terminal_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/terminal"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// colorCommand returns a command with the root command's --no-color flag,
// parsed from args.
func colorCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("no-color", false, "")
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func TestUseColorOnATerminal(t *testing.T) {
	tty := testutil.NewTerminal(t).Stdin
	tests := []struct {
		name    string
		noColor string
		term    string
		args    []string
		want    bool
	}{
		{"by default", "", "xterm-256color", nil, true},
		{"with NO_COLOR set", "1", "xterm-256color", nil, false},
		{"with --no-color", "", "xterm-256color", []string{"--no-color"}, false},
		{"with TERM=dumb", "", "dumb", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("TERM", tt.term)

			assert.Equal(t, tt.want, terminal.UseColor(colorCommand(t, tt.args...), tty))
		})
	}
}

func TestUseColorNeverForOutputThatIsNotATerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	assert.False(t, terminal.UseColor(colorCommand(t), &bytes.Buffer{}))
}

// TestIsEscapeTerminalNeedsATerminalThatIsNotDumb checks that a terminal
// whose TERM is dumb takes no escape sequence: neither colors nor the
// progress bar's line clearing.
func TestIsEscapeTerminalNeedsATerminalThatIsNotDumb(t *testing.T) {
	tty := testutil.NewTerminal(t).Stdin

	t.Setenv("TERM", "xterm-256color")
	assert.True(t, terminal.IsEscapeTerminal(tty))
	t.Setenv("TERM", "dumb")
	assert.False(t, terminal.IsEscapeTerminal(tty))
}
